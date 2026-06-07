package ib

import (
	"context"
	"errors"
	"net/http"
	"testing"

	ibapi "github.com/bishop-bot/ibapi-go"
)

func TestMockClient_WithNotAuthenticated(t *testing.T) {
	mock := NewMockClient().WithNotAuthenticated()

	// Verify auth status is not authenticated
	status, err := mock.AuthStatus(context.Background())
	if err != nil {
		t.Fatalf("AuthStatus should not error: %v", err)
	}
	if status.Authenticated {
		t.Error("expected Authenticated to be false")
	}

	// Verify authenticator is also not authenticated
	if mock.Authenticator.Authenticated {
		t.Error("expected MockAuthenticator.Authenticated to be false")
	}
}

func TestMockClient_AuthError(t *testing.T) {
	mock := NewMockClient().WithAuthError(context.DeadlineExceeded)

	err := mock.Authenticator.Authenticate(context.Background())
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestMockClient_AuthCallCount(t *testing.T) {
	mock := NewMockClient()

	// Authenticate twice
	mock.Authenticator.Authenticate(context.Background())
	mock.Authenticator.Authenticate(context.Background())

	if mock.Authenticator.AuthCallCount.Load() != 2 {
		t.Errorf("expected AuthCallCount to be 2, got %d", mock.Authenticator.AuthCallCount.Load())
	}
}

func TestMockClient_EnsureAuthenticated_NotImplemented(t *testing.T) {
	mock := NewMockClient()

	// MockClient doesn't implement AuthAwareProvider
	// This test just documents the current behavior
	_, ok := interface{}(mock).(AuthAwareProvider)
	if ok {
		t.Error("MockClient should not implement AuthAwareProvider for this test")
	}
}

func TestMockClient_AuthStatusRecording(t *testing.T) {
	mock := NewMockClient().WithNotAuthenticated()

	// Call AuthStatus multiple times
	mock.AuthStatus(context.Background())
	mock.AuthStatus(context.Background())

	// Verify calls were recorded
	if !mock.AssertCalled("AuthStatus") {
		t.Error("AuthStatus should have been called")
	}
	if mock.TotalCalls() < 2 {
		t.Errorf("expected at least 2 calls, got %d", mock.TotalCalls())
	}
}

func TestMockAuthenticator_HTTPClient(t *testing.T) {
	auth := MockAuthenticator{}

	// Should return nil by default
	if auth.HTTPClient() != nil {
		t.Error("expected nil HTTPClient")
	}

	// Set a custom client
	customClient := &http.Client{}
	auth.httpClient = customClient

	if auth.HTTPClient() != customClient {
		t.Error("expected custom HTTPClient")
	}
}

func TestMockAuthenticator_IsAuthenticated(t *testing.T) {
	auth := MockAuthenticator{Authenticated: false}

	authenticated, err := auth.IsAuthenticated(context.Background())
	if err != nil {
		t.Fatalf("IsAuthenticated should not error: %v", err)
	}
	if authenticated {
		t.Error("expected not authenticated")
	}

	// Set authenticated
	auth.Authenticated = true
	authenticated, err = auth.IsAuthenticated(context.Background())
	if err != nil {
		t.Fatalf("IsAuthenticated should not error: %v", err)
	}
	if !authenticated {
		t.Error("expected authenticated")
	}
}

func TestMockAuthenticator_AuthError(t *testing.T) {
	auth := MockAuthenticator{AuthError: errors.New("auth failed")}

	_, err := auth.IsAuthenticated(context.Background())
	if err == nil {
		t.Error("expected error from IsAuthenticated")
	}

	err = auth.Authenticate(context.Background())
	if err == nil {
		t.Error("expected error from Authenticate")
	}
}

func TestMockClient_IsConnected(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		mock := NewMockClient().WithConnected(true)
		if !mock.IsConnected(context.Background()) {
			t.Error("expected connected")
		}
	})

	t.Run("not connected due to ping error", func(t *testing.T) {
		mock := NewMockClient().WithPingError(errors.New("ping failed"))
		if mock.IsConnected(context.Background()) {
			t.Error("expected not connected")
		}
	})

	t.Run("closed", func(t *testing.T) {
		mock := NewMockClient()
		mock.Close()
		if mock.IsConnected(context.Background()) {
			t.Error("expected not connected when closed")
		}
	})
}

func TestMockClient_Close(t *testing.T) {
	mock := NewMockClient()

	err := mock.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}

	// Verify closed state
	if !mock.closed {
		t.Error("expected closed to be true")
	}

	// Verify can be called multiple times without error
	err = mock.Close()
	if err != nil {
		t.Errorf("Close should not error on second call: %v", err)
	}
}

func TestMockAuthenticator_CloseError(t *testing.T) {
	auth := MockAuthenticator{CloseError: errors.New("close failed")}

	err := auth.Close()
	if err == nil {
		t.Error("expected error from Close")
	}
}

func TestMockClient_AuthStatusError(t *testing.T) {
	mock := NewMockClient()
	mock.AuthStatusError = errors.New("auth status error")

	_, err := mock.AuthStatus(context.Background())
	if err == nil {
		t.Error("expected error from AuthStatus")
	}
}

func TestMockClient_WithAuthStatusResponse(t *testing.T) {
	resp := &ibapi.AuthStatusResponse{
		Authenticated: true,
		Connected:     true,
		Message:      "test",
	}

	mock := NewMockClient().WithAuthStatusResponse(resp)

	status, err := mock.AuthStatus(context.Background())
	if err != nil {
		t.Fatalf("AuthStatus should not error: %v", err)
	}
	if status.Authenticated != true {
		t.Error("expected Authenticated to be true")
	}
	if status.Message != "test" {
		t.Errorf("expected Message 'test', got '%s'", status.Message)
	}
}