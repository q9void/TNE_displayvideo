package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// createTestPublisher creates a test publisher for use in tests
func createTestPublisher(publisherID string) *Publisher {
	return &Publisher{
		ID:             "1",
		PublisherID:    publisherID,
		Name:           "Test Publisher",
		AllowedDomains: "example.com,test.com",
		BidderParams: map[string]interface{}{
			"appnexus": map[string]interface{}{
				"placementId": 12345,
			},
			"rubicon": map[string]interface{}{
				"accountId": 67890,
			},
		},
		BidMultiplier: 1.05,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Notes:         sql.NullString{String: "Test notes", Valid: true},
		ContactEmail:  sql.NullString{String: "test@example.com", Valid: true},
	}
}

func TestNewPublisherStore(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	if store == nil {
		t.Fatal("Expected non-nil store")
	}
	if store.db != db {
		t.Error("Store DB does not match provided DB")
	}
}

func TestPublisherStore_GetByPublisherID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	expectedPublisher := createTestPublisher("pub-123")
	bidderParamsJSON, _ := json.Marshal(expectedPublisher.BidderParams)

	rows := sqlmock.NewRows([]string{
		"id", "publisher_id", "name", "allowed_domains", "bidder_params",
		"bid_multiplier", "status", "version", "created_at", "updated_at", "notes", "contact_email",
	}).AddRow(
		expectedPublisher.ID,
		expectedPublisher.PublisherID,
		expectedPublisher.Name,
		expectedPublisher.AllowedDomains,
		bidderParamsJSON,
		expectedPublisher.BidMultiplier,
		expectedPublisher.Status,
		1, // version
		expectedPublisher.CreatedAt,
		expectedPublisher.UpdatedAt,
		expectedPublisher.Notes,
		expectedPublisher.ContactEmail,
	)

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(rows)

	result, err := store.GetByPublisherID(ctx, "pub-123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	publisher, ok := result.(*Publisher)
	if !ok {
		t.Fatal("Expected result to be *Publisher")
	}

	if publisher.PublisherID != "pub-123" {
		t.Errorf("Expected 'pub-123', got '%s'", publisher.PublisherID)
	}
	if publisher.Name != "Test Publisher" {
		t.Errorf("Expected 'Test Publisher', got '%s'", publisher.Name)
	}
	if publisher.BidMultiplier != 1.05 {
		t.Errorf("Expected 1.05, got %f", publisher.BidMultiplier)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetByPublisherID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE publisher_id").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	result, err := store.GetByPublisherID(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Expected no error for non-existent publisher, got: %v", err)
	}
	// Handle typed nil - interface{} containing (*Publisher)(nil)
	if result != nil {
		if pub, ok := result.(*Publisher); ok && pub != nil {
			t.Error("Expected nil publisher")
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetByPublisherID_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnError(sql.ErrConnDone)

	result, err := store.GetByPublisherID(ctx, "pub-123")
	if err == nil {
		t.Error("Expected error from query failure")
	}
	// Handle typed nil - interface{} containing (*Publisher)(nil)
	if result != nil {
		if pub, ok := result.(*Publisher); ok && pub != nil {
			t.Error("Expected nil publisher on error")
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetByPublisherID_InvalidJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "publisher_id", "name", "allowed_domains", "bidder_params",
		"bid_multiplier", "status", "version", "created_at", "updated_at", "notes", "contact_email",
	}).AddRow(
		"1",
		"pub-123",
		"Test Publisher",
		"example.com",
		[]byte("{invalid json}"), // Invalid JSON
		1.05,
		"active",
		1, // version
		time.Now(),
		time.Now(),
		"notes",
		"test@example.com",
	)

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(rows)

	result, err := store.GetByPublisherID(ctx, "pub-123")
	if err == nil {
		t.Error("Expected error from invalid JSON")
	}
	// Handle typed nil - interface{} containing (*Publisher)(nil)
	if result != nil {
		if pub, ok := result.(*Publisher); ok && pub != nil {
			t.Error("Expected nil result on JSON error")
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_List_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	pub1 := createTestPublisher("pub-1")
	pub2 := createTestPublisher("pub-2")
	bidderParamsJSON1, _ := json.Marshal(pub1.BidderParams)
	bidderParamsJSON2, _ := json.Marshal(pub2.BidderParams)

	rows := sqlmock.NewRows([]string{
		"id", "publisher_id", "name", "allowed_domains", "bidder_params",
		"bid_multiplier", "status", "version", "created_at", "updated_at", "notes", "contact_email",
	}).AddRow(
		pub1.ID, pub1.PublisherID, pub1.Name, pub1.AllowedDomains, bidderParamsJSON1,
		pub1.BidMultiplier, pub1.Status, 1, pub1.CreatedAt, pub1.UpdatedAt, pub1.Notes, pub1.ContactEmail,
	).AddRow(
		pub2.ID, pub2.PublisherID, pub2.Name, pub2.AllowedDomains, bidderParamsJSON2,
		pub2.BidMultiplier, pub2.Status, 1, pub2.CreatedAt, pub2.UpdatedAt, pub2.Notes, pub2.ContactEmail,
	)

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE status").
		WillReturnRows(rows)

	publishers, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(publishers) != 2 {
		t.Fatalf("Expected 2 publishers, got %d", len(publishers))
	}

	if publishers[0].PublisherID != "pub-1" {
		t.Errorf("Expected 'pub-1', got '%s'", publishers[0].PublisherID)
	}
	if publishers[1].PublisherID != "pub-2" {
		t.Errorf("Expected 'pub-2', got '%s'", publishers[1].PublisherID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_List_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "publisher_id", "name", "allowed_domains", "bidder_params",
		"bid_multiplier", "status", "version", "created_at", "updated_at", "notes", "contact_email",
	})

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE status").
		WillReturnRows(rows)

	publishers, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(publishers) != 0 {
		t.Errorf("Expected 0 publishers, got %d", len(publishers))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_List_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE status").
		WillReturnError(errors.New("database error"))

	publishers, err := store.List(ctx)
	if err == nil {
		t.Error("Expected error from query failure")
	}
	if publishers != nil {
		t.Error("Expected nil publishers on error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_List_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "publisher_id", "name", "allowed_domains", "bidder_params",
		"bid_multiplier", "status", "version", "created_at", "updated_at", "notes", "contact_email",
	}).AddRow(
		"1", "pub-1", "Test", "example.com", []byte("{invalid}"),
		1.05, "active", 1, time.Now(), time.Now(), "notes", "test@example.com",
	)

	mock.ExpectQuery("SELECT (.+) FROM publishers WHERE status").
		WillReturnRows(rows)

	publishers, err := store.List(ctx)
	if err == nil {
		t.Error("Expected error from invalid JSON during scan")
	}
	if publishers != nil {
		t.Error("Expected nil publishers on scan error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Create_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-new")
	publisher.ID = "" // ID should be generated by database

	rows := sqlmock.NewRows([]string{"id", "version", "created_at", "updated_at"}).
		AddRow("10", 1, time.Now(), time.Now())

	mock.ExpectQuery("INSERT INTO publishers").
		WithArgs(
			publisher.PublisherID,
			publisher.Name,
			publisher.AllowedDomains,
			sqlmock.AnyArg(), // bidder_params JSON
			publisher.BidMultiplier,
			publisher.Status,
			publisher.Notes,
			publisher.ContactEmail,
		).
		WillReturnRows(rows)

	err = store.Create(ctx, publisher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if publisher.ID != "10" {
		t.Errorf("Expected ID '10', got '%s'", publisher.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Create_DefaultBidMultiplier(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-new")
	publisher.BidMultiplier = 0 // Should default to 1.0

	rows := sqlmock.NewRows([]string{"id", "version", "created_at", "updated_at"}).
		AddRow("10", 1, time.Now(), time.Now())

	mock.ExpectQuery("INSERT INTO publishers").
		WithArgs(
			publisher.PublisherID,
			publisher.Name,
			publisher.AllowedDomains,
			sqlmock.AnyArg(), // bidder_params JSON
			1.0,              // Should be defaulted to 1.0
			publisher.Status,
			publisher.Notes,
			publisher.ContactEmail,
		).
		WillReturnRows(rows)

	err = store.Create(ctx, publisher)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if publisher.BidMultiplier != 1.0 {
		t.Errorf("Expected BidMultiplier 1.0, got %f", publisher.BidMultiplier)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Create_MarshalError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-new")
	// Create unmarshalable data (channels cannot be marshaled to JSON)
	publisher.BidderParams = map[string]interface{}{
		"invalid": make(chan int),
	}

	err = store.Create(ctx, publisher)
	if err == nil {
		t.Error("Expected error from marshal failure")
	}
}

func TestPublisherStore_Create_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-new")

	mock.ExpectQuery("INSERT INTO publishers").
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnError(errors.New("database error"))

	err = store.Create(ctx, publisher)
	if err == nil {
		t.Error("Expected error from query failure")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Update_Success(t *testing.T) {
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

	// Expect version check
	versionRows := sqlmock.NewRows([]string{"version"}).AddRow(1)
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("pub-123").
		WillReturnRows(versionRows)

	// Expect update with version
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
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Update_NotFound(t *testing.T) {
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

	// Expect version check - return no rows
	mock.ExpectQuery("SELECT version FROM publishers WHERE publisher_id").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	// Expect rollback
	mock.ExpectRollback()

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Error("Expected error for non-existent publisher")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Update_MarshalError(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	publisher := createTestPublisher("pub-123")
	// Create unmarshalable data
	publisher.BidderParams = map[string]interface{}{
		"invalid": make(chan int),
	}

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Error("Expected error from marshal failure")
	}
}

func TestPublisherStore_Update_QueryError(t *testing.T) {
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

	// Expect update to fail
	mock.ExpectExec("UPDATE publishers").
		WillReturnError(errors.New("database error"))

	// Expect rollback
	mock.ExpectRollback()

	err = store.Update(ctx, publisher)
	if err == nil {
		t.Error("Expected error from query failure")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Delete_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectExec("UPDATE publishers SET status = 'archived'").
		WithArgs("pub-123").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.Delete(ctx, "pub-123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Delete_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectExec("UPDATE publishers SET status = 'archived'").
		WithArgs("nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	err = store.Delete(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent publisher")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_Delete_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectExec("UPDATE publishers SET status = 'archived'").
		WithArgs("pub-123").
		WillReturnError(errors.New("database error"))

	err = store.Delete(ctx, "pub-123")
	if err == nil {
		t.Error("Expected error from query failure")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetBidderParams_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	expectedParams := map[string]interface{}{
		"placementId": float64(12345),
		"accountId":   float64(67890),
	}
	paramsJSON, _ := json.Marshal(expectedParams)

	rows := sqlmock.NewRows([]string{"params"}).AddRow(paramsJSON)

	mock.ExpectQuery("SELECT bidder_params").
		WithArgs("pub-123", "appnexus").
		WillReturnRows(rows)

	params, err := store.GetBidderParams(ctx, "pub-123", "appnexus")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if params["placementId"].(float64) != 12345 {
		t.Errorf("Expected placementId 12345, got %v", params["placementId"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetBidderParams_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT bidder_params").
		WithArgs("pub-123", "nonexistent").
		WillReturnError(sql.ErrNoRows)

	params, err := store.GetBidderParams(ctx, "pub-123", "nonexistent")
	if err != nil {
		t.Errorf("Expected no error for non-existent bidder params, got: %v", err)
	}
	if params != nil {
		t.Error("Expected nil params")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetBidderParams_InvalidJSON(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"params"}).AddRow([]byte("{invalid json}"))

	mock.ExpectQuery("SELECT bidder_params").
		WithArgs("pub-123", "appnexus").
		WillReturnRows(rows)

	params, err := store.GetBidderParams(ctx, "pub-123", "appnexus")
	if err == nil {
		t.Error("Expected error from invalid JSON")
	}
	if params != nil {
		t.Error("Expected nil params on JSON error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisherStore_GetBidderParams_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock DB: %v", err)
	}
	defer db.Close()

	store := NewPublisherStore(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT bidder_params").
		WithArgs("pub-123", "appnexus").
		WillReturnError(errors.New("database error"))

	params, err := store.GetBidderParams(ctx, "pub-123", "appnexus")
	if err == nil {
		t.Error("Expected error from query failure")
	}
	if params != nil {
		t.Error("Expected nil params on error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestPublisher_GetterMethods(t *testing.T) {
	publisher := createTestPublisher("pub-123")

	if publisher.GetPublisherID() != "pub-123" {
		t.Errorf("Expected 'pub-123', got '%s'", publisher.GetPublisherID())
	}

	if publisher.GetAllowedDomains() != "example.com,test.com" {
		t.Errorf("Expected 'example.com,test.com', got '%s'", publisher.GetAllowedDomains())
	}

	if publisher.GetBidMultiplier() != 1.05 {
		t.Errorf("Expected 1.05, got %f", publisher.GetBidMultiplier())
	}
}
