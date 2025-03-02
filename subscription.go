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

type Subscription struct {
	Id        int64    `json:"id"`
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

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", s.Url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "SRRB/0.1")

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	n, err := io.ReadFull(res.Body, buf)
	res.Body.Close()

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

		if i.PublishedParsed == nil {
			t := parseHTTPTime(i.Published)
			i.PublishedParsed = &t
		} else {
			t := i.PublishedParsed.UTC()
			i.PublishedParsed = &t
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
