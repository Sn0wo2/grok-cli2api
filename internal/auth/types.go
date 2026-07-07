package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthFile map[string]Entry

type Entry struct {
	Key                       string `json:"key"`
	AuthMode                  string `json:"auth_mode"`
	CreateTime                string `json:"create_time"`
	UserID                    string `json:"user_id"`
	Email                     string `json:"email"`
	FirstName                 string `json:"first_name"`
	ProfileImageAssetID       string `json:"profile_image_asset_id,omitempty"`
	PrincipalType             string `json:"principal_type"`
	PrincipalID               string `json:"principal_id"`
	TeamID                    string `json:"team_id"`
	CodingDataRetentionOptOut bool   `json:"coding_data_retention_opt_out"`
	RefreshToken              string `json:"refresh_token"`
	ExpiresAt                 string `json:"expires_at"`
	OIDCIssuer                string `json:"oidc_issuer"`
	OIDCClientID              string `json:"oidc_client_id"`
}

type AccountRecord struct {
	FilePath string
	ScopeKey string
	Entry    Entry
}

type AccountInfo struct {
	FilePath  string
	ScopeKey  string
	Email     string
	UserID    string
	FirstName string
	ExpiresAt string
}

type UserInfo struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Picture       string `json:"picture"`
}

type BillingInfo struct {
	Email                  string
	PeriodEnd              string
	CreditUsagePercent     float64
	CreditRemainingPercent float64
	OnDemandCap            int64
	OnDemandUsed           int64
	OnDemandRemaining      int64
	PrepaidBalance         int64
	Products               []ProductUsage
}

type ProductUsage struct {
	Product          string
	UsagePercent     float64
	RemainingPercent float64
}

type jwtClaims struct {
	jwt.RegisteredClaims
	PrincipalType string `json:"principal_type"`
	PrincipalID   string `json:"principal_id"`
	TeamID        string `json:"team_id"`
}

type billingResponse struct {
	Config struct {
		CurrentPeriod struct {
			End string `json:"end"`
		} `json:"currentPeriod"`
		CreditUsagePercent float64 `json:"creditUsagePercent"`
		OnDemandCap        struct {
			Val int64 `json:"val"`
		} `json:"onDemandCap"`
		OnDemandUsed struct {
			Val int64 `json:"val"`
		} `json:"onDemandUsed"`
		PrepaidBalance struct {
			Val int64 `json:"val"`
		} `json:"prepaidBalance"`
		ProductUsage []struct {
			Product      string  `json:"product"`
			UsagePercent float64 `json:"usagePercent"`
		} `json:"productUsage"`
	} `json:"config"`
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000000Z")
}
