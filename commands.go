package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"sync"
)

type Globals struct {
	Jobs        int    `short:"j" default:"16"    env:"SRR_JOBS"         help:"Number of concurrent downloads."`
	PackageSize int    `short:"s" default:"200"   env:"SRR_PACKAGE_SIZE" help:"Target package size in KB."`
	MaxDownload int    `short:"m" default:"5000"  env:"SRR_MAX_DOWNLOAD" help:"Max downloadable file size in KB."`
	OutputPath  string `short:"o" default:"packs" env:"SRR_OUTPUT_PATH"  help:"Packages destination path."`
	DebugPath   string `short:"d" default:"debug" env:"SRR_DEBUG_PATH"   help:"Packages destination debug path."`
	Debug       bool   `                                                 help:"Enable debug mode. Output to debug path and pre-cleanup."`
}

type CLI struct {
	Globals
	Add     AddCmd     `cmd:"" help:"Subscribe to RSS URL."`
	Rm      RmCmd      `cmd:"" help:"Unsubscribe from RSS(s)."`
	RmTags  RmTagsCmd  `cmd:"" help:"Unsubscribe from RSS(s) with tag(s)."`
	Fetch   FetchCmd   `cmd:"" help:"Fetch subscriptions articles."`
	Import  ImportCmd  `cmd:"" help:"Import opml subscriptions file."`
	Version VersionCmd `cmd:"" help:"Print version information."`
}

type AddCmd struct {
	Title  string   `arg:""    help:"Subscription title."`
	URL    url.URL  `arg:""    help:"Subscription RSS URL."`
	Tag    string   `short:"g" help:"Subscription tag."`
	Parser []string `short:"p" help:"Subscription parsers commands."`
}

type VersionCmd struct {
}

func (c *VersionCmd) Run() error {
	fmt.Println("srr version", version)
	return nil
}

func (c *AddCmd) Run() error {
	if err := db.Add_sub(c.Tag, &Subscription{
		Title:   c.Title,
		Url:     c.URL.String(),
		Modules: c.Parser,
	}); err != nil {
		return err
	} else if globals.Debug {
		return (&FetchCmd{}).Run()
	}

	return nil
}

type ImportCmd struct {
	Path string `arg:""    help:"Subscriptions opml file."`
	Tag  string `short:"g" help:"Subscriptions tag."`
}

func (c *ImportCmd) Run() error {
	if err := db.ParseOPML(c.Path); err != nil {
		return err
	} else if globals.Debug {
		return (&FetchCmd{}).Run()
	}

	return nil
}

type RmCmd struct {
	Id []int `arg:"" help:"Subscriptions Ids to remove."`
}

func (c *RmCmd) Run() error {
	return db.Rm_subs(c.Id...)
}

type RmTagsCmd struct {
	Id []int `arg:"" help:"Tags Ids to remove."`
}

func (c *RmTagsCmd) Run() error {
	return db.Rm_subs(c.Id...)
}

type FetchCmd struct {
}

func (c *FetchCmd) Run() error {
	ch := make(chan DBItPair, globals.Jobs)
	var wg sync.WaitGroup

	for range globals.Jobs {
		wg.Add(1)

		go func() {
			defer wg.Done()
			buffer := make([]byte, globals.MaxDownload*(1<<10)+1)
			mod := New_Moduler()

			for o := range ch {
				if last_mod, err := o.Sub.Fetch(buffer); err != nil {
					slog.Info("Something went wrong while fetching subscription.", "", o.Sub, "err", err.Error())
				} else if err := o.Sub.Process(mod); err != nil {
					slog.Info("Something went wrong while processing subscription.", "", o.Sub, "err", err.Error())
				} else {
					o.Sub.Last_Mod_HTTP = last_mod
					o.Tag.Store(o.Sub)
				}
			}
		}()
	}

	err := db.Iterate(ch)
	wg.Wait()

	return err
}
