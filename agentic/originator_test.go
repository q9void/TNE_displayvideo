package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

func TestStampRTBRequest_setsSSPOriginator(t *testing.T) {
	s := OriginatorStamper{SellerID: "9131"}
	req := &pb.RTBRequest{}
	s.StampRTBRequest(req, LifecyclePublisherBidRequest)

	require.NotNil(t, req.Originator)
	require.NotNil(t, req.Originator.Type)
	assert.Equal(t, pb.Originator_TYPE_SSP, *req.Originator.Type)
	require.NotNil(t, req.Originator.Id)
	assert.Equal(t, "9131", *req.Originator.Id)
}

func TestStampRTBRequest_setsLifecycle(t *testing.T) {
	cases := []struct {
		in   Lifecycle
		want pb.Lifecycle
	}{
		{LifecyclePublisherBidRequest, pb.Lifecycle_LIFECYCLE_PUBLISHER_BID_REQUEST},
		{LifecycleDSPBidResponse, pb.Lifecycle_LIFECYCLE_DSP_BID_RESPONSE},
	}
	for _, tc := range cases {
		t.Run(tc.in.String(), func(t *testing.T) {
			s := OriginatorStamper{SellerID: "9131"}
			req := &pb.RTBRequest{}
			s.StampRTBRequest(req, tc.in)
			require.NotNil(t, req.Lifecycle)
			assert.Equal(t, tc.want, *req.Lifecycle)
		})
	}
}

func TestStampRTBRequest_nilSafe(t *testing.T) {
	s := OriginatorStamper{SellerID: "9131"}
	// must not panic
	s.StampRTBRequest(nil, LifecyclePublisherBidRequest)
}

func TestStampRTBRequest_idempotent(t *testing.T) {
	s := OriginatorStamper{SellerID: "9131"}
	req := &pb.RTBRequest{}
	s.StampRTBRequest(req, LifecyclePublisherBidRequest)
	first := *req.Originator.Id
	s.StampRTBRequest(req, LifecyclePublisherBidRequest)
	assert.Equal(t, first, *req.Originator.Id)
}
