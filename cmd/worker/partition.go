package main

import "github.com/twmb/franz-go/pkg/kgo"

// recordsSafeToCommit returns the prefix of records safe to commit for one partition
// batch. Processing stops at the first record for which handle returns false so a
// later offset is never committed after an earlier failure in the same partition.
func recordsSafeToCommit(records []*kgo.Record, handle func(*kgo.Record) bool) []*kgo.Record {
	var committed []*kgo.Record
	for _, rec := range records {
		if !handle(rec) {
			break
		}
		committed = append(committed, rec)
	}
	return committed
}
