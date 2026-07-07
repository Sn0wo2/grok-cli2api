package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/Sn0wo2/grok-cli2api/internal/config"
)

type Client struct {
	oauth *oauth2.Config
	http  *http.Client
	log   *slog.Logger
}

func NewClient(log *slog.Logger) *Client {
	return &Client{
		oauth: &oauth2.Config{
			ClientID: config.OIDCClientID,
			Endpoint: oauth2.Endpoint{
				AuthURL:       config.OIDCIssuer + "/oauth2/authorize",
				TokenURL:      config.OIDCIssuer + "/oauth2/token",
				DeviceAuthURL: config.OIDCIssuer + "/oauth2/device/code",
				AuthStyle:     oauth2.AuthStyleInParams,
			},
			Scopes: strings.Fields(config.OIDCScope),
		},
		http: &http.Client{Timeout: 30 * time.Second},
		log:  log,
	}
}

func (c *Client) oauthCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, c.http)
}

func (c *Client) oauthWithRedirect(redirectURI string) *oauth2.Config {
	cfg := *c.oauth
	cfg.RedirectURL = redirectURI
	return &cfg
}

func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI, verifier string) (*oauth2.Token, error) {
	cfg := c.oauthWithRedirect(redirectURI)
	tok, err := cfg.Exchange(c.oauthCtx(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	return tok, nil
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	tok, err := c.oauth.TokenSource(c.oauthCtx(ctx), &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	return tok, nil
}

func (c *Client) FetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.OIDCIssuer+"/oauth2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo status %d: %s", resp.StatusCode, string(raw))
	}

	var info UserInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo: %w", err)
	}
	return &info, nil
}

func (c *Client) CompleteLogin(ctx context.Context, store *Store, token *oauth2.Token) (*Entry, error) {
	info, err := c.FetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	entry, err := entryFromToken(token, info)
	if err != nil {
		return nil, err
	}
	if err := store.Upsert(entry); err != nil {
		return nil, fmt.Errorf("save auth: %w", err)
	}
	c.log.Info("login success", "email", entry.Email, "user_id", entry.UserID, "expires_at", entry.ExpiresAt)
	return &entry, nil
}

func (c *Client) FetchBilling(rec AccountRecord) (*BillingInfo, error) {
	req, err := http.NewRequest(http.MethodGet, config.ProxyBaseURL()+"/billing?format=credits", nil)
	if err != nil {
		return nil, err
	}
	SetGrokHeaders(req, rec, "")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("billing request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("billing status %d: %s", resp.StatusCode, string(raw))
	}

	var out billingResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse billing: %w", err)
	}

	cfg := out.Config
	info := &BillingInfo{
		Email:                  rec.Entry.Email,
		CreditUsagePercent:     cfg.CreditUsagePercent,
		CreditRemainingPercent: 100 - cfg.CreditUsagePercent,
		OnDemandCap:            cfg.OnDemandCap.Val,
		OnDemandUsed:           cfg.OnDemandUsed.Val,
		OnDemandRemaining:      cfg.OnDemandCap.Val - cfg.OnDemandUsed.Val,
		PrepaidBalance:         cfg.PrepaidBalance.Val,
		PeriodEnd:              cfg.CurrentPeriod.End,
	}
	for _, p := range cfg.ProductUsage {
		info.Products = append(info.Products, ProductUsage{
			Product:          p.Product,
			UsagePercent:     p.UsagePercent,
			RemainingPercent: 100 - p.UsagePercent,
		})
	}
	return info, nil
}

func SetGrokHeaders(req *http.Request, rec AccountRecord, model string) {
	req.Header.Set("Authorization", "Bearer "+rec.Entry.Key)
	req.Header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	req.Header.Set("x-grok-client-version", config.CLIVersion)
	req.Header.Set("x-grok-client-identifier", uuid.New().String())
	req.Header.Set("x-grok-req-id", uuid.New().String())
	req.Header.Set("x-grok-client-surface", "grok-build")
	if model != "" {
		req.Header.Set("x-grok-model-override", model)
	}
	if rec.Entry.UserID != "" {
		req.Header.Set("x-grok-user-id", rec.Entry.UserID)
	}
	if rec.Entry.Email != "" {
		req.Header.Set("x-email", rec.Entry.Email)
	}
}

func entryFromToken(token *oauth2.Token, info *UserInfo) (Entry, error) {
	claims, err := parseJWTClaims(token.AccessToken)
	if err != nil {
		return Entry{}, err
	}

	now := time.Now().UTC()
	expiresAt := token.Expiry
	if expiresAt.IsZero() {
		expiresAt = now.Add(time.Hour)
	}

	firstName := info.GivenName
	if firstName == "" {
		firstName = info.Name
	}

	return Entry{
		Key:                 token.AccessToken,
		AuthMode:            "oidc",
		CreateTime:          formatTime(now),
		UserID:              claims.Subject,
		Email:               info.Email,
		FirstName:           firstName,
		ProfileImageAssetID: info.Picture,
		PrincipalType:       claims.PrincipalType,
		PrincipalID:         claims.PrincipalID,
		TeamID:              claims.TeamID,
		RefreshToken:        token.RefreshToken,
		ExpiresAt:           formatTime(expiresAt),
		OIDCIssuer:          config.OIDCIssuer,
		OIDCClientID:        config.OIDCClientID,
	}, nil
}

func parseJWTClaims(accessToken string) (*jwtClaims, error) {
	var claims jwtClaims
	if _, _, err := jwt.NewParser().ParseUnverified(accessToken, &claims); err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("jwt missing sub claim")
	}
	if claims.PrincipalID == "" {
		claims.PrincipalID = claims.Subject
	}
	if claims.PrincipalType == "" {
		claims.PrincipalType = "User"
	}
	return &claims, nil
}

func applyTokenRefresh(entry Entry, token *oauth2.Token) Entry {
	now := time.Now().UTC()
	entry.Key = token.AccessToken
	entry.CreateTime = formatTime(now)
	if !token.Expiry.IsZero() {
		entry.ExpiresAt = formatTime(token.Expiry)
	}
	if token.RefreshToken != "" {
		entry.RefreshToken = token.RefreshToken
	}
	return entry
}

func formatProducts(products []ProductUsage) string {
	parts := make([]string, len(products))
	for i, p := range products {
		parts[i] = fmt.Sprintf("%s:%.1f%%left", p.Product, p.RemainingPercent)
	}
	return strings.Join(parts, ", ")
}
