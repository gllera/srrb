package main

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
)

var version = "development"
var globals *Globals

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

	if err := ctx.Run(); err != nil {
		fatal(err.Error())
	}
}
