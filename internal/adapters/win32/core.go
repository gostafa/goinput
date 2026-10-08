// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
)

type (
	keyTables struct {
		keyboard map[uint16]uint16
		consumer map[uint16]uint16
		virtual  map[uint16]uint16
	}
	keyboardInput struct {
		makeCode   uint16
		flags      uint16
		virtualKey uint16
	}
	capabilityRange struct {
		firstUsage uint32
		lastUsage  uint32
		firstIndex uint32
		lastIndex  uint32
	}
	command struct {
		cause     func() error
		done      <-chan struct{}
		operation func() error
		reply     chan error
		state     atomic.Int32
	}
	inputPacketRecord[Stamp any] struct {
		stamp  Stamp
		body   []byte
		device uintptr
		kind   uint32
	}
	inputPacket = inputPacketRecord[domain.Timestamp]
)

func allocateBuffer[Value any](length int) []Value {
	buffer := make([]Value, noValue, length)
	for range length {
		buffer = append(buffer, *new(Value))
	}

	return buffer
}

func unsignedWord(value int32) uint32 {
	if value >= noValue {
		return uint32(value)
	}

	return math.MaxUint32 - uint32(-(int64(value)+singleValue)&math.MaxInt32)
}

func signedWord(value uint32) int64 {
	if value <= math.MaxInt32 {
		return int64(value)
	}

	return int64(value) - (int64(singleValue) << thirtySecondValue)
}

func signedHalf(value uint16) int16 {
	if value <= math.MaxInt16 {
		return int16(value)
	}

	return -singleValue - int16(math.MaxUint16-value)
}

func logicalBounds(minimum, maximum int32) domain.Range {
	bounds := domain.Range{Min: int64(minimum), Max: int64(maximum)}
	if minimum >= noValue && maximum < noValue {
		bounds.Max = int64(unsignedWord(maximum))
	}

	return bounds
}

func logicalValue(word uint32, control *ext.NativeControl) int64 {
	bits := control.BitSize
	if bits == noValue || bits > thirtySecondValue {
		return noValue
	}

	mask := uint64(singleValue)<<bits - singleValue
	value := uint64(word) & mask

	signed := control.Bounds.LogicalMin < noValue
	if signed && logicalSignSet(value, bits) {
		return int64(value&math.MaxUint32) - (int64(singleValue) << bits)
	}

	return int64(value & math.MaxUint32)
}

func hidValueAction(kind domain.ControlKind, value int64) domain.EventAction {
	if kind != domain.ControlSwitch {
		return domain.ActionChange
	}

	if value != noValue {
		return domain.ActionPress
	}

	return domain.ActionRelease
}

func validReportBatch(body []byte, size, count uint32) bool {
	return len(body) >= eighthValue && size != noValue && count <= maxDevices &&
		uint64(size)*uint64(count) <= uint64(max(noValue, len(body)-eighthValue))
}

func hidID(report byte, collection, index uint16) domain.ControlID {
	return domain.ControlID(fmt.Sprintf("hid:%02x:%04x:%04x", report, collection, index))
}

func capRange(words [eighthValue]uint16, rangeKind uint8) (capabilityRange, error) {
	span := capabilityRange{
		firstUsage: uint32(words[noValue]), lastUsage: uint32(words[noValue]),
		firstIndex: uint32(words[sixthValue]), lastIndex: uint32(words[sixthValue]),
	}

	if rangeKind != noValue {
		span.lastUsage, span.lastIndex = uint32(words[singleValue]), uint32(words[seventhValue])
	}

	if !capabilityRangeValid(span) {
		return capabilityRange{}, fmt.Errorf(
			"invalid HID capability range: %w",
			domain.ErrUnsupported,
		)
	}

	return span, nil
}

func capabilityRangeValid(span capabilityRange) bool {
	return span.lastUsage >= span.firstUsage && span.lastIndex >= span.firstIndex &&
		span.lastUsage-span.firstUsage == span.lastIndex-span.firstIndex
}

func keyID(usage domain.Usage, scan uint16) domain.ControlID {
	if scan&virtualScanPrefix == virtualScanPrefix {
		return domain.ControlID(fmt.Sprintf("key:virtual:%04x", scan&byteMask))
	}

	if scan != noValue {
		return domain.ControlID(fmt.Sprintf("key:scan:%04x", scan))
	}

	if usage != noValue {
		return domain.ControlID(fmt.Sprintf("key:%08x", uint32(usage)))
	}

	return domain.ControlID(fmt.Sprintf("key:native:%04x", scan))
}

func keyboardInputScanCode(key keyboardInput) uint16 {
	switch {
	case key.flags&secondValue != noValue:
		return key.makeCode | scanPrefixE0
	case key.flags&fourthValue != noValue:
		return key.makeCode | scanPrefixE1
	default:
		return key.makeCode
	}
}

func keyboardInputIgnored(key keyboardInput) bool {
	if key.virtualKey >= byteMask {
		return true
	}

	switch keyboardInputScanCode(key) {
	case scanPrintScreenPrefix, scanPrintScreenSuffix, scanPausePrefix:
		return true
	default:
		return false
	}
}

func keyboardInputIdentity(key keyboardInput, tables *keyTables) domain.ControlID {
	scan := keyboardInputScanCode(key)
	if key.makeCode == noValue && !knownScan(scan, tables) {
		scan = virtualScanPrefix | key.virtualKey
	}

	return keyID(keyboardInputUsage(key, tables), scan)
}

func keyboardInputUsage(key keyboardInput, tables *keyTables) domain.Usage {
	scan := keyboardInputScanCode(key)
	if code := tables.keyboard[scan]; code != noValue {
		return domain.HID(seventhValue, code)
	}

	if code := tables.consumer[scan]; code != noValue {
		return domain.HID(twelfthValue, code)
	}

	if key.makeCode == noValue {
		return virtualKeyUsage(key.virtualKey, tables)
	}

	return systemScanUsage(scan)
}

func knownScan(scan uint16, tables *keyTables) bool {
	return tables.keyboard[scan] != noValue || tables.consumer[scan] != noValue
}

func virtualKeyUsage(key uint16, tables *keyTables) domain.Usage {
	if code := tables.virtual[key]; code != noValue {
		return domain.HID(twelfthValue, code)
	}

	return noValue
}

func systemScanUsage(scan uint16) domain.Usage {
	switch scan {
	case scanPower:
		return domain.HID(singleValue, systemPowerUsage)
	case scanSleep:
		return domain.HID(singleValue, systemSleepUsage)
	case scanWake:
		return domain.HID(singleValue, systemWakeUsage)
	default:
		return noValue
	}
}

func makeKeyTables(data *[thirdValue]string) (*keyTables, error) {
	scans := make([]map[uint16]uint16, noValue, thirdValue)

	for index := range data {
		entries, err := readScanTable(data[index])
		if err != nil {
			return nil, errors.Join(err)
		}

		scans = append(scans, entries)
	}

	return &keyTables{
		keyboard: scans[noValue],
		consumer: scans[singleValue],
		virtual:  scans[secondValue],
	}, nil
}

func readScanTable(data string) (map[uint16]uint16, error) {
	result := make(map[uint16]uint16)
	rows := strings.Split(strings.TrimSpace(data), "\n")

	for index := range rows {
		code, usage, err := readScanEntry(rows[index])
		if err != nil {
			return nil, errors.Join(err)
		}

		result[code] = usage
	}

	return result, nil
}

func readScanEntry(row string) (scanCode, hidUsage uint16, err error) {
	entry, description, found := strings.Cut(row, " ")
	if found && strings.TrimSpace(description) == emptyString {
		return noValue, noValue, domain.ErrUnsupported
	}

	code, usage, valid := strings.Cut(entry, ":")

	if !valid {
		return noValue, noValue, domain.ErrUnsupported
	}

	key, keyErr := strconv.ParseUint(code, sixteenthValue, sixteenthValue)
	value, valueErr := strconv.ParseUint(usage, sixteenthValue, sixteenthValue)

	return uint16(key & wordMask), uint16(value & wordMask), errors.Join(keyErr, valueErr)
}

func makeCommand(ctx context.Context, operation func() error) *command {
	cmd := new(command)

	cmd.cause = func() error { return context.Cause(ctx) }
	cmd.done = ctx.Done()
	cmd.operation = operation
	cmd.reply = make(chan error, singleValue)

	return cmd
}

func cancelPendingCommand(cmd *command) error {
	if cmd.state.CompareAndSwap(noValue, secondValue) {
		return errors.Join(cmd.cause())
	}

	return nil
}

func checkedInputResult(value uintptr) (uint32, error) {
	if value > math.MaxUint32 {
		return noValue, domain.ErrEventLoss
	}

	return uint32(value), nil
}

func decodeInputPacket(data []byte) (inputPacket, error) {
	headerSize := uint32(mousePacketBytes)
	if len(data) < int(headerSize) {
		return inputPacket{}, domain.ErrEventLoss
	}

	declared := binary.LittleEndian.Uint32(data[fourthValue:])

	if declared < headerSize || uint64(declared) > uint64(len(data)) {
		return inputPacket{}, errors.Join(domain.ErrEventLoss)
	}

	now := time.Now()

	return inputPacket{
		kind:   binary.LittleEndian.Uint32(data),
		device: uintptr(binary.LittleEndian.Uint64(data[eighthValue:])),
		body:   data[headerSize:declared],
		stamp:  domain.Timestamp{Time: now.UTC(), ReceivedAt: now, Source: domain.TimestampReceipt},
	}, nil
}

func logicalSignSet(value uint64, bits uint16) bool {
	return value&(uint64(singleValue)<<(bits-singleValue)) != noValue
}
