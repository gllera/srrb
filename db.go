package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gllera/srrb/backend"
)

const (
	dbFileKey = "db.json"
	dbLockKey = ".locked"
)

type DB struct {
	backend.Backend
	core   DBCore
	locked bool
}

type DBCore struct {
	Latest    bool            `json:"latest"`
	NPacks    int             `json:"n_packs"`
	NSubs     int             `json:"n_subs"`
	NExts     int             `json:"n_exts"`
	Subs      []*Subscription `json:"subs"`
	Exts      []*Extern       `json:"exts,omitempty"`
	LastFetch int64           `json:"last_fetch,omitempty"`
}

func (o *DB) Close(ctx context.Context) error {
	if o.locked {
		if err := o.Rm(context.WithoutCancel(ctx), dbLockKey); err != nil {
			slog.Warn("remove lock file", "error", err)
		}
	}
	return o.Backend.Close()
}

func (o *DB) Commit(ctx context.Context) error {
	data, err := jsonEncode(&o.core)
	if err != nil {
		return err
	}
	return o.AtomicPut(ctx, dbFileKey, data)
}

func NewDB(ctx context.Context, locked bool) (*DB, error) {
	backend, err := backend.Open(ctx, globals.OutputPath)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Backend: backend,
		core: DBCore{
			NSubs:  1,
			NExts:  1,
			NPacks: 1,
		},
		locked: locked,
	}

	if locked {
		if err := db.Put(ctx, dbLockKey, nil, globals.Force); err != nil {
			db.Backend.Close()
			return nil, fmt.Errorf("create lock file: %w", err)
		}
	}

	data, err := db.Get(ctx, dbFileKey, true)
	if err != nil {
		db.Close(ctx)
		return nil, err
	}

	if len(data) != 0 {
		if err := json.Unmarshal(data, &db.core); err != nil {
			db.Close(ctx)
			return nil, fmt.Errorf("decode %s: %w", dbFileKey, err)
		}
	}

	return db, nil
}
