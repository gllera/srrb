package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gllera/srrb/mod"
)

type Subscription struct {
	Title        string   `json:"title"`
	URL          string   `json:"url"`
	Parsers      []string `json:"parsers,omitempty"`
	Error        string   `json:"error,omitempty"`
	LastGUID     uint32   `json:"last_guid,omitempty"`
	ETag         string   `json:"etag,omitempty"`
	LastModified string   `json:"last_modified,omitempty"`
	PackID       int      `json:"packid"`
	ID           int      `json:"id"`
	newItems     []*Item
}

func (s Subscription) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("id", s.ID),
		slog.String("url", s.URL),
	)
}

func (s *Subscription) Fetch(ctx context.Context, client *http.Client, buf []byte, processor *mod.Module) error {
	slog.Debug("downloading subscription", "sub", s)

	req, err := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "SRRB/"+version)
	if s.ETag != "" {
		req.Header.Set("If-None-Match", s.ETag)
	}
	if s.LastModified != "" {
		req.Header.Set("If-Modified-Since", s.LastModified)
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotModified {
		slog.Debug("subscription not modified", "sub", s)
		return nil
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %s", res.Status)
	}

	etag := res.Header.Get("ETag")
	lastModified := res.Header.Get("Last-Modified")

	n, err := io.ReadFull(res.Body, buf)

	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
	case errors.Is(err, io.EOF):
		return fmt.Errorf("empty response from subscription")
	case err == nil:
		return fmt.Errorf("subscription file bigger than %d bytes", cap(buf)-1)
	default:
		return err
	}

	s.newItems = nil
	var last *mod.RawItem

	err = parseFeed(buf[:n], func(i *mod.RawItem) error {
		if last == nil {
			last = i
		}
		if s.LastGUID == i.GUID {
			return ErrStopFeed
		}
		for _, m := range s.Parsers {
			if err := processor.Process(ctx, m, i); err != nil {
				return fmt.Errorf("module %q failed: %w", m, err)
			}
		}
		s.newItems = append(s.newItems, &Item{
			SubID:     s.ID,
			Title:     i.Title,
			Content:   i.Content,
			Link:      i.Link,
			Published: i.Published.Unix(),
		})
		return nil
	})

	if err != nil {
		return err
	}
	if last != nil {
		s.LastGUID = last.GUID
	}
	s.ETag = etag
	s.LastModified = lastModified
	return nil
}
