package main

import (
	"context"
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

	db.UpdateLastFetch()

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
	for _, s := range db.Subscriptions() {
		select {
		case ch <- s:
		case <-ctx.Done():
			break loop
		}
	}
	close(ch)
	wg.Wait()

	articles := []*Item{}
	for _, s := range db.Subscriptions() {
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

	if err = db.PutArticles(ctx, articles); err != nil {
		return err
	}

	db.UpdateRetainedPacks()

	return db.Commit(ctx)
}
