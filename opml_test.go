package main

import (
	"os"
	"testing"
)

func TestParseOPMLGrouped(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<body>
	<outline title="Tech" text="Tech">
		<outline title="Blog A" xmlUrl="http://example.com/a.xml"/>
		<outline title="Blog B" xmlUrl="http://example.com/b.xml"/>
	</outline>
	<outline title="News" text="News">
		<outline title="News C" xmlUrl="http://example.com/c.xml"/>
	</outline>
</body>
</opml>`

	f := writeTempFile(t, content)
	mapping, err := ParseOPML(f)
	if err != nil {
		t.Fatalf("ParseOPML: %v", err)
	}

	if len(mapping["Tech"]) != 2 {
		t.Errorf("Tech group: got %d subs, want 2", len(mapping["Tech"]))
	}
	if len(mapping["News"]) != 1 {
		t.Errorf("News group: got %d subs, want 1", len(mapping["News"]))
	}
	if mapping["Tech"][0].URL != "http://example.com/a.xml" {
		t.Errorf("first tech URL = %q", mapping["Tech"][0].URL)
	}
}

func TestParseOPMLFlat(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<body>
	<outline title="Direct Feed" xmlUrl="http://example.com/feed.xml"/>
</body>
</opml>`

	f := writeTempFile(t, content)
	mapping, err := ParseOPML(f)
	if err != nil {
		t.Fatalf("ParseOPML: %v", err)
	}

	root := mapping[""]
	if len(root) != 1 {
		t.Fatalf("root group: got %d subs, want 1", len(root))
	}
	if root[0].Title != "Direct Feed" {
		t.Errorf("title = %q, want %q", root[0].Title, "Direct Feed")
	}
}

func TestParseOPMLInvalidURL(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<body>
	<outline title="No URL"/>
	<outline title="Valid" xmlUrl="http://example.com/feed.xml"/>
</body>
</opml>`

	f := writeTempFile(t, content)
	mapping, err := ParseOPML(f)
	if err != nil {
		t.Fatalf("ParseOPML: %v", err)
	}

	root := mapping[""]
	if len(root) != 1 {
		t.Fatalf("root group: got %d subs, want 1 (invalid URL should be skipped)", len(root))
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.opml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}
