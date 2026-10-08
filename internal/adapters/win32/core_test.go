// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	ext "github.com/gostafa/goinput/extensions/win32"
	"github.com/gostafa/goinput/internal/domain"
)

const (
	keyboardScanCount = 123
	letterAScan       = 0x1e
	invalidScanEntry  = "invalid"
)

func TestSignedNativeWords(t *testing.T) {
	t.Parallel()
	assertCoreEqual(t, signedWord(math.MaxUint32), int64(-singleValue))
	assertCoreEqual(t, signedWord(math.MaxInt32), int64(math.MaxInt32))
	assertCoreEqual(t, signedWord(math.MaxInt32+singleValue), int64(math.MinInt32))
	assertCoreEqual(t, signedHalf(math.MaxUint16), int16(-singleValue))
	assertCoreEqual(t, signedHalf(math.MaxInt16), int16(math.MaxInt16))
	assertCoreEqual(t, signedHalf(math.MaxInt16+singleValue), int16(math.MinInt16))
	assertCoreEqual(t, unsignedWord(-singleValue), uint32(math.MaxUint32))
	assertCoreEqual(t, unsignedWord(math.MinInt32), uint32(math.MaxInt32+singleValue))
	assertCoreEqual(t, unsignedWord(singleValue), uint32(singleValue))
}

func TestLogicalReportWidths(t *testing.T) {
	t.Parallel()

	control := new(ext.NativeControl)
	assertCoreEqual(t, logicalValue(math.MaxUint32, control), int64(noValue))

	control.BitSize = thirtySecondValue + singleValue
	assertCoreEqual(t, logicalValue(math.MaxUint32, control), int64(noValue))

	control.BitSize = eighthValue
	assertCoreEqual(t, logicalValue(math.MaxUint32, control), int64(byteMask))

	control.Bounds.LogicalMin = -singleValue
	assertCoreEqual(t, logicalValue(math.MaxUint32, control), int64(-singleValue))
	assertCoreEqual(t, logicalValue(singleValue, control), int64(singleValue))
}

func TestUnsignedLogicalBounds(t *testing.T) {
	t.Parallel()

	bounds := logicalBounds(noValue, -singleValue)
	assertCoreEqual(t, bounds.Max, int64(math.MaxUint32))

	bounds = logicalBounds(-singleValue, singleValue)
	assertCoreEqual(t, bounds.Min, int64(-singleValue))
	assertCoreEqual(t, bounds.Max, int64(singleValue))
}

func TestCapabilityRangeValidation(t *testing.T) {
	t.Parallel()

	var words [eighthValue]uint16

	words[noValue], words[sixthValue] = singleValue, thirdValue

	span, err := capRange(words, noValue)
	assertCoreError(t, err, nil)
	assertCoreEqual(t, span.lastIndex, uint32(thirdValue))

	words[singleValue], words[seventhValue] = thirdValue, fifthValue
	span, err = capRange(words, singleValue)
	assertCoreError(t, err, nil)
	assertCoreEqual(t, span.lastUsage, uint32(thirdValue))
}

func TestCapabilityRangeRejectsMismatchedSpans(t *testing.T) {
	t.Parallel()

	var words [eighthValue]uint16

	words[noValue], words[singleValue] = thirdValue, singleValue

	span, err := capRange(words, singleValue)
	assertCoreError(t, err, domain.ErrUnsupported)
	assertCoreEqual(t, span.lastUsage, uint32(noValue))

	words[noValue], words[singleValue] = noValue, thirdValue
	span, err = capRange(words, singleValue)
	assertCoreError(t, err, domain.ErrUnsupported)
	assertCoreEqual(t, span.lastIndex, uint32(noValue))
}

func TestRawInputRejectsTruncatedPackets(t *testing.T) {
	t.Parallel()

	for length := range mousePacketBytes {
		packet, err := decodeInputPacket(allocateBuffer[byte](length))
		assertCoreError(t, err, domain.ErrEventLoss)
		assertCoreEqual(t, len(packet.body), noValue)
	}
}

func TestRawInputRejectsInvalidDeclaredSizes(t *testing.T) {
	t.Parallel()

	data := allocateBuffer[byte](mousePacketBytes)
	packet, err := decodeInputPacket(data)
	assertCoreError(t, err, domain.ErrEventLoss)
	assertCoreEqual(t, len(packet.body), noValue)
	binary.LittleEndian.PutUint32(data[fourthValue:], mousePacketBytes+singleValue)

	packet, err = decodeInputPacket(data)
	assertCoreError(t, err, domain.ErrEventLoss)
	assertCoreEqual(t, len(packet.body), noValue)
}

func TestRawInputPreservesHandleAndReceiptTime(t *testing.T) {
	t.Parallel()

	data := testInputPacket()
	packet, err := decodeInputPacket(data)
	assertCoreError(t, err, nil)
	assertCoreEqual(t, packet.device, uintptr(singleValue))
	assertCoreEqual(t, packet.kind, uint32(secondValue))
	assertCoreEqual(t, len(packet.body), singleValue)
	assertCoreEqual(t, packet.stamp.Source, domain.TimestampReceipt)
	assertCoreEqual(t, packet.stamp.Time.Equal(packet.stamp.ReceivedAt), true)
}

func TestReportBatchBounds(t *testing.T) {
	t.Parallel()

	body := allocateBuffer[byte](twelfthValue)
	assertCoreEqual(t, validReportBatch(body, secondValue, secondValue), true)
	assertCoreEqual(t, validReportBatch(body, thirdValue, secondValue), false)
	assertCoreEqual(t, validReportBatch(body, noValue, singleValue), false)
	assertCoreEqual(t, validReportBatch(body, singleValue, maxDevices+singleValue), false)
	assertCoreEqual(t, validReportBatch(nil, singleValue, noValue), false)
}

func TestHIDSwitchActions(t *testing.T) {
	t.Parallel()
	assertCoreEqual(t, hidValueAction(domain.ControlSwitch, noValue), domain.ActionRelease)
	assertCoreEqual(t, hidValueAction(domain.ControlSwitch, singleValue), domain.ActionPress)
	assertCoreEqual(t, hidValueAction(domain.ControlAxis, singleValue), domain.ActionChange)
	assertCoreEqual(
		t,
		hidID(singleValue, secondValue, thirdValue),
		domain.ControlID("hid:01:0002:0003"),
	)
}

func TestScanTablesPreservePhysicalMappings(t *testing.T) {
	t.Parallel()

	tables := testKeyTables(t)
	assertCoreEqual(t, len(tables.keyboard), keyboardScanCount)
	assertCoreEqual(t, tables.keyboard[letterAScan], uint16(domain.KeyA&wordMask))
	assertCoreEqual(t, tables.keyboard[scanPrefixE0|sixteenthValue], uint16(noValue))
	assertCoreEqual(
		t,
		tables.consumer[scanPrefixE0|sixteenthValue],
		uint16(domain.KeyPrevTrack&wordMask),
	)
	assertCoreEqual(t, virtualKeyUsage(byteMask, tables), domain.Usage(noValue))
}

func TestScanTableRejectsMalformedEntries(t *testing.T) {
	t.Parallel()

	code, usage, err := readScanEntry(invalidScanEntry)
	assertCoreError(t, err, domain.ErrUnsupported)
	assertCoreEqual(t, code|usage, uint16(noValue))

	tables, err := readScanTable(invalidScanEntry)
	assertCoreError(t, err, domain.ErrUnsupported)
	assertCoreEqual(t, len(tables), noValue)
}

func TestScanEntryRejectsOverflow(t *testing.T) {
	t.Parallel()

	code, usage, err := readScanEntry("ffff:10000")
	assertCoreEqual(t, err != nil, true)
	assertCoreEqual(t, code, uint16(wordMask))
	assertCoreEqual(t, usage, uint16(wordMask))
}

func TestScanEntryRejectsInvalidCode(t *testing.T) {
	t.Parallel()

	code, usage, err := readScanEntry("invalid:0001")
	assertCoreEqual(t, err != nil, true)
	assertCoreEqual(t, code, uint16(noValue))
	assertCoreEqual(t, usage, uint16(singleValue))
}

func TestKeyTableConstructionReportsInvalidData(t *testing.T) {
	t.Parallel()

	data := [thirdValue]string{scanUsagesData, consumerScansData, consumerVirtualKeysData}
	for index := range data {
		fixture := data

		fixture[index] = invalidScanEntry

		tables, err := makeKeyTables(&fixture)
		assertCoreError(t, err, domain.ErrUnsupported)
		assertCoreEqual(t, tables == nil, true)
	}
}

func TestKeyboardPrefixesAndIgnoredInputs(t *testing.T) {
	t.Parallel()

	key := keyboardInput{
		makeCode:   singleValue,
		flags:      secondValue | fourthValue,
		virtualKey: singleValue,
	}
	assertCoreEqual(t, keyboardInputScanCode(key), uint16(scanPrefixE0|singleValue))

	key.flags = fourthValue
	assertCoreEqual(t, keyboardInputScanCode(key), uint16(scanPrefixE1|singleValue))
}

func TestKeyboardPhysicalIdentity(t *testing.T) {
	t.Parallel()

	tables := testKeyTables(t)
	key := keyboardInput{makeCode: letterAScan, flags: noValue, virtualKey: singleValue}
	assertCoreEqual(t, keyboardInputUsage(key, tables), domain.KeyA)
	assertCoreEqual(t, keyboardInputIdentity(key, tables), keyID(domain.KeyA, letterAScan))
	assertCoreEqual(t, knownScan(letterAScan, tables), true)

	key.flags, key.makeCode = secondValue, sixteenthValue
	assertCoreEqual(t, keyboardInputUsage(key, tables), domain.KeyPrevTrack)
}

func TestKeyboardVirtualIdentity(t *testing.T) {
	t.Parallel()

	tables := testKeyTables(t)
	key := keyboardInput{makeCode: noValue, flags: noValue, virtualKey: singleValue}
	assertCoreEqual(t, keyboardInputUsage(key, tables), domain.Usage(noValue))
	assertCoreEqual(
		t,
		keyboardInputIdentity(key, tables),
		keyID(noValue, virtualScanPrefix|singleValue),
	)

	for code := range tables.virtual {
		assertCoreEqual(
			t,
			virtualKeyUsage(code, tables),
			domain.HID(twelfthValue, tables.virtual[code]),
		)
	}
}

func TestKeyboardSystemUsages(t *testing.T) {
	t.Parallel()

	tables := testKeyTables(t)
	key := keyboardInput{
		makeCode:   scanPower & byteMask,
		flags:      secondValue,
		virtualKey: singleValue,
	}
	assertCoreEqual(t, keyboardInputUsage(key, tables), domain.HID(singleValue, systemPowerUsage))
	assertCoreEqual(t, systemScanUsage(scanSleep), domain.HID(singleValue, systemSleepUsage))
	assertCoreEqual(t, systemScanUsage(scanWake), domain.HID(singleValue, systemWakeUsage))
	assertCoreEqual(t, systemScanUsage(noValue), domain.Usage(noValue))
}

func TestKeyIDsDistinguishNativeAndLogicalKeys(t *testing.T) {
	t.Parallel()
	assertCoreEqual(t, keyID(domain.KeyA, noValue), domain.ControlID("key:00070004"))
	assertCoreEqual(t, keyID(noValue, noValue), domain.ControlID("key:native:0000"))
}

func TestCancellationSkipsPendingCommand(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cmd := makeCommand(ctx, func() error { return nil })

	cancel()
	assertCoreEqual(t, cmd.done, ctx.Done())
	assertCoreError(t, cmd.cause(), context.Canceled)
	assertCoreError(t, cancelPendingCommand(cmd), context.Canceled)
	assertCoreEqual(t, cmd.state.Load(), int32(secondValue))
	assertCoreError(t, cancelPendingCommand(cmd), nil)
	assertCoreError(t, cmd.operation(), nil)
}

func TestCancellationLeavesRunningCommandAlone(t *testing.T) {
	t.Parallel()

	cmd := makeCommand(t.Context(), func() error { return nil })
	cmd.state.Store(singleValue)
	assertCoreError(t, cancelPendingCommand(cmd), nil)
	assertCoreEqual(t, cmd.state.Load(), int32(singleValue))
}

func TestNativeResultRejectsOverflow(t *testing.T) {
	t.Parallel()

	value, err := checkedInputResult(math.MaxUint32)
	assertCoreError(t, err, nil)
	assertCoreEqual(t, value, uint32(math.MaxUint32))

	value, err = checkedInputResult(uintptr(math.MaxUint32) + singleValue)
	assertCoreError(t, err, domain.ErrEventLoss)
	assertCoreEqual(t, value, uint32(noValue))
}

func testInputPacket() []byte {
	data := allocateBuffer[byte](mousePacketBytes + singleValue)
	binary.LittleEndian.PutUint32(data, secondValue)
	binary.LittleEndian.PutUint32(data[fourthValue:], mousePacketBytes+singleValue)
	binary.LittleEndian.PutUint64(data[eighthValue:], singleValue)

	return data
}

func testKeyTables(t *testing.T) *keyTables {
	t.Helper()

	tables, err := makeKeyTables(
		&[thirdValue]string{scanUsagesData, consumerScansData, consumerVirtualKeysData},
	)
	assertCoreError(t, err, nil)

	return tables
}

func assertCoreEqual[Value comparable](t *testing.T, actual, expected Value) {
	t.Helper()

	if actual != expected {
		t.Fatalf("got %v, want %v", actual, expected)
	}
}

func assertCoreError(t *testing.T, actual, expected error) {
	t.Helper()

	if !errors.Is(actual, expected) {
		t.Fatalf("got error %v, want %v", actual, expected)
	}
}

func TestKeyboardIgnoredInputs(t *testing.T) {
	t.Parallel()

	key := keyboardInput{makeCode: singleValue, flags: fourthValue, virtualKey: singleValue}

	key.virtualKey = byteMask
	assertCoreEqual(t, keyboardInputIgnored(key), true)

	key.virtualKey, key.makeCode = singleValue, scanPausePrefix&byteMask
	assertCoreEqual(t, keyboardInputIgnored(key), true)

	key.flags = noValue
	assertCoreEqual(t, keyboardInputIgnored(key), false)
}

func TestScanEntryRejectsEmptyDescription(t *testing.T) {
	t.Parallel()

	code, usage, err := readScanEntry("0001:0001 ")
	assertCoreError(t, err, domain.ErrUnsupported)
	assertCoreEqual(t, code|usage, uint16(noValue))
}
