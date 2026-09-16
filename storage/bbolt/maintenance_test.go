package bbolt

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	uptimestorage "github.com/gofiber/contrib/v3/uptime/storage"
	bolt "go.etcd.io/bbolt"
)

func TestCleanupSafetyAndBoundaries(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	register(t, s, "a", 1)
	for _, day := range []string{"2026-09-12", "2026-09-13", "2026-09-14", "2026-09-15", "2026-09-16"} {
		beat(t, s, "a", day, 1, 0)
	}
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-17"}))
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		days := tx.Bucket([]byte("daily")).Bucket([]byte("a"))
		if err := days.Bucket([]byte("2026-09-13")).Put([]byte("finalized"), encodeBool(false)); err != nil {
			return err
		}
		return days.DeleteBucket([]byte("2026-09-14"))
	}))
	cutoff := testTime.Add(-24 * time.Hour)
	for i, seen := range []time.Time{cutoff.Add(-time.Nanosecond), cutoff, cutoff.Add(time.Nanosecond)} {
		must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: int64(i + 10), ServiceID: "a", LastSeenAt: seen}))
	}
	must(t, s.Cleanup(ctx, uptimestorage.CleanupOptions{SamplesBeforeDay: "2026-09-15", DailyBeforeDay: "2026-09-15"}))
	if hasDay(t, s, "samples", "a", "2026-09-12") || hasDay(t, s, "daily", "a", "2026-09-12") {
		t.Fatal("expired finalized history retained (cleanup ordering)")
	}
	for _, day := range []string{"2026-09-13", "2026-09-14", "2026-09-15", "2026-09-16"} {
		if !hasDay(t, s, "samples", "a", day) {
			t.Fatalf("unsafe sample deletion: %s", day)
		}
	}
	if !hasDay(t, s, "daily", "a", "2026-09-15") {
		t.Fatal("daily boundary deleted")
	}
	if _, found := readTestInstance(t, s, 10); found {
		t.Fatal("stale instance retained")
	}
	for _, id := range []int64{11, 12} {
		if _, found := readTestInstance(t, s, id); !found {
			t.Fatalf("instance cutoff/newer lost: %d", id)
		}
	}
	must(t, s.Cleanup(ctx, uptimestorage.CleanupOptions{}))
	if !hasDay(t, s, "samples", "a", "2026-09-15") || !hasDay(t, s, "daily", "a", "2026-09-15") {
		t.Fatal("empty boundaries did not disable history cleanup")
	}
	rows, err := s.ListServices(ctx)
	must(t, err)
	if len(rows) != 1 {
		t.Fatal("cleanup removed service registration")
	}
}

func TestRemoveServiceAndAtomicRollback(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for i, id := range []string{"a", "b"} {
		register(t, s, id, int64(i+1))
		beat(t, s, id, "2026-09-15", int64(i+1), 0)
	}
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}))
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("instances")).Bucket(encodeInt64(2))
		if err := b.Delete([]byte("service_id")); err != nil {
			return err
		}
		_, err := b.CreateBucket([]byte("service_id"))
		return err
	}))
	wantError(t, s.RemoveService(ctx, "a"), "service_id")
	rows, err := s.ListServices(ctx)
	must(t, err)
	if len(rows) != 2 || !hasDay(t, s, "samples", "a", "2026-09-15") || !hasDay(t, s, "daily", "a", "2026-09-15") {
		t.Fatal("failed removal partially committed")
	}
	if _, found := readTestInstance(t, s, 1); !found {
		t.Fatal("instance partially removed")
	}
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("instances")).Bucket(encodeInt64(2))
		if err := b.DeleteBucket([]byte("service_id")); err != nil {
			return err
		}
		return b.Put([]byte("service_id"), []byte("b"))
	}))
	must(t, s.RemoveService(ctx, "a"))
	rows, err = s.ListServices(ctx)
	must(t, err)
	if len(rows) != 1 || rows[0].ID != "b" {
		t.Fatal(rows)
	}
	if hasDay(t, s, "samples", "a", "2026-09-15") || hasDay(t, s, "daily", "a", "2026-09-15") {
		t.Fatal("target history retained")
	}
	if _, found := readTestInstance(t, s, 1); found {
		t.Fatal("target instance retained")
	}
	if _, found := readTestInstance(t, s, 2); !found {
		t.Fatal("other instance removed")
	}
	if !hasDay(t, s, "samples", "b", "2026-09-15") || !hasDay(t, s, "daily", "b", "2026-09-15") {
		t.Fatal("other history removed")
	}
	must(t, s.RemoveService(ctx, "a"))
	must(t, s.RemoveService(ctx, "unknown"))
	wantError(t, s.RemoveService(ctx, ""), "ID")
	// Orphan history can also be explicitly removed without metadata.
	beat(t, s, "orphan", "2026-09-15", 99, 0)
	must(t, s.RemoveService(ctx, "orphan"))
	if hasDay(t, s, "samples", "orphan", "2026-09-15") {
		t.Fatal("orphan history retained")
	}
}

func TestReopenPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persistent.db")
	s, err := Open(Config{Path: path})
	must(t, err)
	t.Cleanup(func() { must(t, s.Close()) })
	ctx := context.Background()
	register(t, s, "persistent", -99)
	beat(t, s, "persistent", "2026-09-15", -99, 0)
	beat(t, s, "persistent", "2026-09-15", -99, 1)
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int { return 3 }}))
	services, err := s.ListServices(ctx)
	must(t, err)
	daily, err := s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
	must(t, err)
	instance, _ := readTestInstance(t, s, -99)
	must(t, s.Close())
	s, err = Open(Config{Path: path})
	must(t, err)
	gotServices, err := s.ListServices(ctx)
	must(t, err)
	gotDaily, err := s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
	must(t, err)
	gotInstance, found := readTestInstance(t, s, -99)
	if !reflect.DeepEqual(gotServices, services) || !reflect.DeepEqual(gotDaily, daily) || !found || !reflect.DeepEqual(gotInstance, instance) || sampleCount(t, s, "persistent", "2026-09-15") != 2 {
		t.Fatal("persistent state changed across reopen")
	}
}

// A deterministic cancellation checkpoint: Done closes when Err is checked
// the requested number of times. No wall-clock scheduling is involved.
type checkpointContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *checkpointContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		c.cancel()
	}
	return c.Context.Err()
}
func newCheckpointContext(t *testing.T, checks int) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &checkpointContext{Context: ctx, cancel: cancel, remaining: checks}
}

func TestCancellationDuringScansRollsBackWrites(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	register(t, s, "a", 1)
	beat(t, s, "a", "2026-09-15", 1, 0)
	for i := range 30 {
		must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: int64(i + 2), ServiceID: "a", LastSeenAt: testTime.Add(-48 * time.Hour)}))
	}
	if err := s.RemoveService(newCheckpointContext(t, 10), "a"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	rows, err := s.ListServices(ctx)
	must(t, err)
	if len(rows) != 1 || !hasDay(t, s, "samples", "a", "2026-09-15") {
		t.Fatal("canceled removal committed")
	}
	if err := s.Cleanup(newCheckpointContext(t, 10), uptimestorage.CleanupOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, found := readTestInstance(t, s, 2); !found {
		t.Fatal("canceled cleanup committed")
	}
	for i := range 20 {
		must(t, s.UpsertService(ctx, uptimestorage.Service{ID: string(rune('b' + i))}))
	}
	if _, err := s.ListServices(newCheckpointContext(t, 10)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCancellationInRollupCallback(t *testing.T) {
	s := openTestStore(t)
	register(t, s, "a", 1)
	beat(t, s, "a", "2026-09-15", 1, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16", ExpectedSlots: func(uptimestorage.Service, string) int { cancel(); return 10 }})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if hasDay(t, s, "daily", "a", "2026-09-15") {
		t.Fatal("canceled rollup committed")
	}
}
