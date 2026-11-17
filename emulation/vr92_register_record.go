package emulation

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var ErrVR92RegisterRecordInvalid = errors.New("vr92 regulator register record invalid")

const (
	vr92RoomRegisterGroup  = byte(0x0A)
	vr92RoomRegisterNumber = byte(0x0F)
	vr92RoomRegisterSize   = 8
)

// DecodeVR92RoomTemperatureRecord decodes the observed B524 OP06, group 0x0A,
// instance 1, register 0x000F record shape from the public March 2026 fixture.
//
// This is a controller-held *read* record, not a remote-to-controller transfer
// or a valid eBUS write payload. Only the observed eight-byte layout is admitted.
func DecodeVR92RoomTemperatureRecord(record []byte) (float64, error) {
	if len(record) != vr92RoomRegisterSize ||
		record[0] != 0x01 ||
		record[1] != vr92RoomRegisterGroup ||
		record[2] != vr92RoomRegisterNumber ||
		record[3] != 0x00 {
		return 0, fmt.Errorf("unexpected VR92 room register record: %w", ErrVR92RegisterRecordInvalid)
	}

	temperature := float64(math.Float32frombits(binary.LittleEndian.Uint32(record[4:8])))
	if math.IsNaN(temperature) || math.IsInf(temperature, 0) ||
		temperature < VR92MinPlausibleRoomTemperatureC ||
		temperature > VR92MaxPlausibleRoomTemperatureC {
		return 0, fmt.Errorf("invalid VR92 room register temperature: %w", ErrVR92RegisterRecordInvalid)
	}
	return temperature, nil
}
