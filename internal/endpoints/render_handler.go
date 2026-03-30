package endpoints

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// RenderHandler persists SDK render and viewability events from the browser.
type RenderHandler struct {
	db *sql.DB
}

// NewRenderHandler creates a RenderHandler. db may be nil (events are dropped silently).
func NewRenderHandler(db *sql.DB) *RenderHandler {
	return &RenderHandler{db: db}
}

type renderEventPayload struct {
	AuctionID string  `json:"auction_id"`
	DivID     string  `json:"div_id"`
	Bidder    string  `json:"bidder"`
	CPM       float64 `json:"cpm"`
	Event     string  `json:"event"` // "rendered" or "viewable"
}

// HandleRenderEvent handles POST /v1/render.
func (h *RenderHandler) HandleRenderEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var evt renderEventPayload
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if h.db != nil {
		go h.persist(&evt)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *RenderHandler) persist(evt *renderEventPayload) {
	_, err := h.db.Exec(`
		INSERT INTO render_events (
			auction_id, div_id, bidder, cpm, event, timestamp
		) VALUES ($1,$2,$3,$4,$5,$6)`,
		evt.AuctionID,
		evt.DivID,
		evt.Bidder,
		evt.CPM,
		evt.Event,
		time.Now().UTC(),
	)
	if err != nil {
		logger.Log.Error().Err(err).
			Str("auction_id", evt.AuctionID).
			Str("event", evt.Event).
			Msg("render_handler: failed to insert render_event")
	}
}
