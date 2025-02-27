package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Globals struct {
	Jobs        int    `short:"j" default:"16"    env:"SRR_JOBS"         help:"Number of concurrent downloads."`
	PackageSize int    `short:"s" default:"200"   env:"SRR_PACKAGE_SIZE" help:"Target package size in KB."`
	MaxDownload int    `short:"m" default:"5000"  env:"SRR_MAX_DOWNLOAD" help:"Max downloadable file size in KB."`
	OutputPath  string `short:"o" default:"packs" env:"SRR_OUTPUT_PATH"  help:"Packages destination path."`
	Force       bool   `                          env:"SRR_FORCE"        help:"Override DB write lock if needed."`
	Debug       bool   `short:"d"                 env:"SRR_DEBUG"        help:"Enable debug mode."`
}

type CLI struct {
	Globals
	Add     AddCmd     `cmd:"" help:"Subscribe to RSS."`
	Upd     UpdCmd     `cmd:"" help:"Update existing RSS."`
	Rm      RmCmd      `cmd:"" help:"Unsubscribe from RSS(s)."`
	Ls      LsCmd      `cmd:"" help:"List subscriptions."`
	Fetch   FetchCmd   `cmd:"" help:"Fetch subscriptions articles."`
	Import  ImportCmd  `cmd:"" help:"Import opml subscriptions file."`
	Version VersionCmd `cmd:"" help:"Print version information."`
}

type AddCmd struct {
	Title   string   `arg:"" help:"Subscription title."`
	URL     url.URL  `arg:"" help:"Subscription RSS URL."`
	Parsers []string `arg:"" help:"Subscription parsers commands." optional:""`
}

func (o *AddCmd) Run() error {
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	s := &Subscription{
		id:      c.SubIds,
		Title:   o.Title,
		Url:     o.URL.String(),
		Parsers: o.Parsers,
		PackId:  -1,
	}

	c.Subscriptions[s.id] = s
	c.SubIds++

	return CommitDB(db)
}

type UpdCmd struct {
	Id int64 `arg:"" help:"Existing subscription ID."`
	AddCmd
}

func (o *UpdCmd) Run() error {
	if o.Id <= 0 {
		return fmt.Errorf(`subscription id must be greater than 0`)
	}

	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	s, ok := c.Subscriptions[o.Id]
	if !ok {
		return fmt.Errorf(`subscription id "%d" not found`, o.Id)
	}

	s.Url = o.URL.String()
	s.Title = o.Title
	s.Parsers = o.Parsers

	return CommitDB(db)
}

type RmCmd struct {
	Id []int64 `arg:"" help:"Subscriptions Ids to remove."`
}

func (o *RmCmd) Run() error {
	db, c, err := NewDB(true)
	if err != nil {
		return err
	}
	defer UnlockDB(db)

	for _, id := range o.Id {
		delete(c.Subscriptions, id)
	}

	return CommitDB(db)
}

type LsCmd struct {
	Format string `short:"f" default:"yaml" enum:"yaml,json" help:"Number of concurrent downloads."`
}

func (o *LsCmd) Run() error {
	_, c, err := NewDB(false)
	if err != nil {
		return err
	}

	subs_list := make([]*SubscriptionLS, 0, len(c.Subscriptions))
	for _, s := range c.Subscriptions {
		subs_list = append(subs_list, &SubscriptionLS{
			Title:   s.Title,
			Url:     s.Url,
			Id:      s.id,
			Parsers: s.Parsers,
		})
	}

	sort.Slice(subs_list, func(i, j int) bool {
		return subs_list[i].Id < subs_list[j].Id
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

	for _, s := range c.Subscriptions {
		ch <- s
	}
	close(ch)
	wg.Wait()

	articles := []Article{}
	for _, s := range c.Subscriptions {
		for _, i := range s.new_items {
			articles = append(articles, Article{
				SubId:     s.id,
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

type ImportCmd struct {
	Path string `arg:""    help:"Subscriptions opml file."`
}

func (o *ImportCmd) Run() error {
	return nil
	// return db.ParseOPML(c.Path)
}

type VersionCmd struct {
}

func (c *VersionCmd) Run() error {
	fmt.Println("Version:", version)
	return nil
}
