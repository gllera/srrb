package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"time"

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
	Latest         bool            `json:"latest"`
	NArticles      int             `json:"n_articles"`
	NPacks         int             `json:"n_packs"`
	NSubscriptions int             `json:"n_subscriptions"`
	NExterns       int             `json:"n_externs"`
	Subscriptions  []*Subscription `json:"subscriptions"`
	Externs        []*Extern       `json:"externs,omitempty"`
	PackTS         [][2]int64      `json:"pack_ts,omitempty"`
	LastFetch      int64           `json:"last_fetch,omitempty"`
}

func NewDB(ctx context.Context, locked bool) (*DB, error) {
	backend, err := backend.Open(ctx, globals.OutputPath)
	if err != nil {
		return nil, err
	}

	db := &DB{
		Backend: backend,
		locked:  locked,
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

func (o *DB) Subscriptions() []*Subscription {
	return o.core.Subscriptions
}

func (o *DB) AddSubscription(s *Subscription) {
	o.core.NSubscriptions++
	s.ID = o.core.NSubscriptions
	o.core.Subscriptions = append(o.core.Subscriptions, s)
}

func (o *DB) RemoveSubscription(id int) {
	for i, s := range o.core.Subscriptions {
		if s.ID == id {
			o.core.Subscriptions = slices.Delete(o.core.Subscriptions, i, i+1)
			return
		}
	}
}

func (o *DB) UpdateLastFetch() {
	o.core.LastFetch = time.Now().UTC().Unix()
}

type Item struct {
	SubID     int    `json:"s"`
	Title     string `json:"t"`
	Content   string `json:"c"`
	Link      string `json:"l"`
	Published int64  `json:"p"`
}

type Extern struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	ID   int    `json:"id"`
}

func (o *DB) Externs() []*Extern {
	return o.core.Externs
}

func (o *DB) AddExtern(e *Extern) {
	o.core.NExterns++
	e.ID = o.core.NExterns
	o.core.Externs = append(o.core.Externs, e)
}

func (o *DB) RemoveExtern(id int) {
	for i, e := range o.core.Externs {
		if e.ID == id {
			o.core.Externs = slices.Delete(o.core.Externs, i, i+1)
			return
		}
	}
}

func (o *DB) PutArticles(ctx context.Context, articles []*Item) error {
	if len(articles) == 0 {
		return nil
	}

	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	defer gz.Close()

	// Read the latest pack to get the current fetch state
	data, err := o.Get(ctx, fmt.Sprintf("%v.gz", o.core.Latest), true)
	if err != nil {
		return err
	}

	if len(data) != 0 {
		unzipped, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		defer unzipped.Close()

		_, err = io.Copy(gz, unzipped)
		if err != nil {
			return err
		}
	}

	// Create subscriptions map
	subs := make(map[int]*Subscription, len(o.Subscriptions()))
	for _, sub := range o.Subscriptions() {
		subs[sub.ID] = sub
	}

	// Save articles
	for _, item := range articles {
		if buffer.Len() >= globals.PackageSize<<10 {
			if err = o.savePack(ctx, fmt.Sprintf("%d.gz", o.core.NPacks), gz, &buffer); err != nil {
				return err
			}
		}

		if buffer.Len() == 0 {
			ts := time.Now().UTC().Unix()
			o.core.NPacks++
			o.core.PackTS = append(o.core.PackTS, [2]int64{ts, int64(o.core.NPacks)})

			if err := o.writeMeta(gz, ts); err != nil {
				return err
			}
		}

		if err = writeItem(gz, item); err != nil {
			return err
		}

		sub := subs[item.SubID]
		sub.PackID = o.core.NPacks
		sub.NArticles++
		o.core.NArticles++
	}

	// Save remaining articles without final pack
	o.core.Latest = !o.core.Latest
	if err = o.savePack(ctx, fmt.Sprintf("%v.gz", o.core.Latest), gz, &buffer); err != nil {
		return err
	}

	return nil
}

func writeItem(gz *gzip.Writer, item *Item) error {
	data, err := jsonEncode(item)
	if err != nil {
		return err
	}
	_, err = gz.Write(data)
	return err
}

func (o *DB) writeMeta(gz *gzip.Writer, ts int64) error {
	subsMap := make(map[string][2]int)
	for _, s := range o.Subscriptions() {
		subsMap[strconv.Itoa(s.ID)] = [2]int{s.NArticles, s.PackID}
	}
	data, err := jsonEncode(map[string]any{
		"ts": ts,
		"n":  o.core.NArticles,
		"m":  subsMap,
	})
	if err != nil {
		return err
	}
	_, err = gz.Write(data)
	return err
}

func (o *DB) savePack(ctx context.Context, name string, gz *gzip.Writer, buffer *bytes.Buffer) error {
	if err := gz.Close(); err != nil {
		return err
	}
	if err := o.Put(ctx, name, buffer.Bytes(), true); err != nil {
		return err
	}
	buffer.Reset()
	gz.Reset(buffer)
	return nil
}
