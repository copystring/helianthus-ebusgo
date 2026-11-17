package emulation

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	// These are application-level plausibility limits, not Vaillant wire limits.
	VR92MinPlausibleRoomTemperatureC = -20.0
	VR92MaxPlausibleRoomTemperatureC = 50.0
)

var (
	ErrVR92RoomReadingInvalid    = errors.New("vr92 room reading invalid")
	ErrVR92RoomReadingUnavailable = errors.New("vr92 room reading unavailable")
	ErrVR92RoomReadingOutOfOrder = errors.New("vr92 room reading out of order")
)

// VR92RoomReading is a room temperature observation supplied by an external source.
// It does not describe an eBUS telegram or confirm a Vaillant register encoding.
type VR92RoomReading struct {
	TemperatureC float64
	ObservedAt   time.Time
}

// VR92RoomSensor keeps the last validated room reading for future protocol work.
// No method sends, acknowledges, or encodes an eBUS message.
type VR92RoomSensor struct {
	mu       sync.RWMutex
	maxAge   time.Duration
	reading  VR92RoomReading
	hasValue bool
}

// NewVR92RoomSensor rejects an unbounded or non-positive freshness interval.
func NewVR92RoomSensor(maxAge time.Duration) (*VR92RoomSensor, error) {
	if maxAge <= 0 || maxAge > time.Hour {
		return nil, fmt.Errorf("invalid vr92 sensor max age %s: %w", maxAge, ErrInvalidConfiguration)
	}
	return &VR92RoomSensor{maxAge: maxAge}, nil
}

// Update accepts only plausible finite readings in strictly increasing time order.
func (s *VR92RoomSensor) Update(reading VR92RoomReading) error {
	if s == nil ||
		reading.ObservedAt.IsZero() ||
		math.IsNaN(reading.TemperatureC) ||
		math.IsInf(reading.TemperatureC, 0) ||
		reading.TemperatureC < VR92MinPlausibleRoomTemperatureC ||
		reading.TemperatureC > VR92MaxPlausibleRoomTemperatureC {
		return ErrVR92RoomReadingInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasValue && !reading.ObservedAt.After(s.reading.ObservedAt) {
		return ErrVR92RoomReadingOutOfOrder
	}
	s.reading = reading
	s.hasValue = true
	return nil
}

// Snapshot returns no reading when the source is absent, expired, or from the future.
func (s *VR92RoomSensor) Snapshot(now time.Time) (VR92RoomReading, error) {
	if s == nil || now.IsZero() {
		return VR92RoomReading{}, ErrVR92RoomReadingUnavailable
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasValue || now.Before(s.reading.ObservedAt) ||
		now.Sub(s.reading.ObservedAt) > s.maxAge {
		return VR92RoomReading{}, ErrVR92RoomReadingUnavailable
	}
	return s.reading, nil
}

// Invalidate prevents use of the previous reading after an external source failure.
func (s *VR92RoomSensor) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reading = VR92RoomReading{}
	s.hasValue = false
}
