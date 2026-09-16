package bbolt

import (
	"math"
	"strconv"
	"testing"
	"time"
)

func TestScalarEncoding(t *testing.T) {
	for _, n := range []int64{math.MinInt64, -42, -1, 0, 1, math.MaxInt64} {
		got, err := decodeInt64(encodeInt64(n))
		must(t, err)
		if got != n {
			t.Fatalf("int64 %d -> %d", n, got)
		}
		gotInt, err := decodeInt(encodeInt64(n))
		if strconv.IntSize == 32 && (n < math.MinInt32 || n > math.MaxInt32) {
			wantError(t, err, "range")
		} else {
			must(t, err)
			if int64(gotInt) != n {
				t.Fatalf("int %d -> %d", n, gotInt)
			}
		}
	}
	for _, n := range []uint64{0, 1, math.MaxUint64} {
		got, err := decodeUint64(encodeUint64(n))
		must(t, err)
		if got != n {
			t.Fatal(got)
		}
	}
	for _, v := range []bool{false, true} {
		got, err := decodeBool(encodeBool(v))
		must(t, err)
		if got != v {
			t.Fatal(got)
		}
	}
	for _, value := range []time.Time{{}, testTime, time.Unix(-1, 500).In(time.FixedZone("offset", 3600))} {
		encoded, err := encodeTime(value)
		must(t, err)
		got, err := decodeTime(encoded)
		must(t, err)
		if !value.Equal(got) {
			t.Fatalf("time %v -> %v", value, got)
		}
		if !got.IsZero() && got.Location() != time.UTC {
			t.Fatal("decoded time is not UTC")
		}
	}
	epoch, err := encodeTime(time.Unix(0, 0))
	must(t, err)
	zero, err := decodeTime(epoch)
	must(t, err)
	if !zero.IsZero() {
		t.Fatal("UnixNano zero must decode as zero time")
	}
	_, err = encodeTime(time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC))
	wantError(t, err, "range")
	for _, duration := range []time.Duration{-time.Second, 0, 123 * time.Nanosecond, time.Hour} {
		n, err := decodeInt64(encodeInt64(int64(duration)))
		must(t, err)
		if time.Duration(n) != duration {
			t.Fatal("duration roundtrip")
		}
	}
}

func TestMalformedScalars(t *testing.T) {
	for _, value := range [][]byte{nil, {}, {1}, make([]byte, 7), make([]byte, 9)} {
		_, err := decodeInt64(value)
		wantError(t, err, "8 bytes")
		_, err = decodeUint64(value)
		wantError(t, err, "8 bytes")
		_, err = decodeInt(value)
		wantError(t, err, "8 bytes")
		_, err = decodeTime(value)
		wantError(t, err, "8 bytes")
	}
	for _, value := range [][]byte{nil, {}, {2}, {255}, {0, 1}} {
		_, err := decodeBool(value)
		wantError(t, err, "boolean")
	}
	for _, day := range []string{"", "2026-02-29", "2026-13-01", "2026-9-1", "not-a-day", "2026-01-01x"} {
		wantError(t, validDay(day), "invalid day")
	}
	for _, day := range []string{"2024-02-29", "2026-09-16", "0000-01-01"} {
		must(t, validDay(day))
	}
}
