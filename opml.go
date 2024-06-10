package main

import (
	"encoding/xml"
	"log/slog"
	"net/url"
	"os"
)

type OPML struct {
	Body Body `xml:"body"`
}

type Body struct {
	Outlines []Outline `xml:"outline"`
}

type Outline struct {
	XMLURL   string    `xml:"xmlUrl,attr"`
	Title    string    `xml:"title,attr"`
	Outlines []Outline `xml:"outline"`
}

func (d *DB) ParseOPML(file string) error {
	var root OPML

	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	if err = xml.Unmarshal(b, &root); err != nil {
		return err
	}

	condAppend := func(o *Outline, tag string) {
		u, err := url.Parse(o.XMLURL)
		if err == nil && u.Scheme != "" && u.Host != "" {
			d.Add_sub(tag, &Subscription{
				Title: o.Title,
				Url:   o.XMLURL,
			})
		} else if o.XMLURL != "" {
			slog.Info("Ignoring invalid URL.", "url", o.XMLURL)
		}
	}

	for _, i := range root.Body.Outlines {
		condAppend(&i, "")
		for _, j := range i.Outlines {
			condAppend(&j, i.Title)
		}
	}

	return nil
}
