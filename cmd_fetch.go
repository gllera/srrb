package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gllera/srrb/mod"
)

type Item struct {
	SubID     int    `json:"subId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Link      string `json:"link"`
	Published int64  `json:"published"`
	Prev      int    `json:"prev,omitempty"`
}

func PutArticles(ctx context.Context, db *DB, articles []*Item) error {
	if len(articles) == 0 {
		return nil
	}

	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	defer gz.Close()

	// Read the latest pack to get the current fetch state
	data, err := db.Get(ctx, fmt.Sprintf("%v.gz", db.core.Latest), true)
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
	subs := make(map[int]*Subscription, len(db.core.Subs))
	for _, sub := range db.core.Subs {
		subs[sub.ID] = sub
	}

	// Save articles
	for _, item := range articles {
		if buffer.Len() >= globals.PackageSize<<10 {
			if err = savePack(ctx, fmt.Sprintf("%d.gz", db.core.NPacks), gz, db, &buffer); err != nil {
				return err
			}
			db.core.NPacks++
		}

		sub := subs[item.SubID]
		if sub.PackID != db.core.NPacks {
			item.Prev = sub.PackID
			sub.PackID = db.core.NPacks
		}

		data, err := jsonEncode(item)
		if err != nil {
			return err
		}

		if _, err = gz.Write(data); err != nil {
			return err
		}
	}

	// Save remaining articles without final pack
	db.core.Latest = !db.core.Latest
	if err = savePack(ctx, fmt.Sprintf("%v.gz", db.core.Latest), gz, db, &buffer); err != nil {
		return err
	}

	return nil
}

func savePack(ctx context.Context, name string, gz *gzip.Writer, db *DB, buffer *bytes.Buffer) error {
	if err := gz.Close(); err != nil {
		return err
	}
	if err := db.Put(ctx, name, buffer.Bytes(), true); err != nil {
		return err
	}
	buffer.Reset()
	gz.Reset(buffer)
	return nil
}

type FetchCmd struct {
}

func (o *FetchCmd) Run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := NewDB(ctx, true)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	db.core.LastFetch = time.Now().UTC().Unix()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	processor := mod.New()

	ch := make(chan *Subscription, globals.Jobs)
	var wg sync.WaitGroup

	for range globals.Jobs {
		wg.Add(1)

		go func() {
			defer wg.Done()
			buffer := make([]byte, globals.MaxDownload*(1<<10)+1)

			for s := range ch {
				s.Error = ""
				if err := s.Fetch(ctx, client, buffer, processor); err != nil {
					s.Error = err.Error()
					s.newItems = nil
					slog.Error("fetch failed", "sub", s, "err", err)
				}
			}
		}()
	}

loop:
	for _, s := range db.core.Subs {
		select {
		case ch <- s:
		case <-ctx.Done():
			break loop
		}
	}
	close(ch)
	wg.Wait()

	articles := []*Item{}
	for _, s := range db.core.Subs {
		for _, i := range s.newItems {
			articles = append(articles, i)
		}
	}
	sort.Slice(articles, func(i, j int) bool {
		if articles[i].Published != articles[j].Published {
			return articles[i].Published < articles[j].Published
		}
		return i > j
	})

	if err = PutArticles(ctx, db, articles); err != nil {
		return err
	}
	return db.Commit(ctx)
}
