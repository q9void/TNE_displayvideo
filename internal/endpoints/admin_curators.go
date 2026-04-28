// Package endpoints — admin CRUD for the curator catalog.
//
// Routes (all under /admin/curators):
//
//	GET    /admin/curators                       List all curators
//	POST   /admin/curators                       Create or upsert a curator
//	GET    /admin/curators/{id}                  Get one curator (with deals/seats/publishers)
//	PUT    /admin/curators/{id}                  Update a curator
//	DELETE /admin/curators/{id}                  Soft-delete (status='archived')
//
//	GET    /admin/curators/{id}/deals            List deals owned by curator
//	POST   /admin/curators/{id}/deals            Upsert a deal under curator
//	DELETE /admin/curators/{id}/deals/{deal_id}  Remove a deal
//
//	GET    /admin/curators/{id}/seats            List seat bindings
//	POST   /admin/curators/{id}/seats            Add a seat binding
//	DELETE /admin/curators/{id}/seats/{bidder}/{seat_id}  Remove a seat
//
//	GET    /admin/curators/{id}/publishers       List allow-listed publisher IDs
//	POST   /admin/curators/{id}/publishers       Add publisher to allow-list (body: {"publisher_id": <int>})
//	DELETE /admin/curators/{id}/publishers/{publisher_id} Remove from allow-list
//
//	GET    /admin/curators/{id}/signal-receipts  Signal receipt audit
//	    Query params: deal_id, since (RFC3339), until (RFC3339), limit (default 100, max 1000)
package endpoints

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// CuratorAdminHandler exposes the curator catalog over HTTP.
type CuratorAdminHandler struct {
	store *storage.CuratorStore
	// db is used directly for the signal-receipts audit query, which spans
	// analytics tables not owned by the CuratorStore.
	db *sql.DB
}

// NewCuratorAdminHandler builds a handler. Either argument may be nil for
// tests that exercise routing without DB calls — the handlers fail with 503
// when their backing store isn't wired.
func NewCuratorAdminHandler(store *storage.CuratorStore, db *sql.DB) *CuratorAdminHandler {
	return &CuratorAdminHandler{store: store, db: db}
}

// ServeHTTP routes /admin/curators/* to the appropriate handler.
func (h *CuratorAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		jsonError(w, http.StatusServiceUnavailable, "no_store",
			"curator store not configured")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/admin/curators")
	path = strings.Trim(path, "/")
	parts := []string{}
	if path != "" {
		parts = strings.Split(path, "/")
	}

	switch len(parts) {
	case 0:
		h.serveCollection(w, r)
	case 1:
		h.serveCurator(w, r, parts[0])
	case 2:
		h.serveSubcollection(w, r, parts[0], parts[1])
	case 3:
		h.serveSubItem(w, r, parts[0], parts[1], parts[2])
	case 4:
		// /seats/{bidder}/{seat_id} variant (4 parts: id, "seats", bidder, seat_id)
		h.serveSubItem4(w, r, parts[0], parts[1], parts[2], parts[3])
	default:
		jsonError(w, http.StatusNotFound, "not_found", "unknown route")
	}
}

// ----------------------------------------------------------------------------
// /admin/curators
// ----------------------------------------------------------------------------

func (h *CuratorAdminHandler) serveCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		curators, err := h.store.ListCurators(r.Context())
		if err != nil {
			h.dbError(w, err, "list curators")
			return
		}
		jsonOK(w, map[string]interface{}{
			"curators": curators,
			"count":    len(curators),
		})
	case http.MethodPost:
		var req storage.Curator
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if req.ID == "" || req.Name == "" {
			jsonError(w, http.StatusBadRequest, "missing_field", "id and name are required")
			return
		}
		if err := h.store.UpsertCurator(r.Context(), &req); err != nil {
			h.dbError(w, err, "upsert curator")
			return
		}
		jsonOK(w, &req)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
	}
}

// ----------------------------------------------------------------------------
// /admin/curators/{id}
// ----------------------------------------------------------------------------

func (h *CuratorAdminHandler) serveCurator(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		c, err := h.store.LoadCurator(r.Context(), id)
		if err != nil {
			h.dbError(w, err, "load curator")
			return
		}
		if c == nil {
			jsonError(w, http.StatusNotFound, "not_found", "curator "+id)
			return
		}
		jsonOK(w, c)
	case http.MethodPut:
		var req storage.Curator
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		req.ID = id // Path parameter is authoritative.
		if req.Name == "" {
			jsonError(w, http.StatusBadRequest, "missing_field", "name is required")
			return
		}
		if err := h.store.UpsertCurator(r.Context(), &req); err != nil {
			h.dbError(w, err, "update curator")
			return
		}
		jsonOK(w, &req)
	case http.MethodDelete:
		if err := h.store.DeleteCurator(r.Context(), id); err != nil {
			h.dbError(w, err, "delete curator")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
	}
}

// ----------------------------------------------------------------------------
// /admin/curators/{id}/{deals,seats,publishers,signal-receipts}
// ----------------------------------------------------------------------------

func (h *CuratorAdminHandler) serveSubcollection(w http.ResponseWriter, r *http.Request, id, sub string) {
	switch sub {
	case "deals":
		h.serveDealsCollection(w, r, id)
	case "seats":
		h.serveSeatsCollection(w, r, id)
	case "publishers":
		h.servePublishersCollection(w, r, id)
	case "signal-receipts":
		h.serveSignalReceipts(w, r, id)
	default:
		jsonError(w, http.StatusNotFound, "not_found", "unknown sub-resource: "+sub)
	}
}

func (h *CuratorAdminHandler) serveDealsCollection(w http.ResponseWriter, r *http.Request, curatorID string) {
	switch r.Method {
	case http.MethodGet:
		deals, err := h.store.ListDeals(r.Context(), curatorID)
		if err != nil {
			h.dbError(w, err, "list deals")
			return
		}
		jsonOK(w, map[string]interface{}{"deals": deals, "count": len(deals)})
	case http.MethodPost:
		var d storage.CuratorDeal
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if d.DealID == "" {
			jsonError(w, http.StatusBadRequest, "missing_field", "deal_id is required")
			return
		}
		d.CuratorID = curatorID
		if err := h.store.UpsertDeal(r.Context(), &d); err != nil {
			h.dbError(w, err, "upsert deal")
			return
		}
		jsonOK(w, &d)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
	}
}

func (h *CuratorAdminHandler) serveSeatsCollection(w http.ResponseWriter, r *http.Request, curatorID string) {
	switch r.Method {
	case http.MethodGet:
		seats, err := h.store.ListSeats(r.Context(), curatorID)
		if err != nil {
			h.dbError(w, err, "list seats")
			return
		}
		jsonOK(w, map[string]interface{}{"seats": seats, "count": len(seats)})
	case http.MethodPost:
		var s storage.CuratorSeat
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		s.CuratorID = curatorID
		if s.BidderCode == "" || s.SeatID == "" {
			jsonError(w, http.StatusBadRequest, "missing_field", "bidder_code and seat_id required")
			return
		}
		if err := h.store.UpsertSeat(r.Context(), &s); err != nil {
			h.dbError(w, err, "upsert seat")
			return
		}
		jsonOK(w, &s)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
	}
}

func (h *CuratorAdminHandler) servePublishersCollection(w http.ResponseWriter, r *http.Request, curatorID string) {
	switch r.Method {
	case http.MethodGet:
		ids, err := h.store.ListAllowedPublishers(r.Context(), curatorID)
		if err != nil {
			h.dbError(w, err, "list allowed publishers")
			return
		}
		jsonOK(w, map[string]interface{}{"publisher_ids": ids, "count": len(ids)})
	case http.MethodPost:
		var body struct {
			PublisherID int `json:"publisher_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
		if body.PublisherID <= 0 {
			jsonError(w, http.StatusBadRequest, "missing_field", "publisher_id required")
			return
		}
		if err := h.store.AllowPublisher(r.Context(), curatorID, body.PublisherID); err != nil {
			h.dbError(w, err, "allow publisher")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
	}
}

// ----------------------------------------------------------------------------
// Sub-item routes
//   /admin/curators/{id}/deals/{deal_id}
//   /admin/curators/{id}/publishers/{publisher_id}
// ----------------------------------------------------------------------------

func (h *CuratorAdminHandler) serveSubItem(w http.ResponseWriter, r *http.Request, curatorID, sub, item string) {
	switch sub {
	case "deals":
		if r.Method != http.MethodDelete {
			jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
			return
		}
		if err := h.store.DeleteDeal(r.Context(), item); err != nil {
			h.dbError(w, err, "delete deal")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "publishers":
		if r.Method != http.MethodDelete {
			jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
			return
		}
		pubID, err := strconv.Atoi(item)
		if err != nil || pubID <= 0 {
			jsonError(w, http.StatusBadRequest, "invalid_publisher_id", item)
			return
		}
		if err := h.store.DenyPublisher(r.Context(), curatorID, pubID); err != nil {
			h.dbError(w, err, "deny publisher")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		jsonError(w, http.StatusNotFound, "not_found", sub)
	}
}

// serveSubItem4 handles /admin/curators/{id}/seats/{bidder_code}/{seat_id}
// (DELETE only — seat IDs are composite keys so we need both path segments).
func (h *CuratorAdminHandler) serveSubItem4(w http.ResponseWriter, r *http.Request, curatorID, sub, p3, p4 string) {
	if sub != "seats" || r.Method != http.MethodDelete {
		jsonError(w, http.StatusNotFound, "not_found", "")
		return
	}
	if err := h.store.DeleteSeat(r.Context(), curatorID, p3, p4); err != nil {
		h.dbError(w, err, "delete seat")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------------
// Signal-receipts audit (chunk 1.4 — implemented inline here so all curator
// admin routes live in one file)
// ----------------------------------------------------------------------------

// SignalReceiptAggregateRow is the per-bidder roll-up returned by
// /admin/curators/{id}/signal-receipts.
type SignalReceiptAggregateRow struct {
	DealID         string         `json:"deal_id"`
	BidderCode     string         `json:"bidder_code"`
	Seat           string         `json:"seat,omitempty"`
	ReceiptCount   int            `json:"receipt_count"`
	LastSeen       time.Time      `json:"last_seen"`
	EIDSourceCount map[string]int `json:"eid_source_count"`
	SegmentCount   map[string]int `json:"segment_count"`
}

func (h *CuratorAdminHandler) serveSignalReceipts(w http.ResponseWriter, r *http.Request, curatorID string) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
		return
	}
	if h.db == nil {
		jsonError(w, http.StatusServiceUnavailable, "no_db",
			"signal_receipts queries require a database handle")
		return
	}
	q := r.URL.Query()
	dealFilter := q.Get("deal_id")
	limit := 100
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 {
		limit = l
		if limit > 1000 {
			limit = 1000
		}
	}
	since, until := parseTimeRange(q.Get("since"), q.Get("until"))

	rows, err := queryReceipts(r.Context(), h.db, curatorID, dealFilter, since, until, limit)
	if err != nil {
		h.dbError(w, err, "query signal_receipts")
		return
	}
	jsonOK(w, map[string]interface{}{
		"curator_id": curatorID,
		"rows":       rows,
		"count":      len(rows),
	})
}

func queryReceipts(
	ctx context.Context,
	db *sql.DB,
	curatorID, dealFilter string,
	since, until time.Time,
	limit int,
) ([]SignalReceiptAggregateRow, error) {
	q := `
		SELECT deal_id, bidder_code, COALESCE(seat,''),
		       COUNT(*) AS receipt_count, MAX(sent_at) AS last_seen,
		       COALESCE(array_agg(DISTINCT eid) FILTER (WHERE eid IS NOT NULL), '{}') AS eids,
		       COALESCE(array_agg(DISTINCT seg) FILTER (WHERE seg IS NOT NULL), '{}') AS segs
		FROM signal_receipts
		LEFT JOIN LATERAL unnest(eids_sent) AS eid ON TRUE
		LEFT JOIN LATERAL unnest(segments_sent) AS seg ON TRUE
		WHERE curator_id = $1
		  AND ($2 = '' OR deal_id = $2)
		  AND ($3::timestamp IS NULL OR sent_at >= $3)
		  AND ($4::timestamp IS NULL OR sent_at <= $4)
		GROUP BY deal_id, bidder_code, seat
		ORDER BY last_seen DESC
		LIMIT $5
	`
	var sinceArg, untilArg interface{}
	if !since.IsZero() {
		sinceArg = since
	}
	if !until.IsZero() {
		untilArg = until
	}
	rows, err := db.QueryContext(ctx, q, curatorID, dealFilter, sinceArg, untilArg, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SignalReceiptAggregateRow, 0, 32)
	for rows.Next() {
		var r SignalReceiptAggregateRow
		var eids, segs []string
		if err := rows.Scan(
			&r.DealID, &r.BidderCode, &r.Seat,
			&r.ReceiptCount, &r.LastSeen,
			pq.Array(&eids), pq.Array(&segs),
		); err != nil {
			return nil, err
		}
		r.EIDSourceCount = countOccurrences(eids)
		r.SegmentCount = countOccurrences(segs)
		out = append(out, r)
	}
	return out, rows.Err()
}

func countOccurrences(xs []string) map[string]int {
	out := make(map[string]int, len(xs))
	for _, x := range xs {
		out[x]++
	}
	return out
}

func parseTimeRange(sinceStr, untilStr string) (time.Time, time.Time) {
	var since, until time.Time
	if sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}
	if untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = t
		}
	}
	return since, until
}

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func jsonOK(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

func jsonError(w http.ResponseWriter, code int, errID, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errID,
		"message": msg,
	})
}

func (h *CuratorAdminHandler) dbError(w http.ResponseWriter, err error, op string) {
	logger.Log.Error().Err(err).Str("op", op).Msg("curator admin: db error")
	jsonError(w, http.StatusInternalServerError, "db_error",
		fmt.Sprintf("%s failed", op))
}
