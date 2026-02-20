package analytics

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockModule is a mock analytics module for testing
type mockModule struct {
	auctionCalls int
	videoCalls   int
	shutdownErr  error
	logErr       error
}

func (m *mockModule) LogAuctionObject(ctx context.Context, auction *AuctionObject) error {
	m.auctionCalls++
	return m.logErr
}

func (m *mockModule) LogVideoObject(ctx context.Context, video *VideoObject) error {
	m.videoCalls++
	return m.logErr
}

func (m *mockModule) Shutdown() error {
	return m.shutdownErr
}

func TestMultiModule_LogAuctionObject(t *testing.T) {
	ctx := context.Background()

	t.Run("broadcasts to all modules", func(t *testing.T) {
		mock1 := &mockModule{}
		mock2 := &mockModule{}
		mock3 := &mockModule{}

		multi := NewMultiModule(mock1, mock2, mock3)

		auction := &AuctionObject{
			AuctionID: "test-auction-123",
			Timestamp: time.Now(),
			Status:    "success",
		}

		err := multi.LogAuctionObject(ctx, auction)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if mock1.auctionCalls != 1 {
			t.Errorf("Expected mock1 to be called once, got %d", mock1.auctionCalls)
		}
		if mock2.auctionCalls != 1 {
			t.Errorf("Expected mock2 to be called once, got %d", mock2.auctionCalls)
		}
		if mock3.auctionCalls != 1 {
			t.Errorf("Expected mock3 to be called once, got %d", mock3.auctionCalls)
		}
	})

	t.Run("error in one module does not affect others", func(t *testing.T) {
		mock1 := &mockModule{}
		mock2 := &mockModule{logErr: errors.New("mock error")}
		mock3 := &mockModule{}

		multi := NewMultiModule(mock1, mock2, mock3)

		auction := &AuctionObject{
			AuctionID: "test-auction-456",
			Timestamp: time.Now(),
			Status:    "success",
		}

		// Should not return error even if one module fails
		err := multi.LogAuctionObject(ctx, auction)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// All modules should still be called
		if mock1.auctionCalls != 1 {
			t.Errorf("Expected mock1 to be called once, got %d", mock1.auctionCalls)
		}
		if mock2.auctionCalls != 1 {
			t.Errorf("Expected mock2 to be called once, got %d", mock2.auctionCalls)
		}
		if mock3.auctionCalls != 1 {
			t.Errorf("Expected mock3 to be called once, got %d", mock3.auctionCalls)
		}
	})

	t.Run("empty module list", func(t *testing.T) {
		multi := NewMultiModule()

		auction := &AuctionObject{
			AuctionID: "test-auction-789",
			Timestamp: time.Now(),
			Status:    "success",
		}

		err := multi.LogAuctionObject(ctx, auction)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestMultiModule_LogVideoObject(t *testing.T) {
	ctx := context.Background()

	t.Run("broadcasts to all modules", func(t *testing.T) {
		mock1 := &mockModule{}
		mock2 := &mockModule{}

		multi := NewMultiModule(mock1, mock2)

		video := &VideoObject{
			AuctionID: "test-auction-123",
			VideoID:   "video-456",
			Event:     "start",
			Timestamp: time.Now(),
		}

		err := multi.LogVideoObject(ctx, video)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if mock1.videoCalls != 1 {
			t.Errorf("Expected mock1 to be called once, got %d", mock1.videoCalls)
		}
		if mock2.videoCalls != 1 {
			t.Errorf("Expected mock2 to be called once, got %d", mock2.videoCalls)
		}
	})
}

func TestMultiModule_Shutdown(t *testing.T) {
	t.Run("all modules shut down successfully", func(t *testing.T) {
		mock1 := &mockModule{}
		mock2 := &mockModule{}

		multi := NewMultiModule(mock1, mock2)

		err := multi.Shutdown()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("collects errors from failed shutdowns", func(t *testing.T) {
		mock1 := &mockModule{shutdownErr: errors.New("shutdown error 1")}
		mock2 := &mockModule{shutdownErr: errors.New("shutdown error 2")}

		multi := NewMultiModule(mock1, mock2)

		err := multi.Shutdown()
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})
}
