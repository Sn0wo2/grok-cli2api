package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/Sn0wo2/grok-cli2api/internal/config"
)

type Pool struct {
	store  *Store
	client *Client
	log    *slog.Logger
	cursor uint64
}

func NewPool(store *Store, client *Client, log *slog.Logger) *Pool {
	return &Pool{store: store, client: client, log: log}
}

func (p *Pool) Accounts() ([]AccountRecord, error) {
	return p.store.scanAll()
}

func (p *Pool) Refresh(selector string) (*Entry, error) {
	rec, err := p.store.GetRecord(selector)
	if err != nil {
		return nil, err
	}
	return p.refreshRecord(context.Background(), rec)
}

func (p *Pool) RunAutoRefresh(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	p.tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

func (p *Pool) tick() {
	records, err := p.store.scanAll()
	if err != nil {
		p.log.Warn("periodic scan failed", "error", err)
		return
	}

	for _, rec := range records {
		if !rec.Entry.IsExpired(config.TokenEarlyRefreshSecs) {
			continue
		}
		if _, err := p.refreshRecord(context.Background(), rec); err != nil {
			p.log.Warn("auto-refresh failed", "email", rec.Entry.Email, "error", err)
		}
	}

	for _, rec := range records {
		info, err := p.client.FetchBilling(rec)
		if err != nil {
			p.log.Warn("quota fetch failed", "email", rec.Entry.Email, "error", err)
			continue
		}
		attrs := []any{
			"email", info.Email,
			"credit_remaining", fmt.Sprintf("%.1f%%", info.CreditRemainingPercent),
			"credit_used", fmt.Sprintf("%.1f%%", info.CreditUsagePercent),
			"prepaid", info.PrepaidBalance,
			"period_end", info.PeriodEnd,
		}
		if info.OnDemandCap > 0 {
			attrs = append(attrs, "on_demand_remaining", info.OnDemandRemaining)
		}
		if len(info.Products) > 0 {
			attrs = append(attrs, "products", formatProducts(info.Products))
		}
		p.log.Info("account quota", attrs...)
	}
}

func (p *Pool) WithRetry(fn func(AccountRecord) (int, error)) error {
	records, err := p.store.scanAll()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("no accounts in pool (%s)", p.store.Dir())
	}

	start := int(atomic.AddUint64(&p.cursor, 1) % uint64(len(records)))
	var lastErr error
	for i := 0; i < len(records); i++ {
		rec := records[(start+i)%len(records)]

		if err := p.ensureFresh(&rec); err != nil {
			p.log.Warn("skip account, refresh failed", "email", rec.Entry.Email, "error", err)
			lastErr = err
			continue
		}

		status, err := fn(rec)
		if err == nil && status < 400 {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("upstream status %d", status)
		}

		if status == 401 || status == 403 {
			if entry, refreshErr := p.refreshRecord(context.Background(), rec); refreshErr == nil {
				rec.Entry = *entry
				if status, err = fn(rec); err == nil && status < 400 {
					return nil
				}
			}
			p.log.Warn("account unauthorized, trying next", "email", rec.Entry.Email, "status", status)
			continue
		}
		if status >= 500 {
			p.log.Warn("upstream error, trying next account", "email", rec.Entry.Email, "status", status)
			continue
		}
		if status >= 400 {
			return nil
		}
		return lastErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all accounts failed")
	}
	return lastErr
}

func (p *Pool) ensureFresh(rec *AccountRecord) error {
	if !rec.Entry.IsExpired(config.TokenEarlyRefreshSecs) {
		return nil
	}
	entry, err := p.refreshRecord(context.Background(), *rec)
	if err != nil {
		return err
	}
	rec.Entry = *entry
	return nil
}

func (p *Pool) refreshRecord(ctx context.Context, rec AccountRecord) (*Entry, error) {
	token, err := p.client.RefreshToken(ctx, rec.Entry.RefreshToken)
	if err != nil {
		return nil, err
	}
	rec.Entry = applyTokenRefresh(rec.Entry, token)
	if err := p.store.SaveRecord(rec); err != nil {
		return nil, err
	}
	p.log.Info("token refreshed", "email", rec.Entry.Email, "expires_at", rec.Entry.ExpiresAt)
	return &rec.Entry, nil
}
