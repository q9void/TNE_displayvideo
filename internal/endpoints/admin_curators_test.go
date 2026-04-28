package endpoints

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/thenexusengine/tne_springwire/internal/storage"
)

// TestCuratorAdmin_NoStoreReturns503 verifies that handlers fail fast when no
// store is wired — production deployments should always wire one, but tests
// exercise the routing layer in isolation.
func TestCuratorAdmin_NoStoreReturns503(t *testing.T) {
	h := NewCuratorAdminHandler(nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/curators", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestCuratorAdmin_PostUpsertsCurator(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery("INSERT INTO curators").
		WithArgs("c1", "Curator One", "active", "c1.example", "sid-1", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(now, now))

	h := NewCuratorAdminHandler(storage.NewCuratorStore(db), db)
	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(
		`{"id":"c1","name":"Curator One","schain_asi":"c1.example","schain_sid":"sid-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/curators", body)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCuratorAdmin_RoutesUnknownSubcollection(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()
	h := NewCuratorAdminHandler(storage.NewCuratorStore(db), db)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/curators/c1/bogus", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCuratorAdmin_DeleteSeatTakesFourSegments(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("DELETE FROM curator_seats").
		WithArgs("c1", "rubicon", "seat-c1-rub").
		WillReturnResult(sqlmock.NewResult(0, 1))

	h := NewCuratorAdminHandler(storage.NewCuratorStore(db), db)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete,
		"/admin/curators/c1/seats/rubicon/seat-c1-rub", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
}
