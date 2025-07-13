package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

type DB interface {
	Get(key string, ignore_missing bool) ([]byte, error)
	Put(key string, val []byte, ignore_existing bool) error
	AtomicPut(key string, val []byte) error
	Rm(key string) error
	Mkdir() error
	Core() *DB_core
}

type DB_core struct {
	Latest     bool            `json:"latest"`
	N_Packs    int             `json:"n_packs"`
	N_Subs     int             `json:"n_subs"`
	N_Exts     int             `json:"n_exts"`
	Subs       []*Subscription `json:"subs"`
	Exts       []*Extern       `json:"exts,omitempty"`
	Last_fetch int64           `json:"last_fetch,omitempty"`

	is_writable bool
}

type Extern struct {
	Name string `json:"name"`
	Url  string `json:"url"`
	Id   int    `json:"id"`
}

func (d *DB_core) Core() *DB_core {
	return d
}

func newDB_Core(is_writable bool) DB_core {
	return DB_core{
		N_Subs:      1,
		N_Exts:      1,
		N_Packs:     1,
		is_writable: is_writable,
	}
}

func (d *DB_core) unmarshal(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	var err error
	if err = json.Unmarshal(data, d); err != nil {
		return fmt.Errorf(`unable to parse db file. %v`, err)
	}

	return nil
}

func PutArticles(db DB, articles []Article) error {
	if len(articles) == 0 {
		return nil
	}
	c := db.Core()

	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	defer gz.Close()

	// Read the latest pack to get the current fetch state
	data, err := db.Get(fmt.Sprintf("%v.gz", c.Latest), true)
	if err != nil {
		return err
	}

	if len(data) != 0 {
		unziped, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		defer unziped.Close()

		_, err = io.Copy(gz, unziped)
		if err != nil {
			return err
		}
	}

	// Create subscriptions map
	subs := make(map[int]*Subscription)
	for _, sub := range c.Subs {
		subs[sub.Id] = sub
	}

	// Save articles
	jsonEncoder := New_JsonEncoder()
	for _, item := range articles {
		sub := subs[item.SubId]
		if sub.PackId != c.N_Packs {
			item.Prev = sub.PackId
			sub.PackId = c.N_Packs
		}

		data, _ := jsonEncoder.Encode(item)
		gz.Write(data)
		gz.Flush()

		if buffer.Len() >= (globals.PackageSize << 10) {
			save(fmt.Sprintf("%d.gz", c.N_Packs), gz, db, &buffer)
			c.N_Packs++
		}
	}

	// Save remaining articles without final pack
	if len(articles) > 0 {
		c.Latest = !c.Latest
		save(fmt.Sprintf("%v.gz", c.Latest), gz, db, &buffer)
	}

	return nil
}

func save(name string, gz *gzip.Writer, db DB, buffer *bytes.Buffer) error {
	gz.Close()
	if err := db.Put(name, buffer.Bytes(), true); err != nil {
		return err
	}
	buffer.Reset()
	gz.Reset(buffer)
	return nil
}

func UnlockDB(db DB) {
	db.Rm(".locked")
}

func CommitDB(db DB) error {
	data, _ := New_JsonEncoder().Encode(db.Core())
	return db.AtomicPut("db.json", data)
}

func NewDB(is_writable bool) (DB, *DB_core, error) {
	u, err := url.Parse(globals.OutputPath)
	if err != nil {
		panic(err)
	}

	var db DB
	var c *DB_core
	switch u.Scheme {
	case "":
		db, c, err = NewDB_Local(u, is_writable)
	case "s3":
		db, c, err = NewDB_S3(u, is_writable)
	default:
		err = fmt.Errorf(`unsupported output URL scheme %s`, u.Scheme)
	}
	if err != nil {
		return nil, nil, err
	}

	if is_writable {
		if err = db.Mkdir(); err != nil {
			return nil, nil, err
		} else if err = db.Put(".locked", []byte{}, globals.Force); err != nil {
			return nil, nil, err
		}
	}

	if data, err := db.Get("db.json", true); err != nil {
		return nil, nil, err
	} else if err = c.unmarshal(data); err != nil {
		return nil, nil, err
	}

	return db, c, nil
}
