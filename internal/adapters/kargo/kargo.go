// Package kargo implements the Kargo bidder adapter
package kargo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thenexusengine/tne_springwire/internal/adapters"
	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

const (
	defaultEndpoint = "https://krk.kargo.com/api/v1/openrtb"
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

	// NOTE: ID clearing is now handled by Privacy/Consent hook (no longer needed here)
	// NOTE: SetUserID is now handled by Identity Gating hook (no longer needed here)

	requestBody, err := json.Marshal(requestCopy)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to marshal request: %w", err)}
	}

	// Compress request body with GZIP
	var compressedBody bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBody)
	if _, err := gzipWriter.Write(requestBody); err != nil {
		return nil, []error{fmt.Errorf("failed to gzip request body: %w", err)}
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, []error{fmt.Errorf("failed to close gzip writer: %w", err)}
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json;charset=utf-8")
	headers.Set("Content-Encoding", "gzip")
	headers.Set("Accept", "application/json")
	headers.Set("Accept-Encoding", "gzip")

	return []*adapters.RequestData{{
		Method:  "POST",
		URI:     a.endpoint,
		Body:    compressedBody.Bytes(),
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
	responseBody := responseData.Body
	if responseData.Headers.Get("Content-Encoding") == "gzip" {
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
			// Check Kargo's extension first for authoritative media type
			bidType := getMediaTypeForBid(bid, impMap)

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
func getMediaTypeForBid(bid *openrtb.Bid, impMap map[string]*openrtb.Imp) adapters.BidType {
	// Check Kargo's extension first (authoritative signal)
	if bid.Ext != nil {
		var kargoExt struct {
			MediaType string `json:"mediaType"`
		}
		if err := json.Unmarshal(bid.Ext, &kargoExt); err == nil && kargoExt.MediaType != "" {
			switch kargoExt.MediaType {
			case "video":
				return adapters.BidTypeVideo
			case "native":
				return adapters.BidTypeNative
			case "banner":
				return adapters.BidTypeBanner
			}
		}
	}

	// Fallback to impression-based detection
	return adapters.GetBidTypeFromMap(bid, impMap)
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

func init() {
	if err := adapters.RegisterAdapter("kargo", New(""), Info()); err != nil {
		logger.Log.Error().Err(err).Str("adapter", "kargo").Msg("failed to register adapter")
	}
}
