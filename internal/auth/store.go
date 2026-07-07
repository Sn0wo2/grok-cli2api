package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"github.com/Sn0wo2/grok-cli2api/internal/config"
)

type Store struct {
	dir string
	log *slog.Logger
}

func NewStore(log *slog.Logger) *Store {
	return &Store{dir: config.AuthsDir(), log: log}
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) withFileLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire lock %s: %w", path, err)
	}
	defer func() {
		_ = lock.Unlock()
		_ = os.Remove(lockPath)
	}()
	return fn()
}

func (s *Store) Upsert(entry Entry) error {
	path := s.filePathFor(entry)
	return s.withFileLock(path, func() error {
		if err := s.saveFile(path, AuthFile{config.ScopeKey(): entry}); err != nil {
			return err
		}
		s.log.Info("auth saved", "path", path, "email", entry.Email)
		return nil
	})
}

func (s *Store) SaveRecord(rec AccountRecord) error {
	return s.withFileLock(rec.FilePath, func() error {
		file, err := s.loadFile(rec.FilePath)
		if err != nil {
			return err
		}
		file[rec.ScopeKey] = rec.Entry
		return s.saveFile(rec.FilePath, file)
	})
}

func (s *Store) Remove(selector string) error {
	rec, err := s.GetRecord(selector)
	if err != nil {
		return err
	}
	if err := os.Remove(rec.FilePath); err != nil {
		return fmt.Errorf("remove %s: %w", rec.FilePath, err)
	}
	s.log.Info("account removed", "path", rec.FilePath, "email", rec.Entry.Email)
	return nil
}

func (s *Store) List() ([]AccountInfo, error) {
	records, err := s.scanAll()
	if err != nil {
		return nil, err
	}
	out := make([]AccountInfo, len(records))
	for i, rec := range records {
		out[i] = AccountInfo{
			FilePath:  rec.FilePath,
			ScopeKey:  rec.ScopeKey,
			Email:     rec.Entry.Email,
			UserID:    rec.Entry.UserID,
			FirstName: rec.Entry.FirstName,
			ExpiresAt: rec.Entry.ExpiresAt,
		}
	}
	return out, nil
}

func (s *Store) GetRecord(selector string) (AccountRecord, error) {
	records, err := s.scanAll()
	if err != nil {
		return AccountRecord{}, err
	}
	if len(records) == 0 {
		return AccountRecord{}, fmt.Errorf("no accounts in %s", s.dir)
	}
	return resolveRecord(records, selector)
}

func (s *Store) scanAll() ([]AccountRecord, error) {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return nil, fmt.Errorf("create auths dir: %w", err)
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read auths dir: %w", err)
	}

	var records []AccountRecord
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".json") {
			continue
		}
		path := filepath.Join(s.dir, ent.Name())
		file, err := s.loadFile(path)
		if err != nil {
			s.log.Warn("skip invalid auth file", "path", path, "error", err)
			continue
		}
		for scopeKey, entry := range file {
			if entry.Key == "" {
				continue
			}
			records = append(records, AccountRecord{
				FilePath: path,
				ScopeKey: scopeKey,
				Entry:    entry,
			})
		}
	}
	return records, nil
}

func (s *Store) loadFile(path string) (AuthFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return AuthFile{}, nil
	}
	var file AuthFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return file, nil
}

func (s *Store) saveFile(path string, file AuthFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (s *Store) filePathFor(entry Entry) string {
	name := sanitizeFilename(entry.Email)
	if name == "" {
		name = entry.UserID
	}
	return filepath.Join(s.dir, name+".json")
}

func sanitizeFilename(email string) string {
	var b strings.Builder
	for _, r := range email {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func resolveRecord(records []AccountRecord, selector string) (AccountRecord, error) {
	if selector == "" || selector == "default" {
		for _, rec := range records {
			if rec.ScopeKey == config.ScopeKey() {
				return rec, nil
			}
		}
		return records[0], nil
	}

	needle := strings.ToLower(selector)
	for _, rec := range records {
		if strings.EqualFold(rec.Entry.Email, selector) ||
			strings.EqualFold(rec.Entry.UserID, selector) ||
			strings.EqualFold(rec.FilePath, selector) ||
			strings.Contains(strings.ToLower(rec.FilePath), needle) ||
			strings.Contains(strings.ToLower(rec.ScopeKey), needle) {
			return rec, nil
		}
	}
	return AccountRecord{}, fmt.Errorf("account not found: %s", selector)
}

func (e Entry) IsExpired(earlySecs int) bool {
	for _, layout := range []string{"2006-01-02T15:04:05.000000000Z", time.RFC3339Nano} {
		if t, err := time.Parse(layout, e.ExpiresAt); err == nil {
			return time.Until(t) < time.Duration(earlySecs)*time.Second
		}
	}
	return true
}
