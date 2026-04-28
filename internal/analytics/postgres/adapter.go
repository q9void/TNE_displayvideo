// Package postgres provides a Postgres analytics adapter for persisting auction data.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/lib/pq"

	"github.com/thenexusengine/tne_springwire/internal/analytics"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// Adapter implements analytics.Module by writing to Postgres.
// All inserts run in a background goroutine so analytics never block the auction path.
type Adapter struct {
	db *sql.DB
}

// NewAdapter creates a new Postgres analytics adapter using the given connection pool.
func NewAdapter(db *sql.DB) *Adapter {
	return &Adapter{db: db}
}

// LogAuctionObject persists auction, bidder, and win events to Postgres.
// Runs asynchronously — errors are logged but never propagated.
func (a *Adapter) LogAuctionObject(ctx context.Context, auction *analytics.AuctionObject) error {
	// Capture values before launching goroutine to avoid data races on the caller's struct.
	go a.persist(auction)
	return nil
}

// LogVideoObject is a no-op — video events are not persisted by this adapter.
func (a *Adapter) LogVideoObject(_ context.Context, _ *analytics.VideoObject) error {
	return nil
}

// LogSignalReceipts persists curated-deal signal receipts to signal_receipts.
// Runs asynchronously; per-row failures are logged but never block the caller.
func (a *Adapter) LogSignalReceipts(_ context.Context, receipts []analytics.SignalReceipt) error {
	if len(receipts) == 0 || a.db == nil {
		return nil
	}
	go a.persistReceipts(receipts)
	return nil
}

// AckSignalReceipts marks rows in signal_receipts as acknowledged when the
// bidder echoes bid.ext.signal_receipt back. Idempotent: repeated acks
// update acknowledged_at to the latest timestamp.
func (a *Adapter) AckSignalReceipts(_ context.Context, acks []analytics.SignalReceiptAck) error {
	if len(acks) == 0 || a.db == nil {
		return nil
	}
	go a.persistAcks(acks)
	return nil
}

func (a *Adapter) persistAcks(acks []analytics.SignalReceiptAck) {
	for _, ack := range acks {
		_, err := a.db.Exec(`
			UPDATE signal_receipts
			SET acknowledged_at = $4
			WHERE auction_id = $1 AND bidder_code = $2 AND deal_id = $3`,
			ack.AuctionID, ack.BidderCode, ack.DealID, ack.AckedAt,
		)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("auction_id", ack.AuctionID).
				Str("bidder_code", ack.BidderCode).
				Str("deal_id", ack.DealID).
				Msg("postgres analytics: failed to ack signal_receipt")
		}
	}
}

func (a *Adapter) persistReceipts(receipts []analytics.SignalReceipt) {
	for _, r := range receipts {
		schainJSON := []byte("null")
		if len(r.SChainNodesSent) > 0 {
			if b, err := json.Marshal(r.SChainNodesSent); err == nil {
				schainJSON = b
			}
		}
		_, err := a.db.Exec(`
			INSERT INTO signal_receipts (
				auction_id, bidder_code, deal_id, curator_id, seat,
				eids_sent, segments_sent, schain_nodes_sent, sent_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			r.AuctionID, r.BidderCode, r.DealID, r.CuratorID, r.Seat,
			pq.Array(r.EIDsSent), pq.Array(r.SegmentsSent),
			schainJSON, r.SentAt,
		)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("auction_id", r.AuctionID).
				Str("deal_id", r.DealID).
				Msg("postgres analytics: failed to insert signal_receipt")
		}
	}
}

// Shutdown is a no-op — the shared *sql.DB pool is managed by the caller.
func (a *Adapter) Shutdown() error {
	return nil
}

// persist writes all three event tables inside a single transaction.
func (a *Adapter) persist(auction *analytics.AuctionObject) {
	tx, err := a.db.Begin()
	if err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to begin transaction")
		return
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = insertAuctionEvent(tx, auction); err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to insert auction_event")
		return
	}

	if err = insertBidderEvents(tx, auction); err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to insert bidder_events")
		return
	}

	if err = insertWinEvents(tx, auction); err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to insert win_events")
		return
	}

	if err = insertRequestEvent(tx, auction); err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to insert request_events")
		return
	}

	if err = insertIdentityEvent(tx, auction); err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to insert identity_events")
		return
	}

	if err = tx.Commit(); err != nil {
		logger.Log.Error().Err(err).Str("auction_id", auction.AuctionID).
			Msg("postgres analytics: failed to commit transaction")
	}
}

func insertAuctionEvent(tx *sql.Tx, a *analytics.AuctionObject) error {
	var deviceCountry, deviceType string
	if a.Device != nil {
		deviceCountry = a.Device.Country
		deviceType = a.Device.Type
	}

	var adUnit string
	if len(a.Impressions) > 0 {
		adUnit = a.Impressions[0].TagID
	}

	_, err := tx.Exec(`
		INSERT INTO auction_events (
			auction_id, request_id, publisher_id, timestamp,
			bidders_selected, bidders_excluded, total_bidders,
			total_bids, winning_bids, duration_ms, status,
			bid_multiplier, total_revenue, total_payout,
			device_country, device_type, impression_count,
			consent_ok, validation_errors, ad_unit,
			deal_count, curator_ids
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		a.AuctionID,
		a.RequestID,
		a.PublisherID,
		a.Timestamp,
		len(a.SelectedBidders),
		len(a.ExcludedBidders),
		a.TotalBidders,
		a.TotalBids,
		len(a.WinningBids),
		a.AuctionDuration.Milliseconds(),
		a.Status,
		a.BidMultiplier,
		a.TotalRevenue,
		a.TotalPayout,
		deviceCountry,
		deviceType,
		len(a.Impressions),
		a.ConsentOK,
		len(a.ValidationErrors),
		adUnit,
		a.DealCount,
		pq.Array(a.CuratorIDs),
	)
	return err
}

func insertBidderEvents(tx *sql.Tx, a *analytics.AuctionObject) error {
	var deviceCountry, deviceType string
	if a.Device != nil {
		deviceCountry = a.Device.Country
		deviceType = a.Device.Type
	}

	mediaType := "banner"
	var adUnit string
	if len(a.Impressions) > 0 {
		if len(a.Impressions[0].MediaTypes) > 0 {
			mediaType = a.Impressions[0].MediaTypes[0]
		}
		adUnit = a.Impressions[0].TagID
	}

	var floorPrice *float64
	if len(a.Impressions) > 0 && a.Impressions[0].Floor > 0 {
		f := a.Impressions[0].Floor
		floorPrice = &f
	}

	for bidder, result := range a.BidderResults {
		hadBid := len(result.Bids) > 0

		var firstBidCPM *float64
		if hadBid {
			cpm := result.Bids[0].OriginalPrice
			firstBidCPM = &cpm
		}

		belowFloor := false
		for _, bid := range result.Bids {
			if bid.BelowFloor {
				belowFloor = true
				break
			}
		}

		// Curated-deal attribution: prefer the per-bidder summary recorded on
		// BidderResult (populated by applyReceiptsToBidderResults from the
		// auction's signal_receipt buffer). Fall back to the first bid's
		// DealID for resilience.
		var dealID, curatorID, seat *string
		if result.DealID != "" {
			v := result.DealID
			dealID = &v
		}
		if result.CuratorID != "" {
			v := result.CuratorID
			curatorID = &v
		}
		if result.Seat != "" {
			v := result.Seat
			seat = &v
		}
		if dealID == nil {
			for _, b := range result.Bids {
				if b.DealID != "" {
					v := b.DealID
					dealID = &v
					if b.CuratorID != "" {
						cv := b.CuratorID
						curatorID = &cv
					}
					if b.Seat != "" {
						sv := b.Seat
						seat = &sv
					}
					break
				}
			}
		}

		_, err := tx.Exec(`
			INSERT INTO bidder_events (
				auction_id, bidder_code,
				latency_ms, had_bid, bid_count,
				first_bid_cpm, floor_price, below_floor,
				timed_out, had_error, no_bid_reason,
				country, device_type, media_type,
				ad_unit, sizes,
				deal_id, curator_id, seat,
				signal_sources
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
			a.AuctionID,
			bidder,
			result.Latency.Milliseconds(),
			hadBid,
			len(result.Bids),
			firstBidCPM,
			floorPrice,
			belowFloor,
			result.TimedOut,
			len(result.Errors) > 0,
			result.NoBidReason,
			deviceCountry,
			deviceType,
			mediaType,
			adUnit,
			strings.Join(firstImpSizes(a), ","),
			dealID,
			curatorID,
			seat,
			pq.Array(result.SignalSources),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func firstImpSizes(a *analytics.AuctionObject) []string {
	if len(a.Impressions) > 0 {
		return a.Impressions[0].Sizes
	}
	return nil
}

func insertIdentityEvent(tx *sql.Tx, a *analytics.AuctionObject) error {
	var (
		totalEIDs   int
		fpid        *string
		id5UID      *string
		rubiconUID  *string
		kargoUID    *string
		pubmaticUID *string
		sovrnUID    *string
		appnexusUID *string
		buyerUID    *string
	)

	if a.User != nil {
		totalEIDs = a.User.TotalEIDs
		if a.User.FPID != "" {
			fpid = &a.User.FPID
		}
		if a.User.ID5UID != "" {
			id5UID = &a.User.ID5UID
		}
		if a.User.RubiconUID != "" {
			rubiconUID = &a.User.RubiconUID
		}
		if a.User.KargoUID != "" {
			kargoUID = &a.User.KargoUID
		}
		if a.User.PubmaticUID != "" {
			pubmaticUID = &a.User.PubmaticUID
		}
		if a.User.SovrnUID != "" {
			sovrnUID = &a.User.SovrnUID
		}
		if a.User.AppNexusUID != "" {
			appnexusUID = &a.User.AppNexusUID
		}
		if a.User.BuyerUID != "" {
			buyerUID = &a.User.BuyerUID
		}
	}

	_, err := tx.Exec(`
		INSERT INTO identity_events (
			auction_id, total_eids,
			fpid, id5_uid, rubicon_uid,
			kargo_uid, pubmatic_uid, sovrn_uid,
			appnexus_uid, buyer_uid
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		a.AuctionID,
		totalEIDs,
		fpid,
		id5UID,
		rubiconUID,
		kargoUID,
		pubmaticUID,
		sovrnUID,
		appnexusUID,
		buyerUID,
	)
	return err
}

func insertRequestEvent(tx *sql.Tx, a *analytics.AuctionObject) error {
	// Derive timed-out bidder list and outcome from bidder results
	timedOutBidders := []string{}
	for bidder, result := range a.BidderResults {
		if result.TimedOut {
			timedOutBidders = append(timedOutBidders, bidder)
		}
	}

	outcome := "no_bids"
	if len(a.WinningBids) > 0 {
		outcome = "bids_returned"
	} else if len(timedOutBidders) > 0 {
		outcome = "timeout"
	} else if a.Status == "error" {
		outcome = "error"
	}

	var timedOutStr *string
	if len(timedOutBidders) > 0 {
		s := strings.Join(timedOutBidders, ",")
		timedOutStr = &s
	}

	var fpid *string
	var eidCount int
	if a.User != nil {
		if a.User.FPID != "" {
			fpid = &a.User.FPID
		}
		eidCount = a.User.TotalEIDs
	}

	var deviceType, deviceCountry string
	if a.Device != nil {
		deviceType = a.Device.Type
		deviceCountry = a.Device.Country
	}

	var firstAdUnit string
	if len(a.Impressions) > 0 {
		firstAdUnit = a.Impressions[0].TagID
	}

	_, err := tx.Exec(`
		INSERT INTO request_events (
			auction_id, publisher_id,
			page_url, page_domain, first_ad_unit,
			slot_count, device_type, device_country,
			fpid, eid_count, consent_ok,
			tmax_ms, auction_ms,
			total_bids, bids_returned,
			timed_out_bidders, outcome,
			timestamp
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		a.AuctionID,
		a.PublisherID,
		a.PageURL,
		a.PublisherDomain,
		firstAdUnit,
		len(a.Impressions),
		deviceType,
		deviceCountry,
		fpid,
		eidCount,
		a.ConsentOK,
		a.TMax,
		a.AuctionDuration.Milliseconds(),
		a.TotalBids,
		len(a.WinningBids),
		timedOutStr,
		outcome,
		a.Timestamp,
	)
	return err
}

func insertWinEvents(tx *sql.Tx, a *analytics.AuctionObject) error {
	for _, win := range a.WinningBids {
		var dealID, curatorID, seat *string
		if win.DealID != "" {
			v := win.DealID
			dealID = &v
		}
		if win.CuratorID != "" {
			v := win.CuratorID
			curatorID = &v
		}
		if win.Seat != "" {
			v := win.Seat
			seat = &v
		}
		_, err := tx.Exec(`
			INSERT INTO win_events (
				auction_id, bid_id, imp_id, bidder_code,
				original_cpm, adjusted_cpm, platform_cut, clear_price,
				demand_type, deal_id, curator_id, seat
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			a.AuctionID,
			win.BidID,
			win.ImpID,
			win.BidderCode,
			win.OriginalPrice,
			win.AdjustedPrice,
			win.PlatformCut,
			win.ClearPrice,
			win.DemandType,
			dealID,
			curatorID,
			seat,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
