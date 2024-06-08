package main

import (
	"log/slog"
	"net/url"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
)

var version = "development"
var globals *Globals

func main() {
	var cli CLI
	globals = &cli.Globals

	ctx := kong.Parse(&cli,
		kong.Name("srr"),
		kong.Description("Static RSS Reader backend."),
		kong.Configuration(kongyaml.Loader, "config.yaml"),
		kong.ShortUsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:   true,
			FlagsLast: true,
		}),
	)

	if cli.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		cli.OutputPath = cli.DebugPath
		if err := db.Erase(); err != nil {
			fatal("Something went wrong while cleaning debug folder.", "path", cli.OutputPath, "err", err.Error())
		}
	}

	if err := ctx.Run(); err != nil {
		fatal("Something went wrong while executing command.", "err", err.Error())
	}

	if err := db.Commit(); err != nil {
		fatal("Something went wrong while saving changes.", "err", err.Error())
	}
}

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

type RmCmd struct {
	Id []int `arg:"" help:"Subscriptions Ids to remove."`
}

type FetchCmd struct {
}

type ImportCmd struct {
	Path string `arg:""    help:"Subscriptions opml file."`
	Tag  string `short:"g" help:"Subscriptions tag."`
}

type VersionCmd struct {
}
