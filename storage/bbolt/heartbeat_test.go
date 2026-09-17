package bbolt

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	uptimestorage "github.com/gofiber/contrib/v3/uptime/storage"
	bolt "go.etcd.io/bbolt"
)

func sampleCount(t *testing.T, s *Store, id, day string) int {
	t.Helper()
	rows, err := s.QueryTodaySamples(context.Background(), uptimestorage.QueryTodaySamplesOptions{ServiceIDs: []string{id}, Day: day})
	must(t, err)
	if len(rows) == 0 {
		return 0
	}
	if len(rows) != 1 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	return rows[0].UpSlots
}

func TestHeartbeatDeduplicationAndMetadata(t *testing.T) {
	s := openTestStore(t)
	register(t, s, "a", 1)
	register(t, s, "b", 2)
	for _, h := range []uptimestorage.Heartbeat{
		{ServiceID: "a", InstanceID: 1, Day: "2026-09-16", Slot: 0, SeenAt: testTime.Add(time.Hour)},
		{ServiceID: "a", InstanceID: 1, Day: "2026-09-16", Slot: 0, SeenAt: testTime.Add(-time.Hour)},
		{ServiceID: "a", InstanceID: 2, Day: "2026-09-16", Slot: 0},
		{ServiceID: "a", InstanceID: 1, Day: "2026-09-16", Slot: 1},
		{ServiceID: "a", InstanceID: 1, Day: "2026-09-15", Slot: 0},
		{ServiceID: "b", InstanceID: 2, Day: "2026-09-16", Slot: 0},
	} {
		must(t, s.WriteHeartbeat(context.Background(), h))
	}
	if got := sampleCount(t, s, "a", "2026-09-16"); got != 2 {
		t.Fatal(got)
	}
	if got := sampleCount(t, s, "a", "2026-09-15"); got != 1 {
		t.Fatal(got)
	}
	if got := sampleCount(t, s, "b", "2026-09-16"); got != 1 {
		t.Fatal(got)
	}
	services, err := s.ListServices(context.Background())
	must(t, err)
	for _, service := range services {
		if service.ID == "a" && !service.LastSeenAt.Equal(testTime.Add(time.Hour)) {
			t.Fatal("service moved backwards")
		}
	}
	instance, _ := readTestInstance(t, s, 1)
	if !instance.LastSeenAt.Equal(testTime.Add(time.Hour)) {
		t.Fatal("instance moved backwards")
	}
	must(t, s.db.View(func(tx *bolt.Tx) error {
		day, err := historyDay(tx, "samples", "a", "2026-09-16", false)
		if err != nil {
			return err
		}
		stored, err := readCount(day, "up_slots", "test")
		if err != nil {
			return err
		}
		if stored != day.Bucket([]byte("slots")).Stats().KeyN {
			t.Fatal("stored counter differs from real slots")
		}
		return nil
	}))
}

func TestDuplicateHeartbeatRejectsCorruptMarker(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker []byte // nil represents a nested bucket instead of a marker.
	}{
		{"zero marker", []byte{0}},
		{"empty marker", []byte{}},
		{"long marker", []byte{1, 1}},
		{"bucket marker", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := openTestStore(t)
			register(t, s, "a", 1)
			beat(t, s, "a", "2026-09-16", 1, 0)
			must(t, s.db.Update(func(tx *bolt.Tx) error {
				day, err := historyDay(tx, "samples", "a", "2026-09-16", false)
				if err != nil {
					return err
				}
				slots := day.Bucket([]byte("slots"))
				if tc.marker != nil {
					return slots.Put(encodeInt64(0), tc.marker)
				}
				if err := slots.Delete(encodeInt64(0)); err != nil {
					return err
				}
				_, err = slots.CreateBucket(encodeInt64(0))
				return err
			}))
			before := snapshot(t, s)
			err := s.WriteHeartbeat(context.Background(), uptimestorage.Heartbeat{
				ServiceID: "a", InstanceID: 1, Day: "2026-09-16", Slot: 0, SeenAt: testTime.Add(time.Hour),
			})
			wantError(t, err, `samples/"a"/2026-09-16/slots/0: invalid slot marker`)
			if !bytes.Equal(before, snapshot(t, s)) {
				t.Fatal("failed duplicate heartbeat changed metadata, counter, or corrupt marker")
			}
		})
	}
}

func TestHeartbeatDoesNotRegisterMetadata(t *testing.T) {
	s := openTestStore(t)
	beat(t, s, "orphan / id", "2026-09-16", 123, 0)
	rows, err := s.ListServices(context.Background())
	must(t, err)
	if len(rows) != 0 {
		t.Fatal("heartbeat fabricated service metadata")
	}
	if _, found := readTestInstance(t, s, 123); found {
		t.Fatal("heartbeat fabricated instance metadata")
	}
	if sampleCount(t, s, "orphan / id", "2026-09-16") != 1 {
		t.Fatal("missing raw sample")
	}
}

func TestHeartbeatValidation(t *testing.T) {
	s := openTestStore(t)
	for _, h := range []uptimestorage.Heartbeat{
		{Day: "2026-09-16"}, {ServiceID: "a"},
		{ServiceID: "a", Day: "2026-02-29"}, {ServiceID: "a", Day: "2026-09-16", Slot: -1},
	} {
		if err := s.WriteHeartbeat(context.Background(), h); err == nil {
			t.Fatalf("accepted %+v", h)
		}
	}
}

func TestConcurrentHeartbeats(t *testing.T) {
	for _, duplicate := range []bool{true, false} {
		name := "distinct slots"
		if duplicate {
			name = "same slot"
		}
		t.Run(name, func(t *testing.T) {
			s := openTestStore(t)
			register(t, s, "a", 1)
			register(t, s, "b", 2)
			const workers = 48
			var wg sync.WaitGroup
			errors := make(chan error, workers)
			for i := range workers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					seen := testTime.Add(time.Duration(i) * time.Second)
					ctx := context.Background()
					if err := s.UpsertService(ctx, uptimestorage.Service{ID: "a", LastSeenAt: seen}); err != nil {
						errors <- err
						return
					}
					if err := s.UpsertInstance(ctx, uptimestorage.Instance{ID: int64(i + 1), ServiceID: "a", LastSeenAt: seen}); err != nil {
						errors <- err
						return
					}
					slot := int64(i)
					if duplicate {
						slot = 0
					}
					if err := s.WriteHeartbeat(ctx, uptimestorage.Heartbeat{ServiceID: "a", InstanceID: int64(i + 1), Day: "2026-09-16", Slot: slot, SeenAt: seen}); err != nil {
						errors <- err
						return
					}
					if err := s.WriteHeartbeat(ctx, uptimestorage.Heartbeat{ServiceID: "b", Day: "2026-09-16", Slot: int64(i)}); err != nil {
						errors <- err
					}
				}()
			}
			wg.Wait()
			close(errors)
			for err := range errors {
				must(t, err)
			}
			want := workers
			if duplicate {
				want = 1
			}
			if got := sampleCount(t, s, "a", "2026-09-16"); got != want {
				t.Fatalf("got %d, want %d", got, want)
			}
			if got := sampleCount(t, s, "b", "2026-09-16"); got != workers {
				t.Fatal(got)
			}
			must(t, s.db.View(func(tx *bolt.Tx) error {
				for _, id := range []string{"a", "b"} {
					day, err := historyDay(tx, "samples", id, "2026-09-16", false)
					if err != nil {
						return err
					}
					count, err := readCount(day, "up_slots", "test")
					if err != nil {
						return err
					}
					if actual := day.Bucket([]byte("slots")).Stats().KeyN; actual != count {
						t.Fatalf("%s: persisted count %d, actual slots %d", id, count, actual)
					}
				}
				return nil
			}))
			rows, err := s.ListServices(context.Background())
			must(t, err)
			for _, row := range rows {
				if row.ID == "a" && !row.LastSeenAt.Equal(testTime.Add((workers-1)*time.Second)) {
					t.Fatal("monotonic metadata lost under concurrency")
				}
			}
		})
	}
}
