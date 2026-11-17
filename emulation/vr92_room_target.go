package emulation

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/Project-Helianthus/helianthus-ebusgo/protocol"
)

const (
	// These bounds are deliberately conservative until the VR92 wire format is confirmed.
	maxVR92RoomRequestBytes  = 16
	maxVR92RoomResponseBytes = 16
)

// VR92RoomEncoder translates a temperature into response data for a command
// verified by an independent VR92 wire capture. No built-in encoder or
// remote-to-controller telegram format is assumed.
type VR92RoomEncoder func(temperatureC float64) ([]byte, error)

// VR92RoomReplyMapping describes one explicitly matched, caller-verified
// request. RequestData must contain at least one byte so an unknown command
// cannot accidentally become an unconditional PB/SB responder.
type VR92RoomReplyMapping struct {
	Primary     byte
	Secondary   byte
	RequestData []byte
	Encode      VR92RoomEncoder
}

// NewVR92RoomTarget adds a dynamic, fail-closed temperature reply to the
// existing identification-only VR92 target. The caller supplies the verified
// request signature, response encoder and clock. The sensor is shared with
// the caller and may be updated as measurements arrive.
//
// This builds an in-memory target only. No eBUS transport, arbitration,
// address ownership, autonomous traffic or Home Assistant connection is set up.
func NewVR92RoomTarget(
	profile VR92Profile,
	sensor *VR92RoomSensor,
	mapping VR92RoomReplyMapping,
	now func() time.Time,
) (*Target, error) {
	if sensor == nil || mapping.Encode == nil || now == nil ||
		len(mapping.RequestData) == 0 ||
		len(mapping.RequestData) > maxVR92RoomRequestBytes {
		return nil, fmt.Errorf("invalid VR92 room reply dependency or request: %w", ErrInvalidConfiguration)
	}
	if mapping.Primary == 0x07 && mapping.Secondary == 0x04 {
		return nil, fmt.Errorf("VR92 room reply cannot override identification: %w", ErrInvalidConfiguration)
	}
	if mapping.Primary == 0xB5 && mapping.Secondary == 0x09 &&
		len(mapping.RequestData) == 1 &&
		isVR90B509ScanIDSelector(mapping.RequestData[0]) {
		return nil, fmt.Errorf("VR92 room reply cannot impersonate scan identification: %w", ErrInvalidConfiguration)
	}

	normalized, err := normalizeVR92Profile(profile)
	if err != nil {
		return nil, err
	}
	for _, existing := range normalized.MappedCommands {
		if existing.Primary != mapping.Primary || existing.Secondary != mapping.Secondary {
			continue
		}
		if existing.PayloadExact == nil && existing.PayloadPrefix == nil {
			return nil, fmt.Errorf("VR92 static reply shadows room reply: %w", ErrInvalidConfiguration)
		}
		if existing.PayloadExact != nil && bytes.Equal(existing.PayloadExact, mapping.RequestData) {
			return nil, fmt.Errorf("VR92 static exact reply shadows room reply: %w", ErrInvalidConfiguration)
		}
		if existing.PayloadPrefix != nil && bytes.HasPrefix(mapping.RequestData, existing.PayloadPrefix) {
			return nil, fmt.Errorf("VR92 static prefix reply shadows room reply: %w", ErrInvalidConfiguration)
		}
	}

	target, err := NewVR92Target(normalized)
	if err != nil {
		return nil, err
	}
	requestData := append([]byte(nil), mapping.RequestData...)

	roomRule := Rule{
		Name: "vr92-room-temperature",
		Matcher: MatchFunc(func(frame protocol.Frame) bool {
			return frame.Primary == mapping.Primary &&
				frame.Secondary == mapping.Secondary &&
				bytes.Equal(frame.Data, requestData)
		}),
		Builder: BuildFunc(func(_ protocol.Frame) (ResponsePlan, error) {
			reading, err := sensor.Snapshot(now())
			if err != nil {
				return ResponsePlan{}, fmt.Errorf("VR92 room reply sensor unavailable: %w", err)
			}
			payload, err := mapping.Encode(reading.TemperatureC)
			if err != nil {
				return ResponsePlan{}, fmt.Errorf("VR92 room reply encoding failed: %w", err)
			}
			if len(payload) == 0 || len(payload) > maxVR92RoomResponseBytes {
				return ResponsePlan{}, fmt.Errorf("VR92 room reply invalid encoded size %d: %w", len(payload), ErrInvalidConfiguration)
			}
			return ResponsePlan{
				Delay: normalized.ResponseDelay,
				Data:  append([]byte(nil), payload...),
			}, nil
		}),
	}

	// The first rule always handles identification. A disabled or absent
	// scan-ID rule cannot interfere with this exact room request matcher.
	// Place the dynamic reply before caller-provided static mappings.
	insertAt := 1
	if normalized.EnableB509Discovery {
		insertAt++
	}
	target.Rules = append(target.Rules, Rule{})
	copy(target.Rules[insertAt+1:], target.Rules[insertAt:])
	target.Rules[insertAt] = roomRule
	return target, nil
}

// VR92RoomReplyUnavailable returns true only when no fresh temperature was
// available. Callers must not treat that error as permission to transmit a
// stale value or an invented substitute payload.
func VR92RoomReplyUnavailable(err error) bool {
	return errors.Is(err, ErrVR92RoomReadingUnavailable)
}
