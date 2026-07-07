package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sn0wo2/grok-cli2api/internal/auth"
	"github.com/Sn0wo2/grok-cli2api/internal/config"
	"github.com/Sn0wo2/grok-cli2api/internal/server"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	os.Exit(newApp(log).execute())
}

type app struct {
	log    *slog.Logger
	store  *auth.Store
	client *auth.Client
	login  *auth.Login
}

func newApp(log *slog.Logger) *app {
	store := auth.NewStore(log)
	client := auth.NewClient(log)
	return &app{
		log:    log,
		store:  store,
		client: client,
		login:  auth.NewLogin(client, store, log),
	}
}

func (a *app) execute() int {
	port := config.ServePort()
	root := &cobra.Command{
		Use:           "grok-cli2api",
		Short:         "Grok CLI OAuth login & OpenAI-compatible API proxy",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			return a.runServe(port)
		},
	}
	root.Flags().StringVarP(&port, "port", "p", config.ServePort(), "Server port")
	root.AddCommand(
		a.loginCmd(),
		a.accountCmd("logout", "Remove an account file from pool", a.runLogout),
		a.listCmd(),
		a.accountCmd("refresh", "Refresh access token manually", a.runRefresh),
		a.accountCmd("whoami", "Show account info", a.runWhoami),
		a.serveCmd(),
	)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	return 0
}

func (a *app) accountCmd(use, short string, run func(string) error) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(*cobra.Command, []string) error {
			return run(account)
		},
	}
	cmd.Flags().StringVarP(&account, "account", "a", "", "Target account (email or user_id)")
	return cmd
}

func (a *app) serveCmd() *cobra.Command {
	port := config.ServePort()
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start API proxy server",
		RunE: func(*cobra.Command, []string) error {
			return a.runServe(port)
		},
	}
	cmd.Flags().StringVarP(&port, "port", "p", config.ServePort(), "Server port")
	return cmd
}

func (a *app) loginCmd() *cobra.Command {
	var device bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Browser PKCE login (default) or device code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
			defer cancel()

			fmt.Printf("Auths dir: %s\n", a.store.Dir())

			var entry *auth.Entry
			var err error
			if device {
				entry, err = a.login.Device(ctx)
			} else {
				entry, err = a.login.Browser(ctx)
			}
			if err != nil {
				a.log.Error("login failed", "error", err)
				return fmt.Errorf("login failed: %w", err)
			}

			fmt.Printf("Logged in as %s (%s)\n", entry.FirstName, entry.Email)
			fmt.Printf("Token expires at %s\n", entry.ExpiresAt)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&device, "device", "d", false, "Use device code flow")
	return cmd
}

func (a *app) listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all accounts in pool",
		RunE: func(*cobra.Command, []string) error {
			return a.runList()
		},
	}
}

func (a *app) runLogout(account string) error {
	if err := a.store.Remove(account); err != nil {
		a.log.Error("logout failed", "error", err)
		return fmt.Errorf("logout failed: %w", err)
	}
	if account == "" {
		fmt.Println("Removed account.")
		return nil
	}
	fmt.Printf("Removed account: %s\n", account)
	return nil
}

func (a *app) runList() error {
	accounts, err := a.store.List()
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		fmt.Printf("No accounts in %s\n", a.store.Dir())
		fmt.Println("Run: grok-cli2api login")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "EMAIL\tUSER_ID\tEXPIRES_AT\tFILE")
	for _, acc := range accounts {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", acc.Email, shortID(acc.UserID), acc.ExpiresAt, acc.FilePath)
	}
	return w.Flush()
}

func (a *app) runRefresh(account string) error {
	pool := auth.NewPool(a.store, a.client, a.log)
	entry, err := pool.Refresh(account)
	if err != nil {
		a.log.Error("refresh failed", "error", err)
		return fmt.Errorf("refresh failed: %w", err)
	}
	fmt.Printf("Refreshed %s, expires at %s\n", entry.Email, entry.ExpiresAt)
	return nil
}

func (a *app) runWhoami(account string) error {
	rec, err := a.store.GetRecord(account)
	if err != nil {
		return err
	}
	e := rec.Entry
	fmt.Printf("Email:       %s\n", e.Email)
	fmt.Printf("Name:        %s\n", e.FirstName)
	fmt.Printf("User ID:     %s\n", e.UserID)
	fmt.Printf("Team ID:     %s\n", e.TeamID)
	fmt.Printf("Expires at:  %s\n", e.ExpiresAt)
	fmt.Printf("Auth file:   %s\n", rec.FilePath)
	fmt.Printf("Scope key:   %s\n", rec.ScopeKey)
	return nil
}

func (a *app) runServe(port string) error {
	pool := auth.NewPool(a.store, a.client, a.log)
	accounts, err := pool.Accounts()
	if err != nil {
		return fmt.Errorf("load pool failed: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts in %s, run: grok-cli2api login", a.store.Dir())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pool.RunAutoRefresh(ctx)

	a.log.Info("account pool", "accounts", len(accounts), "dir", a.store.Dir())
	a.log.Info("periodic task enabled", "interval", "60s", "early_secs", config.TokenEarlyRefreshSecs)

	if err := server.New(a.log, pool).Run(":" + port); err != nil {
		a.log.Error("server failed", "error", err)
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "..."
}
