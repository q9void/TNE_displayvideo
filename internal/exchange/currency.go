package exchange

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
	"github.com/thenexusengine/tne_springwire/pkg/currency"
	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// PrebidExt represents the ext.prebid object in OpenRTB requests
type PrebidExt struct {
	Currency *PrebidCurrency `json:"currency,omitempty"`
}

// PrebidCurrency represents currency configuration in ext.prebid.currency
type PrebidCurrency struct {
	Rates       map[string]map[string]float64 `json:"rates,omitempty"`
	UsePBSRates *bool                          `json:"usepbsrates,omitempty"` // If true, prefer external rates
}

// RequestExt represents the full ext object structure
type RequestExt struct {
	Prebid *PrebidExt `json:"prebid,omitempty"`
}

// extractTargetCurrency extracts the target currency from the bid request
// Returns the first currency from request.cur[], or the default currency
func (e *Exchange) extractTargetCurrency(req *openrtb.BidRequest) string {
	// Check if cur array is provided and has at least one element
	if len(req.Cur) > 0 && req.Cur[0] != "" {
		return req.Cur[0]
	}

	// Fall back to default currency
	if e.config.DefaultCurrency != "" {
		return e.config.DefaultCurrency
	}

	// Ultimate fallback
	return "USD"
}

// extractCustomRates extracts custom currency rates from ext.prebid.currency.rates
func extractCustomRates(req *openrtb.BidRequest) (map[string]map[string]float64, bool) {
	if req.Ext == nil {
		return nil, false
	}

	var ext RequestExt
	if err := json.Unmarshal(req.Ext, &ext); err != nil {
		logger.Log.Warn().Err(err).Msg("failed to parse request.ext")
		return nil, false
	}

	if ext.Prebid == nil || ext.Prebid.Currency == nil {
		return nil, false
	}

	// Check if custom rates should be preferred
	useExternal := false
	if ext.Prebid.Currency.UsePBSRates != nil {
		useExternal = *ext.Prebid.Currency.UsePBSRates
	}

	return ext.Prebid.Currency.Rates, useExternal
}

// convertBidCurrency converts a bid's price to the target currency
// Returns the converted price and the currency it was converted from
func (e *Exchange) convertBidCurrency(
	bidPrice float64,
	bidCurrency string,
	targetCurrency string,
	customRates map[string]map[string]float64,
	useExternalRates bool,
) (float64, error) {
	// No conversion needed if same currency
	if bidCurrency == targetCurrency {
		return bidPrice, nil
	}

	// If no currency converter available, reject conversion
	if e.currencyConverter == nil {
		return 0, fmt.Errorf("currency conversion not available")
	}

	// Create aggregate conversions (combines custom + external rates)
	conversions := currency.NewAggregateConversions(
		customRates,
		e.currencyConverter,
		useExternalRates,
	)

	// Convert the bid price
	convertedPrice, err := conversions.Convert(bidPrice, bidCurrency, targetCurrency)
	if err != nil {
		return 0, fmt.Errorf("convert %s to %s: %w", bidCurrency, targetCurrency, err)
	}

	logger.Log.Debug().
		Str("from", bidCurrency).
		Str("to", targetCurrency).
		Float64("originalPrice", bidPrice).
		Float64("convertedPrice", convertedPrice).
		Msg("converted bid currency")

	return convertedPrice, nil
}

// convertBidderResponse converts all bids in a bidder response to the target currency
func (e *Exchange) convertBidderResponse(
	response *openrtb.BidResponse,
	bidderCode string,
	targetCurrency string,
	customRates map[string]map[string]float64,
	useExternalRates bool,
) error {
	if response == nil || len(response.SeatBid) == 0 {
		return nil
	}

	// Determine the bid currency (from response or assume target currency)
	bidCurrency := response.Cur
	if bidCurrency == "" {
		bidCurrency = targetCurrency // Assume target currency if not specified
	}

	// If already in target currency, no conversion needed
	if bidCurrency == targetCurrency {
		response.Cur = targetCurrency
		return nil
	}

	// Convert each bid
	for seatIdx := range response.SeatBid {
		for bidIdx := range response.SeatBid[seatIdx].Bid {
			bid := &response.SeatBid[seatIdx].Bid[bidIdx]

			// Convert the bid price
			convertedPrice, err := e.convertBidCurrency(
				bid.Price,
				bidCurrency,
				targetCurrency,
				customRates,
				useExternalRates,
			)

			if err != nil {
				logger.Log.Warn().
					Str("bidder", bidderCode).
					Str("bidId", bid.ID).
					Str("from", bidCurrency).
					Str("to", targetCurrency).
					Err(err).
					Msg("failed to convert bid currency")
				// Reject the bid if conversion fails
				return fmt.Errorf("currency conversion failed for bid %s: %w", bid.ID, err)
			}

			// Update the bid price
			bid.Price = convertedPrice
		}
	}

	// Update the response currency
	response.Cur = targetCurrency

	return nil
}

// isCurrencySupported checks if a currency is supported for conversion
func (e *Exchange) isCurrencySupported(currencyCode string) bool {
	if e.currencyConverter == nil {
		return false
	}

	// Try to get rates for this currency
	stats := e.currencyConverter.Stats()
	if ratesLoaded, ok := stats["ratesLoaded"].(bool); !ok || !ratesLoaded {
		return false
	}

	// Check if currency exists in rates
	rates := e.currencyConverter.GetRates()
	if rates == nil {
		return false
	}

	_, exists := rates[currencyCode]
	return exists
}

// normalizeIsoCurrency normalizes currency codes to uppercase ISO 4217 format
// Fixes issue #16: "usd" → "USD", "eur" → "EUR", etc.
func normalizeIsoCurrency(code string) string {
	// Convert to uppercase for ISO 4217 compliance
	return strings.ToUpper(code)
}
