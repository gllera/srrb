package backend

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
)

func init() {
	Register("", newLocal)
}

type Local struct {
	path string
}

func newLocal(_ context.Context, u *url.URL) (Backend, error) {
	info, err := os.Stat(u.Path)
	if err != nil {
		return nil, fmt.Errorf("checking path %s: %w", u.Path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", u.Path)
	}

	return &Local{
		path: u.Path,
	}, nil
}

func (d *Local) localPath(op, key string) string {
	full := filepath.Join(d.path, key)
	slog.Debug("db "+op, "url", full)
	return full
}

func (d *Local) Get(_ context.Context, key string, ignoreMissing bool) ([]byte, error) {
	file := d.localPath("read", key)
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) && ignoreMissing {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", file, err)
	}
	return data, nil
}

func writeOpenFlags(ignoreExisting bool) int {
	if ignoreExisting {
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	return os.O_WRONLY | os.O_CREATE | os.O_EXCL
}

func (d *Local) Put(_ context.Context, key string, val []byte, ignoreExisting bool) error {
	file := d.localPath("write", key)
	fs, err := os.OpenFile(file, writeOpenFlags(ignoreExisting), 0o644)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", file, err)
	}
	defer fs.Close()

	if _, err := fs.Write(val); err != nil {
		return fmt.Errorf("writing file %s: %w", file, err)
	}
	return nil
}

func (d *Local) AtomicPut(_ context.Context, key string, val []byte) error {
	file := d.localPath("atomic write", key)
	tmpFile := file + ".tmp"

	fs, err := os.OpenFile(tmpFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("opening file %s: %w", tmpFile, err)
	}
	defer fs.Close()

	if _, err := fs.Write(val); err != nil {
		return fmt.Errorf("writing file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, file); err != nil {
		return fmt.Errorf("renaming %s to %s: %w", tmpFile, file, err)
	}
	return nil
}

func (d *Local) Rm(_ context.Context, key string) error {
	file := d.localPath("delete", key)

	if err := os.Remove(file); err != nil {
		if os.IsNotExist(err) {
			slog.Warn("db not found", "key", file)
		} else {
			return fmt.Errorf("removing %s: %w", file, err)
		}
	}
	return nil
}

func (d *Local) Close() error {
	return nil
}
