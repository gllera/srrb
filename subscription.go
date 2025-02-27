package main

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

type SubscriptionLS struct {
	Id      int64    `json:"id"                yaml:"id"`
	Title   string   `json:"title"             yaml:"title"`
	Url     string   `json:"url"               yaml:"url"`
	Parsers []string `json:"parsers,omitempty" yaml:"parsers,omitempty"`
}

type Subscription struct {
	id        int64
	Url       string   `json:"url"`
	Title     string   `json:"title,omitempty"`
	Parsers   []string `json:"parsers,omitempty"`
	GUID      uint     `json:"uuid,omitempty"`
	PackId    int64    `json:"packid,omitempty"`
	Error     string   `json:"error,omitempty"`
	new_items []*gofeed.Item
}

func (s Subscription) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int64("id", s.id),
		slog.String("url", s.Url),
	)
}

func (s *Subscription) Fetch(buf []byte, mod *Module) error {
	slog.Debug(`downloading subscription articles.`, "", s)

	last_fetch := time.Now().UTC()
	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(s.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	n, err := io.ReadFull(res.Body, buf)

	switch err {
	case io.ErrUnexpectedEOF:
	case io.EOF:
		return fmt.Errorf(`empty response from subscription`)
	case nil:
		return fmt.Errorf(`subscription file bigger than %d bytes`, cap(buf)-1)
	default:
		return err
	}

	buf[n] = 0
	reader := bytes.NewReader(buf[0 : n+1])
	feeds, err := gofeed.NewParser().Parse(reader)
	if err != nil {
		return err
	}

	s.new_items = make([]*gofeed.Item, 0, len(feeds.Items))
	for _, i := range feeds.Items {
		if s.GUID == hash(i.GUID) {
			break
		}

		if i.Published == "" {
			i.Published = fmt.Sprintf("%d", last_fetch.Unix())
			i.PublishedParsed = &last_fetch
		}

		if i.Content == "" {
			i.Content = i.Description
			i.Description = ""
		}
		i.Author = nil

		s.new_items = append(s.new_items, i)
	}

	// Process new items
	for _, i := range s.new_items {
		for _, m := range s.Parsers {
			if err := mod.Process(m, i); err != nil {
				return fmt.Errorf(`module "%s" failed. %v`, m, err)
			}
		}
		mod.Sanitize(i)
		mod.Minify(i)
	}

	if len(s.new_items) > 0 {
		s.GUID = hash(s.new_items[0].GUID)
	}

	return nil
}
