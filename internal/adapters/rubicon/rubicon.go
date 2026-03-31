// Package rubicon implements the Rubicon/Magnite bidder adapter
package rubicon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/adapters/routing"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/internal/storage"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// defaultLoader is set by the server after startup via SetLoader.
var defaultLoader *routing.Loader

// SetLoader injects the routing Loader. Call once from cmd/server/server.go after startup.
func SetLoader(l *routing.Loader) { defaultLoader = l }

// filterNonRPRules returns only rules that don't conflict with Rubicon's
// native ext.rp nesting logic.
func filterNonRPRules(rules []storage.BidderFieldRule) []storage.BidderFieldRule {
	out := make([]storage.BidderFieldRule, 0, len(rules))
	for _, r := range rules {
		if !strings.HasPrefix(r.FieldPath, "imp.ext.rubicon.") {
			out = append(out, r)
		}
	}
	return out
}

// extractSlotParams reads imp[0].ext.bidder or imp[0].ext.rubicon into a flat map
// for the Composer's slotParams argument.
func extractSlotParams(imps []openrtb.Imp) map[string]interface{} {
	if len(imps) == 0 || imps[0].Ext == nil {
		return nil
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(imps[0].Ext, &outer); err != nil {
		return nil
	}
	raw, ok := outer["bidder"]
	if !ok {
		raw, ok = outer["rubicon"]
		if !ok {
			return nil
		}
	}
	var params map[string]interface{}
	json.Unmarshal(raw, &params) //nolint:errcheck
	return params
}

const (
	defaultEndpoint = "http://exapi-us-east.rubiconproject.com/a/api/exchange.json?tk_sdc=us-east"
	maxBAdv         = 50
)

// Rubicon-specific extension structures

type rubiconParams struct {
	AccountID        int             `json:"accountId"`
	SiteID           int             `json:"siteId"`
	ZoneID           int             `json:"zoneId"`
	SizeID           int             `json:"sizeId"`
	Inventory        json.RawMessage `json:"inventory,omitempty"`
	Visitor          json.RawMessage `json:"visitor,omitempty"`
	BidOnMultiformat bool            `json:"bidonmultiformat"`
	Video            *rubiconVideo   `json:"video,omitempty"`
}

type rubiconVideo struct {
	Language     string `json:"language,omitempty"`
	PlayerHeight int    `json:"playerHeight,omitempty"`
	PlayerWidth  int    `json:"playerWidth,omitempty"`
	SizeID       int    `json:"size_id,omitempty"`
	Skip         *int   `json:"skip,omitempty"`
	SkipDelay    int    `json:"skipdelay,omitempty"`
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

type rubiconBannerExt struct {
	RP rubiconBannerExtRP `json:"rp"`
}

type rubiconBannerExtRP struct {
	Mime   string `json:"mime"`
	SizeID int    `json:"size_id,omitempty"`
}

type rubiconVideoExt struct {
	RP rubiconVideoExtRP `json:"rp"`
}

type rubiconVideoExtRP struct {
	SizeID    int  `json:"size_id,omitempty"`
	Skip      *int `json:"skip,omitempty"`
	SkipDelay int  `json:"skipdelay,omitempty"`
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

// extractRubiconParams reads Rubicon params from imp.ext.bidder (PBS standard) or imp.ext.rubicon (legacy).
func extractRubiconParams(impExt json.RawMessage) (*rubiconParams, error) {
	if len(impExt) == 0 {
		return nil, fmt.Errorf("imp.ext is empty or nil")
	}

	var outer map[string]json.RawMessage
	if err := json.Unmarshal(impExt, &outer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal imp.ext: %w", err)
	}

	// Prefer imp.ext.bidder (PBS standard injected by bid handler), fall back to imp.ext.rubicon
	raw, ok := outer["bidder"]
	if !ok {
		raw, ok = outer["rubicon"]
	}
	if !ok {
		return nil, fmt.Errorf("no Rubicon parameters found in imp.ext (checked bidder and rubicon)")
	}

	var params rubiconParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rubicon params: %w", err)
	}

	if params.AccountID == 0 {
		return nil, fmt.Errorf("accountId is required")
	}
	if params.SiteID == 0 {
		return nil, fmt.Errorf("siteId is required")
	}
	if params.ZoneID == 0 {
		return nil, fmt.Errorf("zoneId is required")
	}

	return &params, nil
}

// updateRequestTo26 migrates legacy field locations to their OpenRTB 2.6 / Rubicon-expected locations:
//   - source.schain (top-level) → source.ext.schain
//   - regs.gdpr (top-level) → regs.ext.gdpr
//   - regs.us_privacy (top-level) → regs.ext.us_privacy
func updateRequestTo26(reqCopy *openrtb.BidRequest) {
	if reqCopy.Source != nil && reqCopy.Source.SChain != nil {
		sourceCopy := *reqCopy.Source
		var sourceExt map[string]json.RawMessage
		if len(sourceCopy.Ext) > 0 {
			json.Unmarshal(sourceCopy.Ext, &sourceExt) //nolint:errcheck
		}
		if sourceExt == nil {
			sourceExt = make(map[string]json.RawMessage)
		}
		if _, alreadySet := sourceExt["schain"]; !alreadySet {
			if schainBytes, err := json.Marshal(sourceCopy.SChain); err == nil {
				sourceExt["schain"] = schainBytes
			}
		}
		if extBytes, err := json.Marshal(sourceExt); err == nil {
			sourceCopy.Ext = extBytes
		}
		sourceCopy.SChain = nil
		reqCopy.Source = &sourceCopy
	}

	if reqCopy.Regs != nil && (reqCopy.Regs.GDPR != nil || reqCopy.Regs.USPrivacy != "") {
		regsCopy := *reqCopy.Regs
		var regsExt map[string]json.RawMessage
		if len(regsCopy.Ext) > 0 {
			json.Unmarshal(regsCopy.Ext, &regsExt) //nolint:errcheck
		}
		if regsExt == nil {
			regsExt = make(map[string]json.RawMessage)
		}
		if regsCopy.GDPR != nil {
			if _, alreadySet := regsExt["gdpr"]; !alreadySet {
				if v, err := json.Marshal(*regsCopy.GDPR); err == nil {
					regsExt["gdpr"] = v
				}
			}
			regsCopy.GDPR = nil
		}
		if regsCopy.USPrivacy != "" {
			if _, alreadySet := regsExt["us_privacy"]; !alreadySet {
				if v, err := json.Marshal(regsCopy.USPrivacy); err == nil {
					regsExt["us_privacy"] = v
				}
			}
			regsCopy.USPrivacy = ""
		}
		if extBytes, err := json.Marshal(regsExt); err == nil {
			regsCopy.Ext = extBytes
		}
		reqCopy.Regs = &regsCopy
	}
}

// splitMultiFormatImp returns one imp per format when bidonmultiformat is false.
// When bidonmultiformat is true or only one format is present, returns [imp] unchanged.
func splitMultiFormatImp(imp openrtb.Imp, bidonmultiformat bool) []openrtb.Imp {
	formats := 0
	if imp.Banner != nil {
		formats++
	}
	if imp.Video != nil {
		formats++
	}
	if imp.Native != nil {
		formats++
	}

	if bidonmultiformat || formats <= 1 {
		return []openrtb.Imp{imp}
	}

	// One imp per format
	var result []openrtb.Imp
	if imp.Banner != nil {
		copy := imp
		copy.Video = nil
		copy.Native = nil
		result = append(result, copy)
	}
	if imp.Video != nil {
		copy := imp
		copy.Banner = nil
		copy.Native = nil
		result = append(result, copy)
	}
	if imp.Native != nil {
		copy := imp
		copy.Banner = nil
		copy.Video = nil
		result = append(result, copy)
	}
	return result
}

// getVideoSizeID returns the Rubicon video size_id based on ad start delay.
// pre-roll: -1 or 0 → 201, mid-roll: >0 → 202, post-roll: 2 → 203.
func getVideoSizeID(video *openrtb.Video) int {
	if video.StartDelay == nil {
		return 201 // default: pre-roll
	}
	switch *video.StartDelay {
	case -1:
		return 201 // generic pre-roll
	case 0:
		return 201 // pre-roll
	case -2:
		return 202 // generic mid-roll
	case 2:
		return 203 // post-roll
	default:
		if *video.StartDelay > 0 {
			return 202 // specific mid-roll
		}
		return 201
	}
}

// buildImpRPTarget merges FPD inventory data and pbs_login target into a single JSON object.
func buildImpRPTarget(inventory json.RawMessage, xapiUser string, existingTarget json.RawMessage) json.RawMessage {
	merged := map[string]interface{}{
		"pbs_login":   xapiUser,
		"pbs_version": "pbs-go/tne-1.0",
		"pbs_url":     "https://ads.thenexusengine.com",
	}

	// Merge existing target without overriding PBS keys
	if len(existingTarget) > 0 {
		var existing map[string]interface{}
		if json.Unmarshal(existingTarget, &existing) == nil {
			for k, v := range existing {
				if _, set := merged[k]; !set {
					merged[k] = v
				}
			}
		}
	}

	// Merge inventory FPD without overriding existing keys
	if len(inventory) > 0 {
		var inv map[string]interface{}
		if json.Unmarshal(inventory, &inv) == nil {
			for k, v := range inv {
				if _, set := merged[k]; !set {
					merged[k] = v
				}
			}
		}
	}

	result, _ := json.Marshal(merged)
	return result
}

// MakeRequests builds HTTP requests for Rubicon — one per impression.
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	var errors []error
	requests := make([]*adapters.RequestData, 0, len(request.Imp))

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("impressions", len(request.Imp)).
		Str("request_id", request.ID).
		Msg("Rubicon MakeRequests called")

	for _, imp := range request.Imp {
		params, err := extractRubiconParams(imp.Ext)
		if err != nil {
			errors = append(errors, fmt.Errorf("imp %s: %w", imp.ID, err))
			continue
		}

		// Split multi-format imps per PBS logic
		impsToProcess := splitMultiFormatImp(imp, params.BidOnMultiformat)

		for _, impToProcess := range impsToProcess {
			reqData, impErrors := a.buildRequest(request, impToProcess, params)
			errors = append(errors, impErrors...)
			if reqData != nil {
				requests = append(requests, reqData)
			}
		}
	}

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("requests_created", len(requests)).
		Int("errors", len(errors)).
		Msg("Rubicon MakeRequests completed")

	return requests, errors
}

// buildRequest creates one HTTP request for a single imp.
func (a *Adapter) buildRequest(request *openrtb.BidRequest, imp openrtb.Imp, params *rubiconParams) (*adapters.RequestData, []error) {
	var errors []error

	reqCopy := *request

	// Apply standard-field routing rules (non-rp fields only)
	if defaultLoader != nil {
		rules := defaultLoader.Get(context.Background(), "rubicon")
		safeRules := filterNonRPRules(rules)
		composer := routing.NewComposer(safeRules)
		composed, _ := composer.Apply("rubicon", &reqCopy, extractSlotParams(reqCopy.Imp), nil, nil)
		reqCopy = *composed
	}

	// Migrate legacy field locations (schain, gdpr, us_privacy)
	updateRequestTo26(&reqCopy)

	// BAdv: Rubicon enforces a max of 50 blocked advertisers
	if len(reqCopy.BAdv) > maxBAdv {
		reqCopy.BAdv = reqCopy.BAdv[:maxBAdv]
	}

	// Rubicon doesn't use top-level Cur or Ext
	reqCopy.Cur = nil
	reqCopy.Ext = nil

	impCopy := imp

	// Build imp.ext.rp — extract existing target/track for merging
	var existingTarget json.RawMessage
	var existingTrack *rubiconTrack

	if len(impCopy.Ext) > 0 {
		var existingImpExt map[string]interface{}
		if json.Unmarshal(impCopy.Ext, &existingImpExt) == nil {
			if rpData, ok := existingImpExt["rp"].(map[string]interface{}); ok {
				if target, ok := rpData["target"]; ok {
					if targetBytes, err := json.Marshal(target); err == nil {
						existingTarget = targetBytes
					}
				}
				if trackData, ok := rpData["track"].(map[string]interface{}); ok {
					mint, _ := trackData["mint"].(string)
					mintVer, _ := trackData["mint_version"].(string)
					if mint != "" || mintVer != "" {
						existingTrack = &rubiconTrack{Mint: mint, MintVersion: mintVer}
					}
				}
			}
		}
	}

	rpExt := rubiconImpExtRP{
		ZoneID: params.ZoneID,
		Target: buildImpRPTarget(params.Inventory, a.xapiUser, existingTarget),
	}
	if existingTrack != nil {
		rpExt.Track = *existingTrack
	} else {
		rpExt.Track = rubiconTrack{Mint: "", MintVersion: ""}
	}

	impExtMap := map[string]interface{}{"rp": rpExt}
	if params.BidOnMultiformat {
		impExtMap["bidonmultiformat"] = true
	}

	var err error
	impCopy.Ext, err = json.Marshal(impExtMap)
	if err != nil {
		return nil, []error{fmt.Errorf("imp %s: failed to marshal imp.ext: %w", imp.ID, err)}
	}

	// Banner ext: mime + size_id
	if impCopy.Banner != nil {
		bannerCopy := *impCopy.Banner
		bannerRP := rubiconBannerExtRP{Mime: "text/html", SizeID: params.SizeID}
		bannerCopy.Ext, _ = json.Marshal(rubiconBannerExt{RP: bannerRP})
		impCopy.Banner = &bannerCopy
	}

	// Video ext: size_id (derived from startdelay), skip, skipdelay
	if impCopy.Video != nil {
		videoCopy := *impCopy.Video
		videoRP := rubiconVideoExtRP{}

		// Prefer explicit video size_id from params, otherwise derive from startdelay
		if params.Video != nil && params.Video.SizeID > 0 {
			videoRP.SizeID = params.Video.SizeID
		} else {
			videoRP.SizeID = getVideoSizeID(&videoCopy)
		}

		if videoCopy.Skip != nil {
			videoRP.Skip = videoCopy.Skip
		}
		if videoCopy.SkipAfter > 0 {
			videoRP.SkipDelay = videoCopy.SkipAfter
		}

		videoCopy.Ext, _ = json.Marshal(rubiconVideoExt{RP: videoRP})
		impCopy.Video = &videoCopy
	}

	reqCopy.Imp = []openrtb.Imp{impCopy}

	// Site
	if reqCopy.Site != nil {
		siteCopy := *reqCopy.Site
		siteExt := rubiconSiteExt{RP: rubiconSiteExtRP{SiteID: params.SiteID}}
		siteCopy.Ext, err = json.Marshal(siteExt)
		if err != nil {
			return nil, []error{fmt.Errorf("imp %s: failed to marshal site.ext: %w", imp.ID, err)}
		}
		if siteCopy.Publisher == nil {
			siteCopy.Publisher = &openrtb.Publisher{}
		}
		siteCopy.Publisher.ID = fmt.Sprintf("%d", params.AccountID)
		pubExt := rubiconPubExt{RP: rubiconPubExtRP{AccountID: params.AccountID}}
		siteCopy.Publisher.Ext, _ = json.Marshal(pubExt)
		reqCopy.Site = &siteCopy
	}

	// App
	if reqCopy.App != nil {
		appCopy := *reqCopy.App
		appExt := rubiconAppExt{RP: rubiconAppExtRP{SiteID: params.SiteID}}
		appCopy.Ext, err = json.Marshal(appExt)
		if err != nil {
			return nil, []error{fmt.Errorf("imp %s: failed to marshal app.ext: %w", imp.ID, err)}
		}
		if appCopy.Publisher == nil {
			appCopy.Publisher = &openrtb.Publisher{}
		}
		appCopy.Publisher.ID = fmt.Sprintf("%d", params.AccountID)
		pubExt := rubiconPubExt{RP: rubiconPubExtRP{AccountID: params.AccountID}}
		appCopy.Publisher.Ext, _ = json.Marshal(pubExt)
		reqCopy.App = &appCopy
	}

	// Device ext: pxratio
	if reqCopy.Device != nil && reqCopy.Device.PxRatio > 0 {
		deviceCopy := *reqCopy.Device
		var deviceExt map[string]json.RawMessage
		if len(deviceCopy.Ext) > 0 {
			json.Unmarshal(deviceCopy.Ext, &deviceExt) //nolint:errcheck
		}
		if deviceExt == nil {
			deviceExt = make(map[string]json.RawMessage)
		}
		if _, hasRP := deviceExt["rp"]; !hasRP {
			rpBytes, _ := json.Marshal(map[string]float64{"pixelratio": deviceCopy.PxRatio})
			deviceExt["rp"] = rpBytes
		}
		if extBytes, merr := json.Marshal(deviceExt); merr == nil {
			deviceCopy.Ext = extBytes
		}
		reqCopy.Device = &deviceCopy
	}

	// User: buyeruid from EIDs, visitor FPD, user.ext.rp
	if reqCopy.User != nil {
		userCopy := *reqCopy.User

		// Set buyeruid from Rubicon-synced EID
		for _, eid := range userCopy.EIDs {
			if eid.Source == "rubiconproject.com" && len(eid.UIDs) > 0 {
				userCopy.BuyerUID = eid.UIDs[0].ID
				break
			}
		}

		var userExt map[string]json.RawMessage
		if len(userCopy.Ext) > 0 {
			json.Unmarshal(userCopy.Ext, &userExt) //nolint:errcheck
		}
		if userExt == nil {
			userExt = make(map[string]json.RawMessage)
		}

		// visitor FPD → user.ext.rp.target
		if len(params.Visitor) > 0 {
			var rpData map[string]json.RawMessage
			if existing, ok := userExt["rp"]; ok {
				json.Unmarshal(existing, &rpData) //nolint:errcheck
			}
			if rpData == nil {
				rpData = make(map[string]json.RawMessage)
			}
			rpData["target"] = params.Visitor
			rpBytes, _ := json.Marshal(rpData)
			userExt["rp"] = rpBytes
		} else if _, hasRP := userExt["rp"]; !hasRP {
			userExt["rp"] = json.RawMessage(`{}`)
		}

		if extBytes, merr := json.Marshal(userExt); merr == nil {
			userCopy.Ext = extBytes
		}
		reqCopy.User = &userCopy
	}

	requestBody, merr := json.Marshal(reqCopy)
	if merr != nil {
		return nil, []error{fmt.Errorf("imp %s: failed to marshal request: %w", imp.ID, merr)}
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json;charset=utf-8")
	headers.Set("Accept", "application/json")

	if a.xapiUser != "" && a.xapiPass != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(a.xapiUser + ":" + a.xapiPass))
		headers.Set("Authorization", "Basic "+auth)
	}

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Str("imp_id", imp.ID).
		Int("account_id", params.AccountID).
		Int("site_id", params.SiteID).
		Int("zone_id", params.ZoneID).
		Int("body_size", len(requestBody)).
		Msg("Rubicon request built")

	return &adapters.RequestData{
		Method:  "POST",
		URI:     a.endpoint,
		Body:    requestBody,
		Headers: headers,
	}, errors
}

// MakeBids parses Rubicon responses into bids.
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

	logger.Log.Debug().
		Str("adapter", "rubicon").
		Int("status_code", responseData.StatusCode).
		Int("body_size", len(responseData.Body)).
		Msg("Rubicon response received")

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseData.Body, &bidResp); err != nil {
		return nil, []error{fmt.Errorf("failed to parse response: %w", err)}
	}

	response := &adapters.BidderResponse{
		Currency:   bidResp.Cur,
		ResponseID: bidResp.ID,
		Bids:       make([]*adapters.TypedBid, 0),
	}

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
				Str("bid_type", string(bidType)).
				Msg("Rubicon bid")

			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     bid,
				BidType: bidType,
			})
		}
	}

	return response, nil
}

// Info returns bidder information.
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
		DemandType:  adapters.DemandTypePlatform,
	}
}

func init() {
	if err := adapters.RegisterAdapter("rubicon", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "rubicon").Msg("failed to register adapter")
	}
}
