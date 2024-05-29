package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

func info(msg any) {
	log.Println("INFO", msg)
}

func warning(msg any) {
	log.Println("ERROR", msg)
}

func fatal(msg any) {
	log.Fatalln("FATAL", msg)
}

type command struct {
	args string
	help string
}

type flag struct {
	long    string
	short   string
	ptr     interface{}
	defined bool
	help    string
}

func (c *command) Help() string {
	return fmt.Sprintf("  %s\n        %s\n", c.args, c.help)
}

func (c *flag) Long() string {
	return "--" + c.long
}

func (c *flag) Short() string {
	return "-" + c.short
}

func (c *flag) Env() string {
	return "SRR_" + strings.ToUpper(c.long)
}

func (c *flag) Help() string {
	varType := "[INVALID]"
	def := "[INVALID]"

	switch c.ptr.(type) {
	case *int:
		varType = "int"
		def = fmt.Sprint(*c.ptr.(*int))
	case *string:
		varType = "string"
		def = fmt.Sprintf(`"%s"`, *c.ptr.(*string))
	}

	return fmt.Sprintf("  %s, %s %s\n        %s (default %s)\n", c.Long(), c.Short(), varType, c.help, def)
}

func (c *flag) SetCond(val string, force bool) {
	if (!force && c.defined) || val == "" {
		return
	}

	switch c.ptr.(type) {
	case *int:
		var err error

		v, _ := c.ptr.(*int)
		*v, err = strconv.Atoi(val)

		if err != nil {
			fatal(fmt.Sprintf(`Unable to parse int variable "%s" from "%s"`, c.long, val))
		}
	case *string:
		v, _ := c.ptr.(*string)
		*v = val
	}

	c.defined = true
}

func (c *flag) SetF(val string) {
	c.SetCond(val, true)
}

func (c *flag) Set(val string) {
	c.SetCond(val, false)
}

func pop_str(args []string, idx *int) string {
	(*idx)++

	if *idx == len(args) {
		fatal(fmt.Sprintf(`Missing argument for parameter "%s"`, args[*idx-1]))
	}

	return args[*idx]
}

// func pop_int(args []string, idx *int) int {
// 	name := args[*idx]
// 	txt := pop_str(args, idx)

// 	val, err := strconv.Atoi(txt)
// 	if err != nil {
// 		fatal(fmt.Sprintf(`Unable to parse int parameter "%s" from "%s"`, name, txt))
// 	}

// 	return val
// }
