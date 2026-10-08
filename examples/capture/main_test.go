// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gostafa/goinput"
	deviceview "github.com/gostafa/goinput/internal/ports/device"
	managerview "github.com/gostafa/goinput/internal/ports/manager"
)

type (
	testDevice   = deviceview.Operations[goinput.DeviceInfo, goinput.Capabilities, goinput.Event]
	testManager  = managerview.Operations[goinput.DeviceInfo, goinput.DeviceID, goinput.Device]
	failedWriter struct{}
)

const (
	testEmpty         = 0
	testSingle        = 1
	testMultiple      = 2
	testCountArgument = "2"
	testDeviceID      = "test-device"
	testControlID     = "test-control"
	testCyclesFlag    = "-cycles"
	testManagersFlag  = "-managers"
	testDurationFlag  = "-duration"
	testHelpFlag      = "-h"
)

func (failedWriter) Write([]byte) (int, error) { return testEmpty, errors.Join(goinput.ErrClosed) }

func TestParseOptionsValidatesCountsAndDuration(t *testing.T) {
	t.Parallel()

	args := []string{
		testCyclesFlag,
		testCountArgument,
		testManagersFlag,
		testCountArgument,
		testDurationFlag,
		"1ms",
		testDeviceID,
	}
	options, err := parseOptions(args)
	checkValue(t, err, error(nil))
	checkValue(t, options.cycles, testMultiple)
	checkValue(t, options.managers, testMultiple)
	checkValue(t, options.duration, time.Millisecond)
	checkValue(t, options.args[testEmpty], testDeviceID)
}

func TestParseOptionsRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	cases := invalidArguments()
	for index := range cases {
		options, err := parseOptions(cases[index])
		checkCause(t, err, errUsage)
		checkValue(t, options.logger != nil, true)
	}
}

func invalidArguments() [][]string {
	return [][]string{
		{testCyclesFlag, "0"},
		{testManagersFlag, "0"},
		{testDurationFlag, "-1ms"},
		{testDeviceID, testControlID},
	}
}

func TestRunReportsFlagErrors(t *testing.T) {
	t.Parallel()
	checkCause(t, run([]string{testHelpFlag}), flag.ErrHelp)

	options, err := parseOptions([]string{"-invalid-flag"})
	checkValue(t, options.logger, (*slog.Logger)(nil))
	checkValue(t, err != nil, true)
}

func TestMainAcceptsHelp(t *testing.T) {
	t.Parallel()

	arguments := os.Args
	if len(arguments) <= testSingle {
		t.Skip("test binary has no CLI flags")
	}

	original := arguments[testSingle]

	arguments[testSingle] = testHelpFlag

	defer func() { arguments[testSingle] = original }()

	main()
}

func TestReportResultSelectsFailureExit(t *testing.T) {
	t.Parallel()

	calls := testEmpty

	exit := func(code int) { checkValue(t, code, exitFailure); calls++ }

	reportResult(t.Context(), nil, exit)
	reportResult(t.Context(), flag.ErrHelp, exit)
	reportResult(t.Context(), goinput.ErrClosed, exit)
	checkValue(t, calls, testSingle)
}

func TestRunCompletesDiscoveryLifecycle(t *testing.T) {
	t.Parallel()
	checkValue(t, run(nil), error(nil))
}

func TestRunCyclesPreservesSystemFailure(t *testing.T) {
	t.Parallel()

	err := runCycles(t.Context(), captureTestOptions(), failedSystem)
	checkCause(t, err, goinput.ErrUnsupported)
}

func TestRunCyclesPreservesCycleFailure(t *testing.T) {
	t.Parallel()

	var ctx context.Context

	checkCause(
		t,
		runCycles(ctx, captureTestOptions(), systemCreator(t.Context())),
		goinput.ErrInvalidOptions,
	)
}

func TestExtraManagersReleaseCanceledSessions(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	options := captureTestOptions()

	options.managers = testMultiple

	managers, err := extraManagers(ctx, options, captureTestSystem(t))
	checkValue(t, err, error(nil))
	checkValue(t, len(managers), testSingle)
	closeManagers(t.Context(), options.logger, managers)
}

func TestExtraManagersPreservesCreationFailure(t *testing.T) {
	t.Parallel()

	options := captureTestOptions()

	options.managers = testMultiple

	var ctx context.Context

	managers, err := extraManagers(ctx, options, captureTestSystem(t))
	checkValue(t, len(managers), testEmpty)
	checkCause(t, err, goinput.ErrInvalidOptions)
}

func TestPrintDevicesReportsWriteFailure(t *testing.T) {
	t.Parallel()

	var info goinput.DeviceInfo

	info.ID, info.Name = testDeviceID, testControlID

	var output bytes.Buffer

	checkValue(t, printDevices(&output, []goinput.DeviceInfo{info}), error(nil))
	checkValue(t, output.String(), testDeviceID+"\t"+testControlID+"\n")
	checkCause(t, printDevices(failedWriter{}, []goinput.DeviceInfo{info}), goinput.ErrClosed)
}

func TestReadEventsPrintsAndPreservesTerminalFailure(t *testing.T) {
	t.Parallel()

	device := captureTestDevice()

	var event goinput.Event

	event.DeviceID, event.ControlID = testDeviceID, testControlID
	device.Operations.Read = queuedEvents([]goinput.Event{event})

	var output bytes.Buffer

	checkCause(t, readEvents(t.Context(), device, &output), goinput.ErrClosed)
	checkValue(t, output.Len() > testEmpty, true)
}

func TestReadEventsReportsWriteFailure(t *testing.T) {
	t.Parallel()

	device := captureTestDevice()

	var event goinput.Event

	device.Operations.Read = queuedEvents([]goinput.Event{event})
	checkCause(t, readEvents(t.Context(), device, failedWriter{}), goinput.ErrClosed)
}

func TestReadErrorTreatsCancellationAsCompletion(t *testing.T) {
	t.Parallel()
	checkValue(t, readError(context.Canceled), error(nil))
	checkValue(t, readError(context.DeadlineExceeded), error(nil))
	checkCause(t, readError(goinput.ErrDisconnected), goinput.ErrDisconnected)
}

func TestControlMetadataHandlesMissingDescriptors(t *testing.T) {
	t.Parallel()

	var control goinput.Control

	control.Usage, control.Unit = goinput.HID(
		goinput.PageGenericDesktop,
		testSingle,
	), goinput.UnitUnknown
	checkValue(t, controlUsage(&control), control.Usage)
	checkValue(t, controlUnit(&control), control.Unit)
	checkValue(t, controlUsage(nil), goinput.UsageUnknown)
	checkValue(t, controlUnit(nil), goinput.UnitUnknown)
}

func TestCloseHelpersConsumeCleanupErrors(t *testing.T) {
	t.Parallel()

	options := captureTestOptions()
	closeManager(t.Context(), options.logger, captureTestManager())
	closeDevice(t.Context(), options.logger, captureTestDevice())
}

func TestCaptureDevicePreservesOpenFailure(t *testing.T) {
	t.Parallel()

	manager := captureTestManager()

	manager.Operations.Open = func(context.Context, goinput.DeviceID) (goinput.Device, error) {
		return nil, goinput.ErrNotFound
	}
	checkCause(t, captureDevice(t.Context(), manager, captureTestOptions()), goinput.ErrNotFound)
}

func TestCaptureDeviceUsesOptionalDeadline(t *testing.T) {
	t.Parallel()

	options := captureTestOptions()
	checkCause(t, captureDevice(t.Context(), captureTestManager(), options), goinput.ErrClosed)

	options.duration = time.Millisecond
	checkCause(t, captureDevice(t.Context(), captureTestManager(), options), goinput.ErrClosed)
}

func TestDiscoverAndCapturePreservesDiscoveryDiagnostic(t *testing.T) {
	t.Parallel()

	options := captureTestOptions()

	options.args = nil

	manager := captureTestManager()

	manager.Operations.Devices = failedDiscovery
	checkValue(t, discoverAndCapture(t.Context(), manager, options), error(nil))

	options.args = []string{testDeviceID}
	checkCause(t, discoverAndCapture(t.Context(), manager, options), goinput.ErrClosed)
}

func TestDeviceControlsUsesOneSnapshot(t *testing.T) {
	t.Parallel()

	device := captureTestDevice()

	calls := testEmpty

	device.Operations.Capabilities = countedCapabilities(&calls)

	controls := deviceControls(device)

	checkValue(t, calls, testSingle)
	checkValue(t, controls[testControlID].ID, goinput.ControlID(testControlID))
}

func countedCapabilities(calls *int) func() goinput.Capabilities {
	return func() goinput.Capabilities {
		*calls++

		var control goinput.Control

		control.ID = testControlID

		var capabilities goinput.Capabilities

		capabilities.Controls = []goinput.Control{control}

		return capabilities
	}
}

func captureTestOptions() *captureOptions {
	return &captureOptions{
		output:   io.Discard,
		logger:   slog.New(slog.DiscardHandler),
		args:     []string{testDeviceID},
		cycles:   testSingle,
		managers: testSingle,
		duration: unlimitedTime,
	}
}

func captureTestSystem(t *testing.T) *goinput.System {
	t.Helper()

	system, err := goinput.NewSystem(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return system
}

func failedSystem() (*goinput.System, error) { return nil, errors.Join(goinput.ErrUnsupported) }

func captureTestDevice() *testDevice {
	view := new(testDevice)

	view.Operations.Info = emptyInfo
	view.Operations.Capabilities = emptyCapabilities
	view.Operations.Read = queuedEvents(nil)
	view.Operations.Close = failedClose
	view.Operations.Extension = unavailableExtension

	return view
}

func emptyInfo() goinput.DeviceInfo {
	var info goinput.DeviceInfo

	return info
}

func emptyCapabilities() goinput.Capabilities {
	var caps goinput.Capabilities

	return caps
}

func failedClose() error { return errors.Join(goinput.ErrClosed) }

func unavailableExtension(any) bool { return false }

func captureTestManager() *testManager {
	view := new(testManager)

	view.Operations.Devices = emptyDiscovery
	view.Operations.Open = func(context.Context, goinput.DeviceID) (goinput.Device, error) {
		return captureTestDevice(), nil
	}
	view.Operations.Close = failedClose

	return view
}

func emptyDiscovery(context.Context) ([]goinput.DeviceInfo, error) { return nil, nil }

func failedDiscovery(context.Context) ([]goinput.DeviceInfo, error) {
	return nil, errors.Join(goinput.ErrPermissionDenied)
}

func queuedEvents(events []goinput.Event) func(context.Context) (goinput.Event, error) {
	return func(context.Context) (goinput.Event, error) {
		if len(events) == testEmpty {
			var event goinput.Event

			return event, errors.Join(goinput.ErrClosed)
		}

		event := events[testEmpty]

		events = events[testSingle:]

		return event, nil
	}
}

func checkValue[Value comparable](t *testing.T, actual, expected Value) {
	t.Helper()

	if actual != expected {
		t.Fatalf("got %v, want %v", actual, expected)
	}
}

func checkCause(t *testing.T, actual, expected error) {
	t.Helper()

	if !errors.Is(actual, expected) {
		t.Fatalf("got %v, want cause %v", actual, expected)
	}
}

func TestCyclePreservesExtraManagerFailure(t *testing.T) {
	t.Parallel()

	options := captureTestOptions()

	options.managers = testMultiple

	var ctx context.Context

	checkCause(t, cycle(ctx, options, captureTestSystem(t)), goinput.ErrInvalidOptions)
}

func TestDiscoveryReportsOutputFailure(t *testing.T) {
	t.Parallel()

	options := captureTestOptions()

	options.output = failedWriter{}

	manager := captureTestManager()

	manager.Operations.Devices = func(context.Context) ([]goinput.DeviceInfo, error) {
		return []goinput.DeviceInfo{emptyInfo()}, nil
	}
	checkCause(t, discoverAndCapture(t.Context(), manager, options), goinput.ErrClosed)
}
