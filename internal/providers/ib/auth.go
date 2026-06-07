package ib

import (
	"context"
	"net/http"

	ibapi "github.com/bishop-bot/ibapi-go"
	ibgateway "github.com/bishop-bot/ibgateway-go"
	"github.com/bishop-bot/datajobs/internal/config"
)

// Second factor method constants.
const (
	SecondFactorSMS          = "SMS"
	SecondFactorTOTP         = "TOTP"
	SecondFactorIBKeyAndroid = "IBKeyAndroid"
	SecondFactorIBKeyIOS     = "IBKeyIOS"
)

// Authenticator defines the interface for IB authentication.
// This allows for mocking in tests and alternative auth implementations.
type Authenticator interface {
	// Authenticate performs the full authentication flow.
	Authenticate(ctx context.Context) error

	// IsAuthenticated checks if the session is currently authenticated.
	IsAuthenticated(ctx context.Context) (bool, error)

	// HTTPClient returns the authenticated HTTP client.
	HTTPClient() *http.Client

	// Close releases authentication resources.
	Close() error
}

// IBGatewayAuthenticator wraps the ibgateway-go library for authentication.
type IBGatewayAuthenticator struct {
	authenticator *ibgateway.Authenticator
	baseURL       string
}

// NewIBGatewayAuthenticator creates a new authenticator using ibgateway-go.
// Returns nil if credentials are not configured.
func NewIBGatewayAuthenticator(cfg config.IBConfig) (Authenticator, error) {
	if cfg.Username == "" || cfg.Password == "" {
		return nil, nil // Auth not configured
	}

	// Build auth config
	authCfg := ibgateway.AuthConfig{
		Username: cfg.Username,
		Password: cfg.Password,
		BaseURL:  cfg.BaseURL,
	}

	// Set second factor method
	switch cfg.SecondFactorMethod {
	case SecondFactorTOTP:
		authCfg.SecondFactorMethod = ibgateway.TOTP
		authCfg.TOTPSecret = cfg.TOTPSecret
	case SecondFactorIBKeyAndroid:
		authCfg.SecondFactorMethod = ibgateway.IBKeyAndroid
	case SecondFactorIBKeyIOS:
		authCfg.SecondFactorMethod = ibgateway.IBKeyIOS
	case SecondFactorSMS, "":
		authCfg.SecondFactorMethod = ibgateway.SMS
	}

	auth, err := ibgateway.NewAuthenticator(authCfg)
	if err != nil {
		return nil, err
	}

	return &IBGatewayAuthenticator{
		authenticator: auth,
		baseURL:       cfg.BaseURL,
	}, nil
}

// Authenticate performs the full authentication flow.
func (a *IBGatewayAuthenticator) Authenticate(ctx context.Context) error {
	// The ibgateway library handles the full flow including 2FA
	return a.authenticator.Authenticate()
}

// IsAuthenticated checks if the session is authenticated by querying auth status.
func (a *IBGatewayAuthenticator) IsAuthenticated(ctx context.Context) (bool, error) {
	session := a.authenticator.GetSession()
	
	// Create a temporary client to check auth status
	client, _ := ibapi.NewClient(ibapi.WithHTTPClient(session.GetClient()))
	resp, err := client.Session().AuthStatus(ctx)
	if err != nil {
		return false, err
	}
	return resp.Authenticated, nil
}

// HTTPClient returns the authenticated HTTP client.
func (a *IBGatewayAuthenticator) HTTPClient() *http.Client {
	return a.authenticator.GetSession().GetClient()
}

// Close releases authentication resources.
func (a *IBGatewayAuthenticator) Close() error {
	return a.authenticator.Close()
}