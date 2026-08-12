package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CommittedSaveError reports a directory sync failure after replacing the target.
type CommittedSaveError struct{ Err error }

func (e *CommittedSaveError) Error() string {
	return "save completed but directory sync failed: " + e.Err.Error()
}

func (e *CommittedSaveError) Unwrap() error { return e.Err }

// SaveCommitted reports whether a save replaced its target despite returning an error.
func SaveCommitted(err error) bool {
	var committed *CommittedSaveError
	return errors.As(err, &committed)
}

var syncDirectory = func(dir string) error {
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func loadJSONFile(path, label string, value any) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("%s file permissions must be 0600", label)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", label, err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	return nil
}

func saveJSONFile(path string, value any, tempPattern, label string) error {
	if path == "" {
		return fmt.Errorf("save %s: path is required", label)
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create %s directory: %w", label, err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect %s directory: %w", label, err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", label, err)
	}
	temp, err := createTempFile(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("create temporary %s: %w", label, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("secure temporary %s: %w", label, err)
	}
	if _, err := writeTempFile(temp, append(data, '\n')); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary %s: %w", label, err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary %s: %w", label, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary %s: %w", label, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", label, err)
	}
	if err := syncDirectory(dir); err != nil {
		return &CommittedSaveError{Err: fmt.Errorf("sync %s directory: %w", label, err)}
	}
	return nil
}
