package storage

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func TestCuratorStore_LoadCurator_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "name", "status", "schain_asi", "schain_sid", "notes", "created_at", "updated_at",
	}).AddRow("c1", "Curator One", "active", "c1.example", "sid-1", nil, now, now)
	mock.ExpectQuery("FROM curators").WithArgs("c1").WillReturnRows(rows)

	c, err := NewCuratorStore(db).LoadCurator(context.Background(), "c1")
	if err != nil {
		t.Fatalf("LoadCurator: %v", err)
	}
	if c == nil || c.ID != "c1" || c.SChainASI != "c1.example" {
		t.Fatalf("unexpected curator: %+v", c)
	}
}

func TestCuratorStore_LookupDeal_HydratesArrays(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now()
	cols := []string{
		"deal_id", "curator_id", "bidfloor", "bidfloorcur", "at",
		"wseat", "wadomain", "segtax_allowed", "cattax_allowed",
		"ext", "active", "created_at", "updated_at",
	}
	mock.ExpectQuery("FROM curator_deals").WithArgs("D1").WillReturnRows(
		sqlmock.NewRows(cols).AddRow(
			"D1", "c1", 2.50, "USD", 2,
			pq.Array([]string{"seat-c1-rub"}),
			pq.Array([]string{"brand.example"}),
			pq.Array([]int64{4}),
			pq.Array([]int64{}),
			[]byte("null"), true, now, now,
		),
	)

	d, err := NewCuratorStore(db).LookupDeal(context.Background(), "D1")
	if err != nil {
		t.Fatalf("LookupDeal: %v", err)
	}
	if d == nil || d.DealID != "D1" || d.CuratorID != "c1" {
		t.Fatalf("unexpected deal: %+v", d)
	}
	if len(d.WSeat) != 1 || d.WSeat[0] != "seat-c1-rub" {
		t.Errorf("wseat not hydrated: %v", d.WSeat)
	}
	if len(d.SegtaxAllowed) != 1 || d.SegtaxAllowed[0] != 4 {
		t.Errorf("segtax_allowed not hydrated: %v", d.SegtaxAllowed)
	}
}

func TestCuratorStore_PublisherAllowed_EmptyListMeansAllowAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("curator_publisher_allowlist").
		WithArgs("c1", 42).
		WillReturnRows(sqlmock.NewRows([]string{"total", "match"}).AddRow(0, 0))

	ok, err := NewCuratorStore(db).PublisherAllowedForCurator(context.Background(), 42, "c1")
	if err != nil || !ok {
		t.Fatalf("expected allow-all when empty list: ok=%v err=%v", ok, err)
	}
}

func TestCuratorStore_PublisherAllowed_NonEmptyEnforces(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("curator_publisher_allowlist").
		WithArgs("c1", 99).
		WillReturnRows(sqlmock.NewRows([]string{"total", "match"}).AddRow(3, 0))

	ok, err := NewCuratorStore(db).PublisherAllowedForCurator(context.Background(), 99, "c1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected deny when publisher 99 not in non-empty list")
	}
}

func TestCuratorStore_SeatsForCurator(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("FROM curator_seats").
		WithArgs("c1", "rubicon").
		WillReturnRows(sqlmock.NewRows([]string{"seat_id"}).
			AddRow("seat-c1-rub-a").
			AddRow("seat-c1-rub-b"))

	seats, err := NewCuratorStore(db).SeatsForCurator(context.Background(), "c1", "rubicon")
	if err != nil {
		t.Fatalf("SeatsForCurator: %v", err)
	}
	if len(seats) != 2 {
		t.Fatalf("expected 2 seats, got %d (%v)", len(seats), seats)
	}
}
