package storage

import (
	"errors"
	"fmt"
	"syscall"

	sqliteDriver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	// ErrCorruptDatabase identifies a database that failed SQLite integrity checks.
	ErrCorruptDatabase = errors.New("edge database is corrupt")
	// ErrSchemaIncompatible identifies a development schema requiring explicit reset.
	ErrSchemaIncompatible = errors.New("edge database schema is incompatible")
	// ErrDatabaseBusy identifies bounded lock contention.
	ErrDatabaseBusy = errors.New("edge database is busy")
	// ErrStorageFull identifies a filesystem or SQLite capacity failure.
	ErrStorageFull = errors.New("edge storage is full")
	// ErrSpoolObjectMissing identifies missing or invalid content-addressed data.
	ErrSpoolObjectMissing = errors.New("edge spool object is unavailable")
	// ErrEventConflict means an event ID was reused for different content.
	ErrEventConflict = errors.New("event ID already exists with different content")
	// ErrNoEvent means no event is currently available for delivery.
	ErrNoEvent = errors.New("no event available")
	// ErrNotInflight means an operation requires an in-flight event.
	ErrNotInflight = errors.New("event is not in flight")
	// ErrStaleClaim fences a worker trying to complete an older lease.
	ErrStaleClaim = errors.New("event claim is stale")
	// ErrPayloadTooLarge prevents large hostile payloads from entering local state.
	ErrPayloadTooLarge = errors.New("event payload exceeds the local limit")
	// ErrRecoveryConfirmationRequired protects the explicit destructive recovery path.
	ErrRecoveryConfirmationRequired = errors.New("development database recovery confirmation is required")
	// ErrRecoveryNotRequired prevents reset of a healthy database.
	ErrRecoveryNotRequired = errors.New("edge database does not require recovery")
)

func classifyError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var category error
	if errors.Is(err, syscall.ENOSPC) {
		category = ErrStorageFull
	}
	var sqliteErr *sqliteDriver.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() & 0xff {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			category = ErrDatabaseBusy
		case sqlite3.SQLITE_FULL:
			category = ErrStorageFull
		case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
			category = ErrCorruptDatabase
		}
	}
	if category != nil {
		return fmt.Errorf("%s: %w", operation, errors.Join(category, err))
	}
	return fmt.Errorf("%s: %w", operation, err)
}
