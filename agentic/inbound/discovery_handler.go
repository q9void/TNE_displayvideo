package inbound

// Discovery service handler — Task 5 implementation.
//
// Phase 2A registers a stub here so server.go compiles cleanly. The real
// DescribeCapabilities RPC + tne/v1 proto registration land when Task 5
// of docs/superpowers/plans/2026-04-28-aamp-2a-inbound.md ships.

func registerDiscoveryHandler(_ *Server) {
	// Phase 2A scaffold: real registration in Task 5.
	// Until then, callers attempting Discovery RPCs will get gRPC
	// Unimplemented from the default service handler.
}
