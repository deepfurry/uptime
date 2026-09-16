package bbolt

import (
	"bytes"
	"context"
	"testing"
	"time"

	uptimestorage "github.com/gofiber/contrib/v3/uptime/storage"
	bolt "go.etcd.io/bbolt"
)

func snapshot(t *testing.T, s *Store) []byte {
	t.Helper()
	var buffer bytes.Buffer
	must(t, s.db.View(func(tx *bolt.Tx) error { _, err := tx.WriteTo(&buffer); return err }))
	return buffer.Bytes()
}

func TestCorruptRecordsReturnErrorsWithoutRepair(t *testing.T) {
	type fixture struct {
		name, root, message string
		mutate              func(*bolt.Bucket) error
	}
	fixtures := []fixture{
		{"service missing interval", "services", "sample_interval", func(b *bolt.Bucket) error { return b.Delete([]byte("sample_interval")) }},
		{"service short interval", "services", "sample_interval", func(b *bolt.Bucket) error { return b.Put([]byte("sample_interval"), []byte{1}) }},
		{"service short timestamp", "services", "created_at", func(b *bolt.Bucket) error { return b.Put([]byte("created_at"), []byte{1}) }},
		{"service name is bucket", "services", "name", func(b *bolt.Bucket) error {
			if err := b.Delete([]byte("name")); err != nil {
				return err
			}
			_, err := b.CreateBucket([]byte("name"))
			return err
		}},
		{"daily missing up", "daily", "up_slots", func(b *bolt.Bucket) error { return b.Delete([]byte("up_slots")) }},
		{"daily missing expected", "daily", "expected_slots", func(b *bolt.Bucket) error { return b.Delete([]byte("expected_slots")) }},
		{"daily missing finalized", "daily", "finalized", func(b *bolt.Bucket) error { return b.Delete([]byte("finalized")) }},
		{"daily bad bool", "daily", "finalized", func(b *bolt.Bucket) error { return b.Put([]byte("finalized"), []byte{2}) }},
		{"daily short expected", "daily", "expected_slots", func(b *bolt.Bucket) error { return b.Put([]byte("expected_slots"), []byte{1}) }},
		{"daily negative up", "daily", "negative count", func(b *bolt.Bucket) error { return b.Put([]byte("up_slots"), encodeInt64(-1)) }},
		{"sample missing up", "samples", "up_slots", func(b *bolt.Bucket) error { return b.Delete([]byte("up_slots")) }},
		{"sample short up", "samples", "up_slots", func(b *bolt.Bucket) error { return b.Put([]byte("up_slots"), []byte{1}) }},
		{"sample count mismatch", "samples", "does not match", func(b *bolt.Bucket) error { return b.Put([]byte("up_slots"), encodeInt64(99)) }},
		{"sample missing slots", "samples", "slots", func(b *bolt.Bucket) error { return b.DeleteBucket([]byte("slots")) }},
		{"sample slots value", "samples", "expected bucket", func(b *bolt.Bucket) error {
			if err := b.DeleteBucket([]byte("slots")); err != nil {
				return err
			}
			return b.Put([]byte("slots"), []byte{1})
		}},
		{"short slot key", "samples", "slots key", func(b *bolt.Bucket) error { return b.Bucket([]byte("slots")).Put([]byte{1}, []byte{1}) }},
		{"negative slot key", "samples", "negative slot", func(b *bolt.Bucket) error { return b.Bucket([]byte("slots")).Put(encodeInt64(-1), []byte{1}) }},
		{"slot bucket", "samples", "slot marker", func(b *bolt.Bucket) error {
			_, err := b.Bucket([]byte("slots")).CreateBucket(encodeInt64(2))
			return err
		}},
		{"slot bad marker", "samples", "slot marker", func(b *bolt.Bucket) error { return b.Bucket([]byte("slots")).Put(encodeInt64(0), []byte{0}) }},
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			s := openTestStore(t)
			ctx := context.Background()
			register(t, s, "a", 1)
			beat(t, s, "a", "2026-09-15", 1, 0)
			must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}))
			must(t, s.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte(tc.root)).Bucket([]byte("a"))
				if tc.root != "services" {
					b = b.Bucket([]byte("2026-09-15"))
				}
				return tc.mutate(b)
			}))
			before := snapshot(t, s)
			must(t, s.Ping(ctx)) // Readiness checks schema, not every record.
			var err error
			switch tc.root {
			case "services":
				_, err = s.ListServices(ctx)
			case "daily":
				_, err = s.QueryDaily(ctx, uptimestorage.QueryDailyOptions{})
			case "samples":
				_, err = s.QueryTodaySamples(ctx, uptimestorage.QueryTodaySamplesOptions{Day: "2026-09-15"})
			}
			wantError(t, err, tc.message)
			if !bytes.Equal(before, snapshot(t, s)) {
				t.Fatal("read repaired corrupt data")
			}
			if tc.root == "samples" || tc.root == "services" {
				err = s.WriteHeartbeat(ctx, uptimestorage.Heartbeat{ServiceID: "a", InstanceID: 1, Day: "2026-09-15", Slot: 10, SeenAt: testTime.Add(time.Hour)})
				wantError(t, err, tc.message)
				if !bytes.Equal(before, snapshot(t, s)) {
					t.Fatal("failed heartbeat partially committed")
				}
			}
			wantError(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}), tc.message)
			if !bytes.Equal(before, snapshot(t, s)) {
				t.Fatal("failed rollup repaired data")
			}
		})
	}
}

func TestMalformedBucketHierarchy(t *testing.T) {
	for _, root := range []string{"services", "samples", "daily", "instances"} {
		t.Run(root, func(t *testing.T) {
			s := openTestStore(t)
			must(t, s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte(root)).Put([]byte("invalid"), []byte("not a bucket")) }))
			var err error
			switch root {
			case "services":
				_, err = s.ListServices(context.Background())
			case "samples":
				err = s.Cleanup(context.Background(), uptimestorage.CleanupOptions{SamplesBeforeDay: "2026-09-16"})
			case "daily":
				err = s.Cleanup(context.Background(), uptimestorage.CleanupOptions{DailyBeforeDay: "2026-09-16"})
			case "instances":
				err = s.Cleanup(context.Background(), uptimestorage.CleanupOptions{})
			}
			wantError(t, err, "expected bucket")
		})
	}
	for _, day := range []string{"2026-02-29", "invalid-day"} {
		t.Run(day, func(t *testing.T) {
			s := openTestStore(t)
			register(t, s, "a", 1)
			must(t, s.db.Update(func(tx *bolt.Tx) error { _, err := historyDay(tx, "daily", "a", day, true); return err }))
			_, err := s.QueryDaily(context.Background(), uptimestorage.QueryDailyOptions{})
			wantError(t, err, "invalid day")
		})
	}
}

func TestCorruptInstanceAndCleanupRollback(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	register(t, s, "a", 1)
	beat(t, s, "a", "2026-09-15", 1, 0)
	must(t, s.RollupDaily(ctx, uptimestorage.RollupOptions{BeforeDay: "2026-09-16"}))
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("instances")).Bucket(encodeInt64(1)).Put([]byte("pid"), []byte{1})
	}))
	before := snapshot(t, s)
	wantError(t, s.Cleanup(ctx, uptimestorage.CleanupOptions{SamplesBeforeDay: "2026-09-16", DailyBeforeDay: "2026-09-16"}), "pid")
	if !bytes.Equal(before, snapshot(t, s)) {
		t.Fatal("cleanup did not roll back prior history deletion")
	}
	wantError(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: 1}), "pid")
	if !bytes.Equal(before, snapshot(t, s)) {
		t.Fatal("upsert repaired malformed instance")
	}
}

func TestPingDetectsSchemaMutation(t *testing.T) {
	s := openTestStore(t)
	must(t, s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte("meta")).Put([]byte("format"), []byte("wrong")) }))
	wantError(t, s.Ping(context.Background()), "format")
}
