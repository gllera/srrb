package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type AddCmd struct {
	Upd     *int      `          optional:"" help:"Update existing subscription id instead."`
	Title   *string   `short:"t" optional:"" help:"Subscription title."`
	URL     *url.URL  `short:"u" optional:"" help:"Subscription RSS url."`
	Parsers *[]string `short:"p" optional:"" help:"Subscription parsers commands. Empty (\"\") for default."`
}

func (o *AddCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, true)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	var sub *Subscription
	if o.Upd != nil {
		if *o.Upd <= 0 {
			return fmt.Errorf("subscription id must be greater than 0")
		}

		for _, e := range db.Subscriptions() {
			if e.ID == *o.Upd {
				sub = e
				break
			}
		}
		if sub == nil {
			return fmt.Errorf("subscription id %d not found", *o.Upd)
		}
	} else {
		sub = &Subscription{
			PackID: -1,
		}
		db.AddSubscription(sub)
	}

	if o.Title != nil {
		sub.Title = *o.Title
		if sub.Title == "" {
			return fmt.Errorf("title cannot be empty")
		}
	} else if o.Upd == nil {
		return fmt.Errorf("title is required for new subscription")
	}

	if o.URL != nil {
		sub.URL = o.URL.String()
		if sub.URL == "" {
			return fmt.Errorf("url cannot be empty")
		}
	} else if o.Upd == nil {
		return fmt.Errorf("url is required for new subscription")
	}

	if o.Parsers != nil {
		sub.Parsers = []string{}
		for _, p := range *o.Parsers {
			if p != "" {
				sub.Parsers = append(sub.Parsers, p)
			}
		}
	}

	return db.Commit(ctx)
}

type RmCmd struct {
	ID []int `arg:"" help:"Subscription ids to remove."`
}

func (o *RmCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, true)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	for _, id := range o.ID {
		db.RemoveSubscription(id)
	}

	return db.Commit(ctx)
}

type LsCmd struct {
	Format string `short:"f" default:"yaml" enum:"yaml,json" help:"Output format."`
}

func (o *LsCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, false)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	type SubscriptionLS struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		URL   string `json:"url"`
		Error string `json:"error,omitempty" yaml:"error,omitempty"`
	}

	subsList := make([]*SubscriptionLS, 0, len(db.Subscriptions()))
	for _, s := range db.Subscriptions() {
		subsList = append(subsList, &SubscriptionLS{
			Title: s.Title,
			URL:   s.URL,
			ID:    s.ID,
			Error: s.Error,
		})
	}

	sort.Slice(subsList, func(i, j int) bool {
		return strings.ToLower(subsList[i].Title) < strings.ToLower(subsList[j].Title)
	})

	return printFormatted(o.Format, &subsList)
}
