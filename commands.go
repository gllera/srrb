package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type Globals struct {
	Jobs        int    `short:"j" default:"${nproc}" env:"SRR_JOBS"         help:"Number of concurrent downloads."`
	PackageSize int    `short:"s" default:"200"      env:"SRR_PACKAGE_SIZE" help:"Target package size in KB."`
	MaxDownload int    `short:"m" default:"5000"     env:"SRR_MAX_DOWNLOAD" help:"Max downloadable file size in KB."`
	OutputPath  string `short:"o" default:"packs"    env:"SRR_OUTPUT_PATH"  help:"Packages destination path."`
	Force       bool   `                             env:"SRR_FORCE"        help:"Override DB write lock if needed."`
	Debug       bool   `short:"d"                    env:"SRR_DEBUG"        help:"Enable debug mode."`
}

type CLI struct {
	Globals
	Add     AddCmd     `cmd:"" help:"Subscribe to RSS or update an existing subscription."`
	Rm      RmCmd      `cmd:"" help:"Unsubscribe from RSS(s)."`
	Ls      LsCmd      `cmd:"" help:"List subscriptions."`
	Fetch   FetchCmd   `cmd:"" help:"Fetch subscriptions articles."`
	Extern  ExternCmd  `cmd:"" help:"Manage additional external DBs."`
	Import  ImportCmd  `cmd:"" help:"Import opml subscriptions file."`
	Version VersionCmd `cmd:"" help:"Print version information."`
}

type ImportCmd struct {
	Path string   `arg:""    help:"Subscriptions opml file."`
	Id   []string `short:"i" help:"Ids to import."`
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
	if o.All || len(o.Id) > 0 {
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
			fmt.Fprintf(w, "%s\t%s\t%s\n", idx, s.Title, s.Url)
			y++

			found := o.All
			for _, i := range o.Id {
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
		db, c, err := NewDB(true)
		if err != nil {
			return err
		}
		defer UnlockDB(db)

		for _, s := range newSubs {
			s.PackId = -1
			s.Id = c.N_Subs
			c.N_Subs++
			c.Subs = append(c.Subs, s)
		}

		return CommitDB(db)
	}

	return nil
}

type VersionCmd struct {
}

func (c *VersionCmd) Run() error {
	fmt.Println("Version:", version)
	return nil
}
