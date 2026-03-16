package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

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

		for _, e := range db.Externs() {
			if e.ID == *o.Upd {
				ext = e
				break
			}
		}
		if ext == nil {
			return fmt.Errorf("extern id %d not found", *o.Upd)
		}
	} else {
		ext = &Extern{}
		db.AddExtern(ext)
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
	ID []int `arg:"" help:"External Ids to remove."`
}

func (o *ExternRmCmd) Run() error {
	ctx := context.Background()
	db, err := NewDB(ctx, true)
	if err != nil {
		return err
	}
	defer db.Close(ctx)

	for _, id := range o.ID {
		db.RemoveExtern(id)
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

	externs := db.Externs()
	sort.Slice(externs, func(i, j int) bool {
		return strings.ToLower(externs[i].Name) < strings.ToLower(externs[j].Name)
	})

	return printFormatted(o.Format, externs)
}
