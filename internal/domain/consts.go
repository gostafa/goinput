// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

const (
	// DefaultBufferSize is the default buffer size value.
	// DefaultBufferSize is the default number of queued events per device.
	DefaultBufferSize = 256

	// ClassUnknown is the class unknown value.
	// Enum values occupy distinct ranges; use the named constants, not numeric literals.
	// Device classes identify endpoint categories.
	ClassUnknown DeviceClass = 32
	// ClassKeyboard identifies class keyboard.
	ClassKeyboard DeviceClass = 33

	// ClassMouse identifies class mouse.
	ClassMouse DeviceClass = 34
	// ClassGamepad identifies class gamepad.
	ClassGamepad DeviceClass = 35

	// ClassJoystick identifies class joystick.
	ClassJoystick DeviceClass = 36
	// ClassOther identifies class other.
	ClassOther DeviceClass = 37

	// TransportUnknown is the transport unknown value.
	// Transports identify device connection media.
	TransportUnknown Transport = 38
	// TransportUSB identifies transport usb.
	TransportUSB Transport = 39

	// TransportBluetooth identifies transport bluetooth.
	TransportBluetooth Transport = 40
	// TransportPS2 identifies transport ps2.
	TransportPS2 Transport = 41

	// TransportI2C identifies transport i2 c.
	TransportI2C Transport = 42
	// TransportVirtual identifies transport virtual.
	TransportVirtual Transport = 43

	// ControlUnknown is the control unknown value.
	// Control kinds identify input semantics.
	ControlUnknown ControlKind = 44
	// ControlKey identifies control key.
	ControlKey ControlKind = 45

	// ControlButton identifies control button.
	ControlButton ControlKind = 46
	// ControlAxis identifies control axis.
	ControlAxis ControlKind = 47

	// ControlHat identifies control hat.
	ControlHat ControlKind = 48
	// ControlSwitch identifies control switch.
	ControlSwitch ControlKind = 49

	// AxisUnknown is the axis unknown value.
	// Axis modes distinguish absolute positions and relative deltas.
	AxisUnknown AxisMode = 50
	// AxisRelative identifies axis relative.
	AxisRelative AxisMode = 51

	// AxisAbsolute identifies axis absolute.
	AxisAbsolute AxisMode = 52

	// MappingUnknown is the mapping unknown value.
	// Mapping sources describe the origin of HID usages.
	MappingUnknown MappingSource = 53
	// MappingReported identifies mapping reported.
	MappingReported MappingSource = 54

	// MappingInferred identifies mapping inferred.
	MappingInferred MappingSource = 55

	// UnitUnknown is the unit unknown value.
	// Units describe control value interpretations.
	UnitUnknown Unit = 56
	// UnitLogical identifies unit logical.
	UnitLogical Unit = 57

	// UnitCounts identifies unit counts.
	UnitCounts Unit = 58
	// UnitDetents identifies unit detents.
	UnitDetents Unit = 59

	// UnitDirection identifies unit direction.
	UnitDirection Unit = 60
	// UnitBoolean identifies unit boolean.
	UnitBoolean Unit = 61

	// SupportUnknown is the support unknown value.
	// Support values describe capability availability.
	SupportUnknown Support = 62
	// SupportSupported identifies support supported.
	SupportSupported Support = 63

	// SupportUnsupported identifies support unsupported.
	SupportUnsupported Support = 64

	// ActionUnknown is the action unknown value.
	// Actions describe control transitions.
	ActionUnknown EventAction = 65
	// ActionChange identifies action change.
	ActionChange EventAction = 66

	// ActionPress identifies action press.
	ActionPress EventAction = 67
	// ActionRelease identifies action release.
	ActionRelease EventAction = 68

	// ActionRepeat identifies action repeat.
	ActionRepeat EventAction = 69

	// TimestampUnknown is the timestamp unknown value.
	// Timestamp sources distinguish native and receipt clocks.
	TimestampUnknown TimestampSource = 70
	// TimestampNative identifies timestamp native.
	TimestampNative TimestampSource = 71

	// TimestampEstimated identifies timestamp estimated.
	TimestampEstimated TimestampSource = 72
	// TimestampReceipt identifies timestamp receipt.
	TimestampReceipt TimestampSource = 73

	// HatNeutral is the hat neutral value.
	// Hat directions describe compass positions.
	HatNeutral HatDirection = -1
	// HatNorth identifies hat north.
	HatNorth HatDirection = 16

	// HatNorthEast identifies hat north east.
	HatNorthEast HatDirection = 17
	// HatEast identifies hat east.
	HatEast HatDirection = 18

	// HatSouthEast identifies hat south east.
	HatSouthEast HatDirection = 19
	// HatSouth identifies hat south.
	HatSouth HatDirection = 20

	// HatSouthWest identifies hat south west.
	HatSouthWest HatDirection = 21
	// HatWest identifies hat west.
	HatWest HatDirection = 22

	// HatNorthWest identifies hat north west.
	HatNorthWest HatDirection = 23

	// PageGenericDesktop is the page generic desktop value.
	// HID usage pages used by the portable control vocabulary.
	// HID pages identify standard control namespaces.
	PageGenericDesktop uint16 = 0x01
	// PageSimulation identifies page simulation.
	PageSimulation uint16 = 0x02
	// PageKeyboard identifies page keyboard.
	PageKeyboard uint16 = 0x07
	// PageButton identifies page button.
	PageButton uint16 = 0x09
	// PageConsumer identifies page consumer.
	PageConsumer uint16 = 0x0c

	// UsageUnknown is the usage unknown value.
	// Usages identify standard HID controls.
	UsageUnknown Usage = 0
	// AxisX identifies axis x.
	AxisX Usage = 0x00010030
	// AxisY identifies axis y.
	AxisY Usage = 0x00010031
	// AxisZ identifies axis z.
	AxisZ Usage = 0x00010032
	// AxisRx identifies axis rx.
	AxisRx Usage = 0x00010033
	// AxisRy identifies axis ry.
	AxisRy Usage = 0x00010034
	// AxisRz identifies axis rz.
	AxisRz Usage = 0x00010035
	// AxisSlider identifies axis slider.
	AxisSlider Usage = 0x00010036
	// AxisDial identifies axis dial.
	AxisDial Usage = 0x00010037
	// AxisWheel identifies axis wheel.
	AxisWheel Usage = 0x00010038
	// HatSwitch identifies hat switch.
	HatSwitch Usage = 0x00010039
	// DPadUp identifies dpad up.
	DPadUp Usage = 0x00010090
	// DPadDown identifies dpad down.
	DPadDown Usage = 0x00010091
	// DPadRight identifies dpad right.
	DPadRight Usage = 0x00010092
	// DPadLeft identifies dpad left.
	DPadLeft Usage = 0x00010093
	// AxisRudder identifies axis rudder.
	AxisRudder Usage = 0x000200ba
	// AxisThrottle identifies axis throttle.
	AxisThrottle Usage = 0x000200bb
	// AxisGas identifies axis gas.
	AxisGas Usage = 0x000200c4
	// AxisBrake identifies axis brake.
	AxisBrake Usage = 0x000200c5
	// AxisPan identifies axis pan.
	AxisPan Usage = 0x000c0238

	// Button1 is the button1 value.
	// Button ordinals do not promise physical positions on a game controller.
	// Button usages identify device-local button ordinals.
	Button1 Usage = 0x00090001 + (iota - 77)
	// Button2 identifies button2.
	Button2
	// Button3 identifies button3.
	Button3
	// Button4 identifies button4.
	Button4
	// Button5 identifies button5.
	Button5
	// Button6 identifies button6.
	Button6
	// Button7 identifies button7.
	Button7
	// Button8 identifies button8.
	Button8
	// Button9 identifies button9.
	Button9
	// Button10 identifies button10.
	Button10
	// Button11 identifies button11.
	Button11
	// Button12 identifies button12.
	Button12
	// Button13 identifies button13.
	Button13
	// Button14 identifies button14.
	Button14
	// Button15 identifies button15.
	Button15
	// Button16 identifies button16.
	Button16

	// KeyA is the key a value.
	// Key usages identify HID keyboard controls.
	KeyA Usage = 0x00070004 + (iota - 93)
	// KeyB identifies key b.
	KeyB
	// KeyC identifies key c.
	KeyC
	// KeyD identifies key d.
	KeyD
	// KeyE identifies key e.
	KeyE
	// KeyF identifies key f.
	KeyF
	// KeyG identifies key g.
	KeyG
	// KeyH identifies key h.
	KeyH
	// KeyI identifies key i.
	KeyI
	// KeyJ identifies key j.
	KeyJ
	// KeyK identifies key k.
	KeyK
	// KeyL identifies key l.
	KeyL
	// KeyM identifies key m.
	KeyM
	// KeyN identifies key n.
	KeyN
	// KeyO identifies key o.
	KeyO
	// KeyP identifies key p.
	KeyP
	// KeyQ identifies key q.
	KeyQ
	// KeyR identifies key r.
	KeyR
	// KeyS identifies key s.
	KeyS
	// KeyT identifies key t.
	KeyT
	// KeyU identifies key u.
	KeyU
	// KeyV identifies key v.
	KeyV
	// KeyW identifies key w.
	KeyW
	// KeyX identifies key x.
	KeyX
	// KeyY identifies key y.
	KeyY
	// KeyZ identifies key z.
	KeyZ
	// Key1 identifies key1.
	Key1
	// Key2 identifies key2.
	Key2
	// Key3 identifies key3.
	Key3
	// Key4 identifies key4.
	Key4
	// Key5 identifies key5.
	Key5
	// Key6 identifies key6.
	Key6
	// Key7 identifies key7.
	Key7
	// Key8 identifies key8.
	Key8
	// Key9 identifies key9.
	Key9
	// Key0 identifies key0.
	Key0
	// KeyEnter identifies key enter.
	KeyEnter
	// KeyEscape identifies key escape.
	KeyEscape
	// KeyBackspace identifies key backspace.
	KeyBackspace
	// KeyTab identifies key tab.
	KeyTab
	// KeySpace identifies key space.
	KeySpace
	// KeyMinus identifies key minus.
	KeyMinus
	// KeyEqual identifies key equal.
	KeyEqual
	// KeyLeftBracket identifies key left bracket.
	KeyLeftBracket
	// KeyRightBracket identifies key right bracket.
	KeyRightBracket
	// KeyBackslash identifies key backslash.
	KeyBackslash
	// KeyNonUSHash identifies key non ushash.
	KeyNonUSHash
	// KeySemicolon identifies key semicolon.
	KeySemicolon
	// KeyApostrophe identifies key apostrophe.
	KeyApostrophe
	// KeyGrave identifies key grave.
	KeyGrave
	// KeyComma identifies key comma.
	KeyComma
	// KeyPeriod identifies key period.
	KeyPeriod
	// KeySlash identifies key slash.
	KeySlash
	// KeyCapsLock identifies key caps lock.
	KeyCapsLock
	// KeyF1 identifies key f1.
	KeyF1
	// KeyF2 identifies key f2.
	KeyF2
	// KeyF3 identifies key f3.
	KeyF3
	// KeyF4 identifies key f4.
	KeyF4
	// KeyF5 identifies key f5.
	KeyF5
	// KeyF6 identifies key f6.
	KeyF6
	// KeyF7 identifies key f7.
	KeyF7
	// KeyF8 identifies key f8.
	KeyF8
	// KeyF9 identifies key f9.
	KeyF9
	// KeyF10 identifies key f10.
	KeyF10
	// KeyF11 identifies key f11.
	KeyF11
	// KeyF12 identifies key f12.
	KeyF12
	// KeyPrintScreen identifies key print screen.
	KeyPrintScreen
	// KeyScrollLock identifies key scroll lock.
	KeyScrollLock
	// KeyPause identifies key pause.
	KeyPause
	// KeyInsert identifies key insert.
	KeyInsert
	// KeyHome identifies key home.
	KeyHome
	// KeyPageUp identifies key page up.
	KeyPageUp
	// KeyDelete identifies key delete.
	KeyDelete
	// KeyEnd identifies key end.
	KeyEnd
	// KeyPageDown identifies key page down.
	KeyPageDown
	// KeyRight identifies key right.
	KeyRight
	// KeyLeft identifies key left.
	KeyLeft
	// KeyDown identifies key down.
	KeyDown
	// KeyUp identifies key up.
	KeyUp
	// KeyNumLock identifies key num lock.
	KeyNumLock
	// KeyKeypadDivide identifies key keypad divide.
	KeyKeypadDivide
	// KeyKeypadMultiply identifies key keypad multiply.
	KeyKeypadMultiply
	// KeyKeypadSubtract identifies key keypad subtract.
	KeyKeypadSubtract
	// KeyKeypadAdd identifies key keypad add.
	KeyKeypadAdd
	// KeyKeypadEnter identifies key keypad enter.
	KeyKeypadEnter
	// KeyKeypad1 identifies key keypad1.
	KeyKeypad1
	// KeyKeypad2 identifies key keypad2.
	KeyKeypad2
	// KeyKeypad3 identifies key keypad3.
	KeyKeypad3
	// KeyKeypad4 identifies key keypad4.
	KeyKeypad4
	// KeyKeypad5 identifies key keypad5.
	KeyKeypad5
	// KeyKeypad6 identifies key keypad6.
	KeyKeypad6
	// KeyKeypad7 identifies key keypad7.
	KeyKeypad7
	// KeyKeypad8 identifies key keypad8.
	KeyKeypad8
	// KeyKeypad9 identifies key keypad9.
	KeyKeypad9
	// KeyKeypad0 identifies key keypad0.
	KeyKeypad0
	// KeyKeypadDecimal identifies key keypad decimal.
	KeyKeypadDecimal
	// KeyNonUSBackslash identifies key non usbackslash.
	KeyNonUSBackslash
	// KeyApplication identifies key application.
	KeyApplication
	// KeyPower identifies key power.
	KeyPower
	// KeyKeypadEqual identifies key keypad equal.
	KeyKeypadEqual
	// KeyF13 identifies key f13.
	KeyF13
	// KeyF14 identifies key f14.
	KeyF14
	// KeyF15 identifies key f15.
	KeyF15
	// KeyF16 identifies key f16.
	KeyF16
	// KeyF17 identifies key f17.
	KeyF17
	// KeyF18 identifies key f18.
	KeyF18
	// KeyF19 identifies key f19.
	KeyF19
	// KeyF20 identifies key f20.
	KeyF20
	// KeyF21 identifies key f21.
	KeyF21
	// KeyF22 identifies key f22.
	KeyF22
	// KeyF23 identifies key f23.
	KeyF23
	// KeyF24 identifies key f24.
	KeyF24

	// KeyLeftControl is the key left control value.
	// Modifier usages identify HID keyboard modifiers.
	KeyLeftControl Usage = 0x000700e0 + (iota - 205)
	// KeyLeftShift identifies key left shift.
	KeyLeftShift
	// KeyLeftAlt identifies key left alt.
	KeyLeftAlt
	// KeyLeftGUI identifies key left gui.
	KeyLeftGUI
	// KeyRightControl identifies key right control.
	KeyRightControl
	// KeyRightShift identifies key right shift.
	KeyRightShift
	// KeyRightAlt identifies key right alt.
	KeyRightAlt
	// KeyRightGUI identifies key right gui.
	KeyRightGUI

	// KeyMute is the key mute value.
	// Media usages identify HID consumer controls.
	KeyMute Usage = 0x000c00e2
	// KeyVolumeUp identifies key volume up.
	KeyVolumeUp Usage = 0x000c00e9
	// KeyVolumeDown identifies key volume down.
	KeyVolumeDown Usage = 0x000c00ea
	// KeyPlayPause identifies key play pause.
	KeyPlayPause Usage = 0x000c00cd
	// KeyNextTrack identifies key next track.
	KeyNextTrack Usage = 0x000c00b5
	// KeyPrevTrack identifies key prev track.
	KeyPrevTrack Usage = 0x000c00b6
	// KeyStop identifies key stop.
	KeyStop           Usage = 0x000c00b7
	hatFourPositions  int64 = 4
	hatEightPositions int64 = 8
)
