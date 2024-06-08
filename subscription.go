package main

/**
 * @website http://albulescu.ro
 * @author Cosmin Albulescu <cosmin@albulescu.ro>
 */

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

func (s *Subscription) Process(mod *Moduler) error {
	for _, i := range s.new_items {
		for _, m := range s.Modules {
			if err := mod.Process(m, i); err != nil {
				return fmt.Errorf(`module "%s" failed (%v)`, m, err)
			}
		}
		mod.Sanitize(i)
		mod.Minify(i)
	}

	return nil
}

func (s *Subscription) Fetch(buf []byte) (int64, error) {
	slog.Debug("Downloading subscription articles.", "", s)

	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(s.Url)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	last_mod, ok := parseHTTPTime(res.Header.Get("Last-Modified"))
	if !ok {
		slog.Info(`Unable to parse subscription "Last-Modified" http header.`, "", s)
	}

	n, err := io.ReadFull(res.Body, buf)

	switch err {
	case io.ErrUnexpectedEOF:
	case io.EOF:
		return 0, fmt.Errorf("empty response")
	case nil:
		return 0, fmt.Errorf(`subscription file bigger than %d bytes`, cap(buf)-1)
	default:
		return 0, err
	}

	buf[n] = 0
	reader := bytes.NewReader(buf[0 : n+1])

	if feeds, err := gofeed.NewParser().Parse(reader); err != nil {
		return 0, err
	} else {
		for _, i := range feeds.Items {
			if s.Last_GUID == hash(i.GUID) {
				break
			}

			if i.Published == "" {
				now := time.Now().UTC()
				i.Published = fmt.Sprintf("%d", now.Unix())
				i.PublishedParsed = &now
			}

			if i.Content == "" {
				i.Content = i.Description
				i.Description = ""
			}
			i.Author = nil

			s.new_items = append(s.new_items, i)
		}
		return last_mod, nil
	}
}
