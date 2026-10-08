// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build linux && (amd64 || arm64)

package evdev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	extension "github.com/gostafa/goinput/extensions/evdev"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	captureview "github.com/gostafa/goinput/internal/ports/capture"
	native "github.com/holoplot/go-evdev"
)

// Factory returns the native backend constructor.
func Factory() ports.Factory {
	return func(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
		backend, err := newBackend(ctx, retrier, defaultEnvironment())
		if err != nil {
			return nil, errors.Join(err)
		}

		return backend, nil
	}
}

func defaultEnvironment() environment {
	return environment{
		readDirectory: os.ReadDir,
		openDevice: func(path string, flags int) (*device, error) {
			return openNativeDevice(path, flags, native.OpenWithFlags)
		},
	}
}

func openNativeDevice(path string, flags int, open nativeOpener) (*device, error) {
	endpoint, err := open(path, flags)

	result, wrapErr := wrapOpenedDevice(endpoint, err)
	if wrapErr != nil {
		return nil, errors.Join(wrapErr)
	}

	return result, nil
}

func wrapOpenedDevice(endpoint *native.InputDevice, err error) (*device, error) {
	if err != nil {
		return nil, fmt.Errorf("open native device: %w", err)
	}

	return wrapNativeDevice(endpoint), nil
}

func wrapNativeDevice(endpoint *native.InputDevice) *device {
	device := new(device)
	configureDeviceMetadata(device, endpoint)
	configureDeviceCapabilities(device, endpoint)

	device.nonBlock, device.readOne, device.release = endpoint.NonBlock, endpoint.ReadOne, endpoint.Close

	return device
}

func configureDeviceMetadata(device *device, endpoint *native.InputDevice) {
	device.path, device.name, device.inputID = endpoint.Path, endpoint.Name, endpoint.InputID
	device.uniqueID, device.physicalLocation = endpoint.UniqueID, endpoint.PhysicalLocation
}

func configureDeviceCapabilities(device *device, endpoint *native.InputDevice) {
	device.properties, device.absInfos = endpoint.Properties, endpoint.AbsInfos
	device.capableTypes, device.capableEvents = endpoint.CapableTypes, endpoint.CapableEvents
}

func newBackend(
	ctx context.Context,
	retrier ports.Retrier,
	environment environment,
) (*backendOperations, error) {
	err := ctx.Err()
	if err != nil {
		return nil, fmt.Errorf("new backend: %w", err)
	}

	state := new(backend)

	state.captures, state.retrier = make(map[*capture]struct{}), retrier
	state.closeDone, state.environment = make(chan struct{}), environment

	return backendView(state), nil
}

func backendView(state *backend) *backendOperations {
	view := new(backendOperations)

	view.Operations.Discover = func(ctx context.Context) ([]domain.DeviceInfo, error) {
		return backendDiscover(ctx, state)
	}
	view.Operations.Close = func() error { return backendShutdown(state) }
	configureBackendOpen(view, state)

	return view
}

func backendDiscover(ctx context.Context, state *backend) ([]domain.DeviceInfo, error) {
	err := backendCheck(ctx, state)
	if err != nil {
		return nil, errors.Join(err)
	}

	entries, readErr := backendReadEntries(ctx, state)
	if readErr != nil {
		return nil, errors.Join(operationError(discoverOperation, "", readErr))
	}

	result, err := backendDiscoverEntries(ctx, state, entries)

	return result, errors.Join(err)
}

func backendReadEntries(ctx context.Context, state *backend) ([]os.DirEntry, error) {
	var entries []os.DirEntry

	read := func(ctx context.Context) error { return backendReadDirectory(ctx, state, &entries) }
	err := backendRetry(ctx, state, read)

	if errors.Is(err, os.ErrNotExist) {
		return []os.DirEntry{}, nil
	}

	return entries, errors.Join(err)
}

func backendReadDirectory(ctx context.Context, state *backend, entries *[]os.DirEntry) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	*entries, err = state.environment.readDirectory(inputDirectory)

	return errors.Join(err)
}

func backendRetry(ctx context.Context, state *backend, read func(context.Context) error) error {
	if state.retrier != nil {
		return errors.Join(state.retrier.Do(ctx, read, transient))
	}

	return errors.Join(read(ctx))
}

func backendDiscoverEntries(ctx context.Context, state *backend,
	entries []os.DirEntry,
) ([]domain.DeviceInfo, error) {
	infos := make([]domain.DeviceInfo, nativeZero, len(entries))

	var diagnostics []error

	for idx := range entries {
		err := backendCheck(ctx, state)
		if err != nil {
			return infos, errors.Join(append(diagnostics, err)...)
		}

		backendAppendDiscovered(
			state,
			entries[idx],
			&discoveryResult{infos: &infos, diagnostics: &diagnostics},
		)
	}

	return infos, errors.Join(append(diagnostics, backendCheck(ctx, state))...)
}

func backendAppendDiscovered(state *backend, entry os.DirEntry, result *discoveryResult) {
	if entry.IsDir() || !strings.HasPrefix(entry.Name(), eventPrefix) {
		return
	}

	info, err := backendDiscoverDevice(state, filepath.Join(inputDirectory, entry.Name()))
	if disappeared(err) {
		return
	}

	if err != nil {
		*result.diagnostics = append(*result.diagnostics, err)

		return
	}

	*result.infos = append(*result.infos, info)
}

func backendDiscoverDevice(state *backend, path string) (domain.DeviceInfo, error) {
	endpoint, err := state.environment.openDevice(path, os.O_RDONLY)
	if err != nil {
		return domain.DeviceInfo{}, errors.Join(
			operationError(discoverOperation, deviceID(path), err),
		)
	}

	description, describeErr := describe(endpoint)

	err = errors.Join(describeErr, endpoint.release())
	if err != nil {
		return domain.DeviceInfo{}, errors.Join(
			operationError(discoverOperation, deviceID(path), err),
		)
	}

	return description.info, nil
}

func disappeared(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENODEV)
}
func deviceID(path string) domain.DeviceID { return domain.DeviceID(devicePrefix + path) }

func backendOpen(ctx context.Context, state *backend, request *openRequest) (*capture, error) {
	err := backendCheck(ctx, state)
	if err != nil {
		return nil, errors.Join(err)
	}

	path, valid := devicePath(request.id)
	if !valid {
		return nil, errors.Join(operationError(openOperation, request.id, domain.ErrNotFound))
	}

	result, err := backendOpenDevice(ctx, state, &deviceOpenRequest{request: request, path: path})
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func devicePath(id domain.DeviceID) (string, bool) {
	path, valid := strings.CutPrefix(string(id), devicePrefix)

	return path, valid && filepath.Dir(path) == inputDirectory &&
		strings.HasPrefix(filepath.Base(path), eventPrefix)
}

func backendOpenDevice(
	ctx context.Context,
	state *backend,
	open *deviceOpenRequest,
) (*capture, error) {
	endpoint, err := state.environment.openDevice(open.path, os.O_RDONLY)
	if err != nil {
		return nil, errors.Join(operationError(openOperation, open.request.id, err))
	}

	capture, setupErr := backendPrepareCapture(state, endpoint, open.request.sink)
	if setupErr != nil {
		return nil, errors.Join(closeOpenFailure(endpoint, open.request.id, setupErr))
	}

	result, err := backendStartCapture(ctx, state, capture)
	if err != nil {
		return nil, errors.Join(err)
	}

	return result, nil
}

func closeOpenFailure(endpoint *device, id domain.DeviceID, err error) error {
	return errors.Join(operationError(openOperation, id, errors.Join(err, endpoint.release())))
}

func backendPrepareCapture(
	state *backend,
	endpoint *device,
	sink ports.EventSink,
) (*capture, error) {
	description, err := describe(endpoint)
	if err != nil {
		return nil, errors.Join(err)
	}

	capture := newCapture(state, endpoint, sink)

	configureCapture(capture, description)

	// Fd() restores blocking mode; issue all metadata ioctls before NonBlock.
	err = endpoint.nonBlock()
	if err != nil {
		return nil, errors.Join(err)
	}

	return capture, nil
}

func newCapture(owner *backend, endpoint *device, sink ports.EventSink) *capture {
	capture := new(capture)

	capture.device, capture.sink = endpoint, sink
	capture.unregister = func() { backendUnregister(owner, capture) }
	capture.controls, capture.hats = make(map[eventCode]domain.Control), make(map[int]*hat)
	capture.hatCodes, capture.suppressed = make(map[eventCode]int), make(map[eventCode]bool)
	capture.stop, capture.done = make(chan struct{}), make(chan struct{})

	return capture
}

func backendStartCapture(ctx context.Context, state *backend, capture *capture) (*capture, error) {
	err := backendRegisterCapture(ctx, state, capture)
	if err != nil {
		return nil, errors.Join(closeOpenFailure(capture.device, capture.info.ID, err))
	}

	go captureRead(capture)

	return capture, nil
}

func backendRegisterCapture(ctx context.Context, state *backend, capture *capture) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("register capture: %w", err)
	}

	if state.closed {
		return errors.Join(operationError(openOperation, capture.info.ID, domain.ErrClosed))
	}

	state.captures[capture] = struct{}{}

	return nil
}

func backendShutdown(state *backend) error {
	captures, started := backendBeginClose(state)
	if !started {
		<-state.closeDone

		return errors.Join(state.closeErr)
	}

	state.closeErr = closeCaptures(captures)
	close(state.closeDone)

	return errors.Join(state.closeErr)
}

func backendBeginClose(state *backend) ([]*capture, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.closed {
		return nil, false
	}

	state.closed = true

	captures := make([]*capture, nativeZero, len(state.captures))

	for capture := range state.captures {
		captures = append(captures, capture)
	}

	return captures, true
}

func closeCaptures(captures []*capture) error {
	errs := make([]error, nativeZero, len(captures))

	for idx := range captures {
		errs = append(errs, captureClose(captures[idx]))
	}

	return errors.Join(errs...)
}

func backendCheck(ctx context.Context, state *backend) error {
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("check backend: %w", err)
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.closed {
		return errors.Join(operationError(deviceOperation, "", domain.ErrClosed))
	}

	return nil
}

func captureInfo(capture *capture) domain.DeviceInfo { return domain.CloneInfo(&capture.info) }
func captureCapabilities(capture *capture) domain.Capabilities {
	return domain.CloneCapabilities(&capture.caps)
}

func captureExtension(capture *capture, target any) bool {
	switch target := target.(type) {
	case *extension.Info:
		return captureCopyExtensionInfo(capture, target)
	case *extension.Metadata:
		if target == nil {
			return false
		}

		*target = metadataView[extension.Info](
			func() extension.Info { return captureNativeInfo(capture) },
		)

		return true
	default:
		return false
	}
}

func captureCopyExtensionInfo(capture *capture, target *extension.Info) bool {
	if target == nil {
		return false
	}

	*target = captureNativeInfo(capture)

	return true
}

func captureNativeInfo(capture *capture) extension.Info {
	info := capture.native

	info.Properties = slices.Clone(info.Properties)
	info.Controls = slices.Clone(info.Controls)
	info.Axes = cloneAxes(capture.native.Axes)

	return info
}

func cloneAxes(axes map[uint16]extension.AxisInfo) map[uint16]extension.AxisInfo {
	cloned := make(map[uint16]extension.AxisInfo, len(axes))
	for code := range axes {
		cloned[code] = axes[code]
	}

	return cloned
}

func captureStopReader(capture *capture) {
	capture.closeOnce.Do(func() {
		close(capture.stop)

		capture.closeErr = capture.device.release()
		if errors.Is(capture.closeErr, os.ErrClosed) {
			capture.closeErr = nil
		}
	})
}

func captureClose(capture *capture) error {
	captureStopReader(capture)
	<-capture.done

	return errors.Join(capture.closeErr)
}

func captureFinishReader(capture *capture) {
	captureStopReader(capture)
	capture.unregister()
	close(capture.done)
}

func captureRead(capture *capture) {
	defer captureFinishReader(capture)

	for captureReadNext(capture) {
	}
}

func captureStopped(capture *capture) bool {
	select {
	case <-capture.stop:
		return true
	default:
		return false
	}
}

func captureReadNext(capture *capture) bool {
	event, err := capture.device.readOne()
	if captureStopped(capture) {
		return false
	}

	if err != nil {
		return captureReadFailure(capture, err)
	}

	return captureDispatch(capture, event)
}

func captureReadFailure(capture *capture, err error) bool {
	if errors.Is(err, syscall.EINTR) {
		return true
	}

	capture.sink.Fail(operationError(readOperation, capture.info.ID, err))

	return false
}

func eventTimestamp(event *native.InputEvent) domain.Timestamp {
	return domain.Timestamp{
		Time:       time.Unix(event.Time.Sec, event.Time.Usec*int64(time.Microsecond)).UTC(),
		ReceivedAt: time.Now(), Source: domain.TimestampNative,
	}
}

func captureDispatch(capture *capture, event *native.InputEvent) bool {
	timestamp := eventTimestamp(event)
	if event.Type == native.EV_SYN {
		return captureDispatchSync(capture, event.Code, &timestamp)
	}

	code := eventCode{typeCode: event.Type, code: event.Code}
	if capture.suppressed[code] {
		return true
	}

	if captureUpdateHat(capture, &code, event.Value) {
		return true
	}

	return captureDispatchControl(capture, event, &timestamp)
}

func captureDispatchSync(capture *capture, code native.EvCode, timestamp *domain.Timestamp) bool {
	switch code {
	case native.SYN_DROPPED:
		return captureEventLoss(capture)
	case native.SYN_REPORT:
		return capturePublishHats(capture, timestamp)
	default:
		return true
	}
}

func captureEventLoss(capture *capture) bool {
	capture.sink.Fail(operationError(readOperation, capture.info.ID, domain.ErrEventLoss))

	return false
}

func capturePublishHats(capture *capture, timestamp *domain.Timestamp) bool {
	for idx := range nativeFour {
		if !capturePublishHat(capture, capture.hats[idx], timestamp) {
			return false
		}
	}

	return true
}

func capturePublishHat(capture *capture, hat *hat, timestamp *domain.Timestamp) bool {
	if hat == nil || !hat.dirty {
		return true
	}

	hat.dirty = false
	if !validHat(hat.x, hat.y) {
		return captureEventLoss(capture)
	}

	return capturePublishHatValue(capture, hat, timestamp)
}

func capturePublishHatValue(capture *capture, hat *hat, timestamp *domain.Timestamp) bool {
	value := hatValue(hat.x, hat.y)
	if value == hat.last {
		return true
	}

	hat.last = value

	return capturePublish(
		capture,
		hat.id,
		&domain.Event{
			DeviceID:  "",
			ControlID: "",
			Value:     float64(value),
			Action:    domain.ActionChange,
			Timestamp: *timestamp,
		},
	)
}

func captureUpdateHat(capture *capture, code *eventCode, value int32) bool {
	idx, ok := capture.hatCodes[*code]
	if !ok {
		return false
	}

	hat := capture.hats[idx]
	setHatAxis(hat, code.code, value)

	hat.dirty = true

	return true
}

func setHatAxis(hat *hat, code native.EvCode, value int32) {
	if (code-native.ABS_HAT0X)%nativeTwo == nativeZero {
		hat.x = value

		return
	}

	hat.y = value
}

func captureDispatchControl(capture *capture,
	event *native.InputEvent,
	timestamp *domain.Timestamp,
) bool {
	control, ok := capture.controls[eventCode{typeCode: event.Type, code: event.Code}]
	if !ok || control.Support != domain.SupportSupported {
		return true
	}

	value := normalizedEvent(event, timestamp)
	if value.Action == domain.ActionUnknown {
		return true
	}

	return capturePublish(capture, control.ID, value)
}

func normalizedEvent(event *native.InputEvent, timestamp *domain.Timestamp) *domain.Event {
	value := new(domain.Event)

	value.Value, value.Action, value.Timestamp = float64(
		event.Value,
	), domain.ActionChange, *timestamp

	if event.Type == native.EV_KEY {
		value.Action, value.Value = keyAction(event.Value)
	}

	if highResolutionWheel(event.Type, event.Code) {
		value.Value /= wheelDetent
	}

	return value
}

func keyAction(value int32) (action domain.EventAction, normalized float64) {
	actions := map[int32]domain.EventAction{
		nativeZero: domain.ActionRelease,
		nativeOne:  domain.ActionPress,
		nativeTwo:  domain.ActionRepeat,
	}
	if value == nativeTwo {
		return actions[value], nativeOne
	}

	var ok bool

	action, ok = actions[value]

	if !ok {
		return domain.ActionUnknown, float64(value)
	}

	return action, float64(value)
}

func highResolutionWheel(eventType native.EvType, code native.EvCode) bool {
	return eventType == native.EV_REL &&
		(code == native.REL_WHEEL_HI_RES || code == native.REL_HWHEEL_HI_RES)
}

func capturePublish(capture *capture, id domain.ControlID, event *domain.Event) bool {
	event.DeviceID, event.ControlID = capture.info.ID, id

	return capture.sink.Publish(event)
}

func capturePrepare(capture *capture) {
	capturePrepareControls(capture)
	capturePrepareWheels(capture)
	capturePrepareHats(capture)
	captureRebuildCapabilities(capture)
}

func capturePrepareControls(capture *capture) {
	for idx := range capture.native.Controls {
		capturePrepareControl(capture, &capture.native.Controls[idx])
	}
}

func capturePrepareControl(capture *capture, code *extension.NativeControl) {
	for idx := range capture.caps.Controls {
		control := &capture.caps.Controls[idx]
		if string(control.ID) == code.ID {
			capture.controls[eventCode{typeCode: native.EvType(code.Type), code: native.EvCode(code.Code)}] = *control

			return
		}
	}
}

func wheelPairs() []wheelPair {
	return []wheelPair{
		{low: native.REL_WHEEL, high: native.REL_WHEEL_HI_RES},
		{low: native.REL_HWHEEL, high: native.REL_HWHEEL_HI_RES},
	}
}

func capturePrepareWheels(capture *capture) {
	pairs := wheelPairs()
	for idx := range pairs {
		capturePrepareWheel(capture, &pairs[idx])
	}
}

func capturePrepareWheel(capture *capture, pair *wheelPair) {
	high := eventCode{typeCode: native.EV_REL, code: pair.high}
	control, ok := capture.controls[high]

	if !ok {
		return
	}

	low := eventCode{typeCode: native.EV_REL, code: pair.low}
	captureRenameNativeControl(capture, &low, control.ID)

	capture.suppressed[low] = true
	delete(capture.controls, low)
}

func captureRenameNativeControl(capture *capture, key *eventCode, id domain.ControlID) {
	for idx := range capture.native.Controls {
		code := &capture.native.Controls[idx]
		if native.EvType(code.Type) == key.typeCode && native.EvCode(code.Code) == key.code {
			code.ID = string(id)
		}
	}
}

func capturePrepareHats(capture *capture) {
	for idx := range nativeFour {
		capturePrepareHat(capture, idx)
	}
}

func capturePrepareHat(capture *capture, idx int) {
	xCode := hatAxisCode(idx)
	yCode := xCode + nativeOne
	xAxis, hasX := capture.native.Axes[uint16(xCode)]
	yAxis, hasY := capture.native.Axes[uint16(yCode)]

	if !hasX || !hasY || !hatAxis(&xAxis) || !hatAxis(&yAxis) {
		return
	}

	capture.hats[idx] = newHat(idx, &xAxis, &yAxis)
	captureRegisterHatAxis(capture, idx, xCode)
	captureRegisterHatAxis(capture, idx, yCode)
}

func hatAxis(axis *extension.AxisInfo) bool {
	return axis.Minimum == hatMinimum && axis.Maximum == nativeOne
}

func captureRegisterHatAxis(capture *capture, idx int, code native.EvCode) {
	key := eventCode{typeCode: native.EV_ABS, code: code}

	capture.hatCodes[key] = idx
	delete(capture.controls, key)
	captureRenameNativeControl(capture, &key, capture.hats[idx].id)
}

func captureRebuildCapabilities(capture *capture) {
	capture.caps.Controls = capture.caps.Controls[:nativeZero]
	for key := range capture.controls {
		capture.caps.Controls = append(capture.caps.Controls, capture.controls[key])
	}

	for idx := range capture.hats {
		capture.caps.Controls = append(capture.caps.Controls, hatControl(idx, capture.hats[idx].id))
	}

	sortControls(capture.caps.Controls)
}

func sortControls(controls []domain.Control) {
	ordered := make([]*domain.Control, nativeZero, len(controls))
	for idx := range controls {
		ordered = append(ordered, &controls[idx])
	}

	slices.SortFunc(ordered, compareControlPointers)

	copied := make([]domain.Control, nativeZero, len(controls))

	for idx := range ordered {
		copied = append(copied, *ordered[idx])
	}

	copy(controls, copied)
}

func compareControlPointers(left, right *domain.Control) int {
	return strings.Compare(string(left.ID), string(right.ID))
}

func hatControl(idx int, id domain.ControlID) domain.Control {
	control := new(domain.Control)

	control.ID, control.Name, control.Kind = id, fmt.Sprintf("Hat %d", idx), domain.ControlHat
	control.Usage, control.Mapping, control.Mode = domain.HID(
		nativeOne,
		hatUsage,
	), domain.MappingInferred, domain.AxisAbsolute
	control.Range = &domain.Range{Min: int64(domain.HatNeutral), Max: int64(domain.HatNorthWest)}
	control.Unit, control.Support = domain.UnitDirection, domain.SupportSupported

	return *control
}

func validHat(horizontal, vertical int32) bool {
	return horizontal >= hatMinimum && horizontal <= nativeOne && vertical >= hatMinimum &&
		vertical <= nativeOne
}

func hatValue(horizontal, vertical int32) int64 {
	if !validHat(horizontal, vertical) {
		return int64(domain.HatNeutral)
	}

	directions := [nativeThree][nativeThree]domain.HatDirection{
		{domain.HatNorthWest, domain.HatWest, domain.HatSouthWest},
		{domain.HatNorth, domain.HatNeutral, domain.HatSouth},
		{domain.HatNorthEast, domain.HatEast, domain.HatSouthEast},
	}

	return int64(directions[horizontal+nativeOne][vertical+nativeOne])
}

func describe(endpoint *device) (*description, error) {
	description := new(description)

	err := describeIdentity(endpoint, description)
	if err != nil {
		return nil, errors.Join(err)
	}

	err = describeAxes(endpoint, description)
	if err != nil {
		return nil, errors.Join(err)
	}

	describeCapabilities(endpoint, description)

	return description, nil
}

func describeIdentity(endpoint *device, description *description) error {
	setDevicePath(description, endpoint.path())

	name, err := endpoint.name()
	if err != nil {
		return fmt.Errorf("device name: %w", err)
	}

	description.info.Name = name

	id, idErr := endpoint.inputID()
	if idErr != nil {
		return fmt.Errorf("device identity: %w", idErr)
	}

	setIdentity(description, &id)

	return errors.Join(describeOptionalIdentity(endpoint, description))
}

func setIdentity(description *description, id *native.InputID) {
	description.info.VendorID, description.info.ProductID = &id.Vendor, &id.Product
	description.info.Transport = transport(id.BusType)
	description.native.BusType, description.native.Version = id.BusType, id.Version
}

func describeOptionalIdentity(endpoint *device, description *description) error {
	serial, err := optionalString(endpoint.uniqueID)
	if err != nil {
		return errors.Join(err)
	}

	description.info.Serial = serial

	location, locationErr := optionalString(endpoint.physicalLocation)

	description.native.PhysicalLocation = location

	return errors.Join(locationErr)
}

func optionalString(query func() (string, error)) (string, error) {
	value, err := query()
	if optionalIdentityMissing(err) {
		return "", nil
	}

	return value, errors.Join(err)
}

func optionalIdentityMissing(err error) bool {
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOTTY) ||
		errors.Is(err, syscall.EINVAL)
}

func describeAxes(endpoint *device, description *description) error {
	setProperties(description, endpoint.properties())

	axes, err := endpoint.absInfos()
	if err != nil {
		return fmt.Errorf("device axes: %w", err)
	}

	description.native.Axes = make(map[uint16]extension.AxisInfo, len(axes))
	for code := range axes {
		axis := axes[code]

		description.native.Axes[uint16(code)] = axisInfo(&axis)
	}

	return nil
}

func axisInfo(axis *native.AbsInfo) extension.AxisInfo {
	return extension.AxisInfo{
		Value: axis.Value, Minimum: axis.Minimum, Maximum: axis.Maximum,
		Fuzz: axis.Fuzz, Flat: axis.Flat, Resolution: axis.Resolution,
	}
}

func describeCapabilities(endpoint *device, description *description) {
	// Upstream capability queries suppress errors, so completeness remains unknown.
	description.caps.Repeat = domain.SupportUnknown

	keys := make(map[native.EvCode]bool)
	types := endpoint.capableTypes()

	for idx := range types {
		describeEventType(
			endpoint,
			description,
			&eventTypeRequest{eventType: types[idx], keys: keys},
		)
	}

	description.info.Classes = deviceClasses(keys)
}

func describeEventType(
	endpoint *device,
	description *description,
	request *eventTypeRequest,
) {
	if request.eventType == native.EV_REP {
		description.caps.Repeat = domain.SupportSupported
	}

	if !supportedEventType(request.eventType) {
		return
	}

	codes := endpoint.capableEvents(request.eventType)
	for idx := range codes {
		if request.eventType == native.EV_KEY {
			request.keys[codes[idx]] = true
		}

		appendControl(description, request.eventType, codes[idx])
	}
}

func supportedEventType(eventType native.EvType) bool {
	return slices.Contains([]native.EvType{
		native.EV_KEY, native.EV_REL, native.EV_ABS,
		native.EV_SW, native.EV_MSC, native.EV_FF_STATUS,
	}, eventType)
}

func appendControl(description *description, eventType native.EvType, code native.EvCode) {
	axes := nativeAxes(description.native.Axes)
	control := controlFor(&controlRequest{eventType: eventType, code: code, axes: axes})

	description.caps.Controls = append(description.caps.Controls, *control)
	description.native.Controls = append(
		description.native.Controls,
		extension.NativeControl{
			ID:   string(control.ID),
			Type: uint16(eventType),
			Code: uint16(code),
		},
	)
}

func nativeAxes(axes map[uint16]extension.AxisInfo) map[native.EvCode]native.AbsInfo {
	result := make(map[native.EvCode]native.AbsInfo, len(axes))
	for code := range axes {
		axis := axes[code]

		result[native.EvCode(code)] = native.AbsInfo{
			Value: axis.Value, Minimum: axis.Minimum, Maximum: axis.Maximum,
			Fuzz: axis.Fuzz, Flat: axis.Flat, Resolution: axis.Resolution,
		}
	}

	return result
}

func deviceClasses(keys map[native.EvCode]bool) []domain.DeviceClass {
	var classes []domain.DeviceClass

	if keyboardClass(keys) {
		classes = append(classes, domain.ClassKeyboard)
	}

	if keys[native.BTN_LEFT] {
		classes = append(classes, domain.ClassMouse)
	}

	classes = appendGameClass(classes, keys)
	if len(classes) == nativeZero {
		return []domain.DeviceClass{domain.ClassOther}
	}

	return classes
}

func appendGameClass(
	classes []domain.DeviceClass,
	keys map[native.EvCode]bool,
) []domain.DeviceClass {
	if keys[native.BTN_GAMEPAD] {
		return append(classes, domain.ClassGamepad)
	}

	if keys[native.BTN_JOYSTICK] {
		return append(classes, domain.ClassJoystick)
	}

	return classes
}

func controlFor(request *controlRequest) *domain.Control {
	control := unknownControl()

	control.ID = domain.ControlID(fmt.Sprintf("evdev:%d:%d", request.eventType, request.code))
	control.Name, control.Support = native.CodeName(
		request.eventType,
		request.code,
	), domain.SupportSupported
	configureControl(control, request)

	if control.Usage != domain.UsageUnknown {
		control.Mapping = domain.MappingInferred
	}

	return control
}

func configureControl(control *domain.Control, request *controlRequest) {
	configure := map[native.EvType]func(*domain.Control, *controlRequest){
		native.EV_KEY: configureKey, native.EV_REL: configureRelative,
		native.EV_ABS: configureAbsolute, native.EV_SW: configureSwitch,
	}
	function, ok := configure[request.eventType]

	if !ok {
		control.Support = domain.SupportUnsupported

		return
	}

	function(control, request)
}

func configureKey(control *domain.Control, request *controlRequest) {
	control.Kind, control.Unit, control.Range = domain.ControlKey, domain.UnitBoolean, booleanRange()
	if strings.HasPrefix(control.Name, "BTN_") {
		control.Kind = domain.ControlButton
	}

	control.Usage = keyUsage(request.code)
}

func booleanRange() *domain.Range { return &domain.Range{Min: nativeZero, Max: nativeOne} }

func configureRelative(control *domain.Control, request *controlRequest) {
	control.Kind, control.Mode, control.Unit = domain.ControlAxis, domain.AxisRelative, domain.UnitCounts
	control.Usage = relativeAxisUsage(request.code)

	if isWheel(request.code) {
		control.Unit = domain.UnitDetents
	}
}

func isWheel(code native.EvCode) bool {
	return slices.Contains([]native.EvCode{
		native.REL_WHEEL, native.REL_HWHEEL,
		native.REL_WHEEL_HI_RES, native.REL_HWHEEL_HI_RES,
	}, code)
}

func configureAbsolute(control *domain.Control, request *controlRequest) {
	control.Kind, control.Mode, control.Unit = domain.ControlAxis, domain.AxisAbsolute, domain.UnitLogical
	control.Usage = absoluteAxisUsage(request.code)

	axis, ok := request.axes[request.code]

	if ok {
		control.Range = &domain.Range{Min: int64(axis.Minimum), Max: int64(axis.Maximum)}
	}
}

func configureSwitch(control *domain.Control, _ *controlRequest) {
	control.Kind, control.Unit, control.Range = domain.ControlSwitch, domain.UnitBoolean, booleanRange()
}

func transport(bus uint16) domain.Transport {
	transports := map[uint16]domain.Transport{
		nativeThree:      domain.TransportUSB,
		nativeFive:       domain.TransportBluetooth,
		nativeSix:        domain.TransportVirtual,
		nativeSeventeen:  domain.TransportPS2,
		nativeTwentyFour: domain.TransportI2C,
	}

	value, ok := transports[bus]
	if !ok {
		return domain.TransportUnknown
	}

	return value
}

func transient(err error) bool {
	return errors.Is(err, syscall.EINTR) || errors.Is(err, syscall.EAGAIN)
}

func operationError(operation string, id domain.DeviceID, err error) error {
	return &domain.OpError{Op: operation, DeviceID: id, Err: errors.Join(errorKind(err), err)}
}

func errorKind(err error) error {
	kinds := []error{
		domain.ErrPermissionDenied,
		domain.ErrNotFound,
		domain.ErrDisconnected,
		domain.ErrClosed,
	}
	conditions := []bool{
		errors.Is(err, os.ErrPermission),
		errors.Is(err, os.ErrNotExist),
		disconnected(err),
		errors.Is(err, os.ErrClosed),
	}

	for idx := range conditions {
		if conditions[idx] {
			return kinds[idx]
		}
	}

	return nil
}

func disconnected(err error) bool {
	return errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ENXIO) || errors.Is(err, io.EOF)
}

func relativeAxisUsage(code native.EvCode) domain.Usage {
	if code <= native.REL_RZ {
		return domain.HID(nativeOne, axisUsageBase+uint16(code))
	}

	return relativeUsage(code)
}

func absoluteAxisUsage(code native.EvCode) domain.Usage {
	if code <= native.ABS_RZ {
		return domain.HID(nativeOne, axisUsageBase+uint16(code))
	}

	return absoluteUsage(code)
}

func relativeUsage(code native.EvCode) domain.Usage {
	return usageFromTable(code, relativeUsageTable())
}

func relativeUsageTable() map[native.EvCode]domain.Usage {
	return map[native.EvCode]domain.Usage{
		native.REL_DIAL:          domain.HID(nativeOne, dialUsage),
		native.REL_WHEEL:         domain.HID(nativeOne, wheelUsage),
		native.REL_WHEEL_HI_RES:  domain.HID(nativeOne, wheelUsage),
		native.REL_HWHEEL:        domain.HID(nativeTwelve, horizontalWheelUsage),
		native.REL_HWHEEL_HI_RES: domain.HID(nativeTwelve, horizontalWheelUsage),
	}
}

func absoluteUsage(code native.EvCode) domain.Usage {
	if code == native.ABS_WHEEL {
		return domain.HID(nativeOne, wheelUsage)
	}

	return hidUsage(nativeTwo, map[native.EvCode]uint16{
		native.ABS_THROTTLE: throttleUsage, native.ABS_RUDDER: rudderUsage,
		native.ABS_GAS: gasUsage, native.ABS_BRAKE: brakeUsage,
	}[code])
}

func usageFromTable(code native.EvCode, table map[native.EvCode]domain.Usage) domain.Usage {
	return table[code]
}

func hidUsage(page, usage uint16) domain.Usage {
	if usage == nativeZero {
		return domain.UsageUnknown
	}

	return domain.HID(page, usage)
}

func keyUsage(code native.EvCode) domain.Usage {
	lookups := []usageLookup{keyboardUsage, consumerUsage, desktopKeyUsage, buttonUsage}
	for idx := range lookups {
		usage := lookups[idx](code)
		if usage != domain.UsageUnknown {
			return usage
		}
	}

	return domain.UsageUnknown
}

func desktopKeyUsage(code native.EvCode) domain.Usage {
	usages := map[native.EvCode]uint16{
		native.KEY_POWER: usage81, native.KEY_SLEEP: usage82, native.KEY_WAKEUP: usage83,
		native.BTN_DPAD_UP: usage90, native.BTN_DPAD_DOWN: usage91, native.BTN_DPAD_RIGHT: usage92,
		native.BTN_DPAD_LEFT: usage93, native.BTN_START: usage3D, native.BTN_SELECT: usage3E,
	}

	return hidUsage(nativeOne, usages[code])
}

func buttonRanges() []codeRange {
	return []codeRange{
		{first: native.BTN_0, last: native.BTN_9},
		{first: native.BTN_LEFT, last: native.BTN_TASK},
		{first: native.BTN_TRIGGER, last: native.BTN_DEAD},
		{first: native.BTN_SOUTH, last: native.BTN_THUMBR},
	}
}

func buttonUsage(code native.EvCode) domain.Usage {
	ranges := buttonRanges()
	for idx := range ranges {
		limits := &ranges[idx]
		if code >= limits.first && code <= limits.last {
			return domain.HID(nativeNine, uint16(code-limits.first)+nativeOne)
		}
	}

	return domain.UsageUnknown
}

func unknownControl() *domain.Control {
	control := new(domain.Control)

	control.Kind, control.Mapping, control.Mode = domain.ControlUnknown, domain.MappingUnknown, domain.AxisUnknown
	control.Unit, control.Usage = domain.UnitUnknown, domain.UsageUnknown

	return control
}

func captureView(state *capture) *captureview.Operations[domain.DeviceInfo, domain.Capabilities] {
	view := new(captureview.Operations[domain.DeviceInfo, domain.Capabilities])

	view.Operations.Info = func() domain.DeviceInfo { return captureInfo(state) }
	view.Operations.Capabilities = func() domain.Capabilities { return captureCapabilities(state) }
	view.Operations.Extension = func(target any) bool { return captureExtension(state, target) }
	view.Operations.Close = func() error { return captureClose(state) }

	return view
}

// NativeInfo returns a fresh snapshot of native metadata.
func (view metadataView[T]) NativeInfo() T { return view() }

func keyboardClass(keys map[native.EvCode]bool) bool {
	return keys[native.KEY_A] || keys[native.KEY_ENTER] || keys[native.KEY_SPACE]
}

func hatAxisCode(idx int) native.EvCode {
	codes := [nativeFour]native.EvCode{
		native.ABS_HAT0X,
		native.ABS_HAT1X,
		native.ABS_HAT2X,
		native.ABS_HAT3X,
	}

	return codes[idx]
}

func newHat(idx int, horizontal, vertical *extension.AxisInfo) *hat {
	hat := new(hat)

	hat.id, hat.x, hat.y = domain.ControlID(
		fmt.Sprintf("evdev:hat:%d", idx),
	), horizontal.Value, vertical.Value
	hat.last = hatValue(hat.x, hat.y)

	return hat
}

func setDevicePath(description *description, path string) {
	description.info.ID, description.info.Path, description.info.Transport = deviceID(
		path,
	), path, domain.TransportUnknown
}

func setProperties(description *description, properties []native.EvProp) {
	for idx := range properties {
		description.native.Properties = append(
			description.native.Properties,
			uint16(properties[idx]),
		)
	}
}

func backendUnregister(owner *backend, capture *capture) {
	owner.mu.Lock()
	delete(owner.captures, capture)
	owner.mu.Unlock()
}

func configureBackendOpen(view *backendOperations, state *backend) {
	view.Operations.Open = func(ctx context.Context, id domain.DeviceID, sink ports.EventSink) (ports.Capture, error) {
		capture, err := backendOpen(ctx, state, &openRequest{id: id, sink: sink})
		if err != nil {
			return nil, errors.Join(err)
		}

		return captureView(capture), nil
	}
}

func configureCapture(capture *capture, description *description) {
	capture.info, capture.caps, capture.native = description.info, description.caps, description.native
	capturePrepare(capture)
}
