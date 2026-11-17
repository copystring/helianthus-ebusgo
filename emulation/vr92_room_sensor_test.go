package emulation

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestVR92RoomSensorRejectsInvalidFreshness(t *testing.T) {
	t.Parallel()
	for _, maxAge := range []time.Duration{0, -time.Minute, time.Hour + time.Nanosecond} {
		_, err := NewVR92RoomSensor(maxAge)
		if !errors.Is(err, ErrInvalidConfiguration) {
			t.Errorf("NewVR92RoomSensor(%s) = %v; want invalid configuration", maxAge, err)
		}
	}
}

func TestVR92RoomSensorFreshnessAndOrdering(t *testing.T) {
	t.Parallel()
	sensor, err := NewVR92RoomSensor(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

	if _, err := sensor.Snapshot(now); !errors.Is(err, ErrVR92RoomReadingUnavailable) {
		t.Fatalf("initial Snapshot() error = %v", err)
	}
	reading := VR92RoomReading{TemperatureC: 21.25, ObservedAt: now}
	if err := sensor.Update(reading); err != nil {
		t.Fatal(err)
	}
	got, err := sensor.Snapshot(now.Add(5 * time.Minute))
	if err != nil || got != reading {
		t.Fatalf("fresh Snapshot() = %+v, %v; want %+v", got, err, reading)
	}
	if _, err := sensor.Snapshot(now.Add(5*time.Minute + time.Nanosecond)); !errors.Is(err, ErrVR92RoomReadingUnavailable) {
		t.Fatalf("expired Snapshot() error = %v", err)
	}
	if _, err := sensor.Snapshot(now.Add(-time.Second)); !errors.Is(err, ErrVR92RoomReadingUnavailable) {
		t.Fatalf("future Snapshot() error = %v", err)
	}
	if err := sensor.Update(reading); !errors.Is(err, ErrVR92RoomReadingOutOfOrder) {
		t.Fatalf("duplicate Update() error = %v", err)
	}
	if err := sensor.Update(VR92RoomReading{TemperatureC: 22, ObservedAt: now.Add(-time.Second)}); !errors.Is(err, ErrVR92RoomReadingOutOfOrder) {
		t.Fatalf("out-of-order Update() error = %v", err)
	}
	got, err = sensor.Snapshot(now)
	if err != nil || got != reading {
		t.Fatalf("old value changed after rejected updates: %+v, %v", got, err)
	}
}

func TestVR92RoomSensorRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	sensor, err := NewVR92RoomSensor(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	cases := []VR92RoomReading{
		{TemperatureC: math.NaN(), ObservedAt: now},
		{TemperatureC: math.Inf(1), ObservedAt: now},
		{TemperatureC: math.Inf(-1), ObservedAt: now},
		{TemperatureC: -20.01, ObservedAt: now},
		{TemperatureC: 50.01, ObservedAt: now},
		{TemperatureC: 20, ObservedAt: time.Time{}},
	}
	for i, reading := range cases {
		if err := sensor.Update(reading); !errors.Is(err, ErrVR92RoomReadingInvalid) {
			t.Errorf("invalid reading %d error = %v", i, err)
		}
	}
	if _, err := sensor.Snapshot(now); !errors.Is(err, ErrVR92RoomReadingUnavailable) {
		t.Fatalf("rejected readings must leave sensor unavailable, got %v", err)
	}
	for _, value := range []float64{VR92MinPlausibleRoomTemperatureC, VR92MaxPlausibleRoomTemperatureC} {
		now = now.Add(time.Second)
		if err := sensor.Update(VR92RoomReading{TemperatureC: value, ObservedAt: now}); err != nil {
			t.Fatalf("valid boundary %.2f rejected: %v", value, err)
		}
	}
}

func TestVR92RoomSensorInvalidateAndRecover(t *testing.T) {
	t.Parallel()
	sensor, err := NewVR92RoomSensor(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	if err := sensor.Update(VR92RoomReading{TemperatureC: 20.5, ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	sensor.Invalidate()
	if _, err := sensor.Snapshot(now); !errors.Is(err, ErrVR92RoomReadingUnavailable) {
		t.Fatalf("invalidated Snapshot() error = %v", err)
	}
	replacement := VR92RoomReading{TemperatureC: 21.0, ObservedAt: now.Add(time.Second)}
	if err := sensor.Update(replacement); err != nil {
		t.Fatal(err)
	}
	got, err := sensor.Snapshot(replacement.ObservedAt)
	if err != nil || got != replacement {
		t.Fatalf("recovered Snapshot() = %+v, %v", got, err)
	}
}

func TestVR92RoomSensorNilReceiver(t *testing.T) {
	t.Parallel()
	var sensor *VR92RoomSensor
	now := time.Now()
	if err := sensor.Update(VR92RoomReading{TemperatureC: 21, ObservedAt: now}); !errors.Is(err, ErrVR92RoomReadingInvalid) {
		t.Fatalf("nil Update() error = %v", err)
	}
	if _, err := sensor.Snapshot(now); !errors.Is(err, ErrVR92RoomReadingUnavailable) {
		t.Fatalf("nil Snapshot() error = %v", err)
	}
	sensor.Invalidate()
}
