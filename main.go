package main

import (
	"log/slog"

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
	}

	if err := ctx.Run(); err != nil {
		fatal(err.Error())
	}
}
