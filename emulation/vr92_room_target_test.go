package emulation

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/Project-Helianthus/helianthus-ebusgo/protocol"
)

func testVR92RoomMapping() VR92RoomReplyMapping {
	return VR92RoomReplyMapping{
		// Deliberately synthetic request and encoder: this is not a VR92 wire claim.
		Primary:     0xA3,
		Secondary:   0x5C,
		RequestData: []byte{0x11, 0x22},
		Encode: func(temperature float64) ([]byte, error) {
			return []byte{0x00, byte(math.Round(temperature * 4))}, nil
		},
	}
}

func testVR92RoomFrame() protocol.Frame {
	return protocol.Frame{
		Source:    0x10,
		Target:    testVR92Address,
		Primary:   0xA3,
		Secondary: 0x5C,
		Data:      []byte{0x11, 0x22},
	}
}

func TestVR92RoomTargetDynamicReplyAndIdentity(t *testing.T) {
	t.Parallel()
	sensor, err := NewVR92RoomSensor(5 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	mapping := testVR92RoomMapping()
	target, err := NewVR92RoomTarget(defaultVR92TestProfile(), sensor, mapping, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if target.Name != "vr92-minimal-0x35" {
		t.Fatalf("unexpected VR92 target name %q", target.Name)
	}

	_, err = target.Emulate(RequestEvent{Frame: testVR92RoomFrame()})
	if !VR92RoomReplyUnavailable(err) {
		t.Fatalf("reply before first reading must be unavailable: %v", err)
	}
	if err := sensor.Update(VR92RoomReading{TemperatureC: 21.5, ObservedAt: now}); err != nil {
		t.Fatal(err)
	}

	reply, err := target.Emulate(RequestEvent{At: time.Second, Frame: testVR92RoomFrame()})
	if err != nil {
		t.Fatal(err)
	}
	if reply.Rule != "vr92-room-temperature" || reply.Frame.Source != testVR92Address ||
		reply.Frame.Target != 0x10 || reply.RespondAt != time.Second+8*time.Millisecond ||
		!bytes.Equal(reply.Frame.Data, []byte{0x00, 0x56}) {
		t.Fatalf("unexpected room reply: %+v", reply)
	}

	// The identity response still uses the distinct VR92 identification bytes.
	idFrame := testVR92RoomFrame()
	idFrame.Primary, idFrame.Secondary, idFrame.Data = 0x07, 0x04, nil
	id, err := target.Emulate(RequestEvent{Frame: idFrame})
	if err != nil || id.Rule != "identify" || !bytes.Equal(id.Frame.Data[:6], []byte{0xB5, 'V', 'R', '_', '9', '2'}) {
		t.Fatalf("unexpected VR92 identify response: %+v, %v", id, err)
	}

	now = now.Add(time.Minute)
	if err := sensor.Update(VR92RoomReading{TemperatureC: 19.25, ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	second, err := target.Emulate(RequestEvent{Frame: testVR92RoomFrame()})
	if err != nil || !bytes.Equal(second.Frame.Data, []byte{0x00, 0x4D}) {
		t.Fatalf("second reading not reflected: %+v, %v", second, err)
	}

	// The response must be detached from the encoder's reused buffer.
	second.Frame.Data[0] = 0xFF
	again, err := target.Emulate(RequestEvent{Frame: testVR92RoomFrame()})
	if err != nil || again.Frame.Data[0] != 0x00 {
		t.Fatalf("reply alias leaked into later invocation: %+v, %v", again, err)
	}

	now = now.Add(5*time.Minute + time.Nanosecond)
	if _, err := target.Emulate(RequestEvent{Frame: testVR92RoomFrame()}); !VR92RoomReplyUnavailable(err) {
		t.Fatalf("stale reading must fail closed: %v", err)
	}
	sensor.Invalidate()
	if _, err := target.Emulate(RequestEvent{Frame: testVR92RoomFrame()}); !VR92RoomReplyUnavailable(err) {
		t.Fatalf("invalidated reading must fail closed: %v", err)
	}
}

func TestVR92RoomTargetExactCommandMatcherAndDefensiveCopy(t *testing.T) {
	t.Parallel()
	sensor, _ := NewVR92RoomSensor(time.Minute)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	mapping := testVR92RoomMapping()
	target, err := NewVR92RoomTarget(defaultVR92TestProfile(), sensor, mapping, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err := sensor.Update(VR92RoomReading{TemperatureC: 22, ObservedAt: now}); err != nil {
		t.Fatal(err)
	}

	mapping.RequestData[0] = 0xFF
	for _, mutation := range []func(*protocol.Frame){
		func(f *protocol.Frame) { f.Primary = 0xB5 },
		func(f *protocol.Frame) { f.Secondary = 0x0A },
		func(f *protocol.Frame) { f.Data = []byte{0x11} },
		func(f *protocol.Frame) { f.Data = []byte{0x11, 0x23} },
		func(f *protocol.Frame) { f.Data = []byte{0x11, 0x22, 0x00} },
	} {
		frame := testVR92RoomFrame()
		mutation(&frame)
		if _, err := target.Emulate(RequestEvent{Frame: frame}); !errors.Is(err, ErrNoMatchingRule) {
			t.Errorf("unmatched frame %+v was accepted: %v", frame, err)
		}
	}
	reply, err := target.Emulate(RequestEvent{Frame: testVR92RoomFrame()})
	if err != nil || reply.Rule != "vr92-room-temperature" {
		t.Fatalf("original request not preserved: %+v, %v", reply, err)
	}
}

func TestVR92RoomTargetRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()
	sensor, _ := NewVR92RoomSensor(time.Minute)
	now := func() time.Time { return time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC) }
	profile := defaultVR92TestProfile()
	cases := []struct {
		name    string
		profile VR92Profile
		sensor  *VR92RoomSensor
		mapping VR92RoomReplyMapping
		clock   func() time.Time
	}{
		{"nil sensor", profile, nil, testVR92RoomMapping(), now},
		{"nil clock", profile, sensor, testVR92RoomMapping(), nil},
		{"nil encoder", profile, sensor, VR92RoomReplyMapping{Primary: 0xA3, Secondary: 0x5C, RequestData: []byte{1}}, now},
		{"empty payload", profile, sensor, VR92RoomReplyMapping{Primary: 0xA3, Secondary: 0x5C, Encode: func(float64) ([]byte, error) { return []byte{1}, nil }}, now},
		{"oversized request", profile, sensor, VR92RoomReplyMapping{Primary: 0xA3, Secondary: 0x5C, RequestData: bytes.Repeat([]byte{1}, 17), Encode: testVR92RoomMapping().Encode}, now},
		{"identify conflict", profile, sensor, VR92RoomReplyMapping{Primary: 0x07, Secondary: 0x04, RequestData: []byte{0}, Encode: testVR92RoomMapping().Encode}, now},
		{"scan identifier conflict", profile, sensor, VR92RoomReplyMapping{Primary: 0xB5, Secondary: 0x09, RequestData: []byte{0x24}, Encode: testVR92RoomMapping().Encode}, now},
		{"zero profile address", VR92Profile{}, sensor, testVR92RoomMapping(), now},
		{"static exact command conflict", VR92Profile{Address: testVR92Address, MappedCommands: []VR92MappedCommand{{Primary: 0xA3, Secondary: 0x5C, PayloadExact: []byte{0x11, 0x22}, ResponseData: []byte{0}}}}, sensor, testVR92RoomMapping(), now},
		{"static broad command conflict", VR92Profile{Address: testVR92Address, MappedCommands: []VR92MappedCommand{{Primary: 0xA3, Secondary: 0x5C, ResponseData: []byte{0}}}}, sensor, testVR92RoomMapping(), now},
		{"static prefix command conflict", VR92Profile{Address: testVR92Address, MappedCommands: []VR92MappedCommand{{Primary: 0xA3, Secondary: 0x5C, PayloadPrefix: []byte{0x11}, ResponseData: []byte{0}}}}, sensor, testVR92RoomMapping(), now},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewVR92RoomTarget(tc.profile, tc.sensor, tc.mapping, tc.clock)
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("expected invalid configuration, got %v", err)
			}
		})
	}
}

func TestVR92RoomTargetRejectsEncoderErrorsAndEmptyOutputs(t *testing.T) {
	t.Parallel()
	sensor, _ := NewVR92RoomSensor(time.Minute)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	if err := sensor.Update(VR92RoomReading{TemperatureC: 21.5, ObservedAt: now}); err != nil {
		t.Fatal(err)
	}
	failed := fmt.Errorf("missing wire proof")
	for _, tc := range []struct {
		name   string
		encode VR92RoomEncoder
		want   error
	}{
		{"encoder error", func(float64) ([]byte, error) { return nil, failed }, failed},
		{"empty output", func(float64) ([]byte, error) { return nil, nil }, ErrInvalidConfiguration},
		{"too long", func(float64) ([]byte, error) { return bytes.Repeat([]byte{1}, 17), nil }, ErrInvalidConfiguration},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mapping := testVR92RoomMapping()
			mapping.Encode = tc.encode
			target, err := NewVR92RoomTarget(defaultVR92TestProfile(), sensor, mapping, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			_, err = target.Emulate(RequestEvent{Frame: testVR92RoomFrame()})
			if !errors.Is(err, tc.want) {
				t.Fatalf("encoder failure = %v; want %v", err, tc.want)
			}
		})
	}
}
