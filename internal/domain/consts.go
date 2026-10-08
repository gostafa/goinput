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
	Button1 Usage = 0x00090001
	// Button2 identifies button2.
	Button2 Usage = 0x00090002
	// Button3 identifies button3.
	Button3 Usage = 0x00090003
	// Button4 identifies button4.
	Button4 Usage = 0x00090004
	// Button5 identifies button5.
	Button5 Usage = 0x00090005
	// Button6 identifies button6.
	Button6 Usage = 0x00090006
	// Button7 identifies button7.
	Button7 Usage = 0x00090007
	// Button8 identifies button8.
	Button8 Usage = 0x00090008
	// Button9 identifies button9.
	Button9 Usage = 0x00090009
	// Button10 identifies button10.
	Button10 Usage = 0x0009000a
	// Button11 identifies button11.
	Button11 Usage = 0x0009000b
	// Button12 identifies button12.
	Button12 Usage = 0x0009000c
	// Button13 identifies button13.
	Button13 Usage = 0x0009000d
	// Button14 identifies button14.
	Button14 Usage = 0x0009000e
	// Button15 identifies button15.
	Button15 Usage = 0x0009000f
	// Button16 identifies button16.
	Button16 Usage = 0x00090010

	// KeyA is the key a value.
	// Key usages identify HID keyboard controls.
	KeyA Usage = 0x00070004
	// KeyB identifies key b.
	KeyB Usage = 0x00070005
	// KeyC identifies key c.
	KeyC Usage = 0x00070006
	// KeyD identifies key d.
	KeyD Usage = 0x00070007
	// KeyE identifies key e.
	KeyE Usage = 0x00070008
	// KeyF identifies key f.
	KeyF Usage = 0x00070009
	// KeyG identifies key g.
	KeyG Usage = 0x0007000a
	// KeyH identifies key h.
	KeyH Usage = 0x0007000b
	// KeyI identifies key i.
	KeyI Usage = 0x0007000c
	// KeyJ identifies key j.
	KeyJ Usage = 0x0007000d
	// KeyK identifies key k.
	KeyK Usage = 0x0007000e
	// KeyL identifies key l.
	KeyL Usage = 0x0007000f
	// KeyM identifies key m.
	KeyM Usage = 0x00070010
	// KeyN identifies key n.
	KeyN Usage = 0x00070011
	// KeyO identifies key o.
	KeyO Usage = 0x00070012
	// KeyP identifies key p.
	KeyP Usage = 0x00070013
	// KeyQ identifies key q.
	KeyQ Usage = 0x00070014
	// KeyR identifies key r.
	KeyR Usage = 0x00070015
	// KeyS identifies key s.
	KeyS Usage = 0x00070016
	// KeyT identifies key t.
	KeyT Usage = 0x00070017
	// KeyU identifies key u.
	KeyU Usage = 0x00070018
	// KeyV identifies key v.
	KeyV Usage = 0x00070019
	// KeyW identifies key w.
	KeyW Usage = 0x0007001a
	// KeyX identifies key x.
	KeyX Usage = 0x0007001b
	// KeyY identifies key y.
	KeyY Usage = 0x0007001c
	// KeyZ identifies key z.
	KeyZ Usage = 0x0007001d
	// Key1 identifies key1.
	Key1 Usage = 0x0007001e
	// Key2 identifies key2.
	Key2 Usage = 0x0007001f
	// Key3 identifies key3.
	Key3 Usage = 0x00070020
	// Key4 identifies key4.
	Key4 Usage = 0x00070021
	// Key5 identifies key5.
	Key5 Usage = 0x00070022
	// Key6 identifies key6.
	Key6 Usage = 0x00070023
	// Key7 identifies key7.
	Key7 Usage = 0x00070024
	// Key8 identifies key8.
	Key8 Usage = 0x00070025
	// Key9 identifies key9.
	Key9 Usage = 0x00070026
	// Key0 identifies key0.
	Key0 Usage = 0x00070027
	// KeyEnter identifies key enter.
	KeyEnter Usage = 0x00070028
	// KeyEscape identifies key escape.
	KeyEscape Usage = 0x00070029
	// KeyBackspace identifies key backspace.
	KeyBackspace Usage = 0x0007002a
	// KeyTab identifies key tab.
	KeyTab Usage = 0x0007002b
	// KeySpace identifies key space.
	KeySpace Usage = 0x0007002c
	// KeyMinus identifies key minus.
	KeyMinus Usage = 0x0007002d
	// KeyEqual identifies key equal.
	KeyEqual Usage = 0x0007002e
	// KeyLeftBracket identifies key left bracket.
	KeyLeftBracket Usage = 0x0007002f
	// KeyRightBracket identifies key right bracket.
	KeyRightBracket Usage = 0x00070030
	// KeyBackslash identifies key backslash.
	KeyBackslash Usage = 0x00070031
	// KeyNonUSHash identifies key non ushash.
	KeyNonUSHash Usage = 0x00070032
	// KeySemicolon identifies key semicolon.
	KeySemicolon Usage = 0x00070033
	// KeyApostrophe identifies key apostrophe.
	KeyApostrophe Usage = 0x00070034
	// KeyGrave identifies key grave.
	KeyGrave Usage = 0x00070035
	// KeyComma identifies key comma.
	KeyComma Usage = 0x00070036
	// KeyPeriod identifies key period.
	KeyPeriod Usage = 0x00070037
	// KeySlash identifies key slash.
	KeySlash Usage = 0x00070038
	// KeyCapsLock identifies key caps lock.
	KeyCapsLock Usage = 0x00070039
	// KeyF1 identifies key f1.
	KeyF1 Usage = 0x0007003a
	// KeyF2 identifies key f2.
	KeyF2 Usage = 0x0007003b
	// KeyF3 identifies key f3.
	KeyF3 Usage = 0x0007003c
	// KeyF4 identifies key f4.
	KeyF4 Usage = 0x0007003d
	// KeyF5 identifies key f5.
	KeyF5 Usage = 0x0007003e
	// KeyF6 identifies key f6.
	KeyF6 Usage = 0x0007003f
	// KeyF7 identifies key f7.
	KeyF7 Usage = 0x00070040
	// KeyF8 identifies key f8.
	KeyF8 Usage = 0x00070041
	// KeyF9 identifies key f9.
	KeyF9 Usage = 0x00070042
	// KeyF10 identifies key f10.
	KeyF10 Usage = 0x00070043
	// KeyF11 identifies key f11.
	KeyF11 Usage = 0x00070044
	// KeyF12 identifies key f12.
	KeyF12 Usage = 0x00070045
	// KeyPrintScreen identifies key print screen.
	KeyPrintScreen Usage = 0x00070046
	// KeyScrollLock identifies key scroll lock.
	KeyScrollLock Usage = 0x00070047
	// KeyPause identifies key pause.
	KeyPause Usage = 0x00070048
	// KeyInsert identifies key insert.
	KeyInsert Usage = 0x00070049
	// KeyHome identifies key home.
	KeyHome Usage = 0x0007004a
	// KeyPageUp identifies key page up.
	KeyPageUp Usage = 0x0007004b
	// KeyDelete identifies key delete.
	KeyDelete Usage = 0x0007004c
	// KeyEnd identifies key end.
	KeyEnd Usage = 0x0007004d
	// KeyPageDown identifies key page down.
	KeyPageDown Usage = 0x0007004e
	// KeyRight identifies key right.
	KeyRight Usage = 0x0007004f
	// KeyLeft identifies key left.
	KeyLeft Usage = 0x00070050
	// KeyDown identifies key down.
	KeyDown Usage = 0x00070051
	// KeyUp identifies key up.
	KeyUp Usage = 0x00070052
	// KeyNumLock identifies key num lock.
	KeyNumLock Usage = 0x00070053
	// KeyKeypadDivide identifies key keypad divide.
	KeyKeypadDivide Usage = 0x00070054
	// KeyKeypadMultiply identifies key keypad multiply.
	KeyKeypadMultiply Usage = 0x00070055
	// KeyKeypadSubtract identifies key keypad subtract.
	KeyKeypadSubtract Usage = 0x00070056
	// KeyKeypadAdd identifies key keypad add.
	KeyKeypadAdd Usage = 0x00070057
	// KeyKeypadEnter identifies key keypad enter.
	KeyKeypadEnter Usage = 0x00070058
	// KeyKeypad1 identifies key keypad1.
	KeyKeypad1 Usage = 0x00070059
	// KeyKeypad2 identifies key keypad2.
	KeyKeypad2 Usage = 0x0007005a
	// KeyKeypad3 identifies key keypad3.
	KeyKeypad3 Usage = 0x0007005b
	// KeyKeypad4 identifies key keypad4.
	KeyKeypad4 Usage = 0x0007005c
	// KeyKeypad5 identifies key keypad5.
	KeyKeypad5 Usage = 0x0007005d
	// KeyKeypad6 identifies key keypad6.
	KeyKeypad6 Usage = 0x0007005e
	// KeyKeypad7 identifies key keypad7.
	KeyKeypad7 Usage = 0x0007005f
	// KeyKeypad8 identifies key keypad8.
	KeyKeypad8 Usage = 0x00070060
	// KeyKeypad9 identifies key keypad9.
	KeyKeypad9 Usage = 0x00070061
	// KeyKeypad0 identifies key keypad0.
	KeyKeypad0 Usage = 0x00070062
	// KeyKeypadDecimal identifies key keypad decimal.
	KeyKeypadDecimal Usage = 0x00070063
	// KeyNonUSBackslash identifies key non usbackslash.
	KeyNonUSBackslash Usage = 0x00070064
	// KeyApplication identifies key application.
	KeyApplication Usage = 0x00070065
	// KeyPower identifies key power.
	KeyPower Usage = 0x00070066
	// KeyKeypadEqual identifies key keypad equal.
	KeyKeypadEqual Usage = 0x00070067
	// KeyF13 identifies key f13.
	KeyF13 Usage = 0x00070068
	// KeyF14 identifies key f14.
	KeyF14 Usage = 0x00070069
	// KeyF15 identifies key f15.
	KeyF15 Usage = 0x0007006a
	// KeyF16 identifies key f16.
	KeyF16 Usage = 0x0007006b
	// KeyF17 identifies key f17.
	KeyF17 Usage = 0x0007006c
	// KeyF18 identifies key f18.
	KeyF18 Usage = 0x0007006d
	// KeyF19 identifies key f19.
	KeyF19 Usage = 0x0007006e
	// KeyF20 identifies key f20.
	KeyF20 Usage = 0x0007006f
	// KeyF21 identifies key f21.
	KeyF21 Usage = 0x00070070
	// KeyF22 identifies key f22.
	KeyF22 Usage = 0x00070071
	// KeyF23 identifies key f23.
	KeyF23 Usage = 0x00070072
	// KeyF24 identifies key f24.
	KeyF24 Usage = 0x00070073

	// KeyLeftControl is the key left control value.
	// Modifier usages identify HID keyboard modifiers.
	KeyLeftControl Usage = 0x000700e0
	// KeyLeftShift identifies key left shift.
	KeyLeftShift Usage = 0x000700e1
	// KeyLeftAlt identifies key left alt.
	KeyLeftAlt Usage = 0x000700e2
	// KeyLeftGUI identifies key left gui.
	KeyLeftGUI Usage = 0x000700e3
	// KeyRightControl identifies key right control.
	KeyRightControl Usage = 0x000700e4
	// KeyRightShift identifies key right shift.
	KeyRightShift Usage = 0x000700e5
	// KeyRightAlt identifies key right alt.
	KeyRightAlt Usage = 0x000700e6
	// KeyRightGUI identifies key right gui.
	KeyRightGUI Usage = 0x000700e7

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

const (
	normalizedMinimum = float64(UsageUnknown)
)
