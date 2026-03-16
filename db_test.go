package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var ctx = context.Background()

func setupTestDB(t *testing.T) (*DB, *DBCore, string) {
	t.Helper()
	dir := t.TempDir()
	globals = &Globals{
		PackageSize: 1, // 1 KB, small to test pack splitting
		OutputPath:  dir,
	}

	db, err := NewDB(ctx, false)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() {
		db.Close(ctx)
	})

	return db, &db.core, dir
}

func decompressGz(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return content
}

func readArticlesFromGz(t *testing.T, path string) []*Item {
	t.Helper()
	var articles []*Item
	dec := json.NewDecoder(bytes.NewReader(decompressGz(t, path)))
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatalf("Decode: %v", err)
		}
		var probe map[string]any
		if err := json.Unmarshal(raw, &probe); err != nil {
			t.Fatalf("Unmarshal probe: %v", err)
		}
		if _, ok := probe["ts"]; ok {
			continue
		}
		var a *Item
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Fatalf("Unmarshal item: %v", err)
		}
		articles = append(articles, a)
	}
	return articles
}

func TestPutArticlesBasic(t *testing.T) {
	db, c, dir := setupTestDB(t)
	c.Subscriptions = []*Subscription{
		{ID: 1, PackID: -1},
	}

	articles := []*Item{
		{SubID: 1, Title: "A1", Content: "C1", Link: "http://example.com/1", Published: 1000},
		{SubID: 1, Title: "A2", Content: "C2", Link: "http://example.com/2", Published: 2000},
	}

	if err := db.PutArticles(ctx,articles); err != nil {
		t.Fatalf("PutArticles: %v", err)
	}

	// Latest pack should exist
	latest := filepath.Join(dir, fmt.Sprintf("%v.gz", c.Latest))
	result := readArticlesFromGz(t, latest)
	if len(result) < 1 {
		t.Fatal("expected at least 1 article in latest pack")
	}
}

func TestPutArticlesEmpty(t *testing.T) {
	db, _, _ := setupTestDB(t)

	if err := db.PutArticles(ctx,nil); err != nil {
		t.Fatalf("PutArticles(nil): %v", err)
	}
	if err := db.PutArticles(ctx,[]*Item{}); err != nil {
		t.Fatalf("PutArticles([]): %v", err)
	}
}

func TestPutArticlesMultipleSubs(t *testing.T) {
	db, c, dir := setupTestDB(t)
	c.Subscriptions = []*Subscription{
		{ID: 1, PackID: -1},
		{ID: 2, PackID: -1},
	}

	articles := []*Item{
		{SubID: 1, Title: "Sub1-A", Published: 1000},
		{SubID: 2, Title: "Sub2-A", Published: 2000},
	}

	if err := db.PutArticles(ctx,articles); err != nil {
		t.Fatalf("PutArticles: %v", err)
	}

	latest := filepath.Join(dir, fmt.Sprintf("%v.gz", c.Latest))
	result := readArticlesFromGz(t, latest)

	subIds := map[int]bool{}
	for _, a := range result {
		subIds[a.SubID] = true
	}
	if !subIds[1] || !subIds[2] {
		t.Errorf("expected articles from both subs, got subIds: %v", subIds)
	}
}

func TestPutArticlesPackSplitting(t *testing.T) {
	db, c, dir := setupTestDB(t)
	// Very small pack size to force splitting
	globals.PackageSize = 0 // 0 KB -> split after every flush

	c.Subscriptions = []*Subscription{
		{ID: 1, PackID: -1},
	}

	articles := []*Item{
		{SubID: 1, Title: "A1", Content: "Content 1", Published: 1000},
		{SubID: 1, Title: "A2", Content: "Content 2", Published: 2000},
		{SubID: 1, Title: "A3", Content: "Content 3", Published: 3000},
	}

	if err := db.PutArticles(ctx,articles); err != nil {
		t.Fatalf("PutArticles: %v", err)
	}

	// With PackageSize=0, each article should create a numbered pack
	if c.NPacks <= 1 {
		t.Errorf("expected pack splitting, NPacks = %d", c.NPacks)
	}

	// Verify numbered pack exists
	pack1 := filepath.Join(dir, "1.gz")
	if _, err := os.Stat(pack1); os.IsNotExist(err) {
		t.Error("expected pack 1.gz to exist")
	}
}

func readMetaFromGz(t *testing.T, path string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(decompressGz(t, path)))
	for dec.More() {
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if _, ok := obj["ts"]; ok {
			return obj
		}
	}
	return nil
}

func TestPackMetadata(t *testing.T) {
	db, c, dir := setupTestDB(t)
	globals.PackageSize = 0 // force split after every article

	c.Subscriptions = []*Subscription{
		{ID: 1, PackID: -1},
		{ID: 2, PackID: -1},
	}

	articles := []*Item{
		{SubID: 1, Title: "A1", Content: "Content 1", Published: 1000},
		{SubID: 2, Title: "A2", Content: "Content 2", Published: 2000},
		{SubID: 1, Title: "A3", Content: "Content 3", Published: 3000},
	}

	if err := db.PutArticles(ctx,articles); err != nil {
		t.Fatalf("PutArticles: %v", err)
	}

	// Pack 2.gz should have a meta line with total=1
	// (1 article written before pack 2 started: A1 in pack 1)
	pack2 := filepath.Join(dir, "2.gz")
	meta := readMetaFromGz(t, pack2)
	if meta == nil {
		t.Fatal("expected meta line in pack 2.gz")
	}
	if _, ok := meta["ts"].(float64); !ok {
		t.Errorf("ts field missing or not a number: %v", meta["ts"])
	}
	if n, ok := meta["n"].(float64); !ok || int(n) != 1 {
		t.Errorf("n = %v, want 1", meta["n"])
	}

	// Verify cumulative counts on DBCore and Subscriptions
	if c.NArticles != 3 {
		t.Errorf("DBCore.NArticles = %d, want 3", c.NArticles)
	}
	if c.Subscriptions[0].NArticles != 2 {
		t.Errorf("Sub[0].NArticles = %d, want 2", c.Subscriptions[0].NArticles)
	}
	if c.Subscriptions[1].NArticles != 1 {
		t.Errorf("Sub[1].NArticles = %d, want 1", c.Subscriptions[1].NArticles)
	}
}

func TestCommitAndReadDB(t *testing.T) {
	db, c, dir := setupTestDB(t)
	c.Subscriptions = []*Subscription{
		{ID: 1, Title: "Test Feed", URL: "http://example.com/feed", PackID: -1},
	}
	c.NSubscriptions = 2

	if err := db.Commit(ctx); err != nil {
		t.Fatalf("CommitDB: %v", err)
	}

	// Read it back
	data, err := os.ReadFile(filepath.Join(dir, "db.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var core DBCore
	if err := json.Unmarshal(data, &core); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if core.NSubscriptions != 2 {
		t.Errorf("NSubscriptions = %d, want 2", core.NSubscriptions)
	}
	if len(core.Subscriptions) != 1 {
		t.Fatalf("Subscriptions len = %d, want 1", len(core.Subscriptions))
	}
	if core.Subscriptions[0].Title != "Test Feed" {
		t.Errorf("Sub title = %q, want %q", core.Subscriptions[0].Title, "Test Feed")
	}
}

func TestDBLocalCRUD(t *testing.T) {
	db, _, _ := setupTestDB(t)

	// Put + Get
	if err := db.Put(ctx, "test.txt", []byte("hello"), false); err != nil {
		t.Fatalf("Put: %v", err)
	}
	data, err := db.Get(ctx, "test.txt", false)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("Get = %q, want %q", data, "hello")
	}

	// Put with ignoreExisting=false should fail (file exists)
	if err := db.Put(ctx, "test.txt", []byte("world"), false); err == nil {
		t.Error("expected error for duplicate put with ignoreExisting=false")
	}

	// Put with ignoreExisting=true should overwrite
	if err := db.Put(ctx, "test.txt", []byte("world"), true); err != nil {
		t.Fatalf("Put(overwrite): %v", err)
	}
	data, _ = db.Get(ctx, "test.txt", false)
	if string(data) != "world" {
		t.Errorf("Get after overwrite = %q, want %q", data, "world")
	}

	// Get missing file with ignoreMissing=true
	data, err = db.Get(ctx, "missing.txt", true)
	if err != nil || data != nil {
		t.Errorf("Get(missing, ignore): data=%v, err=%v", data, err)
	}

	// Get missing file with ignoreMissing=false
	_, err = db.Get(ctx, "missing.txt", false)
	if err == nil {
		t.Error("expected error for missing file with ignoreMissing=false")
	}

	// Rm
	if err := db.Rm(ctx, "test.txt"); err != nil {
		t.Fatalf("Rm: %v", err)
	}
	data, _ = db.Get(ctx, "test.txt", true)
	if data != nil {
		t.Error("file still exists after Rm")
	}
}

func TestAtomicPut(t *testing.T) {
	db, _, dir := setupTestDB(t)

	if err := db.AtomicPut(ctx, "state.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("AtomicPut: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("content = %q", data)
	}

	// Temp file should not remain
	if _, err := os.Stat(filepath.Join(dir, "state.json.tmp")); !os.IsNotExist(err) {
		t.Error("temp file still exists after AtomicPut")
	}
}
