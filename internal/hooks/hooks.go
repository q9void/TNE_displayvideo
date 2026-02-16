// Package hooks provides a PBS-style hook framework for request/response processing
package hooks

import (
	"context"

	"github.com/thenexusengine/tne_springwire/internal/openrtb"
)

// RequestHook is executed once per auction request (before adapters)
// Examples: request validation, privacy enforcement, ID clearing
type RequestHook interface {
	ProcessRequest(ctx context.Context, req *openrtb.BidRequest) error
}

// BidderRequestHook is executed per-bidder (after request clone, before adapter)
// Examples: identity gating, schain augmentation, bidder-specific transforms
type BidderRequestHook interface {
	ProcessBidderRequest(ctx context.Context, req *openrtb.BidRequest, bidderName string) error
}

// BidderResponseHook is executed per-bid (after MakeBids, before auction)
// Examples: response normalization, bid validation, currency conversion
type BidderResponseHook interface {
	ProcessBidderResponse(ctx context.Context, req *openrtb.BidRequest, resp *openrtb.BidResponse, bidderName string) error
}

// AuctionHook is executed once after all bidder responses (before final response)
// Examples: multiformat selection, de-duplication, price floor enforcement
type AuctionHook interface {
	ProcessAuction(ctx context.Context, req *openrtb.BidRequest, responses []*BidderResponse) error
}

// BidderResponse represents a response from a single bidder
type BidderResponse struct {
	BidderName string
	Response   *openrtb.BidResponse
	Errors     []error
}

// HookExecutor manages hook execution in the correct order
type HookExecutor struct {
	requestHooks        []RequestHook
	bidderRequestHooks  []BidderRequestHook
	bidderResponseHooks []BidderResponseHook
	auctionHooks        []AuctionHook
}

// NewHookExecutor creates a new hook executor
func NewHookExecutor() *HookExecutor {
	return &HookExecutor{
		requestHooks:        make([]RequestHook, 0),
		bidderRequestHooks:  make([]BidderRequestHook, 0),
		bidderResponseHooks: make([]BidderResponseHook, 0),
		auctionHooks:        make([]AuctionHook, 0),
	}
}

// RegisterRequestHook adds a request-level hook
func (e *HookExecutor) RegisterRequestHook(hook RequestHook) {
	e.requestHooks = append(e.requestHooks, hook)
}

// RegisterBidderRequestHook adds a per-bidder request hook
func (e *HookExecutor) RegisterBidderRequestHook(hook BidderRequestHook) {
	e.bidderRequestHooks = append(e.bidderRequestHooks, hook)
}

// RegisterBidderResponseHook adds a per-bidder response hook
func (e *HookExecutor) RegisterBidderResponseHook(hook BidderResponseHook) {
	e.bidderResponseHooks = append(e.bidderResponseHooks, hook)
}

// RegisterAuctionHook adds an auction-level hook
func (e *HookExecutor) RegisterAuctionHook(hook AuctionHook) {
	e.auctionHooks = append(e.auctionHooks, hook)
}

// ExecuteRequestHooks runs all request-level hooks in order
// Returns first error encountered (short-circuit on error)
func (e *HookExecutor) ExecuteRequestHooks(ctx context.Context, req *openrtb.BidRequest) error {
	for _, hook := range e.requestHooks {
		if err := hook.ProcessRequest(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteBidderRequestHooks runs all per-bidder request hooks in order
func (e *HookExecutor) ExecuteBidderRequestHooks(ctx context.Context, req *openrtb.BidRequest, bidderName string) error {
	for _, hook := range e.bidderRequestHooks {
		if err := hook.ProcessBidderRequest(ctx, req, bidderName); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteBidderResponseHooks runs all per-bidder response hooks in order
func (e *HookExecutor) ExecuteBidderResponseHooks(ctx context.Context, req *openrtb.BidRequest, resp *openrtb.BidResponse, bidderName string) error {
	for _, hook := range e.bidderResponseHooks {
		if err := hook.ProcessBidderResponse(ctx, req, resp, bidderName); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteAuctionHooks runs all auction-level hooks in order
func (e *HookExecutor) ExecuteAuctionHooks(ctx context.Context, req *openrtb.BidRequest, responses []*BidderResponse) error {
	for _, hook := range e.auctionHooks {
		if err := hook.ProcessAuction(ctx, req, responses); err != nil {
			return err
		}
	}
	return nil
}
