package main

type Article struct {
	SubId     int64  `json:"subId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Link      string `json:"link"`
	Published int64  `json:"published"`
	Prev      int64  `json:"prev,omitempty"`
}

func (p *Article) Size() int {
	return len(p.Title) + len(p.Content) + len(p.Link) + 16
}
