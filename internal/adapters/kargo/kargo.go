// Package kargo implements the Kargo bidder adapter
package kargo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const (
	defaultEndpoint = "https://kraken.prod.kargo.com/api/v1/openrtb"
)

// Adapter implements the Kargo bidder
type Adapter struct {
	endpoint string
}

// New creates a new Kargo adapter
func New(endpoint string) *Adapter {
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Adapter{endpoint: endpoint}
}

// MakeRequests builds HTTP requests for Kargo with GZIP compression
func (a *Adapter) MakeRequests(request *openrtb.BidRequest, extraInfo *adapters.ExtraRequestInfo) ([]*adapters.RequestData, []error) {
	// Create a copy of the request to modify
	requestCopy := *request
	// Deep copy Imp slice so we can rewrite imp.ext without mutating the caller's request
	requestCopy.Imp = make([]openrtb.Imp, len(request.Imp))
	copy(requestCopy.Imp, request.Imp)
	var errs []error

	// NOTE: ID clearing is now handled by Privacy/Consent hook (no longer needed here)
	// NOTE: SetUserID is now handled by Identity Gating hook (no longer needed here)

	// Validate imp.ext.bidder.placementId and rewrite imp.ext to PBS bidder format.
	// PBS translates imp.ext.kargo → imp.ext.bidder before forwarding to the bidder
	// endpoint; we mirror that here so Kargo's endpoint receives imp.ext.bidder.placementId.
	for i, imp := range requestCopy.Imp {
		if imp.Ext == nil {
			errs = append(errs, fmt.Errorf("imp %s missing required imp.ext", imp.ID))
			continue
		}

		var impExt struct {
			Kargo  json.RawMessage `json:"kargo"`
			Bidder json.RawMessage `json:"bidder"`
		}
		if err := json.Unmarshal(imp.Ext, &impExt); err != nil {
			errs = append(errs, fmt.Errorf("failed to parse imp.ext for imp %s: %w", imp.ID, err))
			continue
		}

		// Prefer imp.ext.bidder; fall back to imp.ext.kargo (PBS translates kargo → bidder)
		bidderParams := impExt.Bidder
		if len(bidderParams) == 0 {
			bidderParams = impExt.Kargo
		}
		if len(bidderParams) == 0 {
			errs = append(errs, fmt.Errorf("imp %s missing required kargo/bidder extension", imp.ID))
			continue
		}

		var kargoExt struct {
			PlacementID string `json:"placementId"`
		}
		if err := json.Unmarshal(bidderParams, &kargoExt); err != nil {
			errs = append(errs, fmt.Errorf("failed to parse kargo params for imp %s: %w", imp.ID, err))
			continue
		}
		if kargoExt.PlacementID == "" {
			errs = append(errs, fmt.Errorf("imp %s missing required placementId", imp.ID))
			continue
		}

		// Rewrite imp.ext so Kargo's endpoint receives imp.ext.bidder.placementId
		rewritten, err := json.Marshal(map[string]interface{}{
			"bidder": map[string]string{"placementId": kargoExt.PlacementID},
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to rewrite imp.ext for imp %s: %w", imp.ID, err))
			continue
		}
		requestCopy.Imp[i].Ext = rewritten
	}

	// If all impressions failed validation, return errors
	if len(errs) >= len(requestCopy.Imp) {
		return nil, errs
	}

	// Kargo-specific user enrichment
	if request.User != nil {
		userCopy := *request.User

		// 1. Populate user.eids at top level from user.ext.eids
		//    Kargo uses OpenRTB 2.6 top-level eids for identity matching.
		//    The EID filter may have cleared the typed field; restore from raw ext.
		if len(userCopy.EIDs) == 0 && len(userCopy.Ext) > 0 {
			var extEIDs struct {
				EIDs []openrtb.EID `json:"eids"`
			}
			if err := json.Unmarshal(userCopy.Ext, &extEIDs); err == nil && len(extEIDs.EIDs) > 0 {
				userCopy.EIDs = extEIDs.EIDs
			}
		}

		// 2. Set user.buyeruid from kargo.com EID
		//    Kargo matches requests to cookie-synced users via buyeruid.
		if uid := adapters.ExtractUIDFromEids(request.User, "kargo.com"); uid != "" {
			userCopy.BuyerUID = uid
		}

		requestCopy.User = &userCopy
	}

	requestBody, err := json.Marshal(requestCopy)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
	}

	headers := http.Header{}
	headers.Add("Content-Type", "application/json;charset=utf-8")
	headers.Add("Accept", "application/json")

	return []*adapters.RequestData{{
		Method:  "POST",
		URI:     a.endpoint,
		Body:    requestBody,
		Headers: headers,
	}}, nil
}

// MakeBids parses Kargo responses into bids (handles GZIP-compressed responses)
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

	// Decompress response if GZIP-encoded
	// Task #19: Make Content-Encoding detection case-insensitive
	responseBody := responseData.Body
	contentEncoding := responseData.Headers.Get("Content-Encoding")
	if strings.EqualFold(contentEncoding, "gzip") {
		gzipReader, err := gzip.NewReader(bytes.NewReader(responseData.Body))
		if err != nil {
			return nil, []error{fmt.Errorf("failed to create gzip reader: %w", err)}
		}
		defer gzipReader.Close()

		var decompressed bytes.Buffer
		if _, err := decompressed.ReadFrom(gzipReader); err != nil {
			return nil, []error{fmt.Errorf("failed to decompress response: %w", err)}
		}
		responseBody = decompressed.Bytes()
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

	// Build impression map for O(1) bid type detection
	impMap := adapters.BuildImpMap(request.Imp)

	for _, seatBid := range bidResp.SeatBid {
		for i := range seatBid.Bid {
			bid := &seatBid.Bid[i]
			// Task #17: Check Kargo's extension first for authoritative media type and validate against imp
			bidType, err := getMediaTypeForBid(bid, impMap)
			if err != nil {
				// Skip bids with invalid media types
				logger.Log.Warn().
					Str("bidId", bid.ID).
					Str("impId", bid.ImpID).
					Err(err).
					Msg("skipping bid with invalid mediaType")
				continue
			}

			response.Bids = append(response.Bids, &adapters.TypedBid{
				Bid:     bid,
				BidType: bidType,
			})
		}
	}

	return response, nil
}

// getMediaTypeForBid determines bid type by checking Kargo's extension first
// Falls back to impression-based detection if extension not present
// Task #17: Validates that mediaType matches impression capabilities
func getMediaTypeForBid(bid *openrtb.Bid, impMap map[string]*openrtb.Imp) (adapters.BidType, error) {
	// Get the impression to validate against
	imp, ok := impMap[bid.ImpID]
	if !ok {
		// If impression not found, use default but don't error
		return adapters.BidTypeBanner, nil
	}

	// Check Kargo's extension first (authoritative signal)
	if bid.Ext != nil {
		var kargoExt struct {
			MediaType string `json:"mediaType"`
		}
		if err := json.Unmarshal(bid.Ext, &kargoExt); err == nil && kargoExt.MediaType != "" {
			var bidType adapters.BidType
			switch kargoExt.MediaType {
			case "video":
				bidType = adapters.BidTypeVideo
				// Validate impression supports video
				if imp.Video == nil {
					return "", fmt.Errorf("bid.ext.mediaType is 'video' but imp %s does not support video", bid.ImpID)
				}
			case "native":
				bidType = adapters.BidTypeNative
				// Validate impression supports native
				if imp.Native == nil {
					return "", fmt.Errorf("bid.ext.mediaType is 'native' but imp %s does not support native", bid.ImpID)
				}
			case "banner":
				bidType = adapters.BidTypeBanner
				// Validate impression supports banner
				if imp.Banner == nil {
					return "", fmt.Errorf("bid.ext.mediaType is 'banner' but imp %s does not support banner", bid.ImpID)
				}
			default:
				return "", fmt.Errorf("unknown mediaType '%s' in bid.ext for bid %s", kargoExt.MediaType, bid.ID)
			}
			return bidType, nil
		}
	}

	// Fallback to impression-based detection
	return adapters.GetBidTypeFromMap(bid, impMap), nil
}

// Info returns bidder information
func Info() adapters.BidderInfo {
	return adapters.BidderInfo{
		Enabled: true,
		Maintainer: &adapters.MaintainerInfo{
			Email: "kraken@kargo.com",
		},
		Capabilities: &adapters.CapabilitiesInfo{
			Site: &adapters.PlatformInfo{
				MediaTypes: []adapters.BidType{
					adapters.BidTypeBanner,
					adapters.BidTypeVideo,
					adapters.BidTypeNative,
				},
			},
		},
		GVLVendorID: 972,
		Endpoint:    defaultEndpoint,
		DemandType:  adapters.DemandTypePlatform, // Platform demand (obfuscated as "thenexusengine")
	}
}

// NOTE: The direct Kargo adapter is intentionally NOT registered.
// Kargo is routed via PBS (Prebid Server), which handles the imp.ext.kargo →
// imp.ext.bidder translation and endpoint dispatch. Having 'kargo' in the
// imp.ext PBS bidders config is sufficient.
