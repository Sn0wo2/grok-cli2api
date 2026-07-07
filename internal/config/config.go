package config

import "os"

const (
	OIDCIssuer   = "https://auth.x.ai"
	OIDCClientID = "b1a00492-073a-47ea-816f-4c329264a828"
	CLIVersion   = "0.2.87"

	OIDCScope = "openid profile email offline_access grok-cli:access api:access conversations:read conversations:write"

	DefaultProxyBaseURL = "https://cli-chat-proxy.grok.com/v1"
	DefaultAuthsDir     = "./data/auths"
	DefaultServePort    = "8317"

	TokenEarlyRefreshSecs = 300
)

func ScopeKey() string {
	return OIDCIssuer + "::" + OIDCClientID
}

func AuthsDir() string {
	return envOr("GROK_AUTHS_DIR", DefaultAuthsDir)
}

func ProxyBaseURL() string {
	if u := os.Getenv("GROK_CLI_CHAT_PROXY_BASE_URL"); u != "" {
		return u
	}
	return envOr("GROK_PROXY_BASE_URL", DefaultProxyBaseURL)
}

func ServePort() string {
	return envOr("GROK_SERVE_PORT", DefaultServePort)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
