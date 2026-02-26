package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// TestPublisherStore_Update_OptimisticLocking_Success tests successful update with correct version
func TestPublisherStore_Update_OptimisticLocking_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	publisher.Version = 1
	publisher.Name = "Updated Name"

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect version check query
	versionRows := sqlmock.NewRows([]string{"version"}).AddRow(1)
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(versionRows)

	// Expect update query with version check
	mock.ExpectExec("UPDATE publishers").
		WithArgs(
			publisher.Name,
			publisher.AllowedDomains,
			sqlmock.AnyArg(), // bidder_params JSON
			publisher.BidMultiplier,
			publisher.Status,
			publisher.Notes,
			publisher.ContactEmail,
			publisher.PublisherID,
			1, // version
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect commit
	mock.ExpectCommit()

	err = store.Update(ctx, publisher)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if publisher.Version != 2 {
		t.Errorf("Expected version to be incremented to 2, got %d", publisher.Version)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Update_OptimisticLocking_VersionMismatch tests concurrent modification detection
func TestPublisherStore_Update_OptimisticLocking_VersionMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	publisher.Version = 1
	publisher.Name = "Updated Name"

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect version check query - but return different version (simulating concurrent update)
	versionRows := sqlmock.NewRows([]string{"version"}).AddRow(3) // Version changed!
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(versionRows)

	// Expect rollback due to version mismatch
	mock.ExpectRollback()

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Fatal("Expected error for version mismatch, got nil")
	}

	if !contains(err.Error(), "concurrent modification detected") {
		t.Errorf("Expected concurrent modification error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Update_OptimisticLocking_NotFound tests update of non-existent publisher
func TestPublisherStore_Update_OptimisticLocking_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("nonexistent")
	publisher.Version = 1

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect version check query - return no rows
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	// Expect rollback
	mock.ExpectRollback()

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Fatal("Expected error for non-existent publisher, got nil")
	}

	if !contains(err.Error(), "not found") {
		t.Errorf("Expected not found error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Update_OptimisticLocking_ZeroRowsAffected tests when update affects 0 rows
func TestPublisherStore_Update_OptimisticLocking_ZeroRowsAffected(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	publisher.Version = 1

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect version check - returns matching version
	versionRows := sqlmock.NewRows([]string{"version"}).AddRow(1)
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(versionRows)

	// Expect update query - but return 0 rows affected (race condition)
	mock.ExpectExec("UPDATE publishers").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect rollback
	mock.ExpectRollback()

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Fatal("Expected error for zero rows affected, got nil")
	}

	if !contains(err.Error(), "concurrent modification detected") {
		t.Errorf("Expected concurrent modification error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Update_OptimisticLocking_TransactionError tests transaction errors
func TestPublisherStore_Update_OptimisticLocking_TransactionError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	publisher.Version = 1

	// Expect transaction begin to fail
	mock.ExpectBegin().WillReturnError(errors.New("transaction error"))

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Fatal("Expected error from transaction begin, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Update_OptimisticLocking_CommitError tests commit failures
func TestPublisherStore_Update_OptimisticLocking_CommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	publisher.Version = 1

	// Expect transaction begin
	mock.ExpectBegin()

	// Expect version check
	versionRows := sqlmock.NewRows([]string{"version"}).AddRow(1)
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(versionRows)

	// Expect update
	mock.ExpectExec("UPDATE publishers").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect commit to fail
	mock.ExpectCommit().WillReturnError(errors.New("commit failed"))

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Fatal("Expected error from commit, got nil")
	}

	if !contains(err.Error(), "commit") {
		t.Errorf("Expected commit error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_GetByPublisherID_IncludesTMaxMs tests that default_timeout_ms is retrieved
func TestPublisherStore_GetByPublisherID_IncludesTMaxMs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	expectedPublisher := createTestPublisher("pub-123")
	expectedPublisher.TMaxMs = 2000

	rows := sqlmock.NewRows([]string{
		"account_id", "domain", "name", "status", "default_timeout_ms", "bid_multiplier", "notes", "created_at", "updated_at",
	}).AddRow(
		expectedPublisher.PublisherID,
		expectedPublisher.AllowedDomains,
		expectedPublisher.Name,
		expectedPublisher.Status,
		expectedPublisher.TMaxMs,
		expectedPublisher.BidMultiplier,
		expectedPublisher.Notes,
		expectedPublisher.CreatedAt,
		expectedPublisher.UpdatedAt,
	)

	mock.ExpectQuery("SELECT (.+) FROM publishers_new").
		WithArgs("pub-123").
		WillReturnRows(rows)

	result, err := store.GetByPublisherID(ctx, "pub-123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	publisher := result.(*Publisher)
	if publisher.TMaxMs != 2000 {
		t.Errorf("Expected TMaxMs 2000, got %d", publisher.TMaxMs)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// TestPublisherStore_Create_ReturnsVersion tests that create returns initial version
func TestPublisherStore_Create_ReturnsVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-new")
	publisher.ID = ""

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "version", "created_at", "updated_at"}).
		AddRow("10", 1, now, now)

	mock.ExpectQuery("INSERT INTO publishers").
		WillReturnRows(rows)

	err = store.Create(ctx, publisher)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if publisher.Version != 1 {
		t.Errorf("Expected version 1, got %d", publisher.Version)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}
