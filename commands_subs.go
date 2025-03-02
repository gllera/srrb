package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type AddCmd struct {
	Upd     *int      `          optional:"" help:"Update existing subscription id instead."`
	Title   *string   `short:"t" optional:"" help:"Subscription title."`
	URL     *url.URL  `short:"u" optional:"" help:"Subscription RSS url."`
	Parsers *[]string `short:"p" optional:"" help:"Subscription parsers commands. Empty (\"\") for default."`
}

func (o *AddCmd) Run() error {
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	var sub *Subscription
	if o.Upd != nil {
		if *o.Upd <= 0 {
			return fmt.Errorf(`subscription id must be greater than 0`)
		}

		for _, e := range c.Subs {
			if e.Id == *o.Upd {
				sub = e
				break
			}
		}
		if sub == nil {
			return fmt.Errorf(`subscription id "%d" not found`, *o.Upd)
		}
	} else {
		sub = &Subscription{
			Id:     c.N_Subs,
			PackId: -1,
		}
		c.N_Subs++
		c.Subs = append(c.Subs, sub)
	}

	if o.Title != nil {
		sub.Title = *o.Title
		if sub.Title == "" {
			return fmt.Errorf(`title cannot be empty`)
		}
	} else if o.Upd == nil {
		return fmt.Errorf(`title is required for new subscription`)
	}

	if o.URL != nil {
		sub.Url = o.URL.String()
		if sub.Url == "" {
			return fmt.Errorf(`url cannot be empty`)
		}
	} else if o.Upd == nil {
		return fmt.Errorf(`url is required for new subscription`)
	}

	if o.Parsers != nil {
		sub.Parsers = []string{}
		for _, p := range *o.Parsers {
			if p != "" {
				sub.Parsers = append(sub.Parsers, p)
			}
		}
	}

	return CommitDB(db)
}

type RmCmd struct {
	Id []int `arg:"" help:"Subscription ids to remove."`
}

func (o *RmCmd) Run() error {
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	for _, id := range o.Id {
		for i, s := range c.Subs {
			if s.Id == id {
				c.Subs = slices.Delete(c.Subs, i, i+1)
				break
			}
		}
	}

	return CommitDB(db)
}

type LsCmd struct {
	Format string `short:"f" default:"yaml" enum:"yaml,json" help:"Output format."`
}

func (o *LsCmd) Run() error {
	_, c, err := NewDB(false)
	if err != nil {
		return err
	}

	type SubscriptionLS struct {
		Id    int    `json:"id"`
		Title string `json:"title"`
		Url   string `json:"url"`
		Error string `json:"error,omitempty" yaml:"error,omitempty"`
	}

	subs_list := make([]*SubscriptionLS, 0, len(c.Subs))
	for _, s := range c.Subs {
		subs_list = append(subs_list, &SubscriptionLS{
			Title: s.Title,
			Url:   s.Url,
			Id:    s.Id,
			Error: s.Error,
		})
	}

	sort.Slice(subs_list, func(i, j int) bool {
		return strings.ToLower(subs_list[i].Title) < strings.ToLower(subs_list[j].Title)
	})

	var output []byte
	switch o.Format {
	case "yaml":
		output, _ = yaml.Marshal(&subs_list)
	case "json":
		output, _ = json.Marshal(&subs_list)
	}

	fmt.Printf("%s\n", output)
	return nil
}

type FetchCmd struct {
}

func (o *FetchCmd) Run() error {
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	c.Last_fetch = time.Now().UTC().Unix()

	ch := make(chan *Subscription, globals.Jobs)
	var wg sync.WaitGroup

	for range globals.Jobs {
		wg.Add(1)

		go func() {
			defer wg.Done()
			buffer := make([]byte, globals.MaxDownload*(1<<10)+1)
			mod := New_Moduler()

			for s := range ch {
				s.Error = ""
				if err := s.Fetch(buffer, mod); err != nil {
					s.Error = err.Error()
					s.new_items = nil
					slog.Error(`something went wrong while fetching subscription.`, "", s, "err", err.Error())
				}
			}
		}()
	}

	for _, s := range c.Subs {
		ch <- s
	}
	close(ch)
	wg.Wait()

	articles := []Article{}
	for _, s := range c.Subs {
		for _, i := range s.new_items {
			articles = append(articles, Article{
				SubId:     s.Id,
				Title:     i.Title,
				Content:   i.Content,
				Link:      i.Link,
				Published: i.PublishedParsed.Unix(),
			})
		}
	}
	slices.Reverse(articles)
	sort.SliceStable(articles, func(i, j int) bool {
		return articles[i].Published < articles[j].Published
	})

	if err = PutArticles(db, articles); err != nil {
		return err
	}
	return CommitDB(db)
}
