package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

func cmd_debug() {
	_, err := os.Stat(debug_folder)
	if err != nil {
		return
	}

	d, err := os.Open(debug_folder)
	if err != nil {
		fatal(fmt.Sprintf(`Unable to open debug folder "%s". %v`, debug_folder, err))
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		fatal(fmt.Sprintf(`Unable to read debug folder "%s". %v`, debug_folder, err))
	}
	for _, name := range names {
		full_name := filepath.Join(debug_folder, name)
		if err = os.RemoveAll(full_name); err != nil {
			fatal(fmt.Sprintf(`Unable to remove content "%s" inside debug folder "%s". %v`, debug_folder, full_name, err))
		}
	}
}

func cmd_add(subs ...*Subscription) {
	db := New_DB()
	for _, s := range subs {
		db.Add_sub(s)
	}
	db.Commit()
}

func cmd_rm(id int64) {
	db := New_DB()
	delete(db.Subscriptions, id)
	db.Commit()
}

func cmd_fetch() {
	db := New_DB()
	jobs_ch := make(chan *Subscription)

	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)

		go func() {
			defer wg.Done()
			buffer := make([]byte, max_download*(1<<10)+1)
			mod := New_Moduler()

			for s := range jobs_ch {

				if last_mod, err := s.Fetch(buffer); err != nil {
					warning(fmt.Sprintf(`Something went wrong while fetching "%s" (id: %d). %v`, s.Url, s.Id, err))
				} else if err := s.Process(mod); err != nil {
					warning(fmt.Sprintf(`Something went wrong while processing "%s" (id: %d). %v`, s.Url, s.Id, err))
				} else {
					s.Last_Mod_HTTP = last_mod
					db.Store(s)
				}
			}
		}()
	}

	for _, s := range db.Subscriptions {
		jobs_ch <- s
	}

	close(jobs_ch)
	wg.Wait()
	db.Commit()
}
