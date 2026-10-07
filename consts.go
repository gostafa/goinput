// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package goinput

import (
	"github.com/gostafa/goinput/internal/domain"
)

const (
	// ClassUnknown is the class unknown value.
	// Enum values occupy distinct ranges; use the named constants, not numeric literals.
	// Device classes identify endpoint categories.
	ClassUnknown DeviceClass = DeviceClass(domain.ClassUnknown)
	// ClassKeyboard identifies class keyboard.
	ClassKeyboard DeviceClass = DeviceClass(domain.ClassKeyboard)

	// ClassMouse identifies class mouse.
	ClassMouse DeviceClass = DeviceClass(domain.ClassMouse)
	// ClassGamepad identifies class gamepad.
	ClassGamepad DeviceClass = DeviceClass(domain.ClassGamepad)

	// ClassJoystick identifies class joystick.
	ClassJoystick DeviceClass = DeviceClass(domain.ClassJoystick)
	// ClassOther identifies class other.
	ClassOther DeviceClass = DeviceClass(domain.ClassOther)

	// TransportUnknown is the transport unknown value.
	// Transports identify device connection media.
	TransportUnknown Transport = Transport(domain.TransportUnknown)
	// TransportUSB identifies transport usb.
	TransportUSB Transport = Transport(domain.TransportUSB)

	// TransportBluetooth identifies transport bluetooth.
	TransportBluetooth Transport = Transport(domain.TransportBluetooth)
	// TransportPS2 identifies transport ps2.
	TransportPS2 Transport = Transport(domain.TransportPS2)

	// TransportI2C identifies transport i2 c.
	TransportI2C Transport = Transport(domain.TransportI2C)
	// TransportVirtual identifies transport virtual.
	TransportVirtual Transport = Transport(domain.TransportVirtual)

	// ControlUnknown is the control unknown value.
	// Control kinds identify input semantics.
	ControlUnknown ControlKind = ControlKind(domain.ControlUnknown)
	// ControlKey identifies control key.
	ControlKey ControlKind = ControlKind(domain.ControlKey)

	// ControlButton identifies control button.
	ControlButton ControlKind = ControlKind(domain.ControlButton)
	// ControlAxis identifies control axis.
	ControlAxis ControlKind = ControlKind(domain.ControlAxis)

	// ControlHat identifies control hat.
	ControlHat ControlKind = ControlKind(domain.ControlHat)
	// ControlSwitch identifies control switch.
	ControlSwitch ControlKind = ControlKind(domain.ControlSwitch)

	// AxisUnknown is the axis unknown value.
	// Axis modes distinguish absolute positions and relative deltas.
	AxisUnknown AxisMode = AxisMode(domain.AxisUnknown)
	// AxisRelative identifies axis relative.
	AxisRelative AxisMode = AxisMode(domain.AxisRelative)

	// AxisAbsolute identifies axis absolute.
	AxisAbsolute AxisMode = AxisMode(domain.AxisAbsolute)

	// MappingUnknown is the mapping unknown value.
	// Mapping sources describe the origin of HID usages.
	MappingUnknown MappingSource = MappingSource(domain.MappingUnknown)
	// MappingReported identifies mapping reported.
	MappingReported MappingSource = MappingSource(domain.MappingReported)

	// MappingInferred identifies mapping inferred.
	MappingInferred MappingSource = MappingSource(domain.MappingInferred)

	// UnitUnknown is the unit unknown value.
	// Units describe control value interpretations.
	UnitUnknown Unit = Unit(domain.UnitUnknown)
	// UnitLogical identifies unit logical.
	UnitLogical Unit = Unit(domain.UnitLogical)

	// UnitCounts identifies unit counts.
	UnitCounts Unit = Unit(domain.UnitCounts)
	// UnitDetents identifies unit detents.
	UnitDetents Unit = Unit(domain.UnitDetents)

	// UnitDirection identifies unit direction.
	UnitDirection Unit = Unit(domain.UnitDirection)
	// UnitBoolean identifies unit boolean.
	UnitBoolean Unit = Unit(domain.UnitBoolean)

	// SupportUnknown is the support unknown value.
	// Support values describe capability availability.
	SupportUnknown Support = Support(domain.SupportUnknown)
	// SupportSupported identifies support supported.
	SupportSupported Support = Support(domain.SupportSupported)

	// SupportUnsupported identifies support unsupported.
	SupportUnsupported Support = Support(domain.SupportUnsupported)

	// ActionUnknown is the action unknown value.
	// Actions describe control transitions.
	ActionUnknown EventAction = EventAction(domain.ActionUnknown)
	// ActionChange identifies action change.
	ActionChange EventAction = EventAction(domain.ActionChange)

	// ActionPress identifies action press.
	ActionPress EventAction = EventAction(domain.ActionPress)
	// ActionRelease identifies action release.
	ActionRelease EventAction = EventAction(domain.ActionRelease)

	// ActionRepeat identifies action repeat.
	ActionRepeat EventAction = EventAction(domain.ActionRepeat)

	// TimestampUnknown is the timestamp unknown value.
	// Timestamp sources distinguish native and receipt clocks.
	TimestampUnknown TimestampSource = TimestampSource(domain.TimestampUnknown)
	// TimestampNative identifies timestamp native.
	TimestampNative TimestampSource = TimestampSource(domain.TimestampNative)

	// TimestampEstimated identifies timestamp estimated.
	TimestampEstimated TimestampSource = TimestampSource(domain.TimestampEstimated)
	// TimestampReceipt identifies timestamp receipt.
	TimestampReceipt TimestampSource = TimestampSource(domain.TimestampReceipt)

	// HatNeutral is the hat neutral value.
	// Hat directions describe compass positions.
	HatNeutral HatDirection = HatDirection(domain.HatNeutral)
	// HatNorth identifies hat north.
	HatNorth HatDirection = HatDirection(domain.HatNorth)

	// HatNorthEast identifies hat north east.
	HatNorthEast HatDirection = HatDirection(domain.HatNorthEast)
	// HatEast identifies hat east.
	HatEast HatDirection = HatDirection(domain.HatEast)

	// HatSouthEast identifies hat south east.
	HatSouthEast HatDirection = HatDirection(domain.HatSouthEast)
	// HatSouth identifies hat south.
	HatSouth HatDirection = HatDirection(domain.HatSouth)

	// HatSouthWest identifies hat south west.
	HatSouthWest HatDirection = HatDirection(domain.HatSouthWest)
	// HatWest identifies hat west.
	HatWest HatDirection = HatDirection(domain.HatWest)

	// HatNorthWest identifies hat north west.
	HatNorthWest HatDirection = HatDirection(domain.HatNorthWest)

	// PageGenericDesktop is the page generic desktop value.
	// HID usage pages used by the portable control vocabulary.
	// HID pages identify standard control namespaces.
	PageGenericDesktop uint16 = domain.PageGenericDesktop
	// PageSimulation identifies page simulation.
	PageSimulation uint16 = domain.PageSimulation
	// PageKeyboard identifies page keyboard.
	PageKeyboard uint16 = domain.PageKeyboard
	// PageButton identifies page button.
	PageButton uint16 = domain.PageButton
	// PageConsumer identifies page consumer.
	PageConsumer uint16 = domain.PageConsumer

	// UsageUnknown is the usage unknown value.
	// Usages identify standard HID controls.
	UsageUnknown Usage = Usage(domain.UsageUnknown)
	// AxisX identifies axis x.
	AxisX Usage = Usage(domain.AxisX)
	// AxisY identifies axis y.
	AxisY Usage = Usage(domain.AxisY)
	// AxisZ identifies axis z.
	AxisZ Usage = Usage(domain.AxisZ)
	// AxisRx identifies axis rx.
	AxisRx Usage = Usage(domain.AxisRx)
	// AxisRy identifies axis ry.
	AxisRy Usage = Usage(domain.AxisRy)
	// AxisRz identifies axis rz.
	AxisRz Usage = Usage(domain.AxisRz)
	// AxisSlider identifies axis slider.
	AxisSlider Usage = Usage(domain.AxisSlider)
	// AxisDial identifies axis dial.
	AxisDial Usage = Usage(domain.AxisDial)
	// AxisWheel identifies axis wheel.
	AxisWheel Usage = Usage(domain.AxisWheel)
	// HatSwitch identifies hat switch.
	HatSwitch Usage = Usage(domain.HatSwitch)
	// DPadUp identifies dpad up.
	DPadUp Usage = Usage(domain.DPadUp)
	// DPadDown identifies dpad down.
	DPadDown Usage = Usage(domain.DPadDown)
	// DPadRight identifies dpad right.
	DPadRight Usage = Usage(domain.DPadRight)
	// DPadLeft identifies dpad left.
	DPadLeft Usage = Usage(domain.DPadLeft)
	// AxisRudder identifies axis rudder.
	AxisRudder Usage = Usage(domain.AxisRudder)
	// AxisThrottle identifies axis throttle.
	AxisThrottle Usage = Usage(domain.AxisThrottle)
	// AxisGas identifies axis gas.
	AxisGas Usage = Usage(domain.AxisGas)
	// AxisBrake identifies axis brake.
	AxisBrake Usage = Usage(domain.AxisBrake)
	// AxisPan identifies axis pan.
	AxisPan Usage = Usage(domain.AxisPan)

	// Button1 is the button1 value.
	// Button ordinals do not promise physical positions on a game controller.
	// Button usages identify device-local button ordinals.
	Button1 Usage = Usage(domain.Button1)
	// Button2 identifies button2.
	Button2 Usage = Usage(domain.Button2)
	// Button3 identifies button3.
	Button3 Usage = Usage(domain.Button3)
	// Button4 identifies button4.
	Button4 Usage = Usage(domain.Button4)
	// Button5 identifies button5.
	Button5 Usage = Usage(domain.Button5)
	// Button6 identifies button6.
	Button6 Usage = Usage(domain.Button6)
	// Button7 identifies button7.
	Button7 Usage = Usage(domain.Button7)
	// Button8 identifies button8.
	Button8 Usage = Usage(domain.Button8)
	// Button9 identifies button9.
	Button9 Usage = Usage(domain.Button9)
	// Button10 identifies button10.
	Button10 Usage = Usage(domain.Button10)
	// Button11 identifies button11.
	Button11 Usage = Usage(domain.Button11)
	// Button12 identifies button12.
	Button12 Usage = Usage(domain.Button12)
	// Button13 identifies button13.
	Button13 Usage = Usage(domain.Button13)
	// Button14 identifies button14.
	Button14 Usage = Usage(domain.Button14)
	// Button15 identifies button15.
	Button15 Usage = Usage(domain.Button15)
	// Button16 identifies button16.
	Button16 Usage = Usage(domain.Button16)

	// KeyA is the key a value.
	// Key usages identify HID keyboard controls.
	KeyA Usage = Usage(domain.KeyA)
	// KeyB identifies key b.
	KeyB Usage = Usage(domain.KeyB)
	// KeyC identifies key c.
	KeyC Usage = Usage(domain.KeyC)
	// KeyD identifies key d.
	KeyD Usage = Usage(domain.KeyD)
	// KeyE identifies key e.
	KeyE Usage = Usage(domain.KeyE)
	// KeyF identifies key f.
	KeyF Usage = Usage(domain.KeyF)
	// KeyG identifies key g.
	KeyG Usage = Usage(domain.KeyG)
	// KeyH identifies key h.
	KeyH Usage = Usage(domain.KeyH)
	// KeyI identifies key i.
	KeyI Usage = Usage(domain.KeyI)
	// KeyJ identifies key j.
	KeyJ Usage = Usage(domain.KeyJ)
	// KeyK identifies key k.
	KeyK Usage = Usage(domain.KeyK)
	// KeyL identifies key l.
	KeyL Usage = Usage(domain.KeyL)
	// KeyM identifies key m.
	KeyM Usage = Usage(domain.KeyM)
	// KeyN identifies key n.
	KeyN Usage = Usage(domain.KeyN)
	// KeyO identifies key o.
	KeyO Usage = Usage(domain.KeyO)
	// KeyP identifies key p.
	KeyP Usage = Usage(domain.KeyP)
	// KeyQ identifies key q.
	KeyQ Usage = Usage(domain.KeyQ)
	// KeyR identifies key r.
	KeyR Usage = Usage(domain.KeyR)
	// KeyS identifies key s.
	KeyS Usage = Usage(domain.KeyS)
	// KeyT identifies key t.
	KeyT Usage = Usage(domain.KeyT)
	// KeyU identifies key u.
	KeyU Usage = Usage(domain.KeyU)
	// KeyV identifies key v.
	KeyV Usage = Usage(domain.KeyV)
	// KeyW identifies key w.
	KeyW Usage = Usage(domain.KeyW)
	// KeyX identifies key x.
	KeyX Usage = Usage(domain.KeyX)
	// KeyY identifies key y.
	KeyY Usage = Usage(domain.KeyY)
	// KeyZ identifies key z.
	KeyZ Usage = Usage(domain.KeyZ)
	// Key1 identifies key1.
	Key1 Usage = Usage(domain.Key1)
	// Key2 identifies key2.
	Key2 Usage = Usage(domain.Key2)
	// Key3 identifies key3.
	Key3 Usage = Usage(domain.Key3)
	// Key4 identifies key4.
	Key4 Usage = Usage(domain.Key4)
	// Key5 identifies key5.
	Key5 Usage = Usage(domain.Key5)
	// Key6 identifies key6.
	Key6 Usage = Usage(domain.Key6)
	// Key7 identifies key7.
	Key7 Usage = Usage(domain.Key7)
	// Key8 identifies key8.
	Key8 Usage = Usage(domain.Key8)
	// Key9 identifies key9.
	Key9 Usage = Usage(domain.Key9)
	// Key0 identifies key0.
	Key0 Usage = Usage(domain.Key0)
	// KeyEnter identifies key enter.
	KeyEnter Usage = Usage(domain.KeyEnter)
	// KeyEscape identifies key escape.
	KeyEscape Usage = Usage(domain.KeyEscape)
	// KeyBackspace identifies key backspace.
	KeyBackspace Usage = Usage(domain.KeyBackspace)
	// KeyTab identifies key tab.
	KeyTab Usage = Usage(domain.KeyTab)
	// KeySpace identifies key space.
	KeySpace Usage = Usage(domain.KeySpace)
	// KeyMinus identifies key minus.
	KeyMinus Usage = Usage(domain.KeyMinus)
	// KeyEqual identifies key equal.
	KeyEqual Usage = Usage(domain.KeyEqual)
	// KeyLeftBracket identifies key left bracket.
	KeyLeftBracket Usage = Usage(domain.KeyLeftBracket)
	// KeyRightBracket identifies key right bracket.
	KeyRightBracket Usage = Usage(domain.KeyRightBracket)
	// KeyBackslash identifies key backslash.
	KeyBackslash Usage = Usage(domain.KeyBackslash)
	// KeyNonUSHash identifies key non ushash.
	KeyNonUSHash Usage = Usage(domain.KeyNonUSHash)
	// KeySemicolon identifies key semicolon.
	KeySemicolon Usage = Usage(domain.KeySemicolon)
	// KeyApostrophe identifies key apostrophe.
	KeyApostrophe Usage = Usage(domain.KeyApostrophe)
	// KeyGrave identifies key grave.
	KeyGrave Usage = Usage(domain.KeyGrave)
	// KeyComma identifies key comma.
	KeyComma Usage = Usage(domain.KeyComma)
	// KeyPeriod identifies key period.
	KeyPeriod Usage = Usage(domain.KeyPeriod)
	// KeySlash identifies key slash.
	KeySlash Usage = Usage(domain.KeySlash)
	// KeyCapsLock identifies key caps lock.
	KeyCapsLock Usage = Usage(domain.KeyCapsLock)
	// KeyF1 identifies key f1.
	KeyF1 Usage = Usage(domain.KeyF1)
	// KeyF2 identifies key f2.
	KeyF2 Usage = Usage(domain.KeyF2)
	// KeyF3 identifies key f3.
	KeyF3 Usage = Usage(domain.KeyF3)
	// KeyF4 identifies key f4.
	KeyF4 Usage = Usage(domain.KeyF4)
	// KeyF5 identifies key f5.
	KeyF5 Usage = Usage(domain.KeyF5)
	// KeyF6 identifies key f6.
	KeyF6 Usage = Usage(domain.KeyF6)
	// KeyF7 identifies key f7.
	KeyF7 Usage = Usage(domain.KeyF7)
	// KeyF8 identifies key f8.
	KeyF8 Usage = Usage(domain.KeyF8)
	// KeyF9 identifies key f9.
	KeyF9 Usage = Usage(domain.KeyF9)
	// KeyF10 identifies key f10.
	KeyF10 Usage = Usage(domain.KeyF10)
	// KeyF11 identifies key f11.
	KeyF11 Usage = Usage(domain.KeyF11)
	// KeyF12 identifies key f12.
	KeyF12 Usage = Usage(domain.KeyF12)
	// KeyPrintScreen identifies key print screen.
	KeyPrintScreen Usage = Usage(domain.KeyPrintScreen)
	// KeyScrollLock identifies key scroll lock.
	KeyScrollLock Usage = Usage(domain.KeyScrollLock)
	// KeyPause identifies key pause.
	KeyPause Usage = Usage(domain.KeyPause)
	// KeyInsert identifies key insert.
	KeyInsert Usage = Usage(domain.KeyInsert)
	// KeyHome identifies key home.
	KeyHome Usage = Usage(domain.KeyHome)
	// KeyPageUp identifies key page up.
	KeyPageUp Usage = Usage(domain.KeyPageUp)
	// KeyDelete identifies key delete.
	KeyDelete Usage = Usage(domain.KeyDelete)
	// KeyEnd identifies key end.
	KeyEnd Usage = Usage(domain.KeyEnd)
	// KeyPageDown identifies key page down.
	KeyPageDown Usage = Usage(domain.KeyPageDown)
	// KeyRight identifies key right.
	KeyRight Usage = Usage(domain.KeyRight)
	// KeyLeft identifies key left.
	KeyLeft Usage = Usage(domain.KeyLeft)
	// KeyDown identifies key down.
	KeyDown Usage = Usage(domain.KeyDown)
	// KeyUp identifies key up.
	KeyUp Usage = Usage(domain.KeyUp)
	// KeyNumLock identifies key num lock.
	KeyNumLock Usage = Usage(domain.KeyNumLock)
	// KeyKeypadDivide identifies key keypad divide.
	KeyKeypadDivide Usage = Usage(domain.KeyKeypadDivide)
	// KeyKeypadMultiply identifies key keypad multiply.
	KeyKeypadMultiply Usage = Usage(domain.KeyKeypadMultiply)
	// KeyKeypadSubtract identifies key keypad subtract.
	KeyKeypadSubtract Usage = Usage(domain.KeyKeypadSubtract)
	// KeyKeypadAdd identifies key keypad add.
	KeyKeypadAdd Usage = Usage(domain.KeyKeypadAdd)
	// KeyKeypadEnter identifies key keypad enter.
	KeyKeypadEnter Usage = Usage(domain.KeyKeypadEnter)
	// KeyKeypad1 identifies key keypad1.
	KeyKeypad1 Usage = Usage(domain.KeyKeypad1)
	// KeyKeypad2 identifies key keypad2.
	KeyKeypad2 Usage = Usage(domain.KeyKeypad2)
	// KeyKeypad3 identifies key keypad3.
	KeyKeypad3 Usage = Usage(domain.KeyKeypad3)
	// KeyKeypad4 identifies key keypad4.
	KeyKeypad4 Usage = Usage(domain.KeyKeypad4)
	// KeyKeypad5 identifies key keypad5.
	KeyKeypad5 Usage = Usage(domain.KeyKeypad5)
	// KeyKeypad6 identifies key keypad6.
	KeyKeypad6 Usage = Usage(domain.KeyKeypad6)
	// KeyKeypad7 identifies key keypad7.
	KeyKeypad7 Usage = Usage(domain.KeyKeypad7)
	// KeyKeypad8 identifies key keypad8.
	KeyKeypad8 Usage = Usage(domain.KeyKeypad8)
	// KeyKeypad9 identifies key keypad9.
	KeyKeypad9 Usage = Usage(domain.KeyKeypad9)
	// KeyKeypad0 identifies key keypad0.
	KeyKeypad0 Usage = Usage(domain.KeyKeypad0)
	// KeyKeypadDecimal identifies key keypad decimal.
	KeyKeypadDecimal Usage = Usage(domain.KeyKeypadDecimal)
	// KeyNonUSBackslash identifies key non usbackslash.
	KeyNonUSBackslash Usage = Usage(domain.KeyNonUSBackslash)
	// KeyApplication identifies key application.
	KeyApplication Usage = Usage(domain.KeyApplication)
	// KeyPower identifies key power.
	KeyPower Usage = Usage(domain.KeyPower)
	// KeyKeypadEqual identifies key keypad equal.
	KeyKeypadEqual Usage = Usage(domain.KeyKeypadEqual)
	// KeyF13 identifies key f13.
	KeyF13 Usage = Usage(domain.KeyF13)
	// KeyF14 identifies key f14.
	KeyF14 Usage = Usage(domain.KeyF14)
	// KeyF15 identifies key f15.
	KeyF15 Usage = Usage(domain.KeyF15)
	// KeyF16 identifies key f16.
	KeyF16 Usage = Usage(domain.KeyF16)
	// KeyF17 identifies key f17.
	KeyF17 Usage = Usage(domain.KeyF17)
	// KeyF18 identifies key f18.
	KeyF18 Usage = Usage(domain.KeyF18)
	// KeyF19 identifies key f19.
	KeyF19 Usage = Usage(domain.KeyF19)
	// KeyF20 identifies key f20.
	KeyF20 Usage = Usage(domain.KeyF20)
	// KeyF21 identifies key f21.
	KeyF21 Usage = Usage(domain.KeyF21)
	// KeyF22 identifies key f22.
	KeyF22 Usage = Usage(domain.KeyF22)
	// KeyF23 identifies key f23.
	KeyF23 Usage = Usage(domain.KeyF23)
	// KeyF24 identifies key f24.
	KeyF24 Usage = Usage(domain.KeyF24)

	// KeyLeftControl is the key left control value.
	// Modifier usages identify HID keyboard modifiers.
	KeyLeftControl Usage = Usage(domain.KeyLeftControl)
	// KeyLeftShift identifies key left shift.
	KeyLeftShift Usage = Usage(domain.KeyLeftShift)
	// KeyLeftAlt identifies key left alt.
	KeyLeftAlt Usage = Usage(domain.KeyLeftAlt)
	// KeyLeftGUI identifies key left gui.
	KeyLeftGUI Usage = Usage(domain.KeyLeftGUI)
	// KeyRightControl identifies key right control.
	KeyRightControl Usage = Usage(domain.KeyRightControl)
	// KeyRightShift identifies key right shift.
	KeyRightShift Usage = Usage(domain.KeyRightShift)
	// KeyRightAlt identifies key right alt.
	KeyRightAlt Usage = Usage(domain.KeyRightAlt)
	// KeyRightGUI identifies key right gui.
	KeyRightGUI Usage = Usage(domain.KeyRightGUI)

	// KeyMute is the key mute value.
	// Media usages identify HID consumer controls.
	KeyMute Usage = Usage(domain.KeyMute)
	// KeyVolumeUp identifies key volume up.
	KeyVolumeUp Usage = Usage(domain.KeyVolumeUp)
	// KeyVolumeDown identifies key volume down.
	KeyVolumeDown Usage = Usage(domain.KeyVolumeDown)
	// KeyPlayPause identifies key play pause.
	KeyPlayPause Usage = Usage(domain.KeyPlayPause)
	// KeyNextTrack identifies key next track.
	KeyNextTrack Usage = Usage(domain.KeyNextTrack)
	// KeyPrevTrack identifies key prev track.
	KeyPrevTrack Usage = Usage(domain.KeyPrevTrack)
	// KeyStop identifies key stop.
	KeyStop Usage = Usage(domain.KeyStop)
)
