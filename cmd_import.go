package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type ImportCmd struct {
	Path string   `arg:""    help:"Subscriptions opml file."`
	ID   []string `short:"i" help:"Ids to import."`
	All  bool     `short:"a" help:"Import all."`
}

func (o *ImportCmd) Run() error {
	mapping, err := ParseOPML(o.Path)
	if err != nil {
		return err
	}

	keys := make([]string, 0)
	for k, v := range mapping {
		keys = append(keys, k)
		sort.Slice(v, func(i, j int) bool {
			return v[i].Title < v[j].Title
		})
	}
	sort.Strings(keys)

	var output io.Writer = os.Stdout
	if o.All || len(o.ID) > 0 {
		output = io.Discard
	}
	w := tabwriter.NewWriter(output, 1, 1, 2, ' ', 0)

	x := 1
	first := true
	fmt.Fprintf(w, "ID\tTitle\tURL\n")
	fmt.Fprintf(w, "---\t-----\t---\n")

	var newSubs []*Subscription
	for _, key := range keys {
		if !first {
			fmt.Fprintf(w, " \t \t \n")
		}
		first = false

		subs := mapping[key]
		if key == "" {
			key = "ROOT"
		}
		fmt.Fprintf(w, "%d\t[%s]\t-\n", x, key)

		y := 1
		for _, s := range subs {
			idx := fmt.Sprintf("%d.%d", x, y)
			fmt.Fprintf(w, "%s\t%s\t%s\n", idx, s.Title, s.URL)
			y++

			found := o.All
			for _, i := range o.ID {
				if strings.HasPrefix(idx+".", i+".") {
					found = true
					break
				}
			}
			if found {
				newSubs = append(newSubs, s)
			}
		}
		x++
	}
	w.Flush()

	if len(newSubs) > 0 {
		ctx := context.Background()
		db, err := NewDB(ctx, true)
		if err != nil {
			return err
		}
		defer db.Close(ctx)

		for _, s := range newSubs {
			s.PackID = -1
			s.ID = db.core.NSubs
			db.core.NSubs++
			db.core.Subs = append(db.core.Subs, s)
		}

		return db.Commit(ctx)
	}

	return nil
}
