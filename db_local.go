package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
)

type DB_local struct {
	DB_core

	path string
}

func NewDB_Local(u *url.URL, is_writable bool) (DB, *DB_core, error) {
	db := &DB_local{
		DB_core: newDB_Core(is_writable),
		path:    u.Path,
	}
	return db, &db.DB_core, nil
}

func (d *DB_local) Get(key string, ignore_missing bool) ([]byte, error) {
	file := filepath.Join(d.path, key)
	data, err := os.ReadFile(file)
	slog.Debug(`db read`, "file", file)

	if err != nil {
		if os.IsNotExist(err) && ignore_missing {
			return nil, nil
		}
		return nil, fmt.Errorf(`unable to read file %s (%v)`, file, err)
	}

	return data, nil
}

func (d *DB_local) Put(key string, val []byte, ignore_existing bool) error {
	file := filepath.Join(d.path, key)
	slog.Debug(`db write`, "file", file)

	var flag int
	if ignore_existing {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	} else {
		flag = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}

	fs, err := os.OpenFile(file, flag, 0644)
	if err != nil {
		return err
	}
	defer fs.Close()

	_, err = fs.Write(val)
	return err
}

func (d *DB_local) AtomicPut(key string, val []byte) error {
	file := filepath.Join(d.path, key)
	slog.Debug(`db careful write`, "file", file)

	tmp_key := key + ".tmp"
	tmp := filepath.Join(d.path, tmp_key)
	err := d.Put(tmp_key, val, true)

	if err == nil {
		if err = os.Rename(tmp, file); err != nil {
			err = fmt.Errorf(`unable to rename "%s" to "%s" (%v)`, tmp, file, err)
		}
	}

	return err
}

func (d *DB_local) Rm(key string) error {
	file := filepath.Join(d.path, key)
	slog.Debug(`db delete`, "file", file)

	if err := os.Remove(file); err != nil {
		if os.IsNotExist(err) {
			slog.Warn(`db not found`, "file", file)
		} else {
			return fmt.Errorf(`unable to remove "%s" (%v)`, file, err)
		}
	}
	return nil
}

func (d *DB_local) Mkdir() error {
	if err := os.MkdirAll(d.path, 0755); err != nil {
		return fmt.Errorf(`unable create folder %s (%v)`, d.path, err)
	}
	return nil
}
