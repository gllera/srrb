package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"hash/fnv"
	"io"
	"log/slog"
	"net/url"
	"os"
)

type JsonEncoder struct {
	buffer  bytes.Buffer
	encoder *json.Encoder
}

func New_JsonEncoder() *JsonEncoder {
	enc := &JsonEncoder{}
	enc.encoder = json.NewEncoder(&enc.buffer)
	enc.encoder.SetEscapeHTML(false)
	return enc
}

type Packer struct {
	buffer bytes.Buffer
}

func (e *JsonEncoder) Encode(a any) ([]byte, error) {
	e.buffer.Reset()
	if err := e.encoder.Encode(a); err != nil {
		return nil, err
	}
	return e.buffer.Bytes(), nil
}

func New_Packer(path string) *Packer {
	p := &Packer{}

	if _, err := os.Stat(path); err == nil {
		file, err := os.Open(path)
		if err != nil {
			fatal("Unable to open file.", "path", path, "err", err.Error())
		}
		defer file.Close()

		gReader, err := gzip.NewReader(file)
		if err != nil {
			fatal("Unable to read file.", "path", path, "err", err.Error())
		}
		defer gReader.Close()

		data, err := io.ReadAll(gReader)
		if err != nil {
			fatal("Unable to read file.", "path", path, "err", err.Error())
		}

		p.buffer.Write(data)
	}

	return p
}

func hash(s string) uint {
	h := fnv.New32a()
	h.Write([]byte(s))
	return uint(h.Sum32())
}

func fatal(msg string, attr ...any) {
	slog.Error(msg, attr...)
	os.Exit(1)
}

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

func ParseOPML(filePath string) ([]*Subscription, error) {
	var root OPML
	var subs []*Subscription

	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err = xml.Unmarshal(b, &root); err != nil {
		return nil, err
	}

	condAppend := func(o *Outline, tag string) {
		u, err := url.Parse(o.XMLURL)
		if err == nil && u.Scheme != "" && u.Host != "" {
			subs = append(subs, &Subscription{
				Title: o.Title,
				Url:   o.XMLURL,
				Tag:   tag,
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

	return subs, nil
}
