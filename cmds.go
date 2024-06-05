package main

import (
	"fmt"
	"sync"
)

func (c *VersionCmd) Run() error {
	fmt.Println("srr version", version)
	return nil
}

func (c *AddCmd) Run() error {
	return db.Add_subs(&Subscription{
		Title:   c.Title,
		Url:     c.URL.String(),
		Tag:     c.Tag,
		Modules: c.Parser,
	})
}

func (c *RmCmd) Run() error {
	return db.Rm_sub(c.Id...)
}

func (c *FetchCmd) Run() error {
	var wg sync.WaitGroup
	jobs_ch := make(chan *Subscription)
	defer close(jobs_ch)

	for range globals.Jobs {
		wg.Add(1)

		go func() {
			defer wg.Done()
			buffer := make([]byte, globals.MaxDownload*(1<<10)+1)
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

	subs, err := db.Get_subs()
	if err != nil {
		return err
	}
	for _, s := range subs {
		jobs_ch <- s
	}

	wg.Wait()
	return nil
}

func (c *ImportCmd) Run() error {
	ParseOPML(c.Path)
	return nil
}
