package main

import "time"

var retentionPolicy = []struct{ start, end, interval int64 }{
	{0, 12 * 3600, 4 * 3600},             // every 4h,   0–12h
	{12 * 3600, 24 * 3600, 12 * 3600},    // every 12h,  12h–24h
	{24 * 3600, 5 * 86400, 86400},        // every 1d,   1d–5d
	{5 * 86400, 15 * 86400, 5 * 86400},   // every 5d,   5d–15d
	{15 * 86400, 30 * 86400, 15 * 86400}, // every 15d,  15d–30d
}

// UpdateRetainedPacks filters PackTS to keep only the subset of packs
// matching the retention policy. For each time slot, the oldest matching
// pack is retained.
func (o *DB) UpdateRetainedPacks() {
	now := time.Now().UTC().Unix()
	seen := make(map[[2]int]struct{})
	var result [][2]int64

	for _, p := range o.core.PackTS {
		age := now - p[0]
		for bi, b := range retentionPolicy {
			if age > b.start && age <= b.end {
				k := [2]int{bi, int((age - b.start - 1) / b.interval)}
				if _, ok := seen[k]; !ok {
					seen[k] = struct{}{}
					result = append(result, p)
				}
				break
			}
		}
	}

	o.core.PackTS = result
}
