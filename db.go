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

var db DB

type DB struct {
	SubIds        int                   `json:"subids"`
	PackIds       int                   `json:"packids"`
	Latest        bool                  `json:"latest"`
	Subscriptions map[int]*Subscription `json:"subscriptions"`
	packer        *Packer
	enc           *JsonEncoder
	path          string
	mutex         sync.Mutex
}

type Subscription struct {
	Id            int      `json:"-"`
	Url           string   `json:"url"`
	Title         string   `json:"title,omitempty"`
	Tag           string   `json:"tag,omitempty"`
	Modules       []string `json:"modules,omitempty"`
	Last_GUID     uint     `json:"last_uuid,omitempty"`
	Last_Mod_HTTP int64    `json:"last_modified,omitempty"`
	Last_PackId   int      `json:"last_packid,omitempty"`
	new_items     []*gofeed.Item
}

type Item struct {
	GUID      string `json:"-"`
	SubId     int    `json:"subId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Link      string `json:"link"`
	Published int    `json:"published"`
	Prev      int    `json:"prev,omitempty"`
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
			Published: int(fItem.PublishedParsed.Unix()),
		}

		if db.packer.buffer.Len()+item.Size() >= (globals.PackageSize<<10)*7/2 {
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

	sub.Last_GUID = hash(sub.new_items[0].GUID)
}

func (db *DB) Get_subs() (map[int]*Subscription, error) {
	if err := db.Init(); err != nil {
		return nil, err
	}

	return db.Subscriptions, nil
}

func (db *DB) Add_subs(subs ...*Subscription) error {
	if err := db.Init(); err != nil {
		return err
	}

	for _, s := range subs {
		s.Last_PackId = -1
		s.Id = db.SubIds

		db.Subscriptions[s.Id] = s
		db.SubIds++
	}

	return nil
}

func (db *DB) Rm_sub(ids ...int) error {
	if err := db.Init(); err != nil {
		return err
	}

	for _, id := range ids {
		delete(db.Subscriptions, id)
	}

	return nil
}

func (db *DB) Erase() error {
	_, err := os.Stat(globals.OutputPath)
	if err != nil {
		return nil
	}

	dir, err := os.Open(globals.OutputPath)
	if err != nil {
		return fmt.Errorf(`unable to open debug folder "%s". %v`, globals.OutputPath, err)
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf(`unable to read debug folder "%s". %v`, globals.OutputPath, err)
	}

	for _, name := range names {
		full_name := filepath.Join(globals.OutputPath, name)
		if err = os.RemoveAll(full_name); err != nil {
			return fmt.Errorf(`unable to remove content "%s" inside debug folder "%s". %v`, globals.OutputPath, full_name, err)
		}
	}

	return nil
}

func (db *DB) Init() error {
	if db.enc != nil {
		return nil
	}

	db.PackIds = 1
	db.Subscriptions = make(map[int]*Subscription)
	db.path = filepath.Join(globals.OutputPath, "db.json")
	db.enc = New_JsonEncoder()

	if _, err := os.Stat(db.path); err != nil {
		if err = os.MkdirAll(globals.OutputPath, 0755); err != nil {
			return fmt.Errorf(`unable to initialize output folder "%s". %v`, globals.OutputPath, err)
		}
	} else if fi, err := os.ReadFile(db.path); err != nil {
		return fmt.Errorf(`unable to read db file "%s". Msg: %v`, db.path, err)
	} else if err = json.Unmarshal(fi, db); err != nil {
		return fmt.Errorf(`unable to parse db file "%s". Msg: %v`, db.path, err)
	}

	for k, v := range db.Subscriptions {
		v.Id = k
	}

	return nil
}

func (db *DB) Commit() error {
	if db.enc == nil {
		return nil
	}

	var old_path string
	if db.packer != nil {
		old_path = db.pack_latest_path()
		db.Latest = !db.Latest
		db.packer.flush(db.pack_latest_path())
	}

	tmp_db_file := db.path + ".tmp"
	bytes, _ := db.enc.Encode(db)

	if err := os.WriteFile(tmp_db_file, bytes, 0644); err != nil {
		return fmt.Errorf(`unable to write tmp db file "%s". Msg: %v`, tmp_db_file, err)
	} else if err = os.Rename(tmp_db_file, db.path); err != nil {
		return fmt.Errorf(`unable to replace db file "%s" with "%s". Msg: %v`, db.path, tmp_db_file, err)
	}

	if old_path != "" {
		os.Remove(old_path)
	}

	return nil
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
	return filepath.Join(globals.OutputPath, fmt.Sprintf("%v.gz", db.Latest))
}

func (db *DB) pack_path() string {
	return filepath.Join(globals.OutputPath, fmt.Sprintf("%v.gz", db.PackIds))
}
