package entries

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry is a timestamped front-desk activity.
//
// The JSON field names are part of the persisted file format and must remain
// compatible with task files written by earlier versions of tm.
type Entry struct {
	Time    time.Time `json:"Time"`
	Message string    `json:"Message"`
}

// Store persists entries in a JSON file at Path.
type Store struct {
	Path string
}

// Load reads all saved entries. A missing file is treated as an empty store.
func (s Store) Load() ([]Entry, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("read entries: %w", err)
	}

	var saved []Entry
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("decode entries: %w", err)
	}
	if saved == nil {
		return []Entry{}, nil
	}

	return saved, nil
}

// Save replaces the stored entries only after the complete new file has been
// encoded, flushed, and closed in the destination directory.
func (s Store) Save(saved []Entry) error {
	if s.Path == "" {
		return errors.New("save entries: path is empty")
	}

	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create entries directory: %w", err)
	}

	if saved == nil {
		saved = []Entry{}
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return fmt.Errorf("encode entries: %w", err)
	}
	data = append(data, '\n')

	temporary, err := os.CreateTemp(dir, "."+filepath.Base(s.Path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary entries file: %w", err)
	}
	temporaryPath := temporary.Name()
	keepTemporary := true
	defer func() {
		if keepTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary entries permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary entries file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush temporary entries file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary entries file: %w", err)
	}

	if err := replaceFile(temporaryPath, s.Path); err != nil {
		return fmt.Errorf("replace entries file: %w", err)
	}
	keepTemporary = false

	return nil
}

// Clear replaces the stored entries with an empty JSON array.
func (s Store) Clear() error {
	return s.Save([]Entry{})
}

// replaceFile first uses the atomic replacement supported on Unix-like
// systems. Windows does not allow os.Rename to replace an existing file, so it
// falls back to a backup-and-restore sequence there.
func replaceFile(source, destination string) error {
	if err := os.Rename(source, destination); err == nil {
		return nil
	}

	info, err := os.Stat(destination)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("destination is not a regular file")
	}

	dir := filepath.Dir(destination)
	backupFile, err := os.CreateTemp(dir, "."+filepath.Base(destination)+".backup-*")
	if err != nil {
		return fmt.Errorf("reserve backup path: %w", err)
	}
	backupPath := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("close backup placeholder: %w", err)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("prepare backup path: %w", err)
	}

	if err := os.Rename(destination, backupPath); err != nil {
		return fmt.Errorf("back up existing entries file: %w", err)
	}
	if err := os.Rename(source, destination); err != nil {
		if restoreErr := os.Rename(backupPath, destination); restoreErr != nil {
			return fmt.Errorf("install new entries file: %w (restore failed: %v)", err, restoreErr)
		}
		return fmt.Errorf("install new entries file: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}
