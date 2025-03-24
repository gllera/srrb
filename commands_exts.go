package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
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
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	var ext *Extern
	if o.Upd != nil {
		if *o.Upd <= 0 {
			return fmt.Errorf(`extern id must be greater than 0`)
		}

		for _, e := range c.Exts {
			if e.Id == *o.Upd {
				ext = e
				break
			}
		}
		if ext == nil {
			return fmt.Errorf(`extern id "%d" not found`, *o.Upd)
		}
	} else {
		ext = &Extern{
			Id: c.N_Exts,
		}
		c.N_Exts++
		c.Exts = append(c.Exts, ext)
	}

	if o.Name != nil {
		ext.Name = *o.Name
		if ext.Name == "" {
			return fmt.Errorf(`name cannot be empty`)
		}
	} else if o.Upd == nil {
		return fmt.Errorf(`name is required for new subscription`)
	}

	if o.URL != nil {
		ext.Url = o.URL.String()
		if ext.Url == "" {
			return fmt.Errorf(`url cannot be empty`)
		}
	} else if o.Upd == nil {
		return fmt.Errorf(`url is required for new externals`)
	}

	return CommitDB(db)
}

type ExternRmCmd struct {
	Id []int `arg:"" help:"Subscriptions Ids to remove."`
}

func (o *ExternRmCmd) Run() error {
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	for _, id := range o.Id {
		for i, e := range c.Exts {
			if e.Id == id {
				c.Exts = slices.Delete(c.Exts, i, i+1)
				break
			}
		}
	}

	return CommitDB(db)
}

type ExternLsCmd struct {
	Format string `short:"f" default:"yaml" enum:"yaml,json" help:"Output format."`
}

func (o *ExternLsCmd) Run() error {
	_, c, err := NewDB(false)
	if err != nil {
		return err
	}

	sort.Slice(c.Exts, func(i, j int) bool {
		return strings.ToLower(c.Exts[i].Name) < strings.ToLower(c.Exts[j].Name)
	})

	var output []byte
	switch o.Format {
	case "yaml":
		output, _ = yaml.Marshal(c.Exts)
	case "json":
		output, _ = json.Marshal(c.Exts)
	}

	fmt.Printf("%s\n", output)
	return nil
}
