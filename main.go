package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
)

var version = "development"
var globals *Globals

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
	Import  ImportCmd  `cmd:"" help:"Import opml subscriptions file."`
	Preview PreviewCmd `cmd:"" help:"Preview processed feed articles in a browser."`
	Version VersionCmd `cmd:"" help:"Print version information."`
}

type VersionCmd struct{}

func (o *VersionCmd) Run() error {
	fmt.Println("Version:", version)
	return nil
}

func fatal(msg string, attr ...any) {
	slog.Error(msg, attr...)
	os.Exit(1)
}

func main() {
	var cli CLI
	globals = &cli.Globals

	ctx := kong.Parse(&cli,
		kong.Vars{
			"nproc": fmt.Sprint(runtime.NumCPU()),
		},
		kong.Name("srr"),
		kong.Description("Static RSS Reader backend."),
		kong.Configuration(kongyaml.Loader, "config.yaml"),
		kong.ShortUsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			FlagsLast:           true,
			NoExpandSubcommands: true,
		}),
	)

	if globals.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if globals.OutputPath == "" {
		fatal("output path is required")
	}

	if globals.PackageSize < 1 {
		globals.PackageSize = 200
	}

	if globals.MaxDownload < 1 {
		globals.MaxDownload = 5000
	}

	if globals.Jobs < 1 {
		globals.Jobs = runtime.NumCPU()
	}

	if err := ctx.Run(); err != nil {
		fatal(err.Error())
	}
}
