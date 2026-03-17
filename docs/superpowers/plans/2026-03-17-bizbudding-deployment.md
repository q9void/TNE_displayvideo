# Bizbudding Ad Server Deployment — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fork TNE Catalyst into a standalone Bizbudding ad server at `ads.bizbudding.com`, fixing the ads.txt/sellers.json trust chain, adding an embedded admin UI, and providing both native and Docker deployment paths.

**Architecture:** Single Go binary with embedded admin UI served at `/admin`. Config and placement data live in PostgreSQL. The `/ads.txt` endpoint auto-generates from the `ssp_configs` table. All admin changes are live immediately — no restart required.

**Tech Stack:** Go 1.23, PostgreSQL 14+, Redis 7, vanilla JS (no build step), Docker Compose, nginx, Looker Studio.

**Spec:** `docs/superpowers/specs/2026-03-17-bizbudding-deployment-design.md`

**Module path:** `github.com/thenexusengine/tne_springwire` (unchanged — renaming deferred post-v1)

**Run tests:** `go test -v -race -cover ./...`

---

## Chunk 1: Foundation — Fork Setup, sellers.json, ads.txt Endpoint

### File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `Makefile` | Modify | Rename binary output to `catalyst-bizbudding` |
| `config/bizbudding.yaml` | Create | Bizbudding-specific server config |
| `.env.example` | Create | Document all required env vars |
| `assets/sellers.json` | Modify | Replace with Bizbudding-only IAB-compliant entry |
| `migrations/008_ssp_configs.sql` | Create | `ssp_configs` table + seed 5 default SSPs |
| `internal/endpoints/adstxt_handler.go` | Create | `GET /ads.txt` — queries DB, returns plain text |
| `internal/endpoints/adstxt_handler_test.go` | Create | Tests for ads.txt generation |
| `cmd/server/server.go` | Modify | Register `GET /ads.txt` route |

---

### Task 1: Rename binary in Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update the build target**

In `Makefile`, change line:
```makefile
build: ## Build the binary
	go build -o bin/catalyst cmd/server/main.go
```
To:
```makefile
build: ## Build the binary
	go build -o bin/catalyst-bizbudding cmd/server/main.go
```

- [ ] **Step 2: Verify build works**

```bash
make build
ls -la bin/catalyst-bizbudding
```
Expected: binary exists at `bin/catalyst-bizbudding`

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "chore: rename binary output to catalyst-bizbudding"
```

---

### Task 2: Create Bizbudding config file

**Files:**
- Create: `config/bizbudding.yaml`
- Create: `.env.example`

- [ ] **Step 1: Create `config/bizbudding.yaml`**

```yaml
# Bizbudding Ad Server Configuration
# Override any value with environment variables (see .env.example)

server:
  port: ${PBS_PORT:8080}
  host: ${PBS_HOST:0.0.0.0}
  read_timeout: 30s
  write_timeout: 30s

publisher:
  id: "NXS001"
  domain: "bizbudding.com"
  account_id: "12345"  # TODO: confirm Bizbudding's account_id before deploying

database:
  host: ${DB_HOST:localhost}
  port: ${DB_PORT:5432}
  name: ${DB_NAME:catalyst}
  user: ${DB_USER:catalyst}
  password: ${DB_PASSWORD}
  ssl_mode: ${DB_SSL_MODE:disable}

redis:
  host: ${REDIS_HOST:localhost}
  port: ${REDIS_PORT:6379}
  password: ${REDIS_PASSWORD:}

auction:
  timeout: 2000ms
  default_bidders:
    - rubicon
    - kargo
    - sovrn
    - pubmatic
    - triplelift

admin:
  enabled: true
  auth_required: true
  # Set ADMIN_API_KEY env var — see .env.example
```

- [ ] **Step 2: Create `.env.example`**

```bash
# Bizbudding Ad Server — Required Environment Variables
# Copy to .env and fill in values before running

# Server
PBS_PORT=8080
PBS_HOST=0.0.0.0

# PostgreSQL (required)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=catalyst
DB_USER=catalyst
DB_PASSWORD=changeme
DB_SSL_MODE=disable

# Redis (required)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Admin UI auth (required in production)
ADMIN_API_KEY=changeme-generate-with-openssl-rand-hex-32
ADMIN_AUTH_REQUIRED=true

# Optional: override default publisher
PUBLISHER_DOMAIN=bizbudding.com
PUBLISHER_ID=NXS001
```

- [ ] **Step 3: Commit**

```bash
git add config/bizbudding.yaml .env.example
git commit -m "chore: add Bizbudding config and .env.example"
```

---

### Task 3: Update sellers.json

**Files:**
- Modify: `assets/sellers.json`

- [ ] **Step 1: Replace with Bizbudding-only IAB-compliant content**

Replace the entire contents of `assets/sellers.json` with:

```json
{
  "contact_email": "ops@bizbudding.com",
  "contact_address": "Bizbudding Inc, [ADDRESS]",
  "version": "1.0",
  "identifiers": [],
  "sellers": [
    {
      "seller_id": "NXS001",
      "name": "Bizbudding",
      "domain": "bizbudding.com",
      "seller_type": "PUBLISHER"
    }
  ]
}
```

Note: `[ADDRESS]` is a placeholder — fill in Bizbudding's legal address before deploying. The `identifiers` array is empty until Bizbudding registers a TCF GVLID.

- [ ] **Step 2: Verify the existing handler still serves it correctly**

```bash
go run cmd/server/main.go &
sleep 2
curl -s http://localhost:8080/sellers.json | python3 -m json.tool
kill %1
```

Expected: valid JSON with single seller `NXS001`, domain `bizbudding.com`.

- [ ] **Step 3: Commit**

```bash
git add assets/sellers.json
git commit -m "feat: update sellers.json for Bizbudding — single publisher NXS001"
```

---

### Task 4: Create ssp_configs migration

**Files:**
- Create: `migrations/008_ssp_configs.sql`

- [ ] **Step 1: Create the migration**

```sql
-- 008_ssp_configs.sql
-- ssp_configs table: per-SSP global settings used by ads.txt generation and admin UI
-- Phase 2 SSPs (axonix, oms) seeded here but marked inactive until adapters are built

BEGIN;

CREATE TABLE IF NOT EXISTS ssp_configs (
  id          SERIAL PRIMARY KEY,
  ssp_name    TEXT NOT NULL UNIQUE,
  active      BOOLEAN NOT NULL DEFAULT true,
  account_id  TEXT,
  cert_id     TEXT,
  floor_cpm   NUMERIC(10,4) DEFAULT 0,
  params      JSONB,
  created_at  TIMESTAMPTZ DEFAULT NOW(),
  updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Phase 1: default active SSPs with IAB cert IDs
INSERT INTO ssp_configs (ssp_name, active, account_id, cert_id) VALUES
  ('rubicon',    true,  NULL, '0bfd66d529a55807'),
  ('kargo',      true,  NULL, NULL),
  ('sovrn',      true,  NULL, 'fafdf38b16bf6b2b'),
  ('pubmatic',   true,  NULL, '5d62403b186f2ace'),
  ('triplelift', true,  NULL, '6c33edb13117fd86')
ON CONFLICT (ssp_name) DO NOTHING;

-- Phase 2: inactive until adapters are built
INSERT INTO ssp_configs (ssp_name, active, account_id, cert_id) VALUES
  ('axonix', false, '4311ace9-d8cd-437e-bf21-0bae2d463eb0', NULL),
  ('oms',    false, '21146', NULL)
ON CONFLICT (ssp_name) DO NOTHING;

COMMIT;
```

- [ ] **Step 2: Verify SQL syntax**

```bash
psql $DATABASE_URL -f migrations/008_ssp_configs.sql
psql $DATABASE_URL -c "SELECT ssp_name, active, cert_id FROM ssp_configs ORDER BY ssp_name;"
```

Expected: 7 rows — 5 active, 2 inactive.

- [ ] **Step 3: Commit**

```bash
git add migrations/008_ssp_configs.sql
git commit -m "feat(migrations): 008 — ssp_configs table with Phase 1 SSPs seeded"
```

---

### Task 5: Build ads.txt endpoint

**Files:**
- Create: `internal/endpoints/adstxt_handler.go`
- Create: `internal/endpoints/adstxt_handler_test.go`
- Modify: `cmd/server/server.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/endpoints/adstxt_handler_test.go`:

```go
package endpoints_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/endpoints"
)

// stubSSPQuerier implements endpoints.SSPQuerier for tests
type stubSSPQuerier struct {
	rows []endpoints.SSPConfig
	err  error
}

func (s *stubSSPQuerier) ActiveSSPs() ([]endpoints.SSPConfig, error) {
	return s.rows, s.err
}

func TestHandleAdsTxt_ReturnsCorrectLines(t *testing.T) {
	querier := &stubSSPQuerier{rows: []endpoints.SSPConfig{
		{SSPName: "rubicon",    AccountID: sql.NullString{String: "26298", Valid: true},  CertID: sql.NullString{String: "0bfd66d529a55807", Valid: true}},
		{SSPName: "kargo",      AccountID: sql.NullString{String: "kargo-acct", Valid: true}, CertID: sql.NullString{Valid: false}},
		{SSPName: "sovrn",      AccountID: sql.NullString{String: "sovrn-acct", Valid: true}, CertID: sql.NullString{String: "fafdf38b16bf6b2b", Valid: true}},
	}}

	handler := endpoints.NewAdsTxtHandler("ads.bizbudding.com", querier)
	req := httptest.NewRequest(http.MethodGet, "/ads.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if ct := rr.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("wrong Content-Type: %q", ct)
	}
	if !strings.Contains(body, "ads.bizbudding.com, 26298, DIRECT, 0bfd66d529a55807") {
		t.Errorf("missing rubicon line, got:\n%s", body)
	}
	if !strings.Contains(body, "ads.bizbudding.com, kargo-acct, DIRECT\n") {
		t.Errorf("missing kargo line (no cert), got:\n%s", body)
	}
	if !strings.Contains(body, "ads.bizbudding.com, sovrn-acct, DIRECT, fafdf38b16bf6b2b") {
		t.Errorf("missing sovrn line, got:\n%s", body)
	}
}

func TestHandleAdsTxt_EmptySSPs(t *testing.T) {
	querier := &stubSSPQuerier{rows: []endpoints.SSPConfig{}}
	handler := endpoints.NewAdsTxtHandler("ads.bizbudding.com", querier)
	req := httptest.NewRequest(http.MethodGet, "/ads.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Errorf("expected empty body, got: %q", rr.Body.String())
	}
}

func TestHandleAdsTxt_DBError_Returns500(t *testing.T) {
	querier := &stubSSPQuerier{err: sql.ErrNoRows}
	handler := endpoints.NewAdsTxtHandler("ads.bizbudding.com", querier)
	req := httptest.NewRequest(http.MethodGet, "/ads.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleAdsTxt_MethodNotAllowed(t *testing.T) {
	querier := &stubSSPQuerier{}
	handler := endpoints.NewAdsTxtHandler("ads.bizbudding.com", querier)
	req := httptest.NewRequest(http.MethodPost, "/ads.txt", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
go test ./internal/endpoints/... -run TestHandleAdsTxt -v
```

Expected: compile error — `endpoints.SSPQuerier`, `endpoints.SSPConfig`, `endpoints.NewAdsTxtHandler` undefined.

- [ ] **Step 3: Implement the handler**

Create `internal/endpoints/adstxt_handler.go`:

```go
package endpoints

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// SSPConfig holds the ads.txt-relevant fields for one SSP from the ssp_configs table.
type SSPConfig struct {
	SSPName   string
	AccountID sql.NullString
	CertID    sql.NullString
}

// SSPQuerier is the interface the ads.txt handler uses to fetch active SSPs.
// The production implementation queries PostgreSQL; tests use a stub.
type SSPQuerier interface {
	ActiveSSPs() ([]SSPConfig, error)
}

// AdsTxtHandler generates a valid IAB ads.txt file from the ssp_configs table.
type AdsTxtHandler struct {
	domain  string
	querier SSPQuerier
}

// NewAdsTxtHandler returns an http.Handler that serves ads.txt for the given domain.
func NewAdsTxtHandler(domain string, querier SSPQuerier) http.Handler {
	return &AdsTxtHandler{domain: domain, querier: querier}
}

func (h *AdsTxtHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ssps, err := h.querier.ActiveSSPs()
	if err != nil {
		logger.Log.Error().Err(err).Msg("ads.txt: failed to query ssp_configs")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")

	var sb strings.Builder
	for _, ssp := range ssps {
		if !ssp.AccountID.Valid || ssp.AccountID.String == "" {
			continue
		}
		if ssp.CertID.Valid && ssp.CertID.String != "" {
			fmt.Fprintf(&sb, "%s, %s, DIRECT, %s\n", h.domain, ssp.AccountID.String, ssp.CertID.String)
		} else {
			fmt.Fprintf(&sb, "%s, %s, DIRECT\n", h.domain, ssp.AccountID.String)
		}
	}
	w.Write([]byte(sb.String()))
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/endpoints/... -run TestHandleAdsTxt -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Create the PostgreSQL implementation of SSPQuerier**

Add to `internal/storage/` — create `internal/storage/ssp_configs.go`:

```go
package storage

import (
	"database/sql"

	"github.com/thenexusengine/tne_springwire/internal/endpoints"
)

// SSPConfigStore queries ssp_configs from PostgreSQL.
type SSPConfigStore struct {
	db *sql.DB
}

func NewSSPConfigStore(db *sql.DB) *SSPConfigStore {
	return &SSPConfigStore{db: db}
}

// ActiveSSPs returns all SSPs where active = true, ordered by ssp_name.
func (s *SSPConfigStore) ActiveSSPs() ([]endpoints.SSPConfig, error) {
	rows, err := s.db.Query(`
		SELECT ssp_name, account_id, cert_id
		FROM ssp_configs
		WHERE active = true
		ORDER BY ssp_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []endpoints.SSPConfig
	for rows.Next() {
		var c endpoints.SSPConfig
		if err := rows.Scan(&c.SSPName, &c.AccountID, &c.CertID); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}
```

- [ ] **Step 6: Persist raw DB connection on Server struct, then register route**

The `Server` struct (line 43 of `cmd/server/server.go`) stores `db *storage.BidderStore`, not a `*sql.DB`. The raw `*sql.DB` (named `dbConn`) is a local variable inside `initDatabase()` and goes out of scope. Fix in two parts:

**Part A — Add `rawDB` field to `Server`:**

In `cmd/server/server.go`, add `rawDB` to the `Server` struct:

```go
type Server struct {
    // ... existing fields ...
    db     *storage.BidderStore
    rawDB  *sql.DB   // ← add this line
    // ... rest of fields ...
}
```

In `initDatabase()`, store `dbConn` on the struct (after line 138):

```go
s.db = storage.NewBidderStore(dbConn)
s.rawDB = dbConn   // ← add this line
```

**Part B — Register the ads.txt route:**

Find the block where `sellers.json` is registered (around line 371):

```go
mux.HandleFunc("/sellers.json", endpoints.HandleSellersJSON)
mux.HandleFunc("/.well-known/sellers.json", endpoints.HandleSellersJSON)
```

Add immediately after:

```go
adsTxtHandler := endpoints.NewAdsTxtHandler(
    getEnvOrDefault("ADS_TXT_DOMAIN", "ads.bizbudding.com"),
    storage.NewSSPConfigStore(s.rawDB),
)
mux.Handle("/ads.txt", adsTxtHandler)
```

Add `getEnvOrDefault` helper anywhere in `cmd/server/server.go` if it doesn't already exist:

```go
func getEnvOrDefault(key, defaultVal string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return defaultVal
}
```

Add `ADS_TXT_DOMAIN=ads.bizbudding.com` to `.env.example`.

- [ ] **Step 7: Build to confirm no compile errors**

```bash
make build
```

Expected: `bin/catalyst-bizbudding` produced with no errors.

- [ ] **Step 8: Run full test suite**

```bash
go test -v -race ./...
```

Expected: all tests pass, no race conditions.

- [ ] **Step 9: Commit**

```bash
git add internal/endpoints/adstxt_handler.go internal/endpoints/adstxt_handler_test.go \
        internal/storage/ssp_configs.go cmd/server/server.go .env.example
git commit -m "feat: GET /ads.txt endpoint — auto-generated from ssp_configs table"
```

---

## Chunk 2: Admin API

### File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/admin/handler.go` | Create | Registers all admin API routes + serves embedded static files |
| `internal/admin/api_sites.go` | Create | GET/POST `/admin/api/sites` |
| `internal/admin/api_placements.go` | Create | GET/POST/PUT/DELETE `/admin/api/placements` |
| `internal/admin/api_ssp_configs.go` | Create | GET/PUT `/admin/api/ssp-configs` |
| `internal/admin/api_dashboard.go` | Create | GET `/admin/api/dashboard` |
| `internal/admin/api_sites_test.go` | Create | Tests for sites API |
| `internal/admin/api_placements_test.go` | Create | Tests for placements API |
| `internal/admin/api_ssp_configs_test.go` | Create | Tests for ssp-configs API |
| `internal/admin/api_dashboard_test.go` | Create | Tests for dashboard API |
| `cmd/server/server.go` | Modify | Register admin handler |

---

### Task 6: Admin package scaffold + shared types

**Files:**
- Create: `internal/admin/handler.go`

- [ ] **Step 1: Create the package with shared response helpers**

Create `internal/admin/handler.go`:

```go
package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// Handler holds the DB connection and serves admin API routes + static UI.
type Handler struct {
	db *sql.DB
}

// New returns a new admin Handler.
func New(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes registers all admin API routes onto the given mux.
// All routes are under /admin/api/ and are automatically protected by
// the existing AdminAuth middleware (bearer token, ADMIN_API_KEY env var).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/api/sites", h.handleSites)
	mux.HandleFunc("/admin/api/placements", h.handlePlacements)
	mux.HandleFunc("/admin/api/placements/", h.handlePlacementByID)
	mux.HandleFunc("/admin/api/ssp-configs", h.handleSSPConfigs)
	mux.HandleFunc("/admin/api/ssp-configs/", h.handleSSPConfigByName)
	mux.HandleFunc("/admin/api/dashboard", h.handleDashboard)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 2: Verify the package structure is accepted by the toolchain**

```bash
go vet ./internal/admin/...
```

Expected: compile error — `h.handleSites`, `h.handlePlacements`, etc. are undefined. This is expected — they are implemented in Tasks 7–10. The build will succeed only after all handler files are added. The purpose of this step is to confirm the file was created and the package declaration is valid.

---

### Task 7: Sites API

**Files:**
- Create: `internal/admin/api_sites.go`
- Create: `internal/admin/api_sites_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/admin/api_sites_test.go`:

```go
package admin_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/lib/pq"
	"github.com/thenexusengine/tne_springwire/internal/admin"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Integration tests require DB_TEST_URL env var.
	// Unit tests use nil db and stub the queries.
	return nil
}

func TestHandleSites_MethodNotAllowed(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/sites", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleSites_Post_InvalidJSON(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/sites", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSites_Post_MissingDomain(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{"name": "Test Site"})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/sites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] == "" {
		t.Error("expected error message in response body")
	}
}
```

- [ ] **Step 2: Run tests — confirm they fail**

```bash
go test ./internal/admin/... -run TestHandleSites -v
```

Expected: compile error — `handleSites` undefined.

- [ ] **Step 3: Implement sites API**

Create `internal/admin/api_sites.go`:

```go
package admin

import (
	"encoding/json"
	"net/http"
	"strings"
)

type site struct {
	ID     int    `json:"id"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type createSiteRequest struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

func (h *Handler) handleSites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSites(w, r)
	case http.MethodPost:
		h.createSite(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listSites(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT p.id, p.domain, p.name, p.status
		FROM publishers_new p
		JOIN accounts a ON p.account_id = a.id
		WHERE p.status = 'active'
		  AND a.account_id = '12345'
		ORDER BY p.domain
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	var sites []site
	for rows.Next() {
		var s site
		if err := rows.Scan(&s.ID, &s.Domain, &s.Name, &s.Status); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		sites = append(sites, s)
	}
	if sites == nil {
		sites = []site{}
	}
	writeJSON(w, http.StatusOK, sites)
}

func (h *Handler) createSite(w http.ResponseWriter, r *http.Request) {
	var req createSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Domain = strings.TrimSpace(req.Domain)
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}
	if req.Name == "" {
		req.Name = req.Domain
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database not available")
		return
	}

	var id int
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO publishers_new (account_id, domain, name, status)
		VALUES ((SELECT id FROM accounts WHERE account_id = '12345'), $1, $2, 'active')
		RETURNING id
	`, req.Domain, req.Name).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create site")
		return
	}
	writeJSON(w, http.StatusCreated, site{ID: id, Domain: req.Domain, Name: req.Name, Status: "active"})
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/admin/... -run TestHandleSites -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/admin/handler.go internal/admin/api_sites.go internal/admin/api_sites_test.go
git commit -m "feat(admin): sites API — GET/POST /admin/api/sites"
```

---

### Task 8: Placements API

**Files:**
- Create: `internal/admin/api_placements.go`
- Create: `internal/admin/api_placements_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/admin/api_placements_test.go`:

```go
package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/admin"
)

func TestHandlePlacements_MethodNotAllowed(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPatch, "/admin/api/placements", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandlePlacements_Post_MissingSlotPattern(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]interface{}{"site_id": 1})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/placements", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandlePlacementByID_InvalidID(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPut, "/admin/api/placements/notanumber", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandlePlacementByID_MethodNotAllowed(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/placements/123", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests — confirm compile error**

```bash
go test ./internal/admin/... -run TestHandlePlacement -v
```

Expected: compile error — `handlePlacements`, `handlePlacementByID` undefined.

- [ ] **Step 3: Implement placements API**

Create `internal/admin/api_placements.go`:

```go
package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type placement struct {
	ID           int             `json:"id"`
	SiteID       int             `json:"site_id"`
	SlotPattern  string          `json:"slot_pattern"`
	SlotName     string          `json:"slot_name"`
	IsAdhesion   bool            `json:"is_adhesion"`
	Status       string          `json:"status"`
	BidderParams json.RawMessage `json:"bidder_params,omitempty"`
}

type createPlacementRequest struct {
	SiteID      int    `json:"site_id"`
	SlotPattern string `json:"slot_pattern"`
	SlotName    string `json:"slot_name"`
	IsAdhesion  bool   `json:"is_adhesion"`
}

type updatePlacementRequest struct {
	Status      string          `json:"status"`
	BidderCode  string          `json:"bidder_code"`
	DeviceType  string          `json:"device_type"`
	BidderParams json.RawMessage `json:"bidder_params"`
}

func (h *Handler) handlePlacements(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPlacements(w, r)
	case http.MethodPost:
		h.createPlacement(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handlePlacementByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/admin/api/placements/")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid placement id")
		return
	}
	switch r.Method {
	case http.MethodPut:
		h.updatePlacement(w, r, id)
	case http.MethodDelete:
		h.deletePlacement(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listPlacements(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, []placement{})
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT a.id, a.publisher_id, a.slot_pattern, a.slot_name, a.is_adhesion, a.status
		FROM ad_slots a
		JOIN publishers_new p ON a.publisher_id = p.id
		WHERE p.status = 'active'
		ORDER BY a.slot_pattern
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var placements []placement
	for rows.Next() {
		var p placement
		if err := rows.Scan(&p.ID, &p.SiteID, &p.SlotPattern, &p.SlotName, &p.IsAdhesion, &p.Status); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		placements = append(placements, p)
	}
	if placements == nil {
		placements = []placement{}
	}
	writeJSON(w, http.StatusOK, placements)
}

func (h *Handler) createPlacement(w http.ResponseWriter, r *http.Request) {
	var req createPlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.SlotPattern) == "" {
		writeError(w, http.StatusBadRequest, "slot_pattern is required")
		return
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database not available")
		return
	}
	if req.SlotName == "" {
		req.SlotName = req.SlotPattern
	}
	var id int
	err := h.db.QueryRowContext(r.Context(), `
		INSERT INTO ad_slots (publisher_id, slot_pattern, slot_name, is_adhesion, status)
		VALUES ($1, $2, $3, $4, 'active')
		RETURNING id
	`, req.SiteID, req.SlotPattern, req.SlotName, req.IsAdhesion).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create placement")
		return
	}
	writeJSON(w, http.StatusCreated, placement{ID: id, SiteID: req.SiteID, SlotPattern: req.SlotPattern, SlotName: req.SlotName, IsAdhesion: req.IsAdhesion, Status: "active"})
}

func (h *Handler) updatePlacement(w http.ResponseWriter, r *http.Request, id int) {
	var req updatePlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database not available")
		return
	}
	// Status toggle: update ad_slot status if provided (independent of bidder config path)
	if req.Status != "" {
		if _, err := h.db.ExecContext(r.Context(),
			`UPDATE ad_slots SET status = $1 WHERE id = $2`, req.Status, id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update status")
			return
		}
	}
	// Bidder config update: upsert slot_bidder_configs for one SSP.
	// Defaults status to 'active' when not provided to satisfy the NOT NULL constraint.
	if req.BidderCode != "" && req.DeviceType != "" && req.BidderParams != nil {
		bidderStatus := req.Status
		if bidderStatus == "" {
			bidderStatus = "active"
		}
		_, err := h.db.ExecContext(r.Context(), `
			INSERT INTO slot_bidder_configs (ad_slot_id, bidder_id, device_type, bidder_params, status)
			SELECT $1, b.id, $2, $3, $4
			FROM bidders_new b WHERE b.code = $5
			ON CONFLICT (ad_slot_id, bidder_id, device_type)
			DO UPDATE SET bidder_params = EXCLUDED.bidder_params, status = EXCLUDED.status, updated_at = NOW()
		`, id, req.DeviceType, req.BidderParams, bidderStatus, req.BidderCode)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update bidder config")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "updated": true})
}

func (h *Handler) deletePlacement(w http.ResponseWriter, r *http.Request, id int) {
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database not available")
		return
	}
	_, err := h.db.ExecContext(r.Context(),
		`UPDATE ad_slots SET status = 'archived' WHERE id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete placement")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/admin/... -run TestHandlePlacement -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/admin/api_placements.go internal/admin/api_placements_test.go
git commit -m "feat(admin): placements API — GET/POST /admin/api/placements, PUT/DELETE /admin/api/placements/:id"
```

---

### Task 9: SSP Config API

**Files:**
- Create: `internal/admin/api_ssp_configs.go`
- Create: `internal/admin/api_ssp_configs_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/admin/api_ssp_configs_test.go`:

```go
package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/admin"
)

func TestHandleSSPConfigs_MethodNotAllowed(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/ssp-configs", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleSSPConfigByName_EmptyName(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPut, "/admin/api/ssp-configs/", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSSPConfigByName_InvalidJSON(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPut, "/admin/api/ssp-configs/rubicon", bytes.NewBufferString(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleSSPConfigByName_GetMethod_NotAllowed(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/ssp-configs/rubicon", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests — confirm compile error**

```bash
go test ./internal/admin/... -run TestHandleSSP -v
```

- [ ] **Step 3: Implement SSP config API**

Create `internal/admin/api_ssp_configs.go`:

```go
package admin

import (
	"encoding/json"
	"net/http"
	"strings"
)

type sspConfig struct {
	SSPName   string   `json:"ssp_name"`
	Active    bool     `json:"active"`
	AccountID *string  `json:"account_id,omitempty"`
	CertID    *string  `json:"cert_id,omitempty"`
	FloorCPM  float64  `json:"floor_cpm"`
}

type updateSSPRequest struct {
	Active   *bool    `json:"active"`
	FloorCPM *float64 `json:"floor_cpm"`
	AccountID *string `json:"account_id"`
}

func (h *Handler) handleSSPConfigs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.db == nil {
		writeJSON(w, http.StatusOK, []sspConfig{})
		return
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT ssp_name, active, account_id, cert_id, floor_cpm FROM ssp_configs ORDER BY ssp_name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()
	var configs []sspConfig
	for rows.Next() {
		var c sspConfig
		var accountID, certID *string
		if err := rows.Scan(&c.SSPName, &c.Active, &accountID, &certID, &c.FloorCPM); err != nil {
			writeError(w, http.StatusInternalServerError, "scan error")
			return
		}
		c.AccountID = accountID
		c.CertID = certID
		configs = append(configs, c)
	}
	if configs == nil {
		configs = []sspConfig{}
	}
	writeJSON(w, http.StatusOK, configs)
}

func (h *Handler) handleSSPConfigByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/admin/api/ssp-configs/")
	if name == "" {
		writeError(w, http.StatusBadRequest, "ssp name is required")
		return
	}
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req updateSSPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if h.db == nil {
		writeError(w, http.StatusInternalServerError, "database not available")
		return
	}
	if req.Active != nil {
		if _, err := h.db.ExecContext(r.Context(),
			`UPDATE ssp_configs SET active = $1, updated_at = NOW() WHERE ssp_name = $2`,
			*req.Active, name); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update")
			return
		}
	}
	if req.FloorCPM != nil {
		if _, err := h.db.ExecContext(r.Context(),
			`UPDATE ssp_configs SET floor_cpm = $1, updated_at = NOW() WHERE ssp_name = $2`,
			*req.FloorCPM, name); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update floor")
			return
		}
	}
	if req.AccountID != nil {
		if _, err := h.db.ExecContext(r.Context(),
			`UPDATE ssp_configs SET account_id = $1, updated_at = NOW() WHERE ssp_name = $2`,
			*req.AccountID, name); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update account_id")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ssp_name": name, "updated": true})
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/admin/... -run TestHandleSSP -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/admin/api_ssp_configs.go internal/admin/api_ssp_configs_test.go
git commit -m "feat(admin): SSP config API — GET /admin/api/ssp-configs, PUT /admin/api/ssp-configs/:name"
```

---

### Task 10: Dashboard API

**Files:**
- Create: `internal/admin/api_dashboard.go`
- Create: `internal/admin/api_dashboard_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/admin/api_dashboard_test.go`:

```go
package admin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thenexusengine/tne_springwire/internal/admin"
)

func TestHandleDashboard_Get(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/dashboard", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// With nil DB, expect 200 with zeroed stats (graceful degradation)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	for _, key := range []string{"revenue_today", "fill_rate", "impressions_today", "active_ssps"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing key %q in dashboard response", key)
		}
	}
}

func TestHandleDashboard_MethodNotAllowed(t *testing.T) {
	h := admin.New(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/dashboard", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run tests — confirm compile error**

```bash
go test ./internal/admin/... -run TestHandleDashboard -v
```

- [ ] **Step 3: Implement dashboard API**

Create `internal/admin/api_dashboard.go`:

```go
package admin

import (
	"net/http"
)

type dashboardStats struct {
	RevenueToday     float64 `json:"revenue_today"`
	FillRate         float64 `json:"fill_rate"`
	ImpressionsToday int64   `json:"impressions_today"`
	ActiveSSPs       int     `json:"active_ssps"`
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := dashboardStats{}

	if h.db != nil {
		// Revenue and impressions from win_events (today UTC)
		h.db.QueryRowContext(r.Context(), `
			SELECT
				COALESCE(SUM(platform_cut), 0),
				COUNT(*)
			FROM win_events
			WHERE created_at >= CURRENT_DATE
		`).Scan(&stats.RevenueToday, &stats.ImpressionsToday)

		// Fill rate: wins / total auctions today
		var totalAuctions int64
		h.db.QueryRowContext(r.Context(), `
			SELECT COUNT(*) FROM auction_events WHERE created_at >= CURRENT_DATE
		`).Scan(&totalAuctions)
		if totalAuctions > 0 {
			stats.FillRate = float64(stats.ImpressionsToday) / float64(totalAuctions)
		}

		// Active SSP count
		h.db.QueryRowContext(r.Context(), `
			SELECT COUNT(*) FROM ssp_configs WHERE active = true
		`).Scan(&stats.ActiveSSPs)
	}

	writeJSON(w, http.StatusOK, stats)
}
```

- [ ] **Step 4: Run tests — confirm they pass**

```bash
go test ./internal/admin/... -run TestHandleDashboard -v
```

- [ ] **Step 5: Register admin routes in server.go**

In `cmd/server/server.go`, find the admin route block (around line 380) and add after the existing admin routes:

```go
adminHandler := admin.New(s.rawDB)
adminHandler.RegisterRoutes(mux)
```

Add import: `"github.com/thenexusengine/tne_springwire/internal/admin"`

- [ ] **Step 6: Build and run full test suite**

```bash
make build
go test -v -race ./...
```

Expected: binary builds, all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/admin/api_dashboard.go internal/admin/api_dashboard_test.go cmd/server/server.go
git commit -m "feat(admin): dashboard API + wire all admin routes into server"
```

---

## Chunk 3: Admin UI (Embedded Vanilla JS SPA)

### File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/admin/static/index.html` | Create | SPA shell — sidebar nav, content area |
| `internal/admin/static/style.css` | Create | Dark theme, sidebar, cards, form styles |
| `internal/admin/static/app.js` | Create | All views: dashboard, placements, SSP config, ads.txt |
| `internal/admin/embed.go` | Create | `embed.FS` declaration + static file handler |
| `internal/admin/handler.go` | Modify | Register static file serving at `/admin` |

---

### Task 11: Embed static files into the binary

**Files:**
- Create: `internal/admin/embed.go`
- Modify: `internal/admin/handler.go`

- [ ] **Step 1: Create embed.go**

```go
package admin

import "embed"

//go:embed static
var staticFiles embed.FS
```

- [ ] **Step 2: Add static file serving to RegisterRoutes**

In `internal/admin/handler.go`, update the imports block and add to `RegisterRoutes`:

```go
import (
    "database/sql"
    "encoding/json"
    "io/fs"
    "net/http"
    "strings"
)
```

Add to `RegisterRoutes`:

```go
// Serve the embedded SPA at /admin and /admin/
// fs.Sub strips the "static/" prefix so http.FileServer sees index.html at "/"
subFS, err := fs.Sub(staticFiles, "static")
if err != nil {
    panic("admin: failed to sub static FS: " + err.Error())
}
staticHandler := http.FileServer(http.FS(subFS))
mux.Handle("/admin/", http.StripPrefix("/admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // All non-API paths serve static files; SPA handles its own routing
    if strings.HasPrefix(r.URL.Path, "/api/") {
        http.NotFound(w, r)
        return
    }
    // Serve static assets (css, js) directly; anything without an extension → index.html
    if r.URL.Path == "/" || !strings.Contains(r.URL.Path, ".") {
        r.URL.Path = "/"
    }
    staticHandler.ServeHTTP(w, r)
})))
```

- [ ] **Step 3: Create placeholder static files so the build compiles**

```bash
mkdir -p internal/admin/static
touch internal/admin/static/index.html
touch internal/admin/static/style.css
touch internal/admin/static/app.js
```

- [ ] **Step 4: Build to confirm embed compiles**

```bash
make build
```

Expected: binary builds. Visiting `http://localhost:8080/admin/` returns an empty page.

- [ ] **Step 5: Commit**

```bash
git add internal/admin/embed.go internal/admin/handler.go internal/admin/static/
git commit -m "feat(admin): embed static files into binary via embed.FS"
```

---

### Task 12: Admin UI — HTML shell + CSS

**Files:**
- Modify: `internal/admin/static/index.html`
- Modify: `internal/admin/static/style.css`

- [ ] **Step 1: Write index.html**

Replace `internal/admin/static/index.html` with:

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Bizbudding Ad Manager</title>
  <link rel="stylesheet" href="/admin/style.css">
</head>
<body>
  <div class="layout">
    <nav class="sidebar">
      <div class="sidebar-brand">Bizbudding</div>
      <ul class="sidebar-nav">
        <li><a href="#dashboard" class="nav-link active" data-view="dashboard">📊 Dashboard</a></li>
        <li><a href="#sites" class="nav-link" data-view="sites">🌐 Sites</a></li>
        <li><a href="#placements" class="nav-link" data-view="placements">📦 Placements</a></li>
        <li><a href="#ssp-config" class="nav-link" data-view="ssp-config">🔌 SSP Config</a></li>
        <li><a href="#video" class="nav-link" data-view="video">📺 Video Tags</a></li>
        <li><a href="#adstxt" class="nav-link" data-view="adstxt">📄 ads.txt</a></li>
      </ul>
      <div class="sidebar-footer">
        <a href="#settings" class="nav-link" data-view="settings">⚙️ Settings</a>
      </div>
    </nav>
    <main class="content" id="main-content">
      <div id="view-container"></div>
    </main>
  </div>
  <script src="/admin/app.js"></script>
</body>
</html>
```

- [ ] **Step 2: Write style.css**

Replace `internal/admin/static/style.css` with:

```css
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg: #0f172a; --surface: #1e293b; --surface2: #111827;
  --border: #334155; --text: #e2e8f0; --muted: #94a3b8;
  --accent: #3b82f6; --green: #4ade80; --yellow: #fbbf24; --red: #f87171;
  --sidebar-w: 200px;
}

body { background: var(--bg); color: var(--text); font-family: system-ui, sans-serif; font-size: 14px; }

.layout { display: flex; min-height: 100vh; }

.sidebar {
  width: var(--sidebar-w); background: var(--surface2); border-right: 1px solid var(--border);
  display: flex; flex-direction: column; padding: 16px 0; flex-shrink: 0; position: fixed; height: 100vh;
}
.sidebar-brand { color: white; font-weight: 700; font-size: 15px; padding: 0 16px 20px; }
.sidebar-nav { list-style: none; flex: 1; }
.sidebar-footer { padding: 16px 0 0; border-top: 1px solid var(--border); }
.nav-link {
  display: block; padding: 8px 16px; color: var(--muted); text-decoration: none;
  border-radius: 4px; margin: 1px 8px; transition: background .15s;
}
.nav-link:hover { background: var(--surface); color: var(--text); }
.nav-link.active { background: var(--accent); color: white; }

.content { margin-left: var(--sidebar-w); padding: 24px; flex: 1; }

h1 { font-size: 20px; font-weight: 600; margin-bottom: 20px; }
h2 { font-size: 16px; font-weight: 600; margin-bottom: 12px; }

.card { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 16px; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 20px; }
.stat { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 16px; }
.stat-value { font-size: 22px; font-weight: 700; color: var(--green); }
.stat-label { color: var(--muted); font-size: 12px; margin-top: 4px; }

table { width: 100%; border-collapse: collapse; }
th { text-align: left; color: var(--muted); font-size: 11px; text-transform: uppercase; padding: 8px 12px; border-bottom: 1px solid var(--border); }
td { padding: 10px 12px; border-bottom: 1px solid var(--border); }
tr:hover td { background: rgba(255,255,255,.03); }

.badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
.badge-active { background: rgba(74,222,128,.15); color: var(--green); }
.badge-paused { background: rgba(251,191,36,.15); color: var(--yellow); }

.btn { padding: 8px 16px; border-radius: 6px; border: none; cursor: pointer; font-size: 13px; font-weight: 600; }
.btn-primary { background: var(--accent); color: white; }
.btn-secondary { background: var(--surface); border: 1px solid var(--border); color: var(--text); }
.btn-danger { background: rgba(248,113,113,.15); color: var(--red); border: 1px solid rgba(248,113,113,.3); }

input, select { background: var(--surface); border: 1px solid var(--border); color: var(--text); padding: 8px 10px; border-radius: 6px; font-size: 13px; width: 100%; }
input:focus, select:focus { outline: none; border-color: var(--accent); }
label { display: block; color: var(--muted); font-size: 11px; font-weight: 600; text-transform: uppercase; margin-bottom: 4px; }
.form-group { margin-bottom: 16px; }
.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }

.toggle { width: 36px; height: 20px; border-radius: 10px; border: none; cursor: pointer; position: relative; transition: background .2s; }
.toggle.on { background: var(--accent); }
.toggle.off { background: var(--border); }

.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { background: var(--surface); border: 1px solid var(--border); border-radius: 8px; padding: 24px; width: 560px; max-height: 80vh; overflow-y: auto; }
.modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.modal-close { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 18px; }

.ssp-row { border: 1px solid var(--border); border-radius: 6px; margin-bottom: 8px; overflow: hidden; }
.ssp-row-header { display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; }
.ssp-row-params { padding: 10px 12px; border-top: 1px solid var(--border); background: var(--surface2); }
.ssp-params-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }

pre { background: var(--surface2); border: 1px solid var(--border); border-radius: 6px; padding: 16px; font-size: 12px; color: var(--green); white-space: pre-wrap; }

.copy-btn { float: right; margin-top: -8px; }
.loading { color: var(--muted); padding: 40px; text-align: center; }
.error-msg { color: var(--red); background: rgba(248,113,113,.1); border: 1px solid rgba(248,113,113,.3); border-radius: 6px; padding: 10px 14px; margin-bottom: 12px; }
```

- [ ] **Step 3: Build to confirm static files embed correctly**

```bash
make build
```

- [ ] **Step 4: Smoke test in browser**

```bash
ADMIN_AUTH_REQUIRED=false ./bin/catalyst-bizbudding &
sleep 1
curl -s http://localhost:8080/admin/ | head -5
kill %1
```

Expected: HTML output starting with `<!DOCTYPE html>`.

- [ ] **Step 5: Commit**

```bash
git add internal/admin/static/index.html internal/admin/static/style.css
git commit -m "feat(admin): HTML shell and CSS theme for admin UI"
```

---

### Task 13: Admin UI — JavaScript SPA

**Files:**
- Modify: `internal/admin/static/app.js`

- [ ] **Step 1: Write app.js**

Replace `internal/admin/static/app.js` with:

```javascript
// Bizbudding Admin UI
// Single-file vanilla JS SPA. No build step, no dependencies.

const API = '/admin/api';

// ── Router ──────────────────────────────────────────────────────────────────
function router() {
  const hash = location.hash.replace('#', '') || 'dashboard';
  document.querySelectorAll('.nav-link').forEach(l => {
    l.classList.toggle('active', l.dataset.view === hash);
  });
  const views = { dashboard, sites, placements, sspConfig, videoTags, adsTxt, settings };
  const fn = views[hash.replace('-', '')] || dashboard;
  fn();
}
window.addEventListener('hashchange', router);
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('.nav-link').forEach(l => {
    l.addEventListener('click', e => {
      document.querySelectorAll('.nav-link').forEach(x => x.classList.remove('active'));
      l.classList.add('active');
    });
  });
  router();
});

// ── API helpers ──────────────────────────────────────────────────────────────
async function api(path, opts = {}) {
  const r = await fetch(API + path, {
    headers: { 'Content-Type': 'application/json', ...opts.headers },
    ...opts,
  });
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  if (r.status === 204) return null;
  return r.json();
}

function render(html) {
  document.getElementById('view-container').innerHTML = html;
}

function badge(status) {
  return `<span class="badge badge-${status === 'active' ? 'active' : 'paused'}">${status}</span>`;
}

// ── Dashboard ────────────────────────────────────────────────────────────────
async function dashboard() {
  render(`<div class="loading">Loading...</div>`);
  try {
    const d = await api('/dashboard');
    render(`
      <h1>Dashboard</h1>
      <div class="stats-grid">
        <div class="stat"><div class="stat-value">$${(d.revenue_today || 0).toFixed(2)}</div><div class="stat-label">Revenue Today</div></div>
        <div class="stat"><div class="stat-value" style="color:var(--accent)">${((d.fill_rate || 0) * 100).toFixed(1)}%</div><div class="stat-label">Fill Rate</div></div>
        <div class="stat"><div class="stat-value" style="color:#c4b5fd">${(d.impressions_today || 0).toLocaleString()}</div><div class="stat-label">Impressions</div></div>
        <div class="stat"><div class="stat-value" style="color:var(--yellow)">${d.active_ssps || 0}</div><div class="stat-label">Active SSPs</div></div>
      </div>
    `);
  } catch (e) {
    render(`<h1>Dashboard</h1><div class="error-msg">Failed to load stats: ${e.message}</div>`);
  }
}

// ── Sites ────────────────────────────────────────────────────────────────────
async function sites() {
  render(`<div class="loading">Loading...</div>`);
  try {
    const data = await api('/sites');
    render(`
      <h1>Sites <button class="btn btn-primary" style="float:right;margin-top:-4px" onclick="showAddSite()">+ Add Site</button></h1>
      <div class="card">
        <table>
          <thead><tr><th>Domain</th><th>Name</th><th>Status</th></tr></thead>
          <tbody>
            ${(data || []).map(s => `<tr><td>${s.domain}</td><td>${s.name}</td><td>${badge(s.status)}</td></tr>`).join('') || '<tr><td colspan="3" style="color:var(--muted)">No sites yet</td></tr>'}
          </tbody>
        </table>
      </div>
    `);
  } catch (e) {
    render(`<h1>Sites</h1><div class="error-msg">${e.message}</div>`);
  }
}

window.showAddSite = function() {
  document.body.insertAdjacentHTML('beforeend', `
    <div class="modal-backdrop" id="site-modal">
      <div class="modal">
        <div class="modal-header"><h2>Add Site</h2><button class="modal-close" onclick="document.getElementById('site-modal').remove()">×</button></div>
        <div class="form-group"><label>Domain</label><input id="site-domain" placeholder="example.com"></div>
        <div class="form-group"><label>Name</label><input id="site-name" placeholder="Display name"></div>
        <div style="display:flex;gap:8px;justify-content:flex-end">
          <button class="btn btn-secondary" onclick="document.getElementById('site-modal').remove()">Cancel</button>
          <button class="btn btn-primary" onclick="submitAddSite()">Save</button>
        </div>
      </div>
    </div>
  `);
};

window.submitAddSite = async function() {
  const domain = document.getElementById('site-domain').value.trim();
  const name = document.getElementById('site-name').value.trim();
  if (!domain) return alert('Domain is required');
  try {
    await api('/sites', { method: 'POST', body: JSON.stringify({ domain, name }) });
    document.getElementById('site-modal').remove();
    sites();
  } catch (e) { alert('Failed: ' + e.message); }
};

// ── Placements ───────────────────────────────────────────────────────────────
async function placements() {
  render(`<div class="loading">Loading...</div>`);
  try {
    const [data, sitesData] = await Promise.all([api('/placements'), api('/sites')]);
    window._sites = sitesData || [];
    render(`
      <h1>Placements <button class="btn btn-primary" style="float:right;margin-top:-4px" onclick="showAddPlacement()">+ Add Placement</button></h1>
      <div class="card">
        <table>
          <thead><tr><th>Slot Pattern</th><th>Site</th><th>Type</th><th>Status</th><th></th></tr></thead>
          <tbody>
            ${(data || []).map(p => `
              <tr>
                <td>${p.slot_pattern}</td>
                <td>${(window._sites.find(s=>s.id===p.site_id)||{}).domain||'—'}</td>
                <td>${p.is_adhesion ? 'Adhesion' : 'Standard'}</td>
                <td>${badge(p.status)}</td>
                <td><button class="btn btn-secondary" style="padding:4px 10px;font-size:12px" onclick="showEditPlacement(${p.id}, '${p.slot_pattern}')">Edit</button></td>
              </tr>`).join('') || '<tr><td colspan="5" style="color:var(--muted)">No placements yet</td></tr>'}
          </tbody>
        </table>
      </div>
    `);
  } catch (e) {
    render(`<h1>Placements</h1><div class="error-msg">${e.message}</div>`);
  }
}

window.showAddPlacement = function() {
  const siteOptions = (window._sites||[]).map(s=>`<option value="${s.id}">${s.domain}</option>`).join('');
  document.body.insertAdjacentHTML('beforeend', `
    <div class="modal-backdrop" id="placement-modal">
      <div class="modal">
        <div class="modal-header"><h2>Add Placement</h2><button class="modal-close" onclick="document.getElementById('placement-modal').remove()">×</button></div>
        <div class="form-row">
          <div class="form-group"><label>Site</label><select id="p-site">${siteOptions}</select></div>
          <div class="form-group"><label>Slot Pattern</label><input id="p-pattern" placeholder="domain.com/slot-name"></div>
        </div>
        <div class="form-row">
          <div class="form-group"><label>Display Name</label><input id="p-name" placeholder="Billboard Wide"></div>
          <div class="form-group"><label>Type</label><select id="p-adhesion"><option value="false">Standard</option><option value="true">Adhesion</option></select></div>
        </div>
        <div style="display:flex;gap:8px;justify-content:flex-end;margin-top:8px">
          <button class="btn btn-secondary" onclick="document.getElementById('placement-modal').remove()">Cancel</button>
          <button class="btn btn-primary" onclick="submitAddPlacement()">Save</button>
        </div>
      </div>
    </div>
  `);
};

window.submitAddPlacement = async function() {
  const site_id = parseInt(document.getElementById('p-site').value);
  const slot_pattern = document.getElementById('p-pattern').value.trim();
  const slot_name = document.getElementById('p-name').value.trim();
  const is_adhesion = document.getElementById('p-adhesion').value === 'true';
  if (!slot_pattern) return alert('Slot pattern is required');
  try {
    await api('/placements', { method: 'POST', body: JSON.stringify({ site_id, slot_pattern, slot_name, is_adhesion }) });
    document.getElementById('placement-modal').remove();
    placements();
  } catch (e) { alert('Failed: ' + e.message); }
};

window.showEditPlacement = function(id, pattern) {
  alert(`Edit placement ${id} (${pattern}) — SSP param editor coming soon`);
};

// ── SSP Config ───────────────────────────────────────────────────────────────
async function sspConfig() {
  render(`<div class="loading">Loading...</div>`);
  try {
    const data = await api('/ssp-configs');
    render(`
      <h1>SSP Config</h1>
      <p style="color:var(--muted);margin-bottom:16px">Toggle SSPs on/off. Account IDs are used in the auto-generated ads.txt.</p>
      <div class="card">
        <table>
          <thead><tr><th>SSP</th><th>Active</th><th>Account ID</th><th>Floor CPM</th><th>Phase</th></tr></thead>
          <tbody>
            ${(data||[]).map(s => `
              <tr>
                <td><strong>${s.ssp_name}</strong></td>
                <td>
                  <button class="toggle ${s.active ? 'on' : 'off'}" onclick="toggleSSP('${s.ssp_name}', ${!s.active}, this)">${s.active ? 'ON' : 'OFF'}</button>
                </td>
                <td><input value="${s.account_id||''}" placeholder="Enter account ID" style="width:180px" onblur="updateSSPAccountID('${s.ssp_name}', this.value)"></td>
                <td><input type="number" value="${s.floor_cpm||0}" step="0.01" style="width:80px" onblur="updateSSPFloor('${s.ssp_name}', this.value)"></td>
                <td style="color:var(--muted)">${['axonix','oms'].includes(s.ssp_name) ? 'Phase 2' : 'Active'}</td>
              </tr>`).join('')}
          </tbody>
        </table>
      </div>
    `);
  } catch (e) {
    render(`<h1>SSP Config</h1><div class="error-msg">${e.message}</div>`);
  }
}

window.toggleSSP = async function(name, active, btn) {
  try {
    await api('/ssp-configs/' + name, { method: 'PUT', body: JSON.stringify({ active }) });
    btn.classList.toggle('on', active);
    btn.classList.toggle('off', !active);
    btn.textContent = active ? 'ON' : 'OFF';
  } catch (e) { alert('Failed: ' + e.message); }
};

window.updateSSPAccountID = async function(name, account_id) {
  try { await api('/ssp-configs/' + name, { method: 'PUT', body: JSON.stringify({ account_id }) }); }
  catch (e) { alert('Failed to save account ID: ' + e.message); }
};

window.updateSSPFloor = async function(name, floor_cpm) {
  try { await api('/ssp-configs/' + name, { method: 'PUT', body: JSON.stringify({ floor_cpm: parseFloat(floor_cpm) }) }); }
  catch (e) { alert('Failed to save floor: ' + e.message); }
};

// ── Video Tags ───────────────────────────────────────────────────────────────
function videoTags() {
  render(`
    <h1>Video Tags</h1>
    <div class="card">
      <h2>VAST Tag Builder</h2>
      <p style="color:var(--muted);margin-bottom:16px">Build a VAST tag URL for testing or deployment.</p>
      <div class="form-row">
        <div class="form-group"><label>Publisher ID</label><input id="vt-pubid" value="NXS001"></div>
        <div class="form-group"><label>Placement</label><input id="vt-placement" placeholder="bizbudding.com/video-preroll"></div>
      </div>
      <div class="form-row">
        <div class="form-group"><label>Width</label><input id="vt-w" value="640" type="number"></div>
        <div class="form-group"><label>Height</label><input id="vt-h" value="480" type="number"></div>
      </div>
      <button class="btn btn-primary" onclick="buildVASTTag()">Generate Tag</button>
      <div id="vt-result" style="margin-top:16px"></div>
    </div>
  `);
}

window.buildVASTTag = function() {
  const pubid = document.getElementById('vt-pubid').value;
  const placement = document.getElementById('vt-placement').value;
  const w = document.getElementById('vt-w').value;
  const h = document.getElementById('vt-h').value;
  const base = window.location.origin;
  const url = `${base}/video/vast?pub=${encodeURIComponent(pubid)}&placement=${encodeURIComponent(placement)}&w=${w}&h=${h}`;
  document.getElementById('vt-result').innerHTML = `
    <label>VAST Tag URL</label>
    <pre>${url}</pre>
    <button class="btn btn-secondary" onclick="navigator.clipboard.writeText('${url}').then(()=>alert('Copied!'))">Copy</button>
  `;
};

// ── ads.txt ──────────────────────────────────────────────────────────────────
async function adsTxt() {
  render(`<div class="loading">Loading...</div>`);
  try {
    const r = await fetch('/ads.txt');
    const text = await r.text();
    render(`
      <h1>ads.txt</h1>
      <p style="color:var(--muted);margin-bottom:16px">
        Paste this into <strong>bizbudding.com/ads.txt</strong>. It auto-updates when you change SSP configs.
      </p>
      <div class="card">
        <button class="btn btn-primary copy-btn" onclick="navigator.clipboard.writeText(document.getElementById('adstxt-content').textContent).then(()=>alert('Copied!'))">Copy All</button>
        <pre id="adstxt-content">${text || '# No active SSPs configured yet'}</pre>
      </div>
    `);
  } catch (e) {
    render(`<h1>ads.txt</h1><div class="error-msg">${e.message}</div>`);
  }
}

// ── Settings ─────────────────────────────────────────────────────────────────
function settings() {
  render(`
    <h1>Settings</h1>
    <div class="card">
      <h2>Server Configuration</h2>
      <p style="color:var(--muted);margin-bottom:16px">These values are set via environment variables on the server.</p>
      <table>
        <thead><tr><th>Variable</th><th>Purpose</th><th>Default</th></tr></thead>
        <tbody>
          <tr><td><code>PBS_PORT</code></td><td>Server port</td><td>8080</td></tr>
          <tr><td><code>ADS_TXT_DOMAIN</code></td><td>Domain used in ads.txt lines</td><td>ads.bizbudding.com</td></tr>
          <tr><td><code>ADMIN_API_KEY</code></td><td>Admin UI auth token</td><td>(required)</td></tr>
          <tr><td><code>DB_HOST / DB_NAME</code></td><td>PostgreSQL connection</td><td>localhost/catalyst</td></tr>
          <tr><td><code>REDIS_HOST</code></td><td>Redis connection</td><td>localhost:6379</td></tr>
        </tbody>
      </table>
    </div>
    <div class="card">
      <h2>API Access</h2>
      <p style="color:var(--muted);margin-bottom:12px">All admin API calls require:</p>
      <pre>Authorization: Bearer $ADMIN_API_KEY</pre>
    </div>
  `);
}
```

- [ ] **Step 2: Build and verify**

```bash
make build
ADMIN_AUTH_REQUIRED=false ./bin/catalyst-bizbudding &
sleep 1
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
kill %1
```

Expected: `200`

- [ ] **Step 3: Run full test suite to confirm nothing broken**

```bash
go test -v -race ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/admin/static/app.js
git commit -m "feat(admin): complete vanilla JS SPA — dashboard, sites, placements, SSP config, ads.txt, video tags"
```

---

## Chunk 4: Deployment + Reporting

### File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `docker-compose.yml` | Replace | Full rewrite: catalyst + postgres + redis |
| `deployment/catalyst.service` | Create | systemd unit for native install |
| `deployment/nginx-bizbudding.conf` | Create | nginx TLS reverse proxy config |
| `deployment/SSH_SETUP.md` | Create | Server user, SSH key format, deploy permissions |
| `scripts/deploy.sh` | Create | rsync + systemd reload deploy script |
| `migrations/009_reporting_views.sql` | Create | v_daily_revenue, v_ssp_performance, v_placement_fill_rate |
| `docs/deployment.md` | Create | Full native + Docker deploy guide |
| `docs/looker-studio-setup.md` | Create | Step-by-step Looker Studio connection guide |
| `reporting/bizbudding-report-template.json` | Create | Looker Studio importable template descriptor |

---

### Task 14: Docker Compose

**Files:**
- Replace: `docker-compose.yml`

- [ ] **Step 1: Write new docker-compose.yml**

Replace the entire `docker-compose.yml`:

```yaml
# Bizbudding Ad Server — Docker Compose
# Usage: cp .env.example .env && docker compose up -d

services:
  catalyst:
    build: .
    image: catalyst-bizbudding:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      PBS_PORT: "8080"
      PBS_HOST: "0.0.0.0"
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_NAME: ${DB_NAME:-catalyst}
      DB_USER: ${DB_USER:-catalyst}
      DB_PASSWORD: ${DB_PASSWORD}
      DB_SSL_MODE: disable
      REDIS_HOST: redis
      REDIS_PORT: "6379"
      ADMIN_API_KEY: ${ADMIN_API_KEY}
      ADMIN_AUTH_REQUIRED: "true"
      ADS_TXT_DOMAIN: ${ADS_TXT_DOMAIN:-ads.bizbudding.com}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    volumes:
      - ./assets:/app/assets:ro
      - ./migrations:/app/migrations:ro

  postgres:
    image: postgres:16
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DB_NAME:-catalyst}
      POSTGRES_USER: ${DB_USER:-catalyst}
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-catalyst}"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  postgres_data:
  redis_data:
```

- [ ] **Step 2: Verify docker-compose.yml syntax**

```bash
docker compose config --quiet
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "feat(deploy): rewrite docker-compose.yml — catalyst + postgres:16 + redis:7"
```

---

### Task 15: Native deployment files

**Files:**
- Create: `deployment/catalyst.service`
- Create: `deployment/nginx-bizbudding.conf`
- Create: `scripts/deploy.sh`

- [ ] **Step 1: Create systemd unit file**

Create `deployment/catalyst.service`:

```ini
[Unit]
Description=Bizbudding Catalyst Ad Server
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=catalyst
WorkingDirectory=/opt/catalyst
ExecStart=/opt/catalyst/catalyst-bizbudding
EnvironmentFile=/opt/catalyst/.env
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=catalyst

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/opt/catalyst/logs

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Create nginx config**

Create `deployment/nginx-bizbudding.conf`:

```nginx
# Nginx reverse proxy + TLS for ads.bizbudding.com
# Install: cp nginx-bizbudding.conf /etc/nginx/sites-available/ads.bizbudding.com
#          ln -s /etc/nginx/sites-available/ads.bizbudding.com /etc/nginx/sites-enabled/
# Cert:    certbot --nginx -d ads.bizbudding.com

server {
    listen 80;
    server_name ads.bizbudding.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name ads.bizbudding.com;

    ssl_certificate     /etc/letsencrypt/live/ads.bizbudding.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/ads.bizbudding.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # CORS for ad tags
    add_header Access-Control-Allow-Origin "*" always;
    add_header Access-Control-Allow-Methods "GET, POST, OPTIONS" always;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 10s;
        proxy_connect_timeout 5s;
    }

    # Rate limit admin endpoints
    location /admin/ {
        limit_req zone=admin burst=10 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}

# Rate limit zone — add to nginx.conf http block:
# limit_req_zone $binary_remote_addr zone=admin:10m rate=30r/m;
```

- [ ] **Step 3: Create SSH setup guide**

Create `deployment/SSH_SETUP.md`:

```markdown
# SSH Setup for Bizbudding Catalyst Deployment

## 1. Create the deploy user on the server

```bash
sudo useradd -r -m -s /bin/bash deploy
sudo usermod -aG sudo deploy  # optional — only if deploy user needs sudo for systemctl
```

## 2. Add your SSH public key

On the server:
```bash
sudo mkdir -p /home/deploy/.ssh
sudo nano /home/deploy/.ssh/authorized_keys
# Paste your public key (id_rsa.pub or id_ed25519.pub)
sudo chmod 700 /home/deploy/.ssh
sudo chmod 600 /home/deploy/.ssh/authorized_keys
sudo chown -R deploy:deploy /home/deploy/.ssh
```

## 3. Key format requirements

- Use Ed25519 (`ssh-keygen -t ed25519`) or RSA 4096 (`ssh-keygen -t rsa -b 4096`)
- Keep the private key local — never commit it
- Rotate keys if a team member leaves

## 4. Deploy permissions

The deploy user needs permission to:
- Write to `/opt/catalyst/` (owned by `catalyst` user — use `sudo chown -R catalyst:catalyst /opt/catalyst`)
- Restart the systemd service: `sudo systemctl restart catalyst`

Grant passwordless sudo for just the service restart:
```bash
echo "deploy ALL=(ALL) NOPASSWD: /bin/systemctl restart catalyst, /bin/systemctl status catalyst" \
  | sudo tee /etc/sudoers.d/catalyst-deploy
```

## 5. Test the connection

```bash
ssh deploy@your-server.com "systemctl status catalyst --no-pager"
```

## 6. Run a deploy

```bash
./scripts/deploy.sh deploy@your-server.com
```
```

- [ ] **Step 4: Create deploy script**

Create `scripts/deploy.sh`:

```bash
#!/bin/bash
# Deploy catalyst-bizbudding to remote server via rsync + systemd reload
# Usage: ./scripts/deploy.sh user@your-server.com

set -e

REMOTE=${1:?Usage: deploy.sh user@host}
BINARY=bin/catalyst-bizbudding
REMOTE_DIR=/opt/catalyst

echo "→ Building..."
make build

echo "→ Uploading binary to $REMOTE:$REMOTE_DIR/..."
rsync -avz --progress "$BINARY" "$REMOTE:$REMOTE_DIR/catalyst-bizbudding"

echo "→ Uploading assets..."
rsync -avz assets/ "$REMOTE:$REMOTE_DIR/assets/"

echo "→ Reloading service..."
ssh "$REMOTE" "sudo systemctl restart catalyst && sudo systemctl status catalyst --no-pager"

echo "✓ Deploy complete"
```

```bash
chmod +x scripts/deploy.sh
```

- [ ] **Step 5: Commit**

```bash
git add deployment/catalyst.service deployment/nginx-bizbudding.conf deployment/SSH_SETUP.md scripts/deploy.sh
git commit -m "feat(deploy): systemd unit, nginx config, SSH setup guide, deploy script"
```

---

### Task 16: Reporting SQL views

**Files:**
- Create: `migrations/009_reporting_views.sql`

- [ ] **Step 1: Write the migration**

Create `migrations/009_reporting_views.sql`:

```sql
-- 009_reporting_views.sql
-- SQL views for Looker Studio reporting
-- Source tables: auction_events, bidder_events, win_events
--
-- Column reference (from create_analytics_schema.sql):
--   auction_events:  id, auction_id (VARCHAR), publisher_id (VARCHAR), created_at
--   bidder_events:   id, auction_id (VARCHAR), bidder_code (VARCHAR), created_at
--   win_events:      id, auction_id (VARCHAR), bidder_code (VARCHAR), platform_cut (DECIMAL), created_at
--
-- Join win_events to bidder_events via (auction_id + bidder_code) — no bidder_event_id column.
-- Revenue column is platform_cut (not price).
-- auction_events has no slot_pattern — use publisher_id for placement-level grouping.

BEGIN;

-- Daily revenue by SSP
-- Joins win_events directly to bidder_events via auction_id + bidder_code.
CREATE OR REPLACE VIEW v_daily_revenue AS
SELECT
    DATE_TRUNC('day', w.created_at)  AS day,
    w.bidder_code                    AS ssp,
    COUNT(*)                         AS wins,
    COALESCE(SUM(w.platform_cut), 0) AS revenue_usd,
    COALESCE(AVG(w.platform_cut), 0) AS avg_cpm
FROM win_events w
GROUP BY 1, 2
ORDER BY 1 DESC, 3 DESC;

-- SSP performance: win rate, fill rate, avg CPM (rolling 30 days)
CREATE OR REPLACE VIEW v_ssp_performance AS
SELECT
    be.bidder_code                                                                      AS ssp,
    COUNT(DISTINCT ae.auction_id)                                                       AS total_auctions,
    COUNT(DISTINCT be.id)                                                               AS total_bids,
    COUNT(DISTINCT w.id)                                                                AS wins,
    ROUND(COUNT(DISTINCT w.id)::numeric / NULLIF(COUNT(DISTINCT ae.auction_id), 0) * 100, 2) AS fill_rate_pct,
    ROUND(COUNT(DISTINCT w.id)::numeric / NULLIF(COUNT(DISTINCT be.id), 0) * 100, 2)   AS win_rate_pct,
    COALESCE(AVG(w.platform_cut), 0)                                                    AS avg_winning_cpm
FROM auction_events ae
LEFT JOIN bidder_events be ON be.auction_id = ae.auction_id
LEFT JOIN win_events w     ON w.auction_id = be.auction_id AND w.bidder_code = be.bidder_code
WHERE ae.created_at >= NOW() - INTERVAL '30 days'
GROUP BY 1
ORDER BY 4 DESC;

-- Fill rate per publisher per day
-- auction_events has no slot_pattern column; group by publisher_id instead.
CREATE OR REPLACE VIEW v_placement_fill_rate AS
SELECT
    DATE_TRUNC('day', ae.created_at)                                                         AS day,
    ae.publisher_id                                                                            AS publisher,
    COUNT(DISTINCT ae.auction_id)                                                              AS auctions,
    COUNT(DISTINCT w.id)                                                                       AS wins,
    ROUND(COUNT(DISTINCT w.id)::numeric / NULLIF(COUNT(DISTINCT ae.auction_id), 0) * 100, 2) AS fill_rate_pct
FROM auction_events ae
LEFT JOIN win_events w ON w.auction_id = ae.auction_id
GROUP BY 1, 2
ORDER BY 1 DESC, 5 DESC;

COMMIT;
```

- [ ] **Step 2: Run against test DB to verify no syntax errors**

```bash
psql $DATABASE_URL -f migrations/009_reporting_views.sql
psql $DATABASE_URL -c "\dv" | grep "v_"
```

Expected: 3 views created — `v_daily_revenue`, `v_ssp_performance`, `v_placement_fill_rate`.

- [ ] **Step 3: Commit**

```bash
git add migrations/009_reporting_views.sql
git commit -m "feat(migrations): 009 — Looker Studio reporting views"
```

---

### Task 17: Deployment + Looker Studio documentation

**Files:**
- Create: `docs/deployment.md`
- Create: `docs/looker-studio-setup.md`
- Create: `reporting/bizbudding-report-template.json`

- [ ] **Step 1: Create deployment guide**

Create `docs/deployment.md`:

```markdown
# Bizbudding Ad Server — Deployment Guide

## Prerequisites

- Domain: `ads.bizbudding.com` pointing to your server IP
- SSL certificate via Let's Encrypt (instructions below)
- PostgreSQL 14+ and Redis 7+ (or use Docker Compose)

---

## Option A: Docker Compose (recommended)

### 1. Clone and configure

```bash
git clone <repo-url> bizbudding-catalyst
cd bizbudding-catalyst
cp .env.example .env
# Edit .env — set DB_PASSWORD, ADMIN_API_KEY (use: openssl rand -hex 32)
```

### 2. Start services

```bash
docker compose up -d
docker compose logs -f catalyst
```

### 3. Run migrations

Migrations run automatically on first start via `docker-entrypoint-initdb.d`.

For subsequent migrations:

```bash
docker compose exec postgres psql -U catalyst catalyst -f /app/migrations/007_bizbudding_complete_seed.sql
docker compose exec postgres psql -U catalyst catalyst -f /app/migrations/008_ssp_configs.sql
docker compose exec postgres psql -U catalyst catalyst -f /app/migrations/009_reporting_views.sql
```

### 4. Verify

```bash
curl https://ads.bizbudding.com/health
curl https://ads.bizbudding.com/ads.txt
curl https://ads.bizbudding.com/sellers.json
```

---

## Option B: Native (Go binary)

### 1. Build

```bash
go install  # Go 1.23+ required
make build
```

### 2. Install

```bash
sudo useradd -r -s /bin/false catalyst
sudo mkdir -p /opt/catalyst/logs
sudo cp bin/catalyst-bizbudding /opt/catalyst/
sudo cp -r assets/ /opt/catalyst/assets/
sudo cp .env /opt/catalyst/.env
sudo chmod 600 /opt/catalyst/.env
sudo chown -R catalyst:catalyst /opt/catalyst
```

### 3. Install + start systemd service

```bash
sudo cp deployment/catalyst.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable catalyst
sudo systemctl start catalyst
sudo systemctl status catalyst
```

### 4. nginx + SSL

```bash
sudo apt install nginx certbot python3-certbot-nginx
sudo cp deployment/nginx-bizbudding.conf /etc/nginx/sites-available/ads.bizbudding.com
sudo ln -s /etc/nginx/sites-available/ads.bizbudding.com /etc/nginx/sites-enabled/
sudo certbot --nginx -d ads.bizbudding.com
sudo systemctl reload nginx
```

---

## SSH deploy (after initial setup)

```bash
./scripts/deploy.sh user@your-server.com
```

---

## After deploying: ads.txt setup

1. Visit `https://ads.bizbudding.com/admin` → **SSP Config**
2. Enter account IDs for each active SSP
3. Visit **ads.txt** tab — copy the generated content
4. Paste into `bizbudding.com/ads.txt` on their web server
5. Verify: `curl https://bizbudding.com/ads.txt`
```

- [ ] **Step 2: Create Looker Studio setup guide**

Create `docs/looker-studio-setup.md`:

```markdown
# Looker Studio — Bizbudding Reporting Setup

## Prerequisites

- PostgreSQL accessible from the internet (or via Cloud SQL / pgBouncer)
- Google account for Looker Studio

---

## 1. Allow Looker Studio to connect to PostgreSQL

Add to `pg_hba.conf` (or set via DB dashboard if using managed PostgreSQL):

```
host  catalyst  catalyst  [looker-studio-ip-range]  md5
```

Looker Studio IP ranges: https://support.google.com/looker-studio/answer/7088031

Alternatively, use **Cloud SQL Auth Proxy** or a **read-only replica**.

Create a read-only reporting user:

```sql
CREATE USER looker_readonly WITH PASSWORD 'strong-password';
GRANT CONNECT ON DATABASE catalyst TO looker_readonly;
GRANT USAGE ON SCHEMA public TO looker_readonly;
GRANT SELECT ON v_daily_revenue, v_ssp_performance, v_placement_fill_rate TO looker_readonly;
```

---

## 2. Connect Looker Studio

1. Go to [lookerstudio.google.com](https://lookerstudio.google.com)
2. **Create** → **Data source** → **PostgreSQL**
3. Enter connection details:
   - Host: `[your-server-ip or hostname]`
   - Port: `5432`
   - Database: `catalyst`
   - Username: `looker_readonly`
   - Password: `[strong-password]`
4. Click **Authenticate**
5. Select **v_daily_revenue** as the first table → **Connect**

---

## 3. Build the report

Recommended charts:

| View | Chart type | Dimensions | Metrics |
|------|-----------|-----------|---------|
| `v_daily_revenue` | Time series | `day` | `revenue_usd` |
| `v_daily_revenue` | Bar chart | `ssp` | `revenue_usd`, `wins` |
| `v_ssp_performance` | Scorecard table | `ssp` | `fill_rate_pct`, `win_rate_pct`, `avg_winning_cpm` |
| `v_placement_fill_rate` | Table | `publisher`, `day` | `fill_rate_pct`, `wins` |

---

## 4. Share

Click **Share** → enter Bizbudding team email addresses → **Viewer** access.
The report auto-refreshes from the live database.
```

- [ ] **Step 3: Create reporting template descriptor**

Create `reporting/bizbudding-report-template.json`:

```json
{
  "template_version": "1.0",
  "description": "Bizbudding Ad Server — Looker Studio report template",
  "created": "2026-03-17",
  "data_sources": [
    {
      "name": "Daily Revenue",
      "view": "v_daily_revenue",
      "fields": ["day", "ssp", "wins", "revenue_usd", "avg_cpm"]
    },
    {
      "name": "SSP Performance",
      "view": "v_ssp_performance",
      "fields": ["ssp", "total_auctions", "total_bids", "wins", "fill_rate_pct", "win_rate_pct", "avg_winning_cpm"]
    },
    {
      "name": "Publisher Fill Rate",
      "view": "v_placement_fill_rate",
      "fields": ["day", "publisher", "auctions", "wins", "fill_rate_pct"]
    }
  ],
  "note": "Import each data source manually in Looker Studio. See docs/looker-studio-setup.md."
}
```

- [ ] **Step 4: Final full test run**

```bash
go test -v -race ./...
make build
ls -la bin/catalyst-bizbudding
```

Expected: all tests pass, binary builds.

- [ ] **Step 5: Commit all documentation**

```bash
git add docs/deployment.md docs/looker-studio-setup.md reporting/bizbudding-report-template.json
git commit -m "docs: deployment guide, Looker Studio setup, reporting template"
```

---

### Task 18: Final integration smoke test

- [ ] **Step 1: Start server locally with Docker Compose**

```bash
docker compose up -d
sleep 5
```

- [ ] **Step 2: Smoke test all critical endpoints**

```bash
# Health
curl -s http://localhost:8080/health | jq .

# sellers.json
curl -s http://localhost:8080/sellers.json | jq '.sellers[0]'
# Expected: seller_id "NXS001", domain "bizbudding.com"

# ads.txt (empty until SSP account IDs set)
curl -s http://localhost:8080/ads.txt

# Admin UI
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/
# Expected: 401 (auth required)

curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8080/admin/api/dashboard
# Expected: 200

# OpenRTB auction
curl -s -X POST http://localhost:8080/openrtb2/auction \
  -H "Content-Type: application/json" \
  -d '{"id":"smoke-test","imp":[{"id":"1","banner":{"w":300,"h":250}}],"site":{"domain":"bizbudding.com"}}' | jq .id
# Expected: "smoke-test"
```

- [ ] **Step 3: Tear down**

```bash
docker compose down
```

- [ ] **Step 4: Final commit**

```bash
git add .
git commit -m "chore: final smoke test verification — all endpoints healthy"
```

---

## Summary

| Chunk | Tasks | Key deliverables |
|-------|-------|-----------------|
| 1 — Foundation | 1–5 | Binary rename, config, sellers.json, ads.txt endpoint + ssp_configs migration |
| 2 — Admin API | 6–10 | REST API for sites, placements, SSP config, dashboard |
| 3 — Admin UI | 11–13 | Embedded vanilla JS SPA — all 6 sections |
| 4 — Deployment + Reporting | 14–18 | Docker Compose, systemd, nginx, deploy script, SQL views, Looker Studio docs |

**Post-v1 work (out of scope):**
- Go module path rename (`github.com/thenexusengine/tne_springwire` → `github.com/bizbudding/catalyst`)
- Axonix adapter build (Phase 2)
- OMS adapter build (Phase 2)
- `bizbudding.com` domain-specific ad slot params (once provided by Bizbudding)
