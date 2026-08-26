package identity

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const installSuffix = ".guardian-install"

// Install writes a verified certificate/key pair through recoverable staged
// files. Load calls recoverInstall before reading either final path.
func Install(certPath, keyPath string, certificatePEM, privateKeyPEM []byte) error {
	if certPath == keyPath || len(certificatePEM) == 0 || len(privateKeyPEM) == 0 {
		return ErrInvalid
	}
	if _, err := tls.X509KeyPair(certificatePEM, privateKeyPEM); err != nil {
		return fmt.Errorf("%w: install matching identity", ErrInvalid)
	}
	if filepath.Dir(certPath) != filepath.Dir(keyPath) {
		return errors.New("identity certificate and key must share one protected directory")
	}
	directory := filepath.Dir(certPath)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}
	if err := rejectUnsafeDirectory(directory); err != nil {
		return err
	}
	if err := recoverInstall(certPath, keyPath); err != nil {
		return err
	}
	certNext, keyNext := certPath+installSuffix, keyPath+installSuffix
	journal := filepath.Join(directory, ".guardian-identity-install")
	if err := writeExclusive(journal, []byte("guardian.identity.install.v1\n"), 0o600); err != nil {
		return err
	}
	if err := writeExclusive(certNext, certificatePEM, 0o644); err != nil {
		return err
	}
	if err := writeExclusive(keyNext, privateKeyPEM, 0o600); err != nil {
		_ = os.Remove(certNext)
		return err
	}
	if err := promoteIdentity(certPath, keyPath, certNext, keyNext); err != nil {
		return err
	}
	return cleanupInstall(certPath, keyPath, journal)
}

func recoverInstall(certPath, keyPath string) error {
	journal := filepath.Join(filepath.Dir(certPath), ".guardian-identity-install")
	if _, err := os.Lstat(journal); errors.Is(err, os.ErrNotExist) {
		// Staged files without the durable journal predate an atomic promotion.
		// They are never eligible identity material and must not block retry.
		for _, path := range []string{certPath + installSuffix, keyPath + installSuffix} {
			if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return fmt.Errorf("remove uncommitted identity stage: %w", removeErr)
			}
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect identity install journal: %w", err)
	}
	candidates := [][2]string{
		{certPath, keyPath},
		{certPath + installSuffix, keyPath + installSuffix},
		{certPath + ".guardian-old", keyPath + ".guardian-old"},
		{certPath + ".guardian-old", keyPath},
		{certPath, keyPath + ".guardian-old"},
	}
	for _, candidate := range candidates {
		certificatePEM, certErr := os.ReadFile(candidate[0])
		privateKeyPEM, keyErr := os.ReadFile(candidate[1])
		if certErr != nil || keyErr != nil {
			clear(privateKeyPEM)
			continue
		}
		_, pairErr := tls.X509KeyPair(certificatePEM, privateKeyPEM)
		clear(privateKeyPEM)
		if pairErr != nil {
			continue
		}
		if candidate[0] != certPath || candidate[1] != keyPath {
			if err := copyIdentityCandidate(candidate[0], candidate[1], certPath, keyPath); err != nil {
				return err
			}
		}
		return cleanupInstall(certPath, keyPath, journal)
	}
	return fmt.Errorf("%w: no recoverable certificate/key pair", ErrInvalid)
}

func promoteIdentity(certPath, keyPath, certNext, keyNext string) error {
	for _, pair := range [][2]string{{certPath, certPath + ".guardian-old"}, {keyPath, keyPath + ".guardian-old"}} {
		_ = os.Remove(pair[1])
		if _, err := os.Lstat(pair[0]); err == nil {
			if err := os.Rename(pair[0], pair[1]); err != nil {
				return fmt.Errorf("stage previous identity: %w", err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Rename(keyNext, keyPath); err != nil {
		return fmt.Errorf("activate identity key: %w", err)
	}
	if err := os.Rename(certNext, certPath); err != nil {
		return fmt.Errorf("activate identity certificate: %w", err)
	}
	return nil
}

func copyIdentityCandidate(sourceCert, sourceKey, certPath, keyPath string) error {
	certificatePEM, err := os.ReadFile(sourceCert)
	if err != nil {
		return err
	}
	privateKeyPEM, err := os.ReadFile(sourceKey)
	if err != nil {
		return err
	}
	defer clear(privateKeyPEM)
	certNext, keyNext := certPath+installSuffix, keyPath+installSuffix
	_ = os.Remove(certNext)
	_ = os.Remove(keyNext)
	if err := writeExclusive(certNext, certificatePEM, 0o644); err != nil {
		return err
	}
	if err := writeExclusive(keyNext, privateKeyPEM, 0o600); err != nil {
		return err
	}
	return promoteIdentity(certPath, keyPath, certNext, keyNext)
}

func writeExclusive(path string, contents []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create staged identity file: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(contents); err != nil {
		return fmt.Errorf("write staged identity file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync staged identity file: %w", err)
	}
	return nil
}

func rejectUnsafeDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return ErrPermissions
	}
	return nil
}

func cleanupInstall(certPath, keyPath, journal string) error {
	for _, path := range []string{
		certPath + installSuffix, keyPath + installSuffix,
		certPath + ".guardian-old", keyPath + ".guardian-old", journal,
	} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("clean identity install state: %w", err)
		}
	}
	return nil
}
