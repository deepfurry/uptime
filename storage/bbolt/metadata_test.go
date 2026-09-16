package bbolt

import (
	"context"
	"testing"
	"time"

	uptimestorage "github.com/gofiber/contrib/v3/uptime/storage"
	bolt "go.etcd.io/bbolt"
)

func TestServiceMetadata(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	// These IDs intentionally violate standalone endpoint regex constraints.
	id := "service / 汉字 : "
	must(t, s.UpsertService(ctx, uptimestorage.Service{ID: id, SampleInterval: time.Second}))
	rows, err := s.ListServices(ctx)
	must(t, err)
	if len(rows) != 1 || rows[0].Name != id || !rows[0].CreatedAt.IsZero() {
		t.Fatalf("initial metadata: %+v", rows)
	}
	must(t, s.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("services")).Bucket([]byte(id)).Get([]byte("created_at")) != nil {
			t.Fatal("zero created_at persisted")
		}
		return nil
	}))
	must(t, s.UpsertService(ctx, uptimestorage.Service{ID: id, Name: "first", Description: "old", SampleInterval: time.Second, LastSeenAt: testTime}))
	must(t, s.UpsertService(ctx, uptimestorage.Service{ID: id, Name: "new", Description: "changed", SampleInterval: 2 * time.Second, CreatedAt: testTime.Add(-time.Hour), LastSeenAt: testTime.Add(-time.Minute)}))
	rows, err = s.ListServices(ctx)
	must(t, err)
	row := rows[0]
	if !row.CreatedAt.Equal(testTime) || !row.LastSeenAt.Equal(testTime) || row.Name != "new" || row.Description != "changed" || row.SampleInterval != 2*time.Second {
		t.Fatalf("refreshed metadata: %+v", row)
	}
	must(t, s.UpsertService(ctx, uptimestorage.Service{ID: id, LastSeenAt: testTime.Add(time.Hour)}))
	must(t, s.UpsertService(ctx, uptimestorage.Service{ID: id}))
	rows, err = s.ListServices(ctx)
	must(t, err)
	if rows[0].Name != id || !rows[0].CreatedAt.Equal(testTime) || !rows[0].LastSeenAt.Equal(testTime.Add(time.Hour)) {
		t.Fatalf("zero timestamp regressed metadata: %+v", rows)
	}
	must(t, s.UpsertService(ctx, uptimestorage.Service{ID: "explicit", CreatedAt: testTime.Add(-time.Hour), LastSeenAt: testTime}))
	rows, err = s.ListServices(ctx)
	must(t, err)
	for _, row := range rows {
		if row.ID == "explicit" && !row.CreatedAt.Equal(testTime.Add(-time.Hour)) {
			t.Fatal("explicit created time lost")
		}
	}
	wantError(t, s.UpsertService(ctx, uptimestorage.Service{}), "ID")
}

func TestInstanceMetadata(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: -7, ServiceID: "first"}))
	row, found := readTestInstance(t, s, -7)
	if !found || !row.StartedAt.IsZero() {
		t.Fatal(row)
	}
	must(t, s.db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("instances")).Bucket(encodeInt64(-7)).Get([]byte("started_at")) != nil {
			t.Fatal("zero started_at persisted")
		}
		return nil
	}))
	must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: -7, LastSeenAt: testTime}))
	must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: -7, ServiceID: "second", Hostname: "host", PID: 42, StartedAt: testTime.Add(-time.Hour), LastSeenAt: testTime.Add(-time.Minute)}))
	row, _ = readTestInstance(t, s, -7)
	if row.ServiceID != "second" || row.Hostname != "host" || row.PID != 42 || !row.StartedAt.Equal(testTime) || !row.LastSeenAt.Equal(testTime) {
		t.Fatalf("refreshed instance: %+v", row)
	}
	must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: -7, LastSeenAt: testTime.Add(time.Hour)}))
	must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: -7}))
	row, _ = readTestInstance(t, s, -7)
	if !row.StartedAt.Equal(testTime) || !row.LastSeenAt.Equal(testTime.Add(time.Hour)) {
		t.Fatal("timestamps regressed")
	}
	must(t, s.UpsertInstance(ctx, uptimestorage.Instance{ID: 2, StartedAt: testTime.Add(-time.Hour), LastSeenAt: testTime}))
	row, _ = readTestInstance(t, s, 2)
	if !row.StartedAt.Equal(testTime.Add(-time.Hour)) {
		t.Fatal("explicit StartedAt lost")
	}
	wantError(t, s.UpsertInstance(ctx, uptimestorage.Instance{}), "ID")
}

func TestMissingMetadataDefaults(t *testing.T) {
	s := openTestStore(t)
	must(t, s.db.Update(func(tx *bolt.Tx) error {
		service, err := tx.Bucket([]byte("services")).CreateBucket([]byte("minimal"))
		if err != nil {
			return err
		}
		if err := service.Put([]byte("sample_interval"), encodeInt64(0)); err != nil {
			return err
		}
		_, err = tx.Bucket([]byte("instances")).CreateBucket(encodeInt64(1))
		return err
	}))
	rows, err := s.ListServices(context.Background())
	must(t, err)
	if len(rows) != 1 || rows[0].Name != "minimal" || rows[0].Description != "" || !rows[0].CreatedAt.IsZero() || !rows[0].LastSeenAt.IsZero() {
		t.Fatal(rows)
	}
	instance, found := readTestInstance(t, s, 1)
	if !found || instance.ServiceID != "" || instance.Hostname != "" || instance.PID != 0 || !instance.StartedAt.IsZero() || !instance.LastSeenAt.IsZero() {
		t.Fatal(instance)
	}
}
