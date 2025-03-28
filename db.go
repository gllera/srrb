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

func FlushBuffer(buffer *bytes.Buffer) []byte {
	compresed := &bytes.Buffer{}
	gw := gzip.NewWriter(compresed)

	gw.Write(buffer.Bytes())
	gw.Close()

	buffer.Reset()
	return compresed.Bytes()
}

func PutArticles(db DB, articles []Article) error {
	if len(articles) == 0 {
		return nil
	}
	c := db.Core()

	var buffer bytes.Buffer
	if data, err := db.Get(fmt.Sprintf("%v.gz", c.Latest), true); err != nil {
		return err
	} else if len(data) == 0 {
	} else if unziped, err := gzip.NewReader(bytes.NewReader(data)); err != nil {
		return err
	} else {
		defer unziped.Close()
		if _, err = io.Copy(&buffer, unziped); err != nil {
			return err
		}
	}

	subs := make(map[int]*Subscription)
	for _, sub := range c.Subs {
		subs[sub.Id] = sub
	}

	jsonEncoder := New_JsonEncoder()
	for _, item := range articles {
		if buffer.Len()+item.Size() >= (globals.PackageSize<<10)*7/2 {
			c.N_Packs++
			if err := db.Put(fmt.Sprintf("%d.gz", c.N_Packs), FlushBuffer(&buffer), true); err != nil {
				return err
			}
		}

		sub := subs[item.SubId]
		if sub.PackId != c.N_Packs {
			item.Prev = sub.PackId
			sub.PackId = c.N_Packs
		}

		data, _ := jsonEncoder.Encode(item)
		buffer.Write(data)
	}

	if len(articles) > 0 {
		c.Latest = !c.Latest
		if err := db.Put(fmt.Sprintf("%v.gz", c.Latest), FlushBuffer(&buffer), true); err != nil {
			return err
		}
	}

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
