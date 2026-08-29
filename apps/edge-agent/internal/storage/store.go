// Package storage owns the Edge agent's durable SQLite and filesystem state.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

const (
	currentSchemaVersion = 2
	defaultBusyTimeout   = 5 * time.Second
	maxEventPayloadBytes = 1 << 20
)

// Options locates the SQLite metadata database and content-addressed spool.
type Options struct {
	DatabasePath   string
	SpoolDirectory string
	BusyTimeout    time.Duration
}

// Store owns one serialized SQLite handle and the local CAS root.
type Store struct {
	db          *sql.DB
	options     Options
	readOnly    bool
	now         func() time.Time
	beforeWrite func() error
}

// Open initializes or opens the exact current development schema. Existing
// unknown schemas are rejected instead of silently migrated or reset.
func Open(ctx context.Context, options Options) (*Store, error) {
	options = options.withDefaults()
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	if err := ensureDirectory(filepath.Dir(options.DatabasePath), 0o700); err != nil {
		return nil, err
	}
	if err := ensureDirectory(options.SpoolDirectory, 0o700); err != nil {
		return nil, err
	}
	if err := ensureDatabaseFile(options.DatabasePath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", options.DatabasePath)
	if err != nil {
		return nil, classifyError("open edge database", err)
	}
	store := &Store{db: db, options: options, now: time.Now}
	if err := store.prepare(ctx, false); err != nil {
		db.Close()
		return nil, err
	}
	if err := initializeSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := tightenDatabasePermissions(options.DatabasePath); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

// OpenReadOnly opens existing state for status and diagnostics without creating,
// migrating, resetting, or otherwise mutating local data.
func OpenReadOnly(ctx context.Context, options Options) (*Store, error) {
	options = options.withDefaults()
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	if err := validateRegularNonSymlink(options.DatabasePath); err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: options.DatabasePath}
	query := uri.Query()
	query.Set("mode", "ro")
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, classifyError("open edge database read-only", err)
	}
	store := &Store{db: db, options: options, readOnly: true, now: time.Now}
	if err := store.prepare(ctx, true); err != nil {
		db.Close()
		return nil, err
	}
	if err := verifySchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) prepare(ctx context.Context, readOnly bool) error {
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	if err := s.db.PingContext(ctx); err != nil {
		return classifyError("ping edge database", err)
	}
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = " + strconv.FormatInt(s.options.BusyTimeout.Milliseconds(), 10),
	}
	if readOnly {
		pragmas = append(pragmas, "PRAGMA query_only = ON")
	} else {
		pragmas = append(pragmas,
			"PRAGMA journal_mode = WAL",
			"PRAGMA synchronous = FULL",
			"PRAGMA wal_autocheckpoint = 1000",
		)
	}
	for _, pragma := range pragmas {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return classifyError("configure edge database", err)
		}
	}
	return quickCheck(ctx, s.db)
}

func quickCheck(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return classifyError("check edge database integrity", err)
	}
	defer rows.Close()
	checked := false
	for rows.Next() {
		checked = true
		var result string
		if err := rows.Scan(&result); err != nil {
			return classifyError("read edge database integrity result", err)
		}
		if result != "ok" {
			return fmt.Errorf("%w: quick_check failed", ErrCorruptDatabase)
		}
	}
	if err := rows.Err(); err != nil {
		return classifyError("iterate edge database integrity results", err)
	}
	if !checked {
		return fmt.Errorf("%w: quick_check returned no result", ErrCorruptDatabase)
	}
	return nil
}

// Close releases the SQLite handle. Durable files remain on disk.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (o Options) withDefaults() Options {
	if o.BusyTimeout <= 0 {
		o.BusyTimeout = defaultBusyTimeout
	}
	return o
}

func validateOptions(options Options) error {
	if options.DatabasePath == "" || !filepath.IsAbs(options.DatabasePath) || filepath.Clean(options.DatabasePath) != options.DatabasePath {
		return errors.New("edge database path must be absolute and clean")
	}
	if options.SpoolDirectory == "" || !filepath.IsAbs(options.SpoolDirectory) || filepath.Clean(options.SpoolDirectory) != options.SpoolDirectory {
		return errors.New("edge spool directory must be absolute and clean")
	}
	if options.SpoolDirectory == string(filepath.Separator) {
		return errors.New("edge spool directory must not be the filesystem root")
	}
	if options.BusyTimeout < time.Millisecond || options.BusyTimeout > time.Minute {
		return errors.New("edge database busy timeout must be between 1ms and 1m")
	}
	return nil
}

func ensureDirectory(path string, mode os.FileMode) error {
	if err := rejectSymlinkPath(path); err != nil {
		return err
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return classifyError("create edge state directory", err)
	}
	if err := rejectSymlinkPath(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return classifyError("inspect edge state directory", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("edge state directory must be a non-symlink directory")
	}
	if err := os.Chmod(path, mode); err != nil {
		return classifyError("protect edge state directory", err)
	}
	return nil
}

func ensureDatabaseFile(path string) error {
	if err := rejectSymlinkPath(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if createErr != nil {
			return classifyError("create edge database", createErr)
		}
		return file.Close()
	}
	if err != nil {
		return classifyError("inspect edge database", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("edge database must be a regular non-symlink file")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return classifyError("protect edge database", err)
	}
	return nil
}

func validateRegularNonSymlink(path string) error {
	if err := rejectSymlinkPath(path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return classifyError("inspect edge database", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("edge database must be a regular non-symlink file")
	}
	return nil
}

func rejectSymlinkPath(path string) error {
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("edge state path must not contain symlinks")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return classifyError("inspect edge state path", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func tightenDatabasePermissions(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return classifyError("inspect edge database file", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("edge database file must be regular and non-symlink")
		}
		if err := os.Chmod(candidate, 0o600); err != nil {
			return classifyError("protect edge database file", err)
		}
	}
	return nil
}
