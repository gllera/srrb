package main

/**
 * @website http://albulescu.ro
 * @author Cosmin Albulescu <cosmin@albulescu.ro>
 */

import (
	"bytes"
	"fmt"
	"io"
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

func (s *Subscription) Fetch(buf []byte) (string, error) {
	info(fmt.Sprintf(`Downloading articles from "%s" (id: %d) ...`, s.Url, s.Id))
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := client.Get(s.Url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	last_mod := res.Header.Get("Last-Modified")
	if last_mod != "" && last_mod == s.Last_Mod_HTTP {
		info(fmt.Sprintf(`No update since last fetch on "%s" (id: %d)`, s.Url, s.Id))
		return last_mod, nil
	}

	n, err := io.ReadFull(res.Body, buf)

	switch err {
	case io.ErrUnexpectedEOF:
	case io.EOF:
		return "", fmt.Errorf("empty response")
	case nil:
		return "", fmt.Errorf(`subscription file bigger than %d bytes`, cap(buf)-1)
	default:
		return "", err
	}

	buf[n] = 0
	reader := bytes.NewReader(buf[0 : n+1])

	if feeds, err := gofeed.NewParser().Parse(reader); err != nil {
		return "", err
	} else {
		for _, i := range feeds.Items {
			if s.Last_GUID == i.GUID {
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
