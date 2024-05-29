package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mmcdole/gofeed"
)

type Item struct {
	GUID      string `json:"-"`
	SubId     int64  `json:"subId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Link      string `json:"link"`
	Published int64  `json:"published"`
	Prev      int64  `json:"prev,omitempty"`
}

type Subscription struct {
	Id            int64    `json:"-"`
	Url           string   `json:"url"`
	Title         string   `json:"title,omitempty"`
	Tag           string   `json:"tag,omitempty"`
	Modules       []string `json:"modules,omitempty"`
	Last_GUID     string   `json:"last_uuid,omitempty"`
	Last_Mod_HTTP string   `json:"last_modified,omitempty"`
	Last_PackId   int64    `json:"last_packid,omitempty"`
	new_items     []*gofeed.Item
}

type DB struct {
	SubIds        int64                   `json:"subids"`
	PackIds       int64                   `json:"packids"`
	Latest        bool                    `json:"latest"`
	Subscriptions map[int64]*Subscription `json:"subscriptions"`
	packer        *Packer
	enc           *JsonEncoder
	path          string
	mutex         sync.Mutex
}

func New_DB() *DB {
	db := &DB{
		path: filepath.Join(output_folder, "db.json"),
		enc:  New_JsonEncoder(),
	}

	if _, err := os.Stat(db.path); err != nil {
		if err = os.MkdirAll(output_folder, 0755); err != nil {
			fatal(fmt.Sprintf(`Unable to initialize db file "%s". %v`, config_file, err))
		}

		db.Subscriptions = make(map[int64]*Subscription)
		db.PackIds = 1
		db.Commit()
	} else {
		if fi, err := os.ReadFile(db.path); err != nil {
			fatal(fmt.Sprintf(`Unable to read db file "%s". Msg: %v`, db.path, err))
		} else if err = json.Unmarshal(fi, db); err != nil {
			fatal(fmt.Sprintf(`Unable to parse db file "%s". Msg: %v`, db.path, err))
		}

		for k, v := range db.Subscriptions {
			v.Id = k
		}
	}

	return db
}

func (db *DB) Store(sub *Subscription) {
	if len(sub.new_items) == 0 {
		return
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.packer == nil {
		db.packer = New_Packer(db.pack_latest_path())
	}

	for i := len(sub.new_items) - 1; i >= 0; i-- {
		fItem := sub.new_items[i]
		item := Item{
			GUID:      fItem.GUID,
			SubId:     sub.Id,
			Title:     fItem.Title,
			Content:   fItem.Content,
			Link:      fItem.Link,
			Published: fItem.PublishedParsed.Unix(),
		}

		if db.packer.buffer.Len()+item.Size() >= (package_size<<10)*7/2 {
			db.packer.flush(db.pack_path())
			db.PackIds++
		}

		if sub.Last_PackId != db.PackIds {
			item.Prev = sub.Last_PackId
			sub.Last_PackId = db.PackIds
		}

		data, _ := db.enc.Encode(item)
		db.packer.buffer.Write(data)
	}

	sub.Last_GUID = sub.new_items[0].GUID
}

func (db *DB) Add_sub(s *Subscription) {
	s.Last_PackId = -1
	s.Id = db.SubIds

	db.Subscriptions[s.Id] = s
	db.SubIds++
}

func (db *DB) Rm_sub(id int64) {
	delete(db.Subscriptions, id)
}

func (db *DB) Commit() {
	var old_path string

	if db.packer != nil {
		old_path = db.pack_latest_path()
		db.Latest = !db.Latest
		db.packer.flush(db.pack_latest_path())
	}

	tmp_db_file := db.path + ".tmp"
	bytes, _ := db.enc.Encode(db)

	if err := os.WriteFile(tmp_db_file, bytes, 0644); err != nil {
		fatal(fmt.Sprintf(`Unable to write tmp db file "%s". Msg: %v`, tmp_db_file, err))
	} else if err = os.Rename(tmp_db_file, db.path); err != nil {
		fatal(fmt.Sprintf(`Unable to replace db file "%s" with "%s". Msg: %v`, db.path, tmp_db_file, err))
	}

	if old_path != "" {
		os.Remove(old_path)
	}
}

func (p *Item) Size() int {
	return len(p.Title) + len(p.Content) + len(p.Link) + 16
}

func (p *Packer) flush(path string) {
	nf, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fatal(err)
	}
	defer nf.Close()

	gw := gzip.NewWriter(nf)
	if _, err = gw.Write(p.buffer.Bytes()); err != nil {
		fatal(err)
	}
	if err = gw.Close(); err != nil {
		fatal(err)
	}

	p.buffer.Reset()
}

func (db *DB) pack_latest_path() string {
	return filepath.Join(output_folder, fmt.Sprintf("%v.json.gz", db.Latest))
}

func (db *DB) pack_path() string {
	return filepath.Join(output_folder, fmt.Sprintf("%v.json.gz", db.PackIds))
}
