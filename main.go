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
