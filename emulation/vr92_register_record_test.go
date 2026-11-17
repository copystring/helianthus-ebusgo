package emulation

import (
	"errors"
	"math"
	"testing"
)

func TestDecodeVR92RoomTemperatureRecord_ObservedMarch2026(t *testing.T) {
	t.Parallel()

	// GG=0x0A, OP=0x06, II=1, RR=0x000F, captured value 13.625 C.
	record := []byte{0x01, 0x0A, 0x0F, 0x00, 0x00, 0x00, 0x5A, 0x41}
	got, err := DecodeVR92RoomTemperatureRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if got != 13.625 {
		t.Fatalf("decoded temperature = %v; want 13.625", got)
	}
}

func TestDecodeVR92RoomTemperatureRecord_RejectsUnqualifiedShapes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		record []byte
	}{
		{"nil", nil},
		{"short", []byte{0x01, 0x0A, 0x0F}},
		{"wrong response status", []byte{0x00, 0x0A, 0x0F, 0x00, 0x00, 0x00, 0x5A, 0x41}},
		{"wrong group", []byte{0x01, 0x09, 0x0F, 0x00, 0x00, 0x00, 0x5A, 0x41}},
		{"wrong register", []byte{0x01, 0x0A, 0x07, 0x00, 0x00, 0x00, 0x5A, 0x41}},
		{"wrong register high byte", []byte{0x01, 0x0A, 0x0F, 0x01, 0x00, 0x00, 0x5A, 0x41}},
		{"nan sentinel", []byte{0x01, 0x0A, 0x0F, 0x00, 0xFF, 0xFF, 0xFF, 0x7F}},
		{"infinity", []byte{0x01, 0x0A, 0x0F, 0x00, 0x00, 0x00, 0x80, 0x7F}},
		{"unrealistic hot", []byte{0x01, 0x0A, 0x0F, 0x00, 0x00, 0x00, 0xC8, 0x42}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeVR92RoomTemperatureRecord(test.record)
			if !errors.Is(err, ErrVR92RegisterRecordInvalid) {
				t.Fatalf("DecodeVR92RoomTemperatureRecord(%v) = %v, %v; want invalid record", test.record, got, err)
			}
		})
	}
}

func TestDecodeVR92RoomTemperatureRecord_FiniteBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		raw  []byte
		want float64
	}{
		{[]byte{0x01, 0x0A, 0x0F, 0x00, 0x00, 0x00, 0xA0, 0xC1}, -20},
		{[]byte{0x01, 0x0A, 0x0F, 0x00, 0x00, 0x00, 0x48, 0x42}, 50},
	} {
		got, err := DecodeVR92RoomTemperatureRecord(test.raw)
		if err != nil || math.Abs(got-test.want) > 0.00001 {
			t.Errorf("DecodeVR92RoomTemperatureRecord(%v) = %v, %v; want %v", test.raw, got, err, test.want)
		}
	}
}
