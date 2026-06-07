package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bishop-bot/datajobs/internal/providers/ib"
	ibapi "github.com/bishop-bot/ibapi-go"
)

func TestNewMarketDataHandler_WithMock(t *testing.T) {
	mockIB := ib.NewMockClient()

	handler := NewMarketDataHandler(nil, mockIB, nil, nil)

	if handler == nil {
		t.Fatal("handler should not be nil")
	}
	if handler.ibProvider == nil {
		t.Error("ibProvider should not be nil")
	}
}

func TestNewMarketDataHandler_WithNilIB(t *testing.T) {
	handler := NewMarketDataHandler(nil, nil, nil, nil)

	if handler == nil {
		t.Fatal("handler should not be nil")
	}
	if handler.ibProvider != nil {
		t.Error("ibProvider should be nil when not provided")
	}
}

func TestMarketDataHandler_GetHistoricalData_Success(t *testing.T) {
	mockIB := ib.NewMockClient().WithConnected(true).WithHistoricalDataResponse(
		&ibapi.HistoricalDataResponse{
			Symbol: "AAPL",
			Data: []ibapi.HistoricalDataBar{
				{T: 1719792000000, O: 185.50, H: 186.75, L: 184.90, C: 186.20, V: 50000000},
			},
		},
	)

	handler := NewMarketDataHandler(nil, mockIB, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/marketdata/history?conid=265598&exchange=SMART&period=1d&bar=5mins", nil)
	w := httptest.NewRecorder()

	handler.GetHistoricalData(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false: %v", resp.Error)
	}

	// Verify mock was called
	if !mockIB.AssertCalled("HistoricalData") {
		t.Error("HistoricalData should have been called")
	}
}

func TestMarketDataHandler_GetHistoricalData_NoIBProvider(t *testing.T) {
	handler := NewMarketDataHandler(nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/marketdata/history?conid=265598&exchange=SMART", nil)
	w := httptest.NewRecorder()

	handler.GetHistoricalData(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Error != "IB provider not available" {
		t.Errorf("expected error 'IB provider not available', got '%s'", resp.Error)
	}
}

func TestMarketDataHandler_GetHistoricalData_MissingParams(t *testing.T) {
	mockIB := ib.NewMockClient()
	handler := NewMarketDataHandler(nil, mockIB, nil, nil)

	tests := []struct {
		name string
		url  string
	}{
		{"missing conid", "/api/v1/marketdata/history?exchange=SMART"},
		{"missing exchange", "/api/v1/marketdata/history?conid=265598"},
		{"empty params", "/api/v1/marketdata/history"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()

			handler.GetHistoricalData(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			if resp.Success {
				t.Error("expected success=false")
			}
			if resp.Error != "conid and exchange are required" {
				t.Errorf("expected error 'conid and exchange are required', got '%s'", resp.Error)
			}
		})
	}
}

func TestMarketDataHandler_GetHistoricalData_IBError(t *testing.T) {
	mockIB := ib.NewMockClient().WithConnected(true).WithHistoricalDataError(
		context.DeadlineExceeded,
	)

	handler := NewMarketDataHandler(nil, mockIB, nil, nil)

	req := httptest.NewRequest("GET", "/api/v1/marketdata/history?conid=265598&exchange=SMART", nil)
	w := httptest.NewRecorder()

	handler.GetHistoricalData(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestMarketDataHandler_DownloadHistoricalData_NoIBProvider(t *testing.T) {
	handler := NewMarketDataHandler(nil, nil, nil, nil)

	_, err := handler.DownloadHistoricalData(
		context.Background(),
		"", "AAPL", "SMART", "5mins", "1d", "", false,
	)

	if err == nil {
		t.Error("expected error when IB provider is nil")
	}
}

func TestMarketDataHandler_DownloadHistoricalData_NoSymbolOrConid(t *testing.T) {
	mockIB := ib.NewMockClient()
	handler := NewMarketDataHandler(nil, mockIB, nil, nil)

	_, err := handler.DownloadHistoricalData(
		context.Background(),
		"", "", "SMART", "5mins", "1d", "", false,
	)

	if err == nil {
		t.Error("expected error when neither conid nor symbol provided")
	}
}

func TestMarketDataHandler_DownloadHistoricalData_NoQuestDB(t *testing.T) {
	mockIB := ib.NewMockClient()
	handler := NewMarketDataHandler(nil, mockIB, nil, nil) // questdb is nil

	_, err := handler.DownloadHistoricalData(
		context.Background(),
		"265598", "", "SMART", "5mins", "1d", "", false,
	)

	if err == nil {
		t.Error("expected error when QuestDB not available")
	}
}

func TestMockIBClient_BasicOperations(t *testing.T) {
	mock := ib.NewMockClient()

	// Test Ping
	if err := mock.Ping(context.Background()); err != nil {
		t.Errorf("Ping should not error: %v", err)
	}
	if !mock.AssertCalled("Ping") {
		t.Error("Ping should be recorded")
	}

	// Test IsConnected
	if !mock.IsConnected(context.Background()) {
		t.Error("IsConnected should return true when connected")
	}
	if !mock.AssertCalled("IsConnected") {
		t.Error("IsConnected should be recorded")
	}

	// Test TotalCalls
	if mock.TotalCalls() < 2 {
		t.Error("TotalCalls should be at least 2 after Ping and IsConnected")
	}

	// Test Reset
	mock.Reset()
	if mock.TotalCalls() != 0 {
		t.Error("TotalCalls should be 0 after Reset")
	}
}

func TestMockIBClient_ErrorResponses(t *testing.T) {
	t.Run("Ping error", func(t *testing.T) {
		mock := ib.NewMockClient().WithPingError(context.DeadlineExceeded)

		err := mock.Ping(context.Background())
		if err != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", err)
		}
	})

	t.Run("IsConnected false when ping error", func(t *testing.T) {
		mock := ib.NewMockClient().WithPingError(context.DeadlineExceeded)

		if mock.IsConnected(context.Background()) {
			t.Error("IsConnected should return false when Ping fails")
		}
	})

	t.Run("HistoricalData error", func(t *testing.T) {
		mock := ib.NewMockClient().WithHistoricalDataError(context.DeadlineExceeded)

		_, err := mock.HistoricalData(context.Background(), ib.HistoricalDataRequest{})
		if err != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", err)
		}
	})
}

func TestMockIBClient_Close(t *testing.T) {
	mock := ib.NewMockClient()

	err := mock.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}

	// Operations after close should return ErrClientClosed
	_, err = mock.HistoricalData(context.Background(), ib.HistoricalDataRequest{})
	if err != ib.ErrClientClosed {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}

	err = mock.Ping(context.Background())
	if err != ib.ErrClientClosed {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestMockIBClient_WithHistoricalDataResponse(t *testing.T) {
	resp := ib.MockHistoricalDataResponse("AAPL",
		ib.MockHistoricalDataBar(1719792000000, 185.50, 186.75, 184.90, 186.20, 50000000),
		ib.MockHistoricalDataBar(1719878400000, 186.00, 187.00, 185.50, 186.50, 45000000),
	)

	mock := ib.NewMockClient().WithHistoricalDataResponse(resp)

	result, err := mock.HistoricalData(context.Background(), ib.HistoricalDataRequest{
		Conid: "265598",
	})

	if err != nil {
		t.Fatalf("should not error: %v", err)
	}
	if result.Symbol != "AAPL" {
		t.Errorf("expected symbol 'AAPL', got '%s'", result.Symbol)
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 bars, got %d", len(result.Data))
	}
}

func TestMockIBClient_CallRecording(t *testing.T) {
	mock := ib.NewMockClient()

	// Make some calls
	mock.Ping(context.Background())
	mock.IsConnected(context.Background())
	mock.HistoricalData(context.Background(), ib.HistoricalDataRequest{
		Conid: "123", Exchange: "SMART", Period: "1d", Bar: "5mins",
	})

	// Verify all calls recorded
	if mock.TotalCalls() != 3 {
		t.Errorf("expected 3 calls, got %d", mock.TotalCalls())
	}

	// Verify specific calls
	calls := mock.Calls
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls in Calls slice, got %d", len(calls))
	}

	if calls[0].Method != "Ping" {
		t.Errorf("first call should be Ping, got '%s'", calls[0].Method)
	}

	if calls[2].Method != "HistoricalData" {
		t.Errorf("third call should be HistoricalData, got '%s'", calls[2].Method)
	}
}
