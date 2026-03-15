package main

import (
	"encoding/xml"
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

func outlineToSub(o Outline) *Subscription {
	u, err := url.Parse(o.XMLURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	return &Subscription{Title: o.Title, URL: o.XMLURL}
}

func ParseOPML(file string) (map[string][]*Subscription, error) {
	mapping := make(map[string][]*Subscription)

	var root OPML
	if b, err := os.ReadFile(file); err != nil {
		return nil, err
	} else if err = xml.Unmarshal(b, &root); err != nil {
		return nil, err
	}

	// Collect top-level feeds (no group)
	for _, i := range root.Body.Outlines {
		if s := outlineToSub(i); s != nil {
			mapping[""] = append(mapping[""], s)
		}
	}

	// Collect grouped feeds
	for _, i := range root.Body.Outlines {
		for _, j := range i.Outlines {
			if s := outlineToSub(j); s != nil {
				mapping[i.Title] = append(mapping[i.Title], s)
			}
		}
	}

	return mapping, nil
}
