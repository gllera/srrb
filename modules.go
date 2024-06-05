package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/html"
)

type Moduler struct {
	sanitizer *bluemonday.Policy
	minifier  *minify.M
	enc       *JsonEncoder
}

func New_Moduler() *Moduler {
	policy := bluemonday.StrictPolicy()

	policy.AllowLists()
	policy.AllowTables()
	policy.AllowImages()

	policy.RequireParseableURLs(true)
	policy.AllowRelativeURLs(true)
	policy.AllowURLSchemes("mailto", "http", "https")

	policy.AllowElements("article", "aside", "figure", "section", "summary", "hgroup")
	policy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	policy.AllowElements("br", "div", "hr", "p", "span", "wbr")
	policy.AllowElements("abbr", "acronym", "cite", "code", "dfn", "em", "figcaption", "mark", "s", "samp", "strong", "sub", "sup", "var")
	policy.AllowElements("b", "i", "pre", "small", "strike", "tt", "u")
	policy.AllowElements("rp", "rt", "ruby")

	policy.AllowAttrs("dir").Matching(bluemonday.Direction).Globally()
	policy.AllowAttrs("lang").Matching(regexp.MustCompile(`[a-zA-Z]{2,20}`)).Globally()
	policy.AllowAttrs("open").Matching(regexp.MustCompile(`(?i)^(|open)$`)).OnElements("details")
	policy.AllowAttrs("cite").OnElements("blockquote")
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowAttrs("name").Matching(regexp.MustCompile(`^([\p{L}\p{N}_-]+)$`)).OnElements("map")
	policy.AllowAttrs("alt").Matching(bluemonday.Paragraph).OnElements("area")
	policy.AllowAttrs("coords").Matching(regexp.MustCompile(`^([0-9]+,)+[0-9]+$`)).OnElements("area")
	policy.AllowAttrs("href").OnElements("area")
	policy.AllowAttrs("rel").Matching(bluemonday.SpaceSeparatedTokens).OnElements("area")
	policy.AllowAttrs("shape").Matching(regexp.MustCompile(`(?i)^(default|circle|rect|poly)$`)).OnElements("area")
	policy.AllowAttrs("usemap").Matching(regexp.MustCompile(`(?i)^#[\p{L}\p{N}_-]+$`)).OnElements("img")
	policy.AllowAttrs("cite").OnElements("q")
	policy.AllowAttrs("datetime").Matching(bluemonday.ISO8601).OnElements("time")
	policy.AllowAttrs("dir").Matching(bluemonday.Direction).OnElements("bdi", "bdo")
	policy.AllowAttrs("cite").Matching(bluemonday.Paragraph).OnElements("del", "ins")
	policy.AllowAttrs("datetime").Matching(bluemonday.ISO8601).OnElements("del", "ins")
	policy.AllowAttrs("value", "min", "max", "low", "high", "optimum").Matching(bluemonday.Number).OnElements("meter")
	policy.AllowAttrs("value", "max").Matching(bluemonday.Number).OnElements("progress")

	m := &Moduler{
		policy,
		minify.New(),
		New_JsonEncoder(),
	}

	m.minifier.AddFunc("text/html", html.Minify)
	return m
}

func (o *Moduler) Sanitize(i *gofeed.Item) {
	i.Content = o.sanitizer.Sanitize(i.Content)
}

func (o *Moduler) Minify(i *gofeed.Item) {
	i.Content, _ = o.minifier.String("text/html", i.Content)
}

func (o *Moduler) Process(args string, i *gofeed.Item) error {
	o.enc.Encode(i)
	var out bytes.Buffer
	GUID := i.GUID

	cmd := exec.Command("/bin/sh", "-c", args)
	cmd.Stdin = &o.enc.buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("SRR_OUTPUT_PATH=%s", globals.OutputPath),
		fmt.Sprintf("SRR_MAX_DOWNLOAD=%d", globals.MaxDownload),
	)

	if err := cmd.Run(); err != nil {
		return err
	}

	if err := json.Unmarshal(out.Bytes(), i); err != nil {
		return err
	}

	if GUID != i.GUID {
		return fmt.Errorf("field GUID can not be updated")
	}

	return nil
}
