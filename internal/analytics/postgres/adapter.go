// Package postgres provides a Postgres analytics adapter for persisting auction data.
package postgres

import (
	"context"
	"database/sql"
	"strings"

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

// LogVideoObject persists a single video tracking event (impression, quartile,
// complete, error, click, etc.) to the video_events table.
// Runs asynchronously so the player's tracking pixel never blocks on DB latency.
func (a *Adapter) LogVideoObject(_ context.Context, video *analytics.VideoObject) error {
	if video == nil || video.Event == "" {
		return nil
	}
	go a.persistVideoEvent(video)
	return nil
}

func (a *Adapter) persistVideoEvent(v *analytics.VideoObject) {
	_, err := a.db.Exec(`
		INSERT INTO video_events (
			bid_id, account_id, bidder, event, progress,
			error_code, error_message, click_url, session_id, content_id,
			ip_address, user_agent, timestamp
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		v.BidID, v.AccountID, v.Bidder, v.Event, v.Progress,
		v.ErrorCode, v.ErrorMessage, v.ClickURL, v.SessionID, v.ContentID,
		v.IPAddress, v.UserAgent, v.Timestamp,
	)
	if err != nil {
		logger.Log.Error().Err(err).
			Str("bid_id", v.BidID).
			Str("event", v.Event).
			Msg("postgres analytics: failed to insert video_event")
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
			consent_ok, validation_errors, ad_unit
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
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

		_, err := tx.Exec(`
			INSERT INTO bidder_events (
				auction_id, bidder_code,
				latency_ms, had_bid, bid_count,
				first_bid_cpm, floor_price, below_floor,
				timed_out, had_error, no_bid_reason,
				country, device_type, media_type,
				ad_unit, sizes
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
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
		_, err := tx.Exec(`
			INSERT INTO win_events (
				auction_id, bid_id, imp_id, bidder_code,
				original_cpm, adjusted_cpm, platform_cut, clear_price,
				demand_type
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			a.AuctionID,
			win.BidID,
			win.ImpID,
			win.BidderCode,
			win.OriginalPrice,
			win.AdjustedPrice,
			win.PlatformCut,
			win.ClearPrice,
			win.DemandType,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
