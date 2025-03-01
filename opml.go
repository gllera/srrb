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

func ParseOPML(file string) (map[string][]*Subscription, error) {
	mapping := make(map[string][]*Subscription)

	var root OPML
	if b, err := os.ReadFile(file); err != nil {
		return nil, err
	} else if err = xml.Unmarshal(b, &root); err != nil {
		return nil, err
	}

	subs := make([]*Subscription, 0)
	for _, i := range root.Body.Outlines {
		if u, err := url.Parse(i.XMLURL); err == nil && u.Scheme != "" && u.Host != "" {
			subs = append(subs, &Subscription{
				Title: i.Title,
				Url:   i.XMLURL,
			})
		}
	}
	if len(subs) > 0 {
		mapping[""] = subs
	}

	for _, i := range root.Body.Outlines {
		subs := make([]*Subscription, 0)
		for _, j := range i.Outlines {
			if u, err := url.Parse(j.XMLURL); err == nil && u.Scheme != "" && u.Host != "" {
				subs = append(subs, &Subscription{
					Title: j.Title,
					Url:   j.XMLURL,
				})
			}
		}

		if len(subs) > 0 {
			mapping[i.Title] = subs
		}
	}

	return mapping, nil
}
