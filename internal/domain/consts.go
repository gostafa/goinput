// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

const DefaultBufferSize = 256

const (
	ClassUnknown DeviceClass = iota
	ClassKeyboard
	ClassMouse
	ClassGamepad
	ClassJoystick
	ClassOther
)

const (
	TransportUnknown Transport = iota
	TransportUSB
	TransportBluetooth
	TransportPS2
	TransportI2C
	TransportVirtual
)

const (
	ControlUnknown ControlKind = iota
	ControlKey
	ControlButton
	ControlAxis
	ControlHat
	ControlSwitch
)

const (
	AxisUnknown AxisMode = iota
	AxisRelative
	AxisAbsolute
)

const (
	MappingUnknown MappingSource = iota
	MappingReported
	MappingInferred
)

const (
	UnitUnknown Unit = iota
	UnitLogical
	UnitCounts
	UnitDetents
	UnitDirection
	UnitBoolean
)

const (
	SupportUnknown Support = iota
	SupportSupported
	SupportUnsupported
)

const (
	ActionUnknown EventAction = iota
	ActionChange
	ActionPress
	ActionRelease
	ActionRepeat
)

const (
	TimestampUnknown TimestampSource = iota
	TimestampNative
	TimestampEstimated
	TimestampReceipt
)

const (
	HatNeutral HatDirection = -1
	HatNorth   HatDirection = iota - 1
	HatNorthEast
	HatEast
	HatSouthEast
	HatSouth
	HatSouthWest
	HatWest
	HatNorthWest
)

// HID usage pages used by the portable control vocabulary.
const (
	PageGenericDesktop uint16 = 0x01
	PageSimulation     uint16 = 0x02
	PageKeyboard       uint16 = 0x07
	PageButton         uint16 = 0x09
	PageConsumer       uint16 = 0x0c
)

const (
	UsageUnknown Usage = 0
	AxisX        Usage = 0x00010030
	AxisY        Usage = 0x00010031
	AxisZ        Usage = 0x00010032
	AxisRx       Usage = 0x00010033
	AxisRy       Usage = 0x00010034
	AxisRz       Usage = 0x00010035
	AxisSlider   Usage = 0x00010036
	AxisDial     Usage = 0x00010037
	AxisWheel    Usage = 0x00010038
	HatSwitch    Usage = 0x00010039
	DPadUp       Usage = 0x00010090
	DPadDown     Usage = 0x00010091
	DPadRight    Usage = 0x00010092
	DPadLeft     Usage = 0x00010093
	AxisRudder   Usage = 0x000200ba
	AxisThrottle Usage = 0x000200bb
	AxisGas      Usage = 0x000200c4
	AxisBrake    Usage = 0x000200c5
	AxisPan      Usage = 0x000c0238
)

// Button ordinals do not promise physical positions on a game controller.
const (
	Button1 Usage = 0x00090001 + iota
	Button2
	Button3
	Button4
	Button5
	Button6
	Button7
	Button8
	Button9
	Button10
	Button11
	Button12
	Button13
	Button14
	Button15
	Button16
)

const (
	KeyA Usage = 0x00070004 + iota
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	Key0
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeySpace
	KeyMinus
	KeyEqual
	KeyLeftBracket
	KeyRightBracket
	KeyBackslash
	KeyNonUSHash
	KeySemicolon
	KeyApostrophe
	KeyGrave
	KeyComma
	KeyPeriod
	KeySlash
	KeyCapsLock
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyPrintScreen
	KeyScrollLock
	KeyPause
	KeyInsert
	KeyHome
	KeyPageUp
	KeyDelete
	KeyEnd
	KeyPageDown
	KeyRight
	KeyLeft
	KeyDown
	KeyUp
	KeyNumLock
	KeyKeypadDivide
	KeyKeypadMultiply
	KeyKeypadSubtract
	KeyKeypadAdd
	KeyKeypadEnter
	KeyKeypad1
	KeyKeypad2
	KeyKeypad3
	KeyKeypad4
	KeyKeypad5
	KeyKeypad6
	KeyKeypad7
	KeyKeypad8
	KeyKeypad9
	KeyKeypad0
	KeyKeypadDecimal
	KeyNonUSBackslash
	KeyApplication
	KeyPower
	KeyKeypadEqual
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24
)

const (
	KeyLeftControl Usage = 0x000700e0 + iota
	KeyLeftShift
	KeyLeftAlt
	KeyLeftGUI
	KeyRightControl
	KeyRightShift
	KeyRightAlt
	KeyRightGUI
)

const (
	KeyMute       Usage = 0x000c00e2
	KeyVolumeUp   Usage = 0x000c00e9
	KeyVolumeDown Usage = 0x000c00ea
	KeyPlayPause  Usage = 0x000c00cd
	KeyNextTrack  Usage = 0x000c00b5
	KeyPrevTrack  Usage = 0x000c00b6
	KeyStop       Usage = 0x000c00b7
)
