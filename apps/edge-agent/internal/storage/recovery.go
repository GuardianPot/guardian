package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type recoveryMove struct {
	source string
	target string
}

// RecoveryReport identifies quarantined files and the newly initialized DB.
type RecoveryReport struct {
	DatabasePath     string   `json:"database_path"`
	QuarantinedPaths []string `json:"quarantined_paths"`
}

// RecoverDevelopmentDatabase performs the package-approved forward-only
// development reset. It requires explicit confirmation and only proceeds for a
// named corrupt or incompatible state; healthy databases are never reset.
func RecoverDevelopmentDatabase(
	ctx context.Context,
	options Options,
	confirmed bool,
	now time.Time,
) (RecoveryReport, error) {
	if !confirmed {
		return RecoveryReport{}, ErrRecoveryConfirmationRequired
	}
	options = options.withDefaults()
	if err := validateOptions(options); err != nil {
		return RecoveryReport{}, err
	}

	probe, probeErr := OpenReadOnly(ctx, options)
	if probeErr == nil {
		_ = probe.Close()
		return RecoveryReport{}, ErrRecoveryNotRequired
	}
	if !errors.Is(probeErr, ErrCorruptDatabase) && !errors.Is(probeErr, ErrSchemaIncompatible) {
		return RecoveryReport{}, fmt.Errorf("inspect edge database recovery state: %w", probeErr)
	}

	suffix := ".corrupt-" + now.UTC().Format("20060102T150405.000000000Z")
	baseTarget := options.DatabasePath + suffix
	moves := []recoveryMove{
		{options.DatabasePath, baseTarget},
		{options.DatabasePath + "-wal", baseTarget + "-wal"},
		{options.DatabasePath + "-shm", baseTarget + "-shm"},
	}
	completed := []recoveryMove{}
	for _, candidate := range moves {
		info, err := os.Lstat(candidate.source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			rollbackMoves(completed)
			return RecoveryReport{}, classifyError("inspect corrupt edge database file", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			rollbackMoves(completed)
			return RecoveryReport{}, errors.New("corrupt edge database file must be regular and non-symlink")
		}
		if _, err := os.Lstat(candidate.target); !errors.Is(err, os.ErrNotExist) {
			rollbackMoves(completed)
			if err == nil {
				return RecoveryReport{}, errors.New("edge database quarantine target already exists")
			}
			return RecoveryReport{}, classifyError("inspect edge database quarantine target", err)
		}
		if err := os.Rename(candidate.source, candidate.target); err != nil {
			rollbackMoves(completed)
			return RecoveryReport{}, classifyError("quarantine corrupt edge database file", err)
		}
		completed = append(completed, candidate)
	}
	if len(completed) == 0 {
		return RecoveryReport{}, errors.New("no edge database files were available to quarantine")
	}
	if err := syncDirectory(filepath.Dir(options.DatabasePath)); err != nil {
		return RecoveryReport{}, err
	}

	store, err := Open(ctx, options)
	if err != nil {
		return RecoveryReport{}, fmt.Errorf("initialize recovered edge database: %w", err)
	}
	if err := store.Close(); err != nil {
		return RecoveryReport{}, classifyError("close recovered edge database", err)
	}
	report := RecoveryReport{DatabasePath: options.DatabasePath, QuarantinedPaths: make([]string, 0, len(completed))}
	for _, candidate := range completed {
		report.QuarantinedPaths = append(report.QuarantinedPaths, candidate.target)
	}
	return report, nil
}

func rollbackMoves(completed []recoveryMove) {
	for index := len(completed) - 1; index >= 0; index-- {
		_ = os.Rename(completed[index].target, completed[index].source)
	}
}
