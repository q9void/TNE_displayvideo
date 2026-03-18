// Package pubmatic implements the PubMatic bidder adapter
package pubmatic

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const (
	defaultEndpoint = "https://hbopenbid.pubmatic.com/translator?source=prebid-server"
	bidderPubMatic  = "pubmatic"
)

// Adapter implements the PubMatic bidder
type Adapter struct {
	endpoint string
}

// New creates a new PubMatic adapter
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

// MakeRequests builds HTTP requests for PubMatic
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	errs := make([]error, 0, len(request.Imp))

	// Make a copy of the request to avoid modifying the original
	reqCopy := *request
	request = &reqCopy

	pubID := ""
	extractWrapperExtFromImp := true
	extractPubIDFromImp := true

	// Extract display manager info from app if present
	displayManager, displayManagerVer := "", ""
	if request.App != nil && request.App.Ext != nil {
		displayManager, displayManagerVer = getDisplayManagerAndVer(request.App)
	}

	// Extract PubMatic-specific request extensions and preserve original ext
	var origReqExt map[string]json.RawMessage
	if len(request.Ext) > 0 {
		if err := json.Unmarshal(request.Ext, &origReqExt); err != nil {
			return nil, []error{fmt.Errorf("failed to parse request.ext: %w", err)}
		}
	} else {
		origReqExt = make(map[string]json.RawMessage)
	}

	newReqExt, err := extractPubmaticExtFromRequest(request)
	if err != nil {
		return nil, []error{err}
	}
	wrapperExt := newReqExt.Wrapper
	if wrapperExt != nil && wrapperExt.ProfileID != 0 && wrapperExt.VersionID != 0 {
		extractWrapperExtFromImp = false
	}

	// Process each impression
	validImps := make([]openrtb.Imp, 0, len(request.Imp))
	for _, imp := range request.Imp {
		wrapperExtFromImp, pubIDFromImp, err := parseImpressionObject(&imp, extractWrapperExtFromImp, extractPubIDFromImp, displayManager, displayManagerVer)

		// If parsing failed, skip this impression and add the error
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Extract wrapper extension from each impression if needed (merge/validate consistency)
		if extractWrapperExtFromImp && wrapperExtFromImp != nil {
			if wrapperExt == nil {
				wrapperExt = &PubmaticWrapperExt{}
			}

			// Merge ProfileID and VersionID from impressions
			if wrapperExt.ProfileID == 0 && wrapperExtFromImp.ProfileID != 0 {
				wrapperExt.ProfileID = wrapperExtFromImp.ProfileID
			} else if wrapperExt.ProfileID != 0 && wrapperExtFromImp.ProfileID != 0 && wrapperExt.ProfileID != wrapperExtFromImp.ProfileID {
				// Warn if different impressions have conflicting ProfileIDs, use first one
				logger.Log.Warn().
					Str("imp_id", imp.ID).
					Int("existing_profile", wrapperExt.ProfileID).
					Int("imp_profile", wrapperExtFromImp.ProfileID).
					Msg("conflicting wrapper ProfileID across impressions, using first")
			}

			if wrapperExt.VersionID == 0 && wrapperExtFromImp.VersionID != 0 {
				wrapperExt.VersionID = wrapperExtFromImp.VersionID
			} else if wrapperExt.VersionID != 0 && wrapperExtFromImp.VersionID != 0 && wrapperExt.VersionID != wrapperExtFromImp.VersionID {
				// Warn if different impressions have conflicting VersionIDs, use first one
				logger.Log.Warn().
					Str("imp_id", imp.ID).
					Int("existing_version", wrapperExt.VersionID).
					Int("imp_version", wrapperExtFromImp.VersionID).
					Msg("conflicting wrapper VersionID across impressions, using first")
			}

			// Once we have both values, we can stop extracting but continue validating
			if wrapperExt.ProfileID != 0 && wrapperExt.VersionID != 0 {
				extractWrapperExtFromImp = false
			}
		}

		// Extract publisher ID from first impression if needed
		if extractPubIDFromImp && pubIDFromImp != "" {
			pubID = pubIDFromImp
			extractPubIDFromImp = false
		}

		validImps = append(validImps, imp)
	}

	// If all impressions are invalid, return errors
	if len(validImps) == 0 {
		return nil, errs
	}
	request.Imp = validImps

	// Validate publisherId is non-empty
	if pubID == "" {
		return nil, append(errs, fmt.Errorf("publisherId is required"))
	}

	// Only include wrapper if it has actual profile/version data.
	// An empty wrapper with just biddercode confuses the translator.
	if wrapperExt != nil && (wrapperExt.ProfileID != 0 || wrapperExt.VersionID != 0) {
		newReqExt.Wrapper = wrapperExt
	} else {
		newReqExt.Wrapper = nil
	}

	// Marshal PubMatic extensions
	pmExtBytes, err := json.Marshal(newReqExt)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to marshal pubmatic extension: %w", err)}
	}

	// Merge into original request.ext to preserve other fields (currency, floors, etc.)
	var pmExtMap map[string]json.RawMessage
	if err := json.Unmarshal(pmExtBytes, &pmExtMap); err != nil {
		return nil, []error{fmt.Errorf("failed to unmarshal pubmatic extension: %w", err)}
	}

	// Copy PubMatic fields into original ext
	for k, v := range pmExtMap {
		origReqExt[k] = v
	}

	// PubMatic does not want ext.prebid
	delete(origReqExt, "prebid")

	// Marshal final request.ext; omit entirely if nothing remains after stripping prebid
	if len(origReqExt) == 0 {
		request.Ext = nil
	} else {
		rawExt, err := json.Marshal(origReqExt)
		if err != nil {
			return nil, []error{fmt.Errorf("failed to marshal request extension: %w", err)}
		}
		request.Ext = rawExt
	}

	// Set publisher ID on Site or App
	if request.Site != nil {
		siteCopy := *request.Site
		// Clear internal site.id - PubMatic doesn't need our internal account ID
		siteCopy.ID = ""
		// Strip non-IAB content categories (PubMatic rejects numeric/internal codes)
		if len(siteCopy.Cat) > 0 {
			iabCats := siteCopy.Cat[:0]
			for _, c := range siteCopy.Cat {
				if strings.HasPrefix(c, "IAB") {
					iabCats = append(iabCats, c)
				}
			}
			siteCopy.Cat = iabCats
		}
		if siteCopy.Publisher != nil {
			publisherCopy := *siteCopy.Publisher
			publisherCopy.ID = pubID
			siteCopy.Publisher = &publisherCopy
		} else {
			siteCopy.Publisher = &openrtb.Publisher{ID: pubID}
		}
		request.Site = &siteCopy
	} else if request.App != nil {
		appCopy := *request.App
		// Clear internal app.id - PubMatic doesn't need our internal account ID
		appCopy.ID = ""
		if appCopy.Publisher != nil {
			publisherCopy := *appCopy.Publisher
			publisherCopy.ID = pubID
			appCopy.Publisher = &publisherCopy
		} else {
			appCopy.Publisher = &openrtb.Publisher{ID: pubID}
		}
		request.App = &appCopy
	}

	// Set user.id = PubMatic cookie-synced UID (preferred) or TNE FPID as fallback.
	// PubMatic does not want buyeruid — clear it.
	// Move user.consent → user.ext.consent (Prebid PBS convention; translator reads from ext).
	if request.User != nil {
		userCopy := *request.User
		if uid := adapters.ExtractUIDFromEids(request.User, "pubmatic.com"); uid != "" {
			userCopy.ID = uid
		} else if userCopy.ID == "" {
			if fpid := adapters.ExtractUIDFromEids(request.User, "thenexusengine.com"); fpid != "" {
				userCopy.ID = fpid
			}
		}
		userCopy.BuyerUID = ""

		// Move TCF consent string from user.consent (OpenRTB top-level) to
		// user.ext.consent (Prebid Server convention that PubMatic translator expects).
		if userCopy.Consent != "" {
			var userExt map[string]json.RawMessage
			if len(userCopy.Ext) > 0 {
				json.Unmarshal(userCopy.Ext, &userExt)
			}
			if userExt == nil {
				userExt = make(map[string]json.RawMessage)
			}
			if b, err := json.Marshal(userCopy.Consent); err == nil {
				userExt["consent"] = b
			}
			if b, err := json.Marshal(userExt); err == nil {
				userCopy.Ext = b
			}
			userCopy.Consent = ""
		}

		request.User = &userCopy
	}

	// Move regs.us_privacy to regs.ext.us_privacy (PubMatic wants the older location)
	if request.Regs != nil && request.Regs.USPrivacy != "" {
		regsCopy := *request.Regs
		usPrivacy := regsCopy.USPrivacy
		regsCopy.USPrivacy = ""
		var regsExt map[string]json.RawMessage
		if len(regsCopy.Ext) > 0 {
			json.Unmarshal(regsCopy.Ext, &regsExt)
		}
		if regsExt == nil {
			regsExt = make(map[string]json.RawMessage)
		}
		if b, err := json.Marshal(usPrivacy); err == nil {
			regsExt["us_privacy"] = b
		}
		if b, err := json.Marshal(regsExt); err == nil {
			regsCopy.Ext = b
		}
		request.Regs = &regsCopy
	}

	// Marshal final request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
	}

	headers := http.Header{}
	headers.Add("Content-Type", "application/json;charset=utf-8")
	headers.Add("Accept", "application/json")

	return []*adapters.RequestData{
		{
			Method:  "POST",
			URI:     a.endpoint,
			Body:    requestJSON,
			Headers: headers,
		},
	}, errs
}

// MakeBids parses PubMatic responses into bids
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

	// Decompress gzip response if needed
	responseBody := responseData.Body
	if contentEncoding := responseData.Headers.Get("Content-Encoding"); contentEncoding == "gzip" {
		gzipReader, err := gzip.NewReader(bytes.NewReader(responseData.Body))
		if err != nil {
			return nil, []error{fmt.Errorf("failed to create gzip reader: %w", err)}
		}
		defer gzipReader.Close()

		decompressed, err := io.ReadAll(gzipReader)
		if err != nil {
			return nil, []error{fmt.Errorf("failed to decompress gzip response: %w", err)}
		}
		responseBody = decompressed
	}

	var bidResp openrtb.BidResponse
	if err := json.Unmarshal(responseBody, &bidResp); err != nil {
		return nil, []error{fmt.Errorf("failed to parse response: %w", err)}
	}

	response := &adapters.BidderResponse{
		Currency:   bidResp.Cur,
		ResponseID: bidResp.ID,
		Bids:       make([]*adapters.TypedBid, 0),
	}

	// Build impression map for O(1) lookups
	impMap := adapters.BuildImpMap(request.Imp)

	// Extract acat from request for category overriding
	var acat []string
	if len(request.Ext) > 0 {
		var reqExtMap map[string]json.RawMessage
		if err := json.Unmarshal(request.Ext, &reqExtMap); err == nil {
			if acatBytes, ok := reqExtMap["acat"]; ok {
				json.Unmarshal(acatBytes, &acat)
			}
		}
	}

	var errs []error
	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			bid := &seatBid.Bid[i]

			// Override bid categories with acat if present, preserving order
			if len(acat) > 0 {
				bid.Cat = make([]string, len(acat))
				copy(bid.Cat, acat)
			} else if len(bid.Cat) > 1 {
				// Limit categories to first one if multiple and no acat override
				bid.Cat = bid.Cat[0:1]
			}

			// Determine media type
			bidType := adapters.GetBidTypeFromMap(bid, impMap)

			typedBid := &adapters.TypedBid{
				Bid:      bid,
				BidType:  bidType,
				BidVideo: &adapters.BidVideo{},
				BidMeta:  &openrtb.ExtBidPrebidMeta{MediaType: string(bidType)},
			}

			// Parse bid extension for PubMatic-specific data
			if len(bid.Ext) > 0 {
				var bidExt PubmaticBidExt
				if err := json.Unmarshal(bid.Ext, &bidExt); err != nil {
					errs = append(errs, fmt.Errorf("failed to parse bid extension for bid %s: %w", bid.ID, err))
				} else {
					// Set marketplace seat if present
					if bidExt.Marketplace != "" {
						// Note: In your codebase, TypedBid doesn't have a Seat field
						// You may need to add this or handle marketplace differently
					}

					// Set deal priority
					if bidExt.PrebidDealPriority > 0 {
						typedBid.DealPriority = bidExt.PrebidDealPriority
					}

					// Set video duration
					if bidExt.VideoCreativeInfo != nil && bidExt.VideoCreativeInfo.Duration != nil {
						typedBid.BidVideo.Duration = *bidExt.VideoCreativeInfo.Duration
					}

					// Override media type for in-banner video
					if bidExt.InBannerVideo {
						typedBid.BidType = adapters.BidTypeVideo
						typedBid.BidMeta.MediaType = string(adapters.BidTypeVideo)
					}
				}
			}

			// Convert native ad format if needed
			if bidType == adapters.BidTypeNative {
				adm, err := getNativeAdm(bid.AdM)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to process native ad for bid %s: %w", bid.ID, err))
				} else {
					bid.AdM = adm
				}
			}

			response.Bids = append(response.Bids, typedBid)
		}
	}

	return response, errs
}

// parseImpressionObject processes an impression to extract PubMatic parameters
func parseImpressionObject(imp *openrtb.Imp, extractWrapperExtFromImp, extractPubIDFromImp bool, displayManager, displayManagerVer string) (*PubmaticWrapperExt, string, error) {
	var wrapExt *PubmaticWrapperExt
	var pubID string

	// Validate media types - only remove Audio if there are other valid types
	hasValidMediaType := imp.Banner != nil || imp.Video != nil || imp.Native != nil
	if !hasValidMediaType && imp.Audio != nil {
		return wrapExt, pubID, fmt.Errorf("invalid MediaType. PubMatic only supports Banner, Video and Native (not Audio). Ignoring ImpID=%s", imp.ID)
	}

	// Remove audio if other valid media types are present
	if hasValidMediaType && imp.Audio != nil {
		imp.Audio = nil
	}

	// Set display manager if not already set
	if imp.DisplayManager == "" && imp.DisplayManagerVer == "" && displayManager != "" && displayManagerVer != "" {
		imp.DisplayManager = displayManager
		imp.DisplayManagerVer = displayManagerVer
	}

	// Parse impression extension - extract pubmatic params from imp.ext.pubmatic
	var extMap map[string]json.RawMessage
	if err := json.Unmarshal(imp.Ext, &extMap); err != nil {
		return wrapExt, pubID, fmt.Errorf("failed to parse imp.ext for ImpID=%s: %w", imp.ID, err)
	}

	// Extract PubMatic-specific params from ext.pubmatic
	pubmaticData, ok := extMap["pubmatic"]
	if !ok || len(pubmaticData) == 0 {
		return wrapExt, pubID, fmt.Errorf("no PubMatic parameters found in imp.ext for ImpID=%s", imp.ID)
	}

	var pubmaticExt ExtImpPubmatic
	if err := json.Unmarshal(pubmaticData, &pubmaticExt); err != nil {
		return wrapExt, pubID, fmt.Errorf("failed to parse imp.ext.pubmatic for ImpID=%s: %w", imp.ID, err)
	}

	// Extract additional data from other ext keys (data, gpid, ae, skadn)
	var bidderExt ExtImpBidderPubmatic
	if dataRaw, ok := extMap["data"]; ok {
		bidderExt.Data = dataRaw
	}
	if gpidRaw, ok := extMap["gpid"]; ok {
		var gpid string
		json.Unmarshal(gpidRaw, &gpid)
		bidderExt.GPID = gpid
	}
	if aeRaw, ok := extMap["ae"]; ok {
		var ae int
		json.Unmarshal(aeRaw, &ae)
		bidderExt.AE = ae
	}
	if skadnRaw, ok := extMap["skadn"]; ok {
		bidderExt.SKAdNetwork = skadnRaw
	}

	// Extract publisher ID
	if extractPubIDFromImp {
		pubID = strings.TrimSpace(pubmaticExt.PublisherId)
	}

	// Parse wrapper extension
	if extractWrapperExtFromImp && len(pubmaticExt.WrapExt) != 0 {
		err := json.Unmarshal(pubmaticExt.WrapExt, &wrapExt)
		if err != nil {
			return wrapExt, pubID, fmt.Errorf("failed to parse wrapper extension for ImpID=%s: %w", imp.ID, err)
		}
	}

	// Validate and parse ad slot
	if err := validateAdSlot(strings.TrimSpace(pubmaticExt.AdSlot), imp); err != nil {
		return wrapExt, pubID, err
	}

	// Assign banner size if needed
	if imp.Banner != nil {
		if imp.Banner.W == 0 || imp.Banner.H == 0 {
			if len(imp.Banner.Format) > 0 {
				imp.Banner.W = imp.Banner.Format[0].W
				imp.Banner.H = imp.Banner.Format[0].H
			}
		}
	}

	// Apply kadfloor if present
	if pubmaticExt.Kadfloor != "" {
		bidfloor, err := strconv.ParseFloat(strings.TrimSpace(pubmaticExt.Kadfloor), 64)
		if err == nil {
			// Use maximum of original bidfloor and kadfloor
			imp.BidFloor = math.Max(bidfloor, imp.BidFloor)
		}
	}

	// Build impression extension map for PubMatic request - preserve unknown fields
	finalExtMap := make(map[string]interface{})

	// First, preserve unknown fields from original imp.ext (exclude pubmatic, data, gpid, ae, skadn)
	for key, val := range extMap {
		if key != "pubmatic" && key != "data" && key != "gpid" && key != "ae" && key != "skadn" {
			var rawVal interface{}
			if err := json.Unmarshal(val, &rawVal); err == nil {
				finalExtMap[key] = rawVal
			}
		}
	}

	// Add keywords
	if pubmaticExt.Keywords != nil && len(pubmaticExt.Keywords) != 0 {
		addKeywordsToExt(pubmaticExt.Keywords, finalExtMap)
	}

	// Add dctr and pmZoneId
	if pubmaticExt.Dctr != "" {
		finalExtMap[DctrKeyName] = pubmaticExt.Dctr
	}
	if pubmaticExt.PmZoneID != "" {
		finalExtMap[PmZoneIDKeyName] = pubmaticExt.PmZoneID
	}

	// Add first-party data
	if len(bidderExt.Data) > 0 {
		populateFirstPartyDataImpAttributes(bidderExt.Data, finalExtMap)
	}

	// Add other extensions
	if bidderExt.AE != 0 {
		finalExtMap[AEKey] = bidderExt.AE
	}
	if bidderExt.GPID != "" {
		finalExtMap[GPIDKey] = bidderExt.GPID
	}
	if bidderExt.SKAdNetwork != nil {
		var skadnVal interface{}
		if err := json.Unmarshal(bidderExt.SKAdNetwork, &skadnVal); err == nil {
			finalExtMap[SKAdNetworkKey] = skadnVal
		}
	}

	// Rewrite imp.ext to PBS bidder format — PubMatic's translator expects imp.ext.bidder.{params}
	if strings.TrimSpace(pubmaticExt.PublisherId) != "" {
		pmImpExt := map[string]interface{}{
			"publisherId": strings.TrimSpace(pubmaticExt.PublisherId),
		}
		if adSlot := strings.TrimSpace(pubmaticExt.AdSlot); adSlot != "" {
			// Use just the slot name without size suffix (before '@')
			if idx := strings.Index(adSlot, "@"); idx >= 0 {
				adSlot = strings.TrimSpace(adSlot[:idx])
			}
			pmImpExt["adSlot"] = adSlot
		}
		finalExtMap["bidder"] = pmImpExt
	}

	// Set final extension
	imp.Ext = nil
	if len(finalExtMap) > 0 {
		ext, err := json.Marshal(finalExtMap)
		if err == nil {
			imp.Ext = ext
		}
	}

	return wrapExt, pubID, nil
}

// validateAdSlot validates and parses the ad slot string
func validateAdSlot(adslot string, imp *openrtb.Imp) error {
	adSlotStr := strings.TrimSpace(adslot)

	if len(adSlotStr) == 0 {
		return nil
	}

	if !strings.Contains(adSlotStr, "@") {
		imp.TagID = adSlotStr
		return nil
	}

	adSlot := strings.Split(adSlotStr, "@")
	if len(adSlot) == 2 && adSlot[0] != "" && adSlot[1] != "" {
		imp.TagID = strings.TrimSpace(adSlot[0])

		adSize := strings.Split(strings.ToLower(adSlot[1]), "x")
		if len(adSize) != 2 {
			return fmt.Errorf("invalid size provided in adSlot %v", adSlotStr)
		}

		width, err := strconv.Atoi(strings.TrimSpace(adSize[0]))
		if err != nil {
			return fmt.Errorf("invalid width provided in adSlot %v", adSlotStr)
		}

		heightStr := strings.Split(adSize[1], ":")
		height, err := strconv.Atoi(strings.TrimSpace(heightStr[0]))
		if err != nil {
			return fmt.Errorf("invalid height provided in adSlot %v", adSlotStr)
		}

		// Set banner size if banner is present
		if imp.Banner != nil {
			imp.Banner.W = width
			imp.Banner.H = height

			// Update Banner.Format to include the adSlot size
			// Prepend to ensure it's the primary size
			format := openrtb.Format{W: width, H: height}
			if len(imp.Banner.Format) == 0 {
				imp.Banner.Format = []openrtb.Format{format}
			} else {
				// Check if this size already exists in Format
				exists := false
				for _, f := range imp.Banner.Format {
					if f.W == width && f.H == height {
						exists = true
						break
					}
				}
				// Prepend if it doesn't exist
				if !exists {
					imp.Banner.Format = append([]openrtb.Format{format}, imp.Banner.Format...)
				}
			}
		}
	} else {
		return fmt.Errorf("invalid adSlot %v", adSlotStr)
	}

	return nil
}

// addKeywordsToExt adds keywords to extension map
func addKeywordsToExt(keywords []*ExtImpPubmaticKeyVal, extMap map[string]interface{}) {
	for _, keyVal := range keywords {
		if len(keyVal.Values) == 0 {
			continue
		}
		key := keyVal.Key
		if keyVal.Key == PmZoneIDKeyNameOld {
			key = PmZoneIDKeyName
		}
		extMap[key] = strings.Join(keyVal.Values, ",")
	}
}

// populateFirstPartyDataImpAttributes processes first-party data
func populateFirstPartyDataImpAttributes(data json.RawMessage, extMap map[string]interface{}) {
	dataMap := getMapFromJSON(data)
	if dataMap == nil {
		return
	}

	populateAdUnitKey(dataMap, extMap)
	populateDctrKey(dataMap, extMap)
}

// populateAdUnitKey extracts ad unit code from first-party data
func populateAdUnitKey(dataMap, extMap map[string]interface{}) {
	// Check for GAM ad server
	if adserver, ok := dataMap[AdServerKey].(map[string]interface{}); ok {
		if name, ok := adserver["name"].(string); ok && name == AdServerGAM {
			if adslot, ok := adserver["adslot"].(string); ok && adslot != "" {
				extMap[ImpExtAdUnitKey] = adslot
				return
			}
		}
	}

	// Fall back to pbadslot
	if extMap[ImpExtAdUnitKey] == nil && dataMap[PBAdSlotKey] != nil {
		if pbadslot, ok := dataMap[PBAdSlotKey].(string); ok {
			extMap[ImpExtAdUnitKey] = pbadslot
		}
	}
}

// populateDctrKey builds targeting key-value string from first-party data
func populateDctrKey(dataMap, extMap map[string]interface{}) {
	var dctr strings.Builder

	// Append existing dctr if present
	if extMap[DctrKeyName] != nil {
		if dctrStr, ok := extMap[DctrKeyName].(string); ok {
			dctr.WriteString(dctrStr)
		}
	}

	for key, val := range dataMap {
		// Skip special keys
		if key == PBAdSlotKey || key == AdServerKey {
			continue
		}

		// Add separator
		if dctr.Len() > 0 {
			dctr.WriteString("|")
		}

		key = strings.TrimSpace(key)

		switch typedValue := val.(type) {
		case string:
			fmt.Fprintf(&dctr, "%s=%s", key, strings.TrimSpace(typedValue))
		case float64, bool:
			fmt.Fprintf(&dctr, "%s=%v", key, typedValue)
		case []interface{}:
			if valStrArr := getStringArray(typedValue); len(valStrArr) > 0 {
				valStr := strings.Join(valStrArr, ",")
				fmt.Fprintf(&dctr, "%s=%s", key, valStr)
			}
		}
	}

	if dctrStr := dctr.String(); dctrStr != "" {
		extMap[DctrKeyName] = strings.TrimSuffix(dctrStr, "|")
	}
}

// getStringArray converts interface array to string array
func getStringArray(array []interface{}) []string {
	result := make([]string, 0, len(array))
	for _, v := range array {
		if str, ok := v.(string); ok {
			result = append(result, strings.TrimSpace(str))
		} else {
			return nil
		}
	}
	return result
}

// getMapFromJSON converts JSON to map
func getMapFromJSON(source json.RawMessage) map[string]interface{} {
	if source != nil {
		dataMap := make(map[string]interface{})
		err := json.Unmarshal(source, &dataMap)
		if err == nil {
			return dataMap
		}
	}
	return nil
}

// extractPubmaticExtFromRequest extracts PubMatic extensions from request
func extractPubmaticExtFromRequest(request *openrtb.BidRequest) (ExtRequestPubmatic, error) {
	var pmReqExt ExtRequestPubmatic

	if request == nil || len(request.Ext) == 0 {
		pmReqExt.Wrapper = &PubmaticWrapperExt{BidderCode: bidderPubMatic}
		return pmReqExt, nil
	}

	var reqExt ExtRequest
	if err := json.Unmarshal(request.Ext, &reqExt); err != nil {
		return pmReqExt, fmt.Errorf("error decoding request.ext: %w", err)
	}

	// Parse bidder params
	reqExtBidderParams := make(map[string]json.RawMessage)
	if reqExt.Prebid != nil && reqExt.Prebid.BidderParams != nil {
		if err := json.Unmarshal(reqExt.Prebid.BidderParams, &reqExtBidderParams); err != nil {
			return pmReqExt, err
		}
	}

	// Extract wrapper extension
	if wrapperObj, present := reqExtBidderParams["wrapper"]; present && len(wrapperObj) != 0 {
		var wrpExt PubmaticWrapperExt
		if err := json.Unmarshal(wrapperObj, &wrpExt); err != nil {
			return pmReqExt, err
		}
		pmReqExt.Wrapper = &wrpExt
	}

	if pmReqExt.Wrapper == nil {
		pmReqExt.Wrapper = &PubmaticWrapperExt{}
	}

	// Set bidder code
	pmReqExt.Wrapper.BidderCode = bidderPubMatic

	// Override bidder code if alias exists (use deterministic selection)
	if reqExt.Prebid != nil && reqExt.Prebid.Aliases != nil && len(reqExt.Prebid.Aliases) > 0 {
		// Sort aliases for deterministic selection
		aliases := make([]string, 0, len(reqExt.Prebid.Aliases))
		for alias := range reqExt.Prebid.Aliases {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		pmReqExt.Wrapper.BidderCode = aliases[0]
	}

	// Extract acat
	if acatBytes, ok := reqExtBidderParams["acat"]; ok {
		var acat []string
		if err := json.Unmarshal(acatBytes, &acat); err != nil {
			return pmReqExt, err
		}
		for i := 0; i < len(acat); i++ {
			acat[i] = strings.TrimSpace(acat[i])
		}
		pmReqExt.Acat = acat
	}

	// Extract alternate bidder codes
	if allowedBidders := getAlternateBidderCodesFromRequestExt(&reqExt); allowedBidders != nil {
		pmReqExt.Marketplace = &MarketplaceReqExt{AllowedBidders: allowedBidders}
	}

	return pmReqExt, nil
}

// getAlternateBidderCodesFromRequestExt extracts alternate bidder codes
func getAlternateBidderCodesFromRequestExt(reqExt *ExtRequest) []string {
	if reqExt == nil || reqExt.Prebid == nil || reqExt.Prebid.AlternateBidderCodes == nil {
		return nil
	}

	allowedBidders := []string{"pubmatic"}
	if reqExt.Prebid.AlternateBidderCodes.Enabled {
		if pmABC, ok := reqExt.Prebid.AlternateBidderCodes.Bidders["pubmatic"]; ok && pmABC.Enabled {
			if pmABC.AllowedBidderCodes == nil || (len(pmABC.AllowedBidderCodes) == 1 && pmABC.AllowedBidderCodes[0] == "*") {
				return []string{"all"}
			}
			return append(allowedBidders, pmABC.AllowedBidderCodes...)
		}
	}

	return allowedBidders
}

// getDisplayManagerAndVer extracts display manager info from app extension
func getDisplayManagerAndVer(app *openrtb.App) (string, string) {
	if app.Ext == nil {
		return "", ""
	}

	var appExt ExtApp
	if err := json.Unmarshal(app.Ext, &appExt); err != nil {
		return "", ""
	}

	// Try prebid.source and prebid.version first
	if appExt.Prebid != nil && appExt.Prebid.Source != "" && appExt.Prebid.Version != "" {
		return appExt.Prebid.Source, appExt.Prebid.Version
	}

	// Fall back to source and version
	if appExt.Source != "" && appExt.Version != "" {
		return appExt.Source, appExt.Version
	}

	return "", ""
}

// getNativeAdm extracts native ad markup from nested structure
func getNativeAdm(adm string) (string, error) {
	var nativeAdm map[string]interface{}
	if err := json.Unmarshal([]byte(adm), &nativeAdm); err != nil {
		return adm, fmt.Errorf("unable to unmarshal native adm: %w", err)
	}

	// Extract nested native object
	if nativeObj, ok := nativeAdm["native"]; ok {
		nativeBytes, err := json.Marshal(nativeObj)
		if err != nil {
			return adm, fmt.Errorf("unable to marshal native object: %w", err)
		}
		return string(nativeBytes), nil
	}

	return adm, nil
}

// Info returns bidder information
func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled: true,
		Maintainer: &adapters.MaintainerInfo{
			Email: "header-bidding@pubmatic.com",
		},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
					adapters.BidTypeNative,
				},
			},
			App: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
					adapters.BidTypeNative,
				},
			},
		},
		GVLVendorID: 76,
		Endpoint:    defaultEndpoint,
		DemandType:  adapters.DemandTypePlatform,
	}
}

func init() {
	if err := adapters.RegisterAdapter("pubmatic", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "pubmatic").Msg("failed to register adapter")
	}
}
