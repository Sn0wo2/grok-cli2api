package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

type Login struct {
	client *Client
	store  *Store
	log    *slog.Logger
}

func NewLogin(client *Client, store *Store, log *slog.Logger) *Login {
	return &Login{client: client, store: store, log: log}
}

func (l *Login) Browser(ctx context.Context) (*Entry, error) {
	verifier := oauth2.GenerateVerifier()
	state := uuid.NewString()

	port, err := freePort()
	if err != nil {
		return nil, err
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "Invalid state parameter.", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth state mismatch")
			return
		}
		if oauthErr := q.Get("error"); oauthErr != "" {
			http.Error(w, "Authorization failed: "+oauthErr, http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth error: %s (%s)", oauthErr, q.Get("error_description"))
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code.", http.StatusBadRequest)
			errCh <- fmt.Errorf("missing authorization code")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, successHTML)
		codeCh <- code
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()

	authURL := l.client.oauthWithRedirect(redirectURI).AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	l.log.Info("opening browser for login", "url", authURL)
	fmt.Println("Opening browser for xAI login...")
	fmt.Println("If the browser does not open, visit:")
	fmt.Println(authURL)
	if err := browser.OpenURL(authURL); err != nil {
		l.log.Warn("failed to open browser", "error", err)
	}

	select {
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return nil, ctx.Err()
	case err := <-errCh:
		_ = srv.Shutdown(context.Background())
		return nil, err
	case code := <-codeCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)

		token, err := l.client.ExchangeCode(ctx, code, redirectURI, verifier)
		if err != nil {
			return nil, err
		}
		return l.client.CompleteLogin(ctx, l.store, token)
	}
}

func (l *Login) Device(ctx context.Context) (*Entry, error) {
	device, err := l.client.oauth.DeviceAuth(l.client.oauthCtx(ctx))
	if err != nil {
		return nil, fmt.Errorf("device code: %w", err)
	}

	fmt.Println("\nDevice authorization required.")
	fmt.Printf("  User code:  %s\n", device.UserCode)
	fmt.Printf("  Open:       %s\n", device.VerificationURIComplete)
	if device.VerificationURIComplete == "" {
		fmt.Printf("  Or visit:   %s\n", device.VerificationURI)
		fmt.Printf("  Enter code: %s\n", device.UserCode)
	} else if err := browser.OpenURL(device.VerificationURIComplete); err != nil {
		l.log.Warn("failed to open browser", "error", err)
	}
	fmt.Println("\nWaiting for authorization...")

	token, err := l.client.oauth.DeviceAccessToken(l.client.oauthCtx(ctx), device)
	if err != nil {
		return nil, fmt.Errorf("device token: %w", err)
	}
	return l.client.CompleteLogin(ctx, l.store, token)
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

const successHTML = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Login Successful</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0a0a0a;color:#e5e5e5}
.card{text-align:center;padding:2rem;border-radius:12px;background:#171717;border:1px solid #333}h1{color:#22c55e}</style></head>
<body><div class="card"><h1>Login Successful</h1><p>You can close this window and return to the terminal.</p></div></body></html>`
