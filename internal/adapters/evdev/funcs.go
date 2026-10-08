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
	"sort"
	"strings"
	"syscall"
	"time"

	extension "github.com/gostafa/goinput/extensions/evdev"
	"github.com/gostafa/goinput/internal/domain"
	"github.com/gostafa/goinput/internal/ports"
	native "github.com/holoplot/go-evdev"
)

func newBackend(ctx context.Context, retrier ports.Retrier) (ports.Backend, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &backend{
		captures:  make(map[*capture]struct{}),
		retrier:   retrier,
		closeDone: make(chan struct{}),
	}, nil
}

func (b *backend) Discover(ctx context.Context) ([]domain.DeviceInfo, error) {
	if err := b.check(ctx); err != nil {
		return nil, err
	}
	var entries []os.DirEntry
	read := func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		entries, err = os.ReadDir(inputDirectory)
		return err
	}
	var err error
	if b.retrier != nil {
		err = b.retrier.Do(ctx, read, transient)
	} else {
		err = read(ctx)
	}
	if errors.Is(err, os.ErrNotExist) {
		return []domain.DeviceInfo{}, nil
	}
	if err != nil {
		return nil, operationError("discover", "", err)
	}
	infos := make([]domain.DeviceInfo, 0, len(entries))
	var diagnostics []error
	for _, entry := range entries {
		if err := b.check(ctx); err != nil {
			return infos, errors.Join(append(diagnostics, err)...)
		}
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "event") {
			continue
		}
		path := filepath.Join(inputDirectory, entry.Name())
		dev, openErr := native.OpenWithFlags(path, os.O_RDONLY)
		if openErr != nil {
			if errors.Is(openErr, os.ErrNotExist) || errors.Is(openErr, syscall.ENODEV) {
				continue
			}
			diagnostics = append(
				diagnostics,
				operationError("discover", domain.DeviceID(devicePrefix+path), openErr),
			)
			continue
		}
		info, _, _, describeErr := describe(dev)
		_ = dev.Close()
		if describeErr != nil {
			if errors.Is(describeErr, syscall.ENODEV) || errors.Is(describeErr, os.ErrNotExist) {
				continue
			}
			diagnostics = append(
				diagnostics,
				operationError("discover", domain.DeviceID(devicePrefix+path), describeErr),
			)
			continue
		}
		infos = append(infos, info)
	}
	if err := b.check(ctx); err != nil {
		diagnostics = append(diagnostics, err)
	}
	return infos, errors.Join(diagnostics...)
}

func (b *backend) Open(
	ctx context.Context,
	id domain.DeviceID,
	sink ports.EventSink,
) (ports.Capture, error) {
	if err := b.check(ctx); err != nil {
		return nil, err
	}
	path, valid := strings.CutPrefix(string(id), devicePrefix)
	if !valid || filepath.Dir(path) != inputDirectory ||
		!strings.HasPrefix(filepath.Base(path), "event") {
		return nil, operationError("open", id, domain.ErrNotFound)
	}
	dev, err := native.OpenWithFlags(path, os.O_RDONLY)
	if err != nil {
		return nil, operationError("open", id, err)
	}
	info, caps, details, err := describe(dev)
	if err != nil {
		_ = dev.Close()
		return nil, operationError("open", id, err)
	}
	c := &capture{
		owner:  b,
		device: dev,
		sink:   sink,
		info:   info,
		caps:   caps,
		native: details,
		controls: make(
			map[eventCode]domain.Control,
		),
		hats:       make(map[int]*hat),
		hatCodes:   make(map[eventCode]int),
		suppressed: make(map[eventCode]bool),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	c.prepare()
	// Metadata ioctls must precede NonBlock: upstream Fd() calls otherwise
	// restore blocking mode and prevent Close from interrupting a pending read.
	if err := dev.NonBlock(); err != nil {
		_ = dev.Close()
		return nil, operationError("open", id, err)
	}
	b.mu.Lock()
	if b.closed || ctx.Err() != nil {
		b.mu.Unlock()
		_ = dev.Close()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, operationError("open", id, domain.ErrClosed)
	}
	b.captures[c] = struct{}{}
	b.mu.Unlock()
	go c.read()
	return c, nil
}

func (b *backend) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		<-b.closeDone
		return b.closeErr
	}
	b.closed = true
	captures := make([]*capture, 0, len(b.captures))
	for c := range b.captures {
		captures = append(captures, c)
	}
	b.mu.Unlock()
	var errs []error
	for _, c := range captures {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	b.closeErr = errors.Join(errs...)
	close(b.closeDone)
	return b.closeErr
}

func (b *backend) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return operationError("device", "", domain.ErrClosed)
	}
	return nil
}

func (c *capture) Info() domain.DeviceInfo { return domain.CloneInfo(&c.info) }

func (c *capture) Capabilities() domain.Capabilities { return domain.CloneCapabilities(&c.caps) }

func (c *capture) Extension(target any) bool {
	switch target := target.(type) {
	case *extension.Info:
		if target == nil {
			return false
		}
		*target = c.NativeInfo()
		return true
	case *extension.Metadata:
		if target == nil {
			return false
		}
		*target = c
		return true
	default:
		return false
	}
}

func (c *capture) NativeInfo() extension.Info {
	info := c.native
	info.Properties = append([]uint16(nil), info.Properties...)
	info.Controls = append([]extension.NativeControl(nil), info.Controls...)
	info.Axes = make(map[uint16]extension.AxisInfo, len(c.native.Axes))
	for code, axis := range c.native.Axes {
		info.Axes[code] = axis
	}
	return info
}

func (c *capture) stopReader() {
	c.closeOnce.Do(func() {
		close(c.stop)
		c.closeErr = c.device.Close()
		if errors.Is(c.closeErr, os.ErrClosed) {
			c.closeErr = nil
		}
	})
}

func (c *capture) Close() error {
	c.stopReader()
	<-c.done
	return c.closeErr
}

func (c *capture) read() {
	defer func() {
		c.stopReader()
		c.owner.mu.Lock()
		delete(c.owner.captures, c)
		c.owner.mu.Unlock()
		close(c.done)
	}()
	for {
		event, err := c.device.ReadOne()
		if err != nil {
			select {
			case <-c.stop:
				return
			default:
			}
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			c.sink.Fail(operationError("read", c.info.ID, err))
			return
		}
		select {
		case <-c.stop:
			return
		default:
		}
		if !c.dispatch(event) {
			return
		}
	}
}

func (c *capture) dispatch(event *native.InputEvent) bool {
	received := time.Now()
	timestamp := domain.Timestamp{
		Time:       time.Unix(int64(event.Time.Sec), int64(event.Time.Usec)*1000).UTC(),
		ReceivedAt: received,
		Source:     domain.TimestampNative,
	}
	if event.Type == native.EV_SYN {
		switch event.Code {
		case native.SYN_DROPPED:
			c.sink.Fail(operationError("read", c.info.ID, domain.ErrEventLoss))
			return false
		case native.SYN_REPORT:
			for index := 0; index < 4; index++ {
				hat := c.hats[index]
				if hat == nil || !hat.dirty {
					continue
				}
				hat.dirty = false
				if hat.x < -1 || hat.x > 1 || hat.y < -1 || hat.y > 1 {
					c.sink.Fail(operationError("read", c.info.ID, domain.ErrEventLoss))
					return false
				}
				value := hatValue(hat.x, hat.y)
				if value == hat.last {
					continue
				}
				hat.last = value
				if !c.sink.Publish(
					&domain.Event{
						DeviceID:  c.info.ID,
						ControlID: hat.id,
						Action:    domain.ActionChange,
						Value:     float64(value),
						Timestamp: timestamp,
					},
				) {
					return false
				}
			}
		}
		return true
	}
	code := eventCode{event.Type, event.Code}
	if c.suppressed[code] {
		return true
	}
	if index, ok := c.hatCodes[code]; ok {
		hat := c.hats[index]
		if (int(event.Code)-int(native.ABS_HAT0X))%2 == 0 {
			hat.x = event.Value
		} else {
			hat.y = event.Value
		}
		hat.dirty = true
		return true
	}
	control, ok := c.controls[code]
	if !ok || control.Support != domain.SupportSupported {
		return true
	}
	value := float64(event.Value)
	action := domain.ActionChange
	if event.Type == native.EV_KEY {
		switch event.Value {
		case 0:
			action = domain.ActionRelease
		case 1:
			action = domain.ActionPress
		case 2:
			action = domain.ActionRepeat
			value = 1
		default:
			return true
		}
	}
	if event.Type == native.EV_REL &&
		(event.Code == native.REL_WHEEL_HI_RES || event.Code == native.REL_HWHEEL_HI_RES) {
		value /= wheelDetent
	}
	return c.sink.Publish(
		&domain.Event{
			DeviceID:  c.info.ID,
			ControlID: control.ID,
			Action:    action,
			Value:     value,
			Timestamp: timestamp,
		},
	)
}

func (c *capture) prepare() {
	for _, code := range c.native.Controls {
		key := eventCode{native.EvType(code.Type), native.EvCode(code.Code)}
		for _, control := range c.caps.Controls {
			if string(control.ID) == code.ID {
				c.controls[key] = control
				break
			}
		}
	}
	for _, pair := range [][2]native.EvCode{{native.REL_WHEEL, native.REL_WHEEL_HI_RES}, {native.REL_HWHEEL, native.REL_HWHEEL_HI_RES}} {
		if _, ok := c.controls[eventCode{native.EV_REL, pair[1]}]; ok {
			id := c.controls[eventCode{native.EV_REL, pair[1]}].ID
			for i := range c.native.Controls {
				code := &c.native.Controls[i]
				if code.Type == uint16(native.EV_REL) && code.Code == uint16(pair[0]) {
					code.ID = string(id)
				}
			}
			c.suppressed[eventCode{native.EV_REL, pair[0]}] = true
			delete(c.controls, eventCode{native.EV_REL, pair[0]})
		}
	}
	for index := 0; index < 4; index++ {
		xCode := uint16(native.ABS_HAT0X) + uint16(index*2)
		yCode := xCode + 1
		x, hasX := c.native.Axes[xCode]
		y, hasY := c.native.Axes[yCode]
		if !hasX || !hasY || x.Minimum != -1 || x.Maximum != 1 || y.Minimum != -1 ||
			y.Maximum != 1 {
			continue
		}
		id := domain.ControlID(fmt.Sprintf("evdev:hat:%d", index))
		c.hats[index] = &hat{id: id, x: x.Value, y: y.Value, last: hatValue(x.Value, y.Value)}
		c.hatCodes[eventCode{native.EV_ABS, native.EvCode(xCode)}] = index
		c.hatCodes[eventCode{native.EV_ABS, native.EvCode(yCode)}] = index
		delete(c.controls, eventCode{native.EV_ABS, native.EvCode(xCode)})
		delete(c.controls, eventCode{native.EV_ABS, native.EvCode(yCode)})
		for i := range c.native.Controls {
			code := &c.native.Controls[i]
			if code.Type == uint16(native.EV_ABS) && (code.Code == xCode || code.Code == yCode) {
				code.ID = string(id)
			}
		}
	}
	c.caps.Controls = c.caps.Controls[:0]
	for _, control := range c.controls {
		c.caps.Controls = append(c.caps.Controls, control)
	}
	for index, hat := range c.hats {
		c.caps.Controls = append(c.caps.Controls, domain.Control{
			ID:   hat.id,
			Name: fmt.Sprintf("Hat %d", index),
			Kind: domain.ControlHat,
			Usage: domain.HID(
				desktopPage,
				0x39,
			),
			Mapping: domain.MappingInferred,
			Mode:    domain.AxisAbsolute,
			Range: &domain.Range{
				Min: int64(domain.HatNeutral),
				Max: int64(domain.HatNorthWest),
			},
			Unit:    domain.UnitDirection,
			Support: domain.SupportSupported,
		})
	}
	sort.Slice(
		c.caps.Controls,
		func(i, j int) bool { return c.caps.Controls[i].ID < c.caps.Controls[j].ID },
	)
}

func hatValue(x, y int32) int64 {
	if x < -1 || x > 1 || y < -1 || y > 1 {
		return int64(domain.HatNeutral)
	}
	directions := [3][3]domain.HatDirection{
		{domain.HatNorthWest, domain.HatWest, domain.HatSouthWest},
		{domain.HatNorth, domain.HatNeutral, domain.HatSouth},
		{domain.HatNorthEast, domain.HatEast, domain.HatSouthEast},
	}
	return int64(directions[x+1][y+1])
}

func describe(
	dev *native.InputDevice,
) (domain.DeviceInfo, domain.Capabilities, extension.Info, error) {
	path := dev.Path()
	info := domain.DeviceInfo{
		Transport: domain.TransportUnknown,
		ID:        domain.DeviceID(devicePrefix + path),
		Path:      path,
	}
	name, err := dev.Name()
	if err != nil {
		return info, domain.Capabilities{}, extension.Info{}, err
	}
	info.Name = name
	id, err := dev.InputID()
	if err != nil {
		return info, domain.Capabilities{}, extension.Info{}, err
	}
	info.VendorID, info.ProductID = &id.Vendor, &id.Product
	info.Transport = transport(id.BusType)
	info.Serial, _ = dev.UniqueID()
	details := extension.Info{
		BusType: id.BusType,
		Version: id.Version,
		Axes:    make(map[uint16]extension.AxisInfo),
	}
	details.PhysicalLocation, _ = dev.PhysicalLocation()
	for _, property := range dev.Properties() {
		details.Properties = append(details.Properties, uint16(property))
	}
	abs, err := dev.AbsInfos()
	if err != nil {
		return info, domain.Capabilities{}, details, err
	}
	for code, axis := range abs {
		details.Axes[uint16(code)] = extension.AxisInfo{
			Value:      axis.Value,
			Minimum:    axis.Minimum,
			Maximum:    axis.Maximum,
			Fuzz:       axis.Fuzz,
			Flat:       axis.Flat,
			Resolution: axis.Resolution,
		}
	}
	// Upstream capability queries suppress errors, so completeness cannot be
	// established even when the returned schema appears complete.
	caps := domain.Capabilities{Complete: false, Repeat: domain.SupportUnknown}
	keys := make(map[native.EvCode]bool)
	for _, eventType := range dev.CapableTypes() {
		if eventType == native.EV_REP {
			caps.Repeat = domain.SupportSupported
		}
		if eventType != native.EV_KEY && eventType != native.EV_REL && eventType != native.EV_ABS &&
			eventType != native.EV_SW &&
			eventType != native.EV_MSC &&
			eventType != native.EV_FF_STATUS {
			continue
		}
		for _, code := range dev.CapableEvents(eventType) {
			if eventType == native.EV_KEY {
				keys[code] = true
			}
			control := controlFor(eventType, code, abs)
			caps.Controls = append(caps.Controls, control)
			details.Controls = append(
				details.Controls,
				extension.NativeControl{
					ID:   string(control.ID),
					Type: uint16(eventType),
					Code: uint16(code),
				},
			)
		}
	}
	if keys[native.KEY_A] || keys[native.KEY_ENTER] || keys[native.KEY_SPACE] {
		info.Classes = append(info.Classes, domain.ClassKeyboard)
	}
	if keys[native.BTN_LEFT] {
		info.Classes = append(info.Classes, domain.ClassMouse)
	}
	if keys[native.BTN_GAMEPAD] {
		info.Classes = append(info.Classes, domain.ClassGamepad)
	} else if keys[native.BTN_JOYSTICK] {
		info.Classes = append(info.Classes, domain.ClassJoystick)
	}
	if len(info.Classes) == 0 {
		info.Classes = []domain.DeviceClass{domain.ClassOther}
	}
	return info, caps, details, nil
}

func controlFor(
	eventType native.EvType,
	code native.EvCode,
	abs map[native.EvCode]native.AbsInfo,
) domain.Control {
	control := domain.Control{
		Kind:    domain.ControlUnknown,
		Mapping: domain.MappingUnknown,
		Mode:    domain.AxisUnknown,
		Unit:    domain.UnitUnknown,
		Usage:   domain.UsageUnknown,
		ID:      domain.ControlID(fmt.Sprintf("evdev:%d:%d", eventType, code)),
		Name:    native.CodeName(eventType, code),
		Support: domain.SupportSupported,
	}
	switch eventType {
	case native.EV_KEY:
		control.Kind, control.Unit, control.Range = domain.ControlKey, domain.UnitBoolean, &domain.Range{
			Min: 0,
			Max: 1,
		}
		if strings.HasPrefix(control.Name, "BTN_") {
			control.Kind = domain.ControlButton
		}
		control.Usage = keyUsage(code)
	case native.EV_REL:
		control.Kind, control.Mode, control.Unit = domain.ControlAxis, domain.AxisRelative, domain.UnitCounts
		control.Usage = axisUsage(code, true)
		if code == native.REL_WHEEL || code == native.REL_HWHEEL ||
			code == native.REL_WHEEL_HI_RES ||
			code == native.REL_HWHEEL_HI_RES {
			control.Unit = domain.UnitDetents
		}
	case native.EV_ABS:
		control.Kind, control.Mode, control.Unit = domain.ControlAxis, domain.AxisAbsolute, domain.UnitLogical
		control.Usage = axisUsage(code, false)
		if axis, ok := abs[code]; ok {
			control.Range = &domain.Range{Min: int64(axis.Minimum), Max: int64(axis.Maximum)}
		}
	case native.EV_SW:
		control.Kind, control.Unit, control.Range = domain.ControlSwitch, domain.UnitBoolean, &domain.Range{
			Min: 0,
			Max: 1,
		}
	default:
		control.Kind, control.Support = domain.ControlUnknown, domain.SupportUnsupported
	}
	if control.Usage != 0 {
		control.Mapping = domain.MappingInferred
	}
	return control
}

func transport(bus uint16) domain.Transport {
	switch bus {
	case 0x03:
		return domain.TransportUSB
	case 0x05:
		return domain.TransportBluetooth
	case 0x06:
		return domain.TransportVirtual
	case 0x11:
		return domain.TransportPS2
	case 0x18:
		return domain.TransportI2C
	default:
		return domain.TransportUnknown
	}
}

func transient(err error) bool {
	return errors.Is(err, syscall.EINTR) || errors.Is(err, syscall.EAGAIN)
}

func operationError(op string, id domain.DeviceID, err error) error {
	var kind error
	switch {
	case errors.Is(err, os.ErrPermission):
		kind = domain.ErrPermissionDenied
	case errors.Is(err, os.ErrNotExist):
		kind = domain.ErrNotFound
	case errors.Is(err, syscall.ENODEV), errors.Is(err, syscall.ENXIO), errors.Is(err, io.EOF):
		kind = domain.ErrDisconnected
	case errors.Is(err, os.ErrClosed):
		kind = domain.ErrClosed
	}
	if kind != nil {
		err = errors.Join(kind, err)
	}
	return &domain.OpError{Op: op, DeviceID: id, Err: err}
}

func axisUsage(code native.EvCode, relative bool) domain.Usage {
	if relative {
		switch code {
		case native.REL_X, native.REL_Y, native.REL_Z, native.REL_RX, native.REL_RY, native.REL_RZ:
			return domain.HID(desktopPage, 0x30+uint16(code))
		case native.REL_DIAL:
			return domain.HID(desktopPage, 0x37)
		case native.REL_WHEEL, native.REL_WHEEL_HI_RES:
			return domain.HID(desktopPage, 0x38)
		case native.REL_HWHEEL, native.REL_HWHEEL_HI_RES:
			return domain.HID(consumerPage, 0x238)
		}
		return 0
	}
	switch code {
	case native.ABS_X, native.ABS_Y, native.ABS_Z, native.ABS_RX, native.ABS_RY, native.ABS_RZ:
		return domain.HID(desktopPage, 0x30+uint16(code))
	case native.ABS_THROTTLE:
		return domain.HID(0x02, 0xbb)
	case native.ABS_RUDDER:
		return domain.HID(0x02, 0xba)
	case native.ABS_WHEEL:
		// Evdev merges Generic Desktop Wheel and simulation steering controls.
		// Prefer the common Wheel identity without claiming an original collection.
		return domain.HID(desktopPage, 0x38)
	case native.ABS_GAS:
		return domain.HID(0x02, 0xc4)
	case native.ABS_BRAKE:
		return domain.HID(0x02, 0xc5)
	}
	return 0
}

func keyUsage(code native.EvCode) domain.Usage {
	if usage, ok := keyboardUsages[code]; ok {
		return domain.HID(keyboardPage, usage)
	}
	if usage, ok := consumerUsages[code]; ok {
		return domain.HID(consumerPage, usage)
	}
	switch code {
	case native.KEY_POWER:
		return domain.HID(desktopPage, 0x81)
	case native.KEY_SLEEP:
		return domain.HID(desktopPage, 0x82)
	case native.KEY_WAKEUP:
		return domain.HID(desktopPage, 0x83)
	case native.BTN_DPAD_UP:
		return domain.HID(desktopPage, 0x90)
	case native.BTN_DPAD_DOWN:
		return domain.HID(desktopPage, 0x91)
	case native.BTN_DPAD_RIGHT:
		return domain.HID(desktopPage, 0x92)
	case native.BTN_DPAD_LEFT:
		return domain.HID(desktopPage, 0x93)
	case native.BTN_START:
		return domain.HID(desktopPage, 0x3d)
	case native.BTN_SELECT:
		return domain.HID(desktopPage, 0x3e)
	}
	for _, limits := range [][2]native.EvCode{{native.BTN_0, native.BTN_9}, {native.BTN_LEFT, native.BTN_TASK}, {native.BTN_TRIGGER, native.BTN_DEAD}, {native.BTN_SOUTH, native.BTN_THUMBR}} {
		if code >= limits[0] && code <= limits[1] {
			return domain.HID(buttonPage, uint16(code-limits[0])+1)
		}
	}
	return 0
}

// Factory returns the native backend constructor.
func Factory() ports.Factory { return newBackend }
