package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var db DB

type DB struct {
	SubIds int                      `json:"subids"`
	TagIds int                      `json:"tagids"`
	Tags   map[int]*SubscriptionTag `json:"tags"`
}

func (d *DB) Init() error {
	if d.TagIds != 0 {
		return nil
	}

	d.SubIds = 1
	d.TagIds = 1
	d.Tags = make(map[int]*SubscriptionTag)

	path := d.path()
	if _, err := os.Stat(path); err != nil {
		if err = os.MkdirAll(globals.OutputPath, 0755); err != nil {
			return fmt.Errorf(`unable to initialize output folder "%s". %v`, globals.OutputPath, err)
		}
	} else if fi, err := os.ReadFile(path); err != nil {
		return fmt.Errorf(`unable to read db file "%s". Msg: %v`, path, err)
	} else if err = json.Unmarshal(fi, d); err != nil {
		return fmt.Errorf(`unable to parse db file "%s". Msg: %v`, path, err)
	}

	for kT, vT := range d.Tags {
		vT.Id = kT
		for kS, vS := range vT.Subscriptions {
			vS.Id = kS
		}
	}

	return nil
}

func (d *DB) Commit() error {
	path := d.path()
	enc := New_JsonEncoder()

	var tmpFiles []string
	for _, t := range d.Tags {
		if tmp, err := t.Commit(); err != nil {
			return err
		} else if tmp != "" {
			tmpFiles = append(tmpFiles, tmp)
		}
	}

	tmp_db_file := path + ".tmp"
	bytes, _ := enc.Encode(d)

	if err := os.WriteFile(tmp_db_file, bytes, 0644); err != nil {
		return fmt.Errorf(`unable to write tmp db file "%s". Msg: %v`, tmp_db_file, err)
	} else if err = os.Rename(tmp_db_file, path); err != nil {
		return fmt.Errorf(`unable to replace db file "%s" with "%s". Msg: %v`, path, tmp_db_file, err)
	}

	for _, i := range tmpFiles {
		os.Remove(i)
	}

	return nil
}

func (d *DB) Add_sub(tagName string, sub *Subscription) error {
	if err := d.Init(); err != nil {
		return err
	}

	sub.Last_PackId = -1
	sub.Id = d.SubIds
	d.SubIds++

	tag := d.Get_tag(tagName)
	tag.Subscriptions[sub.Id] = sub

	return nil
}

func (d *DB) Rm_subs(ids ...int) error {
	if err := d.Init(); err != nil {
		return err
	}

	for _, t := range d.Tags {
		for _, id := range ids {
			delete(t.Subscriptions, id)
		}
	}

	return nil
}

func (d *DB) Rm_tags(ids ...int) error {
	if err := d.Init(); err != nil {
		return err
	}

	for _, id := range ids {
		delete(d.Tags, id)
	}

	return nil
}

func (d *DB) Get_tag(name string) *SubscriptionTag {
	for _, t := range d.Tags {
		if t.Name == name {
			return t
		}
	}

	tag := &SubscriptionTag{
		Id:            d.TagIds,
		Name:          name,
		PackIds:       1,
		Subscriptions: make(map[int]*Subscription),
	}
	d.Tags[d.TagIds] = tag
	d.TagIds++

	return tag
}

func (d *DB) Erase() error {
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

func (d *DB) path() string {
	return filepath.Join(globals.OutputPath, "db.json")
}

type DBItPair struct {
	Sub *Subscription
	Tag *SubscriptionTag
}

func (d *DB) Iterate(ch chan DBItPair) error {
	defer close(ch)

	if err := d.Init(); err != nil {
		return err
	}

	for _, t := range d.Tags {
		for _, s := range t.Subscriptions {
			ch <- DBItPair{
				Tag: t,
				Sub: s,
			}
		}
	}

	return nil
}
