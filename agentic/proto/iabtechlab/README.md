# Vendored IAB Tech Lab protos

These files are copied (with the minimal patches noted below) from the upstream
IAB Tech Lab agentic-rtb-framework repo. Treat them as read-only — refresh by
re-vendoring, never by hand-editing.

## Provenance

- **Upstream:** https://github.com/IABTechLab/agentic-rtb-framework
- **Commit SHA:** `7428953220937154c86a4451dc662ded715efaf6`
- **Pulled:** 2026-04-27
- **Pulled by:** branch `claude/integrate-iab-agentic-protocol-6bvtJ`
- **ARTF spec status at pull:** v1.0 for Public Comment

## Vendored files

| Vendored path | Upstream path |
|---|---|
| `bidstream/mutation/v1/agenticrtbframework.proto` | `proto/agenticrtbframework.proto` |
| `bidstream/mutation/v1/agenticrtbframeworkservices.proto` | `agenticrtbframeworkservices.proto` (root) |
| `openrtb/v26/openrtb.proto` | `proto/com/iabtechlab/openrtb/v2/openrtb.proto` |

## Vendoring patches (intentional, minimal)

The following one-line edits were applied at vendor time. They are recorded
here so a future re-vendor can re-apply them deterministically. None of them
change wire format or field numbers.

1. **`agenticrtbframework.proto` — added `option go_package`.**
   Upstream omits `go_package`; we set it to our gen path so `protoc-gen-go`
   places generated files at `agentic/gen/iabtechlab/bidstream/mutation/v1/`.

2. **`agenticrtbframework.proto` — patched `import` path.**
   Upstream: `import "com/iabtechlab/openrtb/v2.6/openrtb.proto";`
   Vendored: `import "iabtechlab/openrtb/v26/openrtb.proto";`
   Reason: our `--proto_path=agentic/proto` resolves imports relative to that
   root. `v2.6` was renamed to `v26` because Go cannot import directories with
   a dot in the name.

3. **`agenticrtbframework.proto` — patched `MetricsPayload.metric` type.**
   Upstream: `repeated com.iabtechlab.openrtb.v2.BidRequest.Metric metric = 1;`
   Vendored: `repeated com.iabtechlab.openrtb.v2.BidRequest.Imp.Metric metric = 1;`
   Reason: the upstream reference is broken — `Metric` is nested at
   `BidRequest.Imp.Metric` in the OpenRTB v2.6 schema, not at `BidRequest.Metric`.
   This is a fix, not a deviation. Re-evaluate on next vendor refresh.

4. **`agenticrtbframeworkservices.proto` — added `option go_package` + patched `import` path.**
   Same rationale as (1)/(2).

5. **`openrtb/v26/openrtb.proto` — patched `option go_package`.**
   Upstream: `github.com/iabtechlab/agentic-rtb-framework/pkg/pb/openrtb`
   Vendored: `github.com/thenexusengine/tne_springwire/agentic/gen/iabtechlab/openrtb/v26;openrtbv26`
   Reason: codegen lands at our path.

## Refresh procedure

1. Resolve the latest commit SHA on `main` of the upstream repo.
2. Re-copy the three files to their vendored paths.
3. Re-apply patches (1)–(5) above.
4. Update the SHA + date at the top of this file.
5. From the repo root, run `make generate-protos`.
6. `go build ./agentic/...` should pass.
7. `go test ./agentic/...` should pass — if any applier whitelist breaks, the
   spec changed; reconcile before merging.

## Why `v26` and not `v2.6`?

Go's import system disallows package paths with a dot in the directory name.
Renaming the local directory to `v26` keeps the source-relative codegen layout
(`agentic/proto/iabtechlab/openrtb/v26/` ↔ `agentic/gen/iabtechlab/openrtb/v26/`)
clean. The proto `package` declaration inside the file is still
`com.iabtechlab.openrtb.v2`, matching the wire format expected by ARTF.
