package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
)

type SubscriptionTag struct {
	Id            int                   `json:"-"`
	Name          string                `json:"name"`
	PackIds       int                   `json:"packids"`
	Latest        bool                  `json:"latest"`
	Subscriptions map[int]*Subscription `json:"subscriptions"`
	packer        *Packer
	enc           *JsonEncoder
	mutex         sync.Mutex
}

func (t *SubscriptionTag) Store(sub *Subscription) {
	if len(sub.new_items) == 0 {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.packer == nil {
		t.packer = New_Packer(t.latest_path())
		t.enc = New_JsonEncoder()
	}

	for i := len(sub.new_items) - 1; i >= 0; i-- {
		fItem := sub.new_items[i]
		item := SubscriptionItem{
			SubId:     sub.Id,
			Title:     fItem.Title,
			Content:   fItem.Content,
			Link:      fItem.Link,
			Published: int(fItem.PublishedParsed.Unix()),
		}

		if t.packer.buffer.Len()+item.Size() >= (globals.PackageSize<<10)*7/2 {
			t.packer.flush(t.pack_path())
			t.PackIds++
		}

		if sub.Last_PackId != t.PackIds {
			item.Prev = sub.Last_PackId
			sub.Last_PackId = t.PackIds
		}

		data, _ := t.enc.Encode(item)
		t.packer.buffer.Write(data)
	}

	sub.Last_GUID = hash(sub.new_items[0].GUID)
}

func (t *SubscriptionTag) Commit() (string, error) {
	if t.enc == nil {
		return "", nil
	}

	old_path := ""
	if t.packer != nil {
		old_path = t.latest_path()
		t.Latest = !t.Latest
		t.packer.flush(t.latest_path())
	}

	return old_path, nil
}

func (t *SubscriptionTag) latest_path() string {
	return filepath.Join(globals.OutputPath, strconv.Itoa(t.Id), fmt.Sprintf("%v.gz", t.Latest))
}

func (t *SubscriptionTag) pack_path() string {
	return filepath.Join(globals.OutputPath, strconv.Itoa(t.Id), fmt.Sprintf("%v.gz", t.PackIds))
}
