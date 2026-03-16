package main

import (
	"reflect"
	"testing"
	"time"
)

func retentionDB(packTS [][2]int64) [][2]int64 {
	db := &DB{core: DBCore{PackTS: packTS}}
	db.UpdateRetainedPacks()
	return db.core.PackTS
}

func retentionIDs(ages [][2]int64) []int64 {
	now := time.Now().UTC().Unix()
	var packTS [][2]int64
	for _, a := range ages {
		packTS = append(packTS, [2]int64{now - a[0], a[1]})
	}
	var ids []int64
	for _, p := range retentionDB(packTS) {
		ids = append(ids, p[1])
	}
	return ids
}

func TestRetainedPacks(t *testing.T) {
	h, d := int64(3600), int64(86400)

	// Ages in descending order (oldest first → ascending timestamps).
	tests := []struct {
		name    string
		ages    [][2]int64 // {age_seconds, pack_id}
		wantIDs []int64    // expected pack IDs in order; nil = expect empty
	}{
		{"empty", nil, nil},
		{"age_zero_excluded", [][2]int64{{0, 1}}, nil},
		{"older_than_30d", [][2]int64{{50 * d, 1}, {31 * d, 2}}, nil},
		{"single_pack", [][2]int64{{h, 1}}, []int64{1}},
		{"oldest_in_slot", [][2]int64{{14000, 1}, {10000, 2}, {5000, 3}}, []int64{1}},
		{"same_timestamp", [][2]int64{{h, 1}, {h, 2}, {h, 3}}, []int64{1}},
		{"within_12h", [][2]int64{
			{10 * h, 1}, {9 * h, 2}, {5 * h, 3}, {1800, 4},
		}, []int64{1, 3, 4}},
		{"exact_boundaries", [][2]int64{
			{30 * d, 1}, {15 * d, 2}, {5 * d, 3}, {24 * h, 4}, {12 * h, 5},
		}, []int64{1, 2, 3, 4, 5}},
		{"gaps_between_buckets", [][2]int64{{25 * d, 1}, {2 * h, 2}}, []int64{1, 2}},
		{"mixed_drop_expired", [][2]int64{
			{31 * d, 1}, {14 * d, 2}, {3 * d, 3}, {10 * h, 4}, {1800, 5},
		}, []int64{2, 3, 4, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retentionIDs(tt.ages); !reflect.DeepEqual(got, tt.wantIDs) {
				t.Fatalf("got %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestRetainedPacks_DenseSlots(t *testing.T) {
	now := time.Now().UTC().Unix()
	// 10 packs per slot across bucket 0's 3 slots → oldest per slot retained
	var packTS [][2]int64
	id := int64(1)
	for slot := 2; slot >= 0; slot-- {
		base := now - int64(slot+1)*4*3600
		for i := 0; i < 10; i++ {
			packTS = append(packTS, [2]int64{base + int64(i)*1000, id})
			id++
		}
	}
	got := retentionDB(packTS)
	want := [][2]int64{packTS[0], packTS[10], packTS[20]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRetainedPacks_AllSlotsFilled(t *testing.T) {
	now := time.Now().UTC().Unix()
	// One pack at midpoint of every slot: 3+1+4+2+1 = 11
	buckets := []struct{ start, interval, count int64 }{
		{0, 4 * 3600, 3},
		{12 * 3600, 12 * 3600, 1},
		{24 * 3600, 86400, 4},
		{5 * 86400, 5 * 86400, 2},
		{15 * 86400, 15 * 86400, 1},
	}
	var packTS [][2]int64
	id := int64(1)
	for _, b := range buckets {
		for i := int64(b.count - 1); i >= 0; i-- {
			packTS = append(packTS, [2]int64{now - b.start - i*b.interval - b.interval/2, id})
			id++
		}
	}
	if got := retentionDB(packTS); len(got) != 11 {
		t.Fatalf("expected 11, got %d: %v", len(got), got)
	}
}

func TestRetainedPacks_EveryHalfHour30Days(t *testing.T) {
	now := time.Now().UTC().Unix()
	var packTS [][2]int64
	id := int64(1)
	for ts := now - 30*86400; ts < now; ts += 1800 {
		packTS = append(packTS, [2]int64{ts, id})
		id++
	}
	if got := retentionDB(packTS); len(got) > 11 || len(got) == 0 {
		t.Fatalf("expected 1-11, got %d", len(got))
	}
}

func TestRetainedPacks_Idempotent(t *testing.T) {
	now := time.Now().UTC().Unix()
	d := int64(86400)
	packTS := [][2]int64{
		{now - 25*d, 1}, {now - 10*d, 2}, {now - 3*d, 3},
		{now - d - 1, 4}, {now - 18*3600, 5}, {now - 6*3600, 6}, {now - 1800, 7},
	}
	first := retentionDB(packTS)
	second := retentionDB(first)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("not idempotent: %v vs %v", first, second)
	}
	for i := 1; i < len(first); i++ {
		if first[i][1] <= first[i-1][1] {
			t.Fatalf("not sorted by pack ID at index %d: %v", i, first)
		}
	}
}
