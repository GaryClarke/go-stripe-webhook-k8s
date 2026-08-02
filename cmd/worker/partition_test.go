package main

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestRecordsSafeToCommit(t *testing.T) {
	rec := func(offset int64) *kgo.Record {
		return &kgo.Record{Topic: "stripe-events", Partition: 0, Offset: offset}
	}

	t.Run("all succeed", func(t *testing.T) {
		records := []*kgo.Record{rec(9), rec(10), rec(11)}
		got := recordsSafeToCommit(records, func(*kgo.Record) bool { return true })
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		if got[0].Offset != 9 || got[1].Offset != 10 || got[2].Offset != 11 {
			t.Fatalf("offsets = [%d,%d,%d], want [9,10,11]", got[0].Offset, got[1].Offset, got[2].Offset)
		}
	})

	t.Run("failure stops batch without skipping ahead", func(t *testing.T) {
		records := []*kgo.Record{rec(9), rec(10), rec(11)}
		got := recordsSafeToCommit(records, func(r *kgo.Record) bool {
			return r.Offset != 10 // only the first record (9) should be returned
		})
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		if got[0].Offset != 9 {
			t.Fatalf("committed offset = %d, want 9", got[0].Offset)
		}
	})

	t.Run("first record fails", func(t *testing.T) {
		records := []*kgo.Record{rec(10), rec(11)}
		got := recordsSafeToCommit(records, func(*kgo.Record) bool { return false })
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("empty batch", func(t *testing.T) {
		got := recordsSafeToCommit(nil, func(*kgo.Record) bool { return true })
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})
}
