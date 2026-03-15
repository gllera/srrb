package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONEncodeRoundTrip(t *testing.T) {
	type item struct {
		Name string `json:"name"`
		HTML string `json:"html"`
	}

	input := item{Name: "test", HTML: "<b>bold</b>"}
	data, err := jsonEncode(input)
	if err != nil {
		t.Fatalf("jsonEncode: %v", err)
	}

	var output item
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if output != input {
		t.Errorf("got %+v, want %+v", output, input)
	}
}

func TestJSONEncodeNoHTMLEscape(t *testing.T) {
	data, err := jsonEncode(map[string]string{"html": "<b>test</b>"})
	if err != nil {
		t.Fatalf("jsonEncode: %v", err)
	}

	s := string(data)
	if strings.Contains(s, `\u003c`) || strings.Contains(s, `\u003e`) {
		t.Errorf("HTML was escaped: %s", s)
	}
}
