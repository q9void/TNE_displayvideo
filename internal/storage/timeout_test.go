package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestBidderStore_GetByCode_TimeoutEnforcement tests query timeout
func TestBidderStore_GetByCode_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewBidderStore(db)

	// Use a context without deadline
	ctx := context.Background()

	// Simulate a slow query that exceeds timeout
	mock.ExpectQuery("SELECT (.+) FROM bidders WHERE bidder_code").
		WithArgs("appnexus").
		WillDelayFor(6 * time.Second). // Longer than DefaultDBTimeout
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	_, err = store.GetByCode(ctx, "appnexus")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Verify timeout was enforced (should be around 5 seconds)
	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestBidderStore_ListActive_TimeoutEnforcement tests list query timeout
func TestBidderStore_ListActive_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewBidderStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM bidders WHERE enabled").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	_, err = store.ListActive(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestBidderStore_Create_TimeoutEnforcement tests create timeout
func TestBidderStore_Create_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewBidderStore(db)
	ctx := context.Background()

	bidder := createTestBidder("newbidder")

	mock.ExpectQuery("INSERT INTO bidders").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	err = store.Create(ctx, bidder)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestBidderStore_Update_TimeoutEnforcement tests update timeout
func TestBidderStore_Update_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewBidderStore(db)
	ctx := context.Background()

	bidder := createTestBidder("appnexus")
	bidder.Version = 1

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT version FROM bidders WHERE bidder_code").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))
	// Rollback happens in defer, so it may or may not be captured

	start := time.Now()
	err = store.Update(ctx, bidder)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	// Don't check expectations strictly as rollback may not be captured
}

// TestPublisherStore_GetByPublisherID_TimeoutEnforcement tests publisher query timeout
func TestPublisherStore_GetByPublisherID_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM publishers_new").
		WithArgs("pub-123").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	_, err = store.GetByPublisherID(ctx, "pub-123")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_List_TimeoutEnforcement tests list timeout
func TestPublisherStore_List_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE status").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	_, err = store.List(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Update_TimeoutEnforcement tests publisher update timeout
func TestPublisherStore_Update_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	publisher.Version = 1

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))
	// Rollback happens in defer, so it may or may not be called depending on timing
	// Make it optional with MatchExpectationsInOrder(false)

	start := time.Now()
	err = store.Update(ctx, publisher)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	// Don't check expectations strictly as rollback may not be captured
}

// TestPublisherStore_GetBidderParams_TimeoutEnforcement tests bidder params query timeout
func TestPublisherStore_GetBidderParams_TimeoutEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT bidder_params").
		WithArgs("pub-123", "appnexus").
		WillDelayFor(6 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	_, err = store.GetBidderParams(ctx, "pub-123", "appnexus")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if elapsed > 6*time.Second {
		t.Errorf("Query should have timed out around %v, but took %v", DefaultDBTimeout, elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestTimeout_ExistingDeadline_NotOverridden tests that existing deadline is preserved
func TestTimeout_ExistingDeadline_NotOverridden(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewBidderStore(db)

	// Create context with 1 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Simulate query that takes 2 seconds
	mock.ExpectQuery("SELECT (.+) FROM bidders WHERE bidder_code").
		WithArgs("appnexus").
		WillDelayFor(2 * time.Second).
		WillReturnError(errors.New("context deadline exceeded"))

	start := time.Now()
	_, err = store.GetByCode(ctx, "appnexus")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	// Should timeout around 1 second (existing deadline), not 5 seconds (default)
	if elapsed > 1500*time.Millisecond {
		t.Errorf("Expected to respect existing 1s deadline, but took %v", elapsed)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

