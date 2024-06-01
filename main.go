package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"gopkg.in/yaml.v3"
)

var (
	jobs          int    = runtime.NumCPU() * 2
	max_download  int    = 5000
	package_size  int    = 200
	output_folder string = "packs"
	debug_folder  string = "debug"
	config_file   string = "config.yaml"
)

func main() {
	cmds := []command{
		{"add [--title|-t TITLE] [--tag|-g TAG] [--module|-m MODULE ...] [--opml|-i] URL/PATH", "Subscribe/Import to RSS URL/Opml"},
		{"fetch", "Download and process articles from subscriptions"},
		{"rm ID", "Remove RSS subscription"},
		{`debug (same as "add")`, `Equivalent to "add & fetch" but using "debug" folder and cleaning it up first`},
	}
	params := []*flag{
		{"jobs", "j", &jobs, false, "Number of concurrent downloads"},
		{"max_download", "m", &max_download, false, "Max downloads file KB size"},
		{"package_size", "s", &package_size, false, "Target package KB size"},
		{"output_folder", "o", &output_folder, false, "Destination output folder"},
		{"debug_folder", "d", &debug_folder, false, "Destination debug folder"},
		{"config", "c", &config_file, false, "Configuration file"},
	}
	flags := map[string]*flag{}
	for _, i := range params {
		flags[i.long] = i
	}

	usage := func() {
		fmt.Printf("Usage of %s\n", os.Args[0])
		fmt.Println("\nCommands:")
		for _, v := range cmds {
			fmt.Print(v.Help())
		}

		fmt.Println("\nCommon flags:")
		for _, v := range flags {
			fmt.Print(v.Help())
		}
		os.Exit(0)
	}

	var args []string

	// Parameterized variables
	for i := 1; i < len(os.Args); i++ {
		curr := os.Args[i]

		if curr == "--help" || curr == "-h" {
			usage()
		}

		for _, f := range flags {
			if curr == f.Short() || curr == f.Long() {
				f.SetF(pop_str(os.Args, &i))
				goto another_arg
			}
		}

		args = os.Args[i:]
		break
	another_arg:
	}

	// Environment variables
	for _, v := range flags {
		v.Set(os.Getenv(v.Env()))
	}

	// Configuration file variables
	if _, err := os.Stat(config_file); err == nil {
		configMap := make(map[string]string)

		if fi, err := os.ReadFile(config_file); err != nil {
			fatal(fmt.Sprintf(`Unable to read configuration file "%s". %v`, config_file, err))
		} else if err = yaml.Unmarshal(fi, configMap); err != nil {
			fatal(fmt.Sprintf(`Unable to parse configuration file "%s". %v`, config_file, err))
		}

		for _, v := range flags {
			if newVal, ok := configMap[v.long]; ok {
				v.Set(newVal)
			}
		}
	} else if flags["config"].defined {
		fatal(fmt.Sprintf(`Configuration file not found "%s"`, config_file))
	}

	if len(args) == 0 {
		usage()
	}

	switch args[0] {
	case "debug":
		cmd_debug()
		output_folder = debug_folder
		fallthrough
	case "add":
		isOpml := false
		found := false
		sub := &Subscription{}

		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--opml", "-i":
				isOpml = true
			case "--title", "-t":
				sub.Title = pop_str(args, &i)
			case "--tag", "-g":
				sub.Tag = pop_str(args, &i)
			case "--module", "-m":
				sub.Modules = append(sub.Modules, pop_str(args, &i))
			default:
				if found {
					fatal("Repeated parameter URL")
				}

				sub.Url = args[i]
				found = true
			}
		}

		if !found {
			fatal("Parameter URL/Path not set")
		}

		if isOpml {
			subs, err := ParseOPML(sub.Url)
			if err != nil {
				fatal(fmt.Sprintf(`Failed to parse OPML file "%s". %v`, sub.Url, err))
			} else if sub.Tag != "" {
				for _, s := range subs {
					s.Tag = sub.Tag
				}
			}

			cmd_add(subs...)
		} else {
			cmd_add(sub)
		}

		if args[0] == "debug" {
			cmd_fetch()
		}
	case "fetch":
		if len(args) > 1 {
			fatal(fmt.Sprintf(`Unrecognized parameter "%s"`, args[1]))
		} else {
			cmd_fetch()
		}
	case "rm":
		if len(args) == 1 {
			fatal("No subscription ID defined")
		} else if len(args) > 2 {
			fatal(fmt.Sprintf(`Repeated "ID" parameter "%s"`, args[2]))
		} else if val, err := strconv.ParseInt(args[1], 10, 64); err != nil {
			fatal(fmt.Sprintf(`Unable to parse int parameter "%s"`, args[1]))
		} else {
			cmd_rm(val)
		}
	default:
		fatal(fmt.Sprintf(`Unrecognized command "%s"`, args[0]))
	}
}
