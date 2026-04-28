package agentic

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/bidstream/mutation/v1"
)

// FakeAgentBehaviour configures the in-process gRPC RTBExtensionPoint
// server we use for tests. Most fields can stay zero; the server returns
// an empty RTBResponse if Mutations is nil.
type FakeAgentBehaviour struct {
	Mutations    []*pb.Mutation
	Sleep        time.Duration
	ReturnError  error
	ModelVersion string

	// Telemetry the test can read after the server stops.
	Calls           atomic.Int32
	LastAuthHeader  atomic.Value // string
	LastReqOrigType atomic.Value // pb.Originator_Type as int32
	LastLifecycle   atomic.Value // pb.Lifecycle as int32
}

type fakeServer struct {
	pb.UnimplementedRTBExtensionPointServer
	b *FakeAgentBehaviour
}

func (f *fakeServer) GetMutations(ctx context.Context, req *pb.RTBRequest) (*pb.RTBResponse, error) {
	f.b.Calls.Add(1)
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vs := md.Get("x-aamp-key"); len(vs) > 0 {
			f.b.LastAuthHeader.Store(vs[0])
		} else {
			f.b.LastAuthHeader.Store("")
		}
	}
	if req.Originator != nil && req.Originator.Type != nil {
		f.b.LastReqOrigType.Store(int32(*req.Originator.Type))
	}
	if req.Lifecycle != nil {
		f.b.LastLifecycle.Store(int32(*req.Lifecycle))
	}

	if f.b.Sleep > 0 {
		select {
		case <-time.After(f.b.Sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.b.ReturnError != nil {
		return nil, f.b.ReturnError
	}
	rsp := &pb.RTBResponse{Mutations: f.b.Mutations}
	if f.b.ModelVersion != "" {
		mv := f.b.ModelVersion
		rsp.Metadata = &pb.Metadata{ModelVersion: &mv}
	}
	return rsp, nil
}

// StartFakeAgent spins up a gRPC RTBExtensionPoint on 127.0.0.1:0 that
// dispatches every call to b. The returned addr is what the test should
// use as the agent endpoint URL. The stop closure is required — call it
// in defer.
func StartFakeAgent(t *testing.T, b *FakeAgentBehaviour) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterRTBExtensionPointServer(srv, &fakeServer{b: b})

	done := make(chan struct{})
	go func() {
		_ = srv.Serve(lis)
		close(done)
	}()

	stop = func() {
		srv.GracefulStop()
		<-done
	}
	return lis.Addr().String(), stop
}

// makeRegistryWithAgent builds an in-memory registry pointing at addr for
// tests that don't want to touch testdata files.
func makeRegistryWithAgent(t *testing.T, addr string, lc Lifecycle, intents []string, agentID string, priority int32, tmaxMs int32) *Registry {
	t.Helper()
	doc := []byte(`{
		"$schema": "https://thenexusengine.com/schemas/agents.v1.json",
		"version": "1.0",
		"seller_id": "9131",
		"seller_domain": "thenexusengine.com",
		"agents": [
			{
				"id": "` + agentID + `",
				"role": "segmentation",
				"priority": ` + itoa(priority) + `,
				"tmax_ms": ` + itoa(tmaxMs) + `,
				"endpoints": [{"transport": "grpc", "url": "` + addr + `", "auth": "api_key_header"}],
				"lifecycles": ["` + lc.String() + `"],
				"intents": ` + jsonStrArr(intents) + `
			}
		]
	}`)
	reg, err := LoadRegistryFromBytes(doc)
	if err != nil {
		t.Fatalf("LoadRegistryFromBytes: %v", err)
	}
	return reg
}
