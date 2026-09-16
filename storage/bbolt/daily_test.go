package bbolt

import (
	"context"
	"sync"
	"testing"

	uptimestorage "github.com/gofiber/contrib/v3/uptime/storage"
	bolt "go.etcd.io/bbolt"
)

func TestRollupBoundariesAndFinalization(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	register(t, s, "a", 1)
	for _, day := range []string{"2026-09-14", "2026-09-15", "2026-09-16"} {
		beat(t, s, "a", day, 1, 0)
	}
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{}))
	rows, err := s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("empty BeforeDay rolled up")
	}
	calls := 0
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(service uptimestorage.Service, day string) int {
		calls++
		if service.ID != "a" || day >= "2026-09-16" {
			t.Fatalf("callback inputs: %+v %s", service, day)
		}
		return 24
	}}))
	if calls != 2 {
		t.Fatalf("callback calls: %d", calls)
	}
	rows, err = s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
	must(t, err)
	if len(rows) != 2 {
		t.Fatal(rows)
	}
	for _, row := range rows {
		if row.UpSlots != 1 || row.ExpectedSlots != 24 || !row.Finalized {
			t.Fatal(row)
		}
	}
	beat(t, s, "a", "2026-09-14", 1, 1)
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int { t.Error("finalized row callback invoked"); return 99 }}))
	rows, err = s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{FromDay: "2026-09-14", ToDay: "2026-09-14"})
	must(t, err)
	if len(rows) != 1 || rows[0].UpSlots != 1 || rows[0].ExpectedSlots != 24 {
		t.Fatal("finalized row changed", rows)
	}
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-17"}))
	rows, err = s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{FromDay: "2026-09-16"})
	must(t, err)
	if len(rows) != 1 || rows[0].ExpectedSlots != 0 {
		t.Fatal("nil callback semantics", rows)
	}
}

func TestRollupCallbackOutsideTransactions(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	register(t, s, "a", 1)
	beat(t, s, "a", "2026-09-15", 1, 0)
	calls := 0
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int {
		calls++
		_, err := s.ListServices(ctx)
		must(t, err)
		// Also write: this would deadlock under a retained write transaction.
		beat(t, s, "a", "2026-09-15", 1, 1)
		return 12
	}}))
	rows, err := s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
	must(t, err)
	if calls != 1 || len(rows) != 1 || rows[0].UpSlots != 2 {
		t.Fatal("rollup did not reread latest counter", calls, rows)
	}
	must(t, s.Ping(ctx))
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}))
	if calls != 1 {
		t.Fatal("callback retained")
	}
}

func TestConcurrentRollupAndRemoval(t *testing.T) {
	t.Run("first finalized row wins", func(t *testing.T) {
		s := openTestStore(t)
		register(t, s, "a", 1)
		beat(t, s, "a", "2026-09-15", 1, 0)
		entered := make(chan struct{}, 2)
		firstRelease := make(chan struct{})
		secondRelease := make(chan struct{})
		completed := make(chan int, 2)
		var wg sync.WaitGroup
		for _, call := range []struct {
			expected int
			release  <-chan struct{}
		}{{10, firstRelease}, {20, secondRelease}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := s.RollupDaily(context.Background(), uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int { entered <- struct{}{}; <-call.release; return call.expected }})
				if err != nil {
					t.Error(err)
				}
				completed <- call.expected
			}()
		}
		<-entered
		<-entered
		// Both calls collected the same unfinalized candidate. Commit 10 before
		// allowing the stale candidate with 20 to enter its write transaction.
		close(firstRelease)
		firstCompleted := <-completed
		close(secondRelease)
		wg.Wait()
		rows, err := s.QueryDaily(context.Background(), uptimestorage.QueryDailyOptions{})
		must(t, err)
		if firstCompleted != 10 || len(rows) != 1 || rows[0].ExpectedSlots != 10 {
			t.Fatal(rows)
		}
		first := rows[0]
		must(t, s.RollupDaily(context.Background(), uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}))
		rows, err = s.QueryDaily(context.Background(), uptimestorage.QueryDailyOptions{})
		must(t, err)
		if rows[0] != first {
			t.Fatal("finalized row overwritten")
		}
	})
	t.Run("removed candidate stays removed", func(t *testing.T) {
		s := openTestStore(t)
		register(t, s, "a", 1)
		beat(t, s, "a", "2026-09-15", 1, 0)
		must(t, s.RollupDaily(context.Background(), uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int {
			must(t, s.RemoveService(context.Background(), "a"))
			return 10
		}}))
		if hasDay(t, s, "daily", "a", "2026-09-15") {
			t.Fatal("rollup resurrected removed history")
		}
	})
}

func TestQuerySelectionsAndRanges(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for i, id := range []string{"a", "b"} {
		register(t, s, id, int64(i+1))
		for _, day := range []string{"2026-09-14", "2026-09-15", "2026-09-16"} {
			beat(t, s, id, day, int64(i+1), 0)
		}
	}
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-17"}))
	for _, tc := range []struct {
		name    string
		options uptimestorage.QueryDailyOptions
		count   int
	}{
		{"all", uptimestorage.QueryDailyOptions{}, 6},
		{"empty selection", uptimestorage.QueryDailyOptions{ServiceIDs: []string{}}, 0},
		{"one service", uptimestorage.QueryDailyOptions{ServiceIDs: []string{"a"}}, 3},
		{"duplicate selection", uptimestorage.QueryDailyOptions{ServiceIDs: []string{"a", "a"}}, 3},
		{"inclusive range", uptimestorage.QueryDailyOptions{FromDay: "2026-09-14", ToDay: "2026-09-15"}, 4},
		{"unbounded lower", uptimestorage.QueryDailyOptions{ToDay: "2026-09-14"}, 2},
		{"unbounded upper", uptimestorage.QueryDailyOptions{FromDay: "2026-09-16"}, 2},
		{"unknown service", uptimestorage.QueryDailyOptions{ServiceIDs: []string{"missing"}}, 0},
		{"unknown day", uptimestorage.QueryDailyOptions{FromDay: "2026-09-17"}, 0},
		{"reversed range", uptimestorage.QueryDailyOptions{FromDay: "2026-09-16", ToDay: "2026-09-14"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := s.QueryDaily(ctx, tc.options)
			must(t, err)
			if len(rows) != tc.count {
				t.Fatalf("rows = %+v, want %d", rows, tc.count)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		options uptimestorage.QueryTodaySamplesOptions
		count   int
	}{
		{"all", uptimestorage.QueryTodaySamplesOptions{Day: "2026-09-16"}, 2},
		{"none", uptimestorage.QueryTodaySamplesOptions{ServiceIDs: []string{}, Day: "2026-09-16"}, 0},
		{"one", uptimestorage.QueryTodaySamplesOptions{ServiceIDs: []string{"b"}, Day: "2026-09-16"}, 1},
		{"missing service", uptimestorage.QueryTodaySamplesOptions{ServiceIDs: []string{"missing"}, Day: "2026-09-16"}, 0},
		{"missing day", uptimestorage.QueryTodaySamplesOptions{Day: "2026-09-17"}, 0},
		{"empty day", uptimestorage.QueryTodaySamplesOptions{}, 0},
	} {
		t.Run("today "+tc.name, func(t *testing.T) {
			rows, err := s.QueryTodaySamples(ctx, tc.options)
			must(t, err)
			if len(rows) != tc.count {
				t.Fatalf("rows = %+v, want %d", rows, tc.count)
			}
		})
	}
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		day, err := historyDay(tx, "samples", "a", "2026-09-17", true)
		if err != nil {
			return err
		}
		if err := day.Put([]byte("up_slots"), encodeInt64(0)); err != nil {
			return err
		}
		_, err = day.CreateBucket([]byte("slots"))
		return err
	}))
	zeroRows, err := s.QueryTodaySamples(ctx, uptimestorage.QueryTodaySamplesOptions{Day: "2026-09-17"})
	must(t, err)
	if len(zeroRows) != 0 {
		t.Fatalf("zero row not omitted: %+v", zeroRows)
	}
}

func TestRollupUnfinalizedAndInvalidCallback(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	register(t, s, "a", 1)
	beat(t, s, "a", "2026-09-15", 1, 0)
	wantError(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int { return -1 }}), "negative")
	if hasDay(t, s, "daily", "a", "2026-09-15") {
		t.Fatal("invalid callback committed")
	}
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		b, err := historyDay(tx, "daily", "a", "2026-09-15", true)
		if err != nil {
			return err
		}
		if err := b.Put([]byte("up_slots"), encodeInt64(0)); err != nil {
			return err
		}
		if err := b.Put([]byte("expected_slots"), encodeInt64(0)); err != nil {
			return err
		}
		return b.Put([]byte("finalized"), encodeBool(false))
	}))
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}))
	rows, err := s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
	must(t, err)
	if len(rows) != 1 || !rows[0].Finalized || rows[0].UpSlots != 1 {
		t.Fatal(rows)
	}
}

func TestPublicDayValidation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for _, bad := range []string{"2026-02-29", "2026-13-01", "2026-9-1", "not-a-day"} {
		wantError(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: bad}), "invalid day")
		wantError(t, s.Cleanup(ctx, uptimestorage.CleanupOptions{SamplesBeforeDay: bad}), "invalid day")
		wantError(t, s.Cleanup(ctx, uptimestorage.CleanupOptions{DailyBeforeDay: bad}), "invalid day")
		_, err := s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{FromDay: bad})
		wantError(t, err, "invalid day")
		_, err = s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{ToDay: bad})
		wantError(t, err, "invalid day")
		_, err = s.QueryTodaySamples(ctx, uptimestorage.QueryTodaySamplesOptions{Day: bad})
		wantError(t, err, "invalid day")
	}
}
