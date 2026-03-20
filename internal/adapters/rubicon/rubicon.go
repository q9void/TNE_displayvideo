// Package rubicon implements the Rubicon/Magnite bidder adapter
package rubicon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const (
	// Authenticated regional endpoint (US-East) — requires RUBICON_XAPI_USER/PASS
	defaultEndpoint = "http://exapi-us-east.rubiconproject.com/a/api/exchange.json?tk_sdc=us-east"
)

// Rubicon-specific extension structures
type rubiconParams struct {
	AccountID        int  `json:"accountId"`
	SiteID           int  `json:"siteId"`
	ZoneID           int  `json:"zoneId"`
	BidOnMultiformat bool `json:"bidonmultiformat"`
}

type rubiconImpExt struct {
	RP rubiconImpExtRP `json:"rp"`
}

type rubiconImpExtRP struct {
	ZoneID int             `json:"zone_id"`
	Track  rubiconTrack    `json:"track"`
	Target json.RawMessage `json:"target,omitempty"`
}

type rubiconTrack struct {
	Mint        string `json:"mint"`
	MintVersion string `json:"mint_version"`
}

type rubiconSiteExt struct {
	RP rubiconSiteExtRP `json:"rp"`
}

type rubiconSiteExtRP struct {
	SiteID int `json:"site_id"`
}

type rubiconAppExt struct {
	RP rubiconAppExtRP `json:"rp"`
}

type rubiconAppExtRP struct {
	SiteID int `json:"site_id"`
}

type rubiconPubExt struct {
	RP rubiconPubExtRP `json:"rp"`
}

type rubiconPubExtRP struct {
	AccountID int `json:"account_id"`
}

// Adapter implements the Rubicon bidder
type Adapter struct {
	endpoint string
	xapiUser string
	xapiPass string
}

// New creates a new Rubicon adapter
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = os.Getenv("RUBICON_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}

	// Load XAPI credentials from environment
	// These are required for Rubicon authentication
	xapiUser := os.Getenv("RUBICON_XAPI_USER")
	xapiPass := os.Getenv("RUBICON_XAPI_PASS")

	if xapiUser == "" || xapiPass == "" {
		logger.Log.Warn().
			Bool("has_user", xapiUser != "").
			Bool("has_pass", xapiPass != "").
			Msg("Rubicon XAPI credentials not configured - requests may be rejected")
	}

	return &Adapter{
		endpoint: endpoint,
		xapiUser: xapiUser,
		xapiPass: xapiPass,
	}
}

// MakeRequests builds HTTP requests for Rubicon
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	var errors []error
	requests := make([]*adapters.RequestData, 0, len(request.Imp))

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("impressions", len(request.Imp)).
		Str("request_id", request.ID).
		Msg("Rubicon MakeRequests called")

	// Rubicon requires one request per impression
	for _, imp := range request.Imp {
		// Extract Rubicon parameters from imp.Ext
		rubiconParams, err := extractRubiconParams(imp.Ext)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to extract Rubicon params for imp %s: %w", imp.ID, err))
			continue
		}

		// Create a copy of the request for this impression
		reqCopy := *request
		impCopy := imp

		// Transform impression extension to Rubicon's expected format
		// Task #22: Preserve existing imp.ext.rp.target if it exists
		var existingImpExt map[string]interface{}
		var existingTarget json.RawMessage

		if len(impCopy.Ext) > 0 {
			if err := json.Unmarshal(impCopy.Ext, &existingImpExt); err == nil {
				if rpData, ok := existingImpExt["rp"].(map[string]interface{}); ok {
					if target, ok := rpData["target"]; ok {
						// Re-marshal the target to preserve it
						if targetBytes, err := json.Marshal(target); err == nil {
							existingTarget = targetBytes
						}
					}
				}
			}
		}

		// Task #26: Check for existing tracking data and preserve if valid
		var existingTrack *rubiconTrack
		if existingImpExt != nil {
			if rpData, ok := existingImpExt["rp"].(map[string]interface{}); ok {
				if trackData, ok := rpData["track"].(map[string]interface{}); ok {
					mint, hasMint := trackData["mint"].(string)
					mintVersion, hasVersion := trackData["mint_version"].(string)
					// Only preserve if both fields exist and at least one is non-empty
					if hasMint && hasVersion && (mint != "" || mintVersion != "") {
						existingTrack = &rubiconTrack{
							Mint:        mint,
							MintVersion: mintVersion,
						}
					}
				}
			}
		}

		// Build the new rp extension
		rpExt := rubiconImpExtRP{
			ZoneID: rubiconParams.ZoneID,
		}

		// Set tracking data - use existing if valid, otherwise use empty defaults
		if existingTrack != nil {
			rpExt.Track = *existingTrack
		} else {
			rpExt.Track = rubiconTrack{Mint: "", MintVersion: ""}
		}

		// Build PBS identity target — Magnite requires these to identify and route demand to this PBS instance
		pbsTarget := map[string]interface{}{
			"pbs_login":   a.xapiUser,
			"pbs_version": "pbs-go/tne-1.0",
			"pbs_url":     "https://ads.thenexusengine.com",
		}
		// Merge any existing target fields without overriding PBS keys
		if len(existingTarget) > 0 {
			var existingTargetMap map[string]interface{}
			if merr := json.Unmarshal(existingTarget, &existingTargetMap); merr == nil {
				for k, v := range existingTargetMap {
					if _, alreadySet := pbsTarget[k]; !alreadySet {
						pbsTarget[k] = v
					}
				}
			}
		}
		if targetBytes, merr := json.Marshal(pbsTarget); merr == nil {
			rpExt.Target = targetBytes
		}

		impExtMap := map[string]interface{}{
			"rp": rpExt,
		}

		// Preserve bidonmultiformat parameter if enabled
		if rubiconParams.BidOnMultiformat {
			impExtMap["bidonmultiformat"] = true
		}

		impCopy.Ext, err = json.Marshal(impExtMap)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to marshal imp ext for imp %s: %w", imp.ID, err))
			continue
		}

		// Rubicon requires mime type in banner.ext.rp for all banner impressions
		if impCopy.Banner != nil {
			impCopy.Banner.Ext, _ = json.Marshal(map[string]interface{}{
				"rp": map[string]string{"mime": "text/html"},
			})
		}

		reqCopy.Imp = []openrtb.Imp{impCopy}

		// Transform Site with Rubicon extensions
		if reqCopy.Site != nil {
			siteCopy := *reqCopy.Site

			// NOTE: ID clearing is now handled by Privacy/Consent hook (no longer needed here)

			// Set Rubicon site extension
			siteExt := rubiconSiteExt{
				RP: rubiconSiteExtRP{
					SiteID: rubiconParams.SiteID,
				},
			}
			siteCopy.Ext, err = json.Marshal(siteExt)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to marshal site ext: %w", err))
				continue
			}

			// Task #23: Create Site.Publisher when nil
			// Task #25: Ensure publisher.id is always set for account context
			if siteCopy.Publisher == nil {
				siteCopy.Publisher = &openrtb.Publisher{}
			}

			// CRITICAL: Set publisher.id to Rubicon's account ID
			// Rubicon checks this field BEFORE ext.rp.account_id
			accountIDStr := fmt.Sprintf("%d", rubiconParams.AccountID)

			logger.Log.Debug().
				Str("adapter", "rubicon").
				Str("before_id", siteCopy.Publisher.ID).
				Str("setting_to", accountIDStr).
				Msg("About to set publisher.id")

			siteCopy.Publisher.ID = accountIDStr

			logger.Log.Debug().
				Str("adapter", "rubicon").
				Str("after_id", siteCopy.Publisher.ID).
				Msg("After setting publisher.id")

			pubExt := rubiconPubExt{
				RP: rubiconPubExtRP{
					AccountID: rubiconParams.AccountID,
				},
			}
			siteCopy.Publisher.Ext, err = json.Marshal(pubExt)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to marshal publisher ext: %w", err))
				continue
			}

			reqCopy.Site = &siteCopy
		}

		// Task #21: Add App request handling (parallel to Site logic)
		// Task #24: Transform App traffic properly
		if reqCopy.App != nil {
			appCopy := *reqCopy.App

			// Set Rubicon app extension
			appExt := rubiconAppExt{
				RP: rubiconAppExtRP{
					SiteID: rubiconParams.SiteID,
				},
			}
			appCopy.Ext, err = json.Marshal(appExt)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to marshal app ext: %w", err))
				continue
			}

			// Task #25: Create App.Publisher when nil and ensure publisher.id is always set
			if appCopy.Publisher == nil {
				appCopy.Publisher = &openrtb.Publisher{}
			}

			// Set publisher.id to Rubicon's account ID (same as Site)
			accountIDStr := fmt.Sprintf("%d", rubiconParams.AccountID)

			logger.Log.Debug().
				Str("adapter", "rubicon").
				Str("before_id", appCopy.Publisher.ID).
				Str("setting_to", accountIDStr).
				Msg("About to set app publisher.id")

			appCopy.Publisher.ID = accountIDStr

			logger.Log.Debug().
				Str("adapter", "rubicon").
				Str("after_id", appCopy.Publisher.ID).
				Msg("After setting app publisher.id")

			pubExt := rubiconPubExt{
				RP: rubiconPubExtRP{
					AccountID: rubiconParams.AccountID,
				},
			}
			appCopy.Publisher.Ext, err = json.Marshal(pubExt)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to marshal app publisher ext: %w", err))
				continue
			}

			reqCopy.App = &appCopy
		}

		// Rubicon expects schain in source.ext.schain (legacy location) rather than
		// the OpenRTB 2.5+ top-level source.schain field.
		if reqCopy.Source != nil && reqCopy.Source.SChain != nil {
			sourceCopy := *reqCopy.Source
			var sourceExt map[string]json.RawMessage
			if len(sourceCopy.Ext) > 0 {
				json.Unmarshal(sourceCopy.Ext, &sourceExt) //nolint:errcheck
			}
			if sourceExt == nil {
				sourceExt = make(map[string]json.RawMessage)
			}
			if schainBytes, merr := json.Marshal(sourceCopy.SChain); merr == nil {
				sourceExt["schain"] = schainBytes
			}
			if extBytes, merr := json.Marshal(sourceExt); merr == nil {
				sourceCopy.Ext = extBytes
			}
			sourceCopy.SChain = nil
			reqCopy.Source = &sourceCopy
		}

		// Set user fields required by Magnite: buyeruid from synced UID and user.ext.rp = {}
		if reqCopy.User != nil {
			userCopy := *reqCopy.User

			// Set buyeruid from Rubicon-synced EID in user.eids (standard top-level location)
			for _, eid := range userCopy.EIDs {
				if eid.Source == "rubiconproject.com" && len(eid.UIDs) > 0 {
					userCopy.BuyerUID = eid.UIDs[0].ID
					break
				}
			}

			// Magnite reference adapter always injects user.ext.rp = {}
			var userExt map[string]json.RawMessage
			if len(userCopy.Ext) > 0 {
				json.Unmarshal(userCopy.Ext, &userExt) //nolint:errcheck
			}
			if userExt == nil {
				userExt = make(map[string]json.RawMessage)
			}
			if _, hasRP := userExt["rp"]; !hasRP {
				userExt["rp"] = json.RawMessage(`{}`)
			}
			if extBytes, merr := json.Marshal(userExt); merr == nil {
				userCopy.Ext = extBytes
			}

			reqCopy.User = &userCopy
		}

		requestBody, err := json.Marshal(reqCopy)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to marshal request for imp %s: %w", imp.ID, err))
			continue
		}

		headers := http.Header{}
		headers.Set("Content-Type", "application/json;charset=utf-8")
		headers.Set("Accept", "application/json")

		// Add XAPI basic authentication
		if a.xapiUser != "" && a.xapiPass != "" {
			auth := base64.StdEncoding.EncodeToString([]byte(a.xapiUser + ":" + a.xapiPass))
			headers.Set("Authorization", "Basic "+auth)
		}

		requests = append(requests, &adapters.RequestData{
			Method:  "POST",
			URI:     a.endpoint,
			Body:    requestBody,
			Headers: headers,
		})

		// Log the request body for verification
		requestPreview := string(requestBody)
		if len(requestPreview) > 1500 {
			requestPreview = requestPreview[:1500] + "..."
		}

		logger.Log.Debug().
			Str("adapter", "rubicon").
			Str("imp_id", imp.ID).
			Str("endpoint", a.endpoint).
			Int("account_id", rubiconParams.AccountID).
			Int("site_id", rubiconParams.SiteID).
			Int("zone_id", rubiconParams.ZoneID).
			Int("body_size", len(requestBody)).
			Str("request_body", requestPreview).
			Msg("Rubicon request created")
	}

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("requests_created", len(requests)).
		Int("errors", len(errors)).
		Msg("Rubicon MakeRequests completed")

	return requests, errors
}

// extractRubiconParams extracts Rubicon-specific parameters from impression extension
func extractRubiconParams(impExt json.RawMessage) (*rubiconParams, error) {
	// Handle nil or empty imp.ext
	if len(impExt) == 0 {
		return nil, fmt.Errorf("imp.ext is empty or nil")
	}

	var extMap map[string]interface{}
	if err := json.Unmarshal(impExt, &extMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal imp.ext: %w", err)
	}

	// Look for Rubicon params in ext.rubicon
	var rubiconData map[string]interface{}

	if rubicon, ok := extMap["rubicon"].(map[string]interface{}); ok {
		rubiconData = rubicon
	}

	if rubiconData == nil {
		return nil, fmt.Errorf("no Rubicon parameters found in imp.ext")
	}

	params := &rubiconParams{}

	// Extract accountId (can be int or float64 from JSON)
	if accountID, ok := rubiconData["accountId"]; ok {
		switch v := accountID.(type) {
		case float64:
			params.AccountID = int(v)
		case int:
			params.AccountID = v
		default:
			return nil, fmt.Errorf("accountId must be a number")
		}
	} else {
		return nil, fmt.Errorf("accountId is required")
	}

	// Extract siteId
	if siteID, ok := rubiconData["siteId"]; ok {
		switch v := siteID.(type) {
		case float64:
			params.SiteID = int(v)
		case int:
			params.SiteID = v
		default:
			return nil, fmt.Errorf("siteId must be a number")
		}
	} else {
		return nil, fmt.Errorf("siteId is required")
	}

	// Extract zoneId
	if zoneID, ok := rubiconData["zoneId"]; ok {
		switch v := zoneID.(type) {
		case float64:
			params.ZoneID = int(v)
		case int:
			params.ZoneID = v
		default:
			return nil, fmt.Errorf("zoneId must be a number")
		}
	} else {
		return nil, fmt.Errorf("zoneId is required")
	}

	// Extract bidonmultiformat (optional)
	if bidonmultiformat, ok := rubiconData["bidonmultiformat"]; ok {
		if v, ok := bidonmultiformat.(bool); ok {
			params.BidOnMultiformat = v
		}
	}

	return params, nil
}

// MakeBids parses Rubicon responses into bids
func (a *Adapter) MakeBids(request *openrtb.BidRequest, responseData *adapters.ResponseData) (*adapters.BidderResponse, []error) {
	if responseData.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	if responseData.StatusCode == http.StatusBadRequest {
		return nil, []error{fmt.Errorf("bad request: %s", string(responseData.Body))}
	}

	if responseData.StatusCode != http.StatusOK {
		return nil, []error{fmt.Errorf("unexpected status: %d", responseData.StatusCode)}
	}

	// Log the raw response for debugging
	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("status_code", responseData.StatusCode).
		Int("body_size", len(responseData.Body)).
		Str("raw_response", string(responseData.Body)).
		Msg("Rubicon raw HTTP response")

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
		return nil, []error{fmt.Errorf("failed to parse response: %w", err)}
	}

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Str("response_id", bidResp.ID).
		Str("currency", bidResp.Cur).
		Int("seatbids", len(bidResp.SeatBid)).
		Msg("Rubicon parsed response")

	response := &adapters.BidderResponse{
		Currency:   bidResp.Cur,
		ResponseID: bidResp.ID, // P1-1: Include ResponseID for validation
		Bids:       make([]*adapters.TypedBid, 0),
	}

	// P2-3: Build impression map once for O(1) lookups instead of O(n) per bid
	impMap := adapters.BuildImpMap(request.Imp)

	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			bid := &seatBid.Bid[i]
			bidType := adapters.GetBidTypeFromMap(bid, impMap)

			logger.Log.Debug().
				Str("adapter", "rubicon").
				Str("bid_id", bid.ID).
				Str("imp_id", bid.ImpID).
				Float64("price", bid.Price).
				Str("currency", bidResp.Cur).
				Str("creative_id", bid.CRID).
				Str("deal_id", bid.DealID).
				Int("width", bid.W).
				Int("height", bid.H).
				Str("bid_type", string(bidType)).
				Msg("Rubicon bid details")

			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     bid,
				BidType: bidType,
			})
		}
	}

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("total_bids", len(response.Bids)).
		Msg("Rubicon MakeBids completed")

	return response, nil
}

// Info returns bidder information
func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled: true,
		Maintainer: &adapters.MaintainerInfo{
			Email: "header-bidding@rubiconproject.com",
		},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
				},
			},
			App: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
				},
			},
		},
		GVLVendorID: 52,
		Endpoint:    defaultEndpoint,
		DemandType:  adapters.DemandTypePlatform, // Platform demand (obfuscated as "thenexusengine")
	}
}

func init() {
	if err := adapters.RegisterAdapter("rubicon", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "rubicon").Msg("failed to register adapter")
	}
}
