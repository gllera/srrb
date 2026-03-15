package main

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

type Extern struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	ID   int    `json:"id"`
}

type ExternCmd struct {
	Add ExternAddCmd `cmd:"" help:"Add external DB or update an existing one."`
	Rm  ExternRmCmd  `cmd:"" help:"Remove external DB(s)."`
	Ls  ExternLsCmd  `cmd:"" help:"List external DBs."`
}

type ExternAddCmd struct {
	Upd  *int     `          optional:"" help:"Update existing external id instead."`
	Name *string  `short:"n" optional:"" help:"External name."`
	URL  *url.URL `short:"u" optional:"" help:"External url."`
}

func (o *ExternAddCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, true)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	var ext *Extern
	if o.Upd != nil {
		if *o.Upd <= 0 {
			return fmt.Errorf("extern id must be greater than 0")
		}

		for _, e := range db.core.Exts {
			if e.ID == *o.Upd {
				ext = e
				break
			}
		}
		if ext == nil {
			return fmt.Errorf("extern id %d not found", *o.Upd)
		}
	} else {
		ext = &Extern{
			ID: db.core.NExts,
		}
		db.core.NExts++
		db.core.Exts = append(db.core.Exts, ext)
	}

	if o.Name != nil {
		ext.Name = *o.Name
		if ext.Name == "" {
			return fmt.Errorf("name cannot be empty")
		}
	} else if o.Upd == nil {
		return fmt.Errorf("name is required for new externals")
	}

	if o.URL != nil {
		ext.URL = o.URL.String()
		if ext.URL == "" {
			return fmt.Errorf("url cannot be empty")
		}
	} else if o.Upd == nil {
		return fmt.Errorf("url is required for new externals")
	}

	return db.Commit(ctx)
}

type ExternRmCmd struct {
	ID []int `arg:"" help:"Subscriptions Ids to remove."`
}

func (o *ExternRmCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, true)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	for _, id := range o.ID {
		for i, e := range db.core.Exts {
			if e.ID == id {
				db.core.Exts = slices.Delete(db.core.Exts, i, i+1)
				break
			}
		}
	}

	return db.Commit(ctx)
}

type ExternLsCmd struct {
	Format string `short:"f" default:"yaml" enum:"yaml,json" help:"Output format."`
}

func (o *ExternLsCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, false)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	sort.Slice(db.core.Exts, func(i, j int) bool {
		return strings.ToLower(db.core.Exts[i].Name) < strings.ToLower(db.core.Exts[j].Name)
	})

	return printFormatted(o.Format, db.core.Exts)
}
