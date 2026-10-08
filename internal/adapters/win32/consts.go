// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package win32

const (
	// Used for byteMask, messageInput.
	byteMask                   = 0xff
	cardinalExtent             = 270
	commandCapacity            = 64
	compassExtent              = 315
	consumerBackUsage          = 0x224
	consumerBookmarksUsage     = 0x22a
	consumerCalculatorUsage    = 0x192
	consumerConfigurationUsage = 0x183
	consumerEmailUsage         = 0x18a
	consumerForwardUsage       = 0x225
	consumerHomeUsage          = 0x223
	consumerRefreshUsage       = 0x227
	consumerSearchUsage        = 0x221
	consumerStopUsage          = 0x226
	deviceInfoCommand          = 0x2000000b
	deviceNameCommand          = 0x20000007
	// Used for packetDeviceOffset, hidPacketHeaderBytes, capabilityWords, compassDirections.
	eighthValue             = 8
	emptyString             = ""
	errorDeviceDisconnected = 1167
	errorInsufficientBuffer = 122
	errorInvalidParameter   = 87
	errorMoreData           = 234
	errorNotReady           = 21
	// Used for gamepadUsage, mouseButtonCount.
	fifthValue = 5
	// Used for joystickUsage, packetSizeOffset, keyExtendedE1, mouseButtonsOffset, cardinalDirections,
	// waitInputAvailable.
	fourthValue                 = 4
	hatUsage                    = 0x39
	hidNameFormat               = "HID %04x:%04x"
	hidStringBytes              = 512
	infiniteWait                = 0xffffffff
	inputDataCommand            = 0x10000003
	keyboardInternational1Usage = 0x87
	keyboardInternational2Usage = 0x88
	keyboardInternational3Usage = 0x89
	keyboardInternational4Usage = 0x8a
	keyboardInternational5Usage = 0x8b
	keyboardKeypadCommaUsage    = 0x85
	maxDevices                  = 1 << 16
	maxNativeBuffer             = 16 << 20
	messageDeviceChange         = 0x00fe
	messageQuit                 = 0x0012
	messageWake                 = 0x8000 + 0x473
	mouseHorizontalWheel        = 0x800
	mousePacketBytes            = 24
	mouseVerticalWheel          = 0x400
	// Used for noValue, commandPending, deviceMouse.
	noValue               = 0
	panControlID          = "pan"
	panUsage              = 0x238
	preparsedDataCommand  = 0x20000005
	queueAllInput         = 0x04ff
	registrationFlags     = 0x00000100 | 0x00002000 // INPUTSINK | DEVNOTIFY
	scanPausePrefix       = 0xe11d
	scanPower             = 0xe05e
	scanPrefixE0          = 0xe000
	scanPrefixE1          = 0xe100
	scanPrintScreenPrefix = 0xe02a
	scanPrintScreenSuffix = 0xe036
	scanSleep             = 0xe05f
	scanWake              = 0xe063
	// Used for keyboardFlagsOffset, keyExtendedE0, buttonFlagStride, mouseUsage, secondIndex,
	// wideCharBytes, commandCanceled, deviceHID, deviceRemoved.
	secondValue = 2
	// Used for keypadUsage, keyboardPage, dataEndOffset.
	seventhValue = 7
	// Used for keyBreak, mouseAbsolute, genericDesktopPage, singleValue, commandRunning, deviceKeyboard,
	// registrationRemove.
	singleValue = 1
	// Used for keyboardPacketBytes, mouseYOffset, wordBits.
	sixteenthValue = 16
	// Used for errorInvalidHandle, keyboardVirtualKeyOffset, mouseWheelOffset, dataIndexOffset,
	// keyboardUsage, deviceInfoWords.
	sixthValue       = 6
	systemPowerUsage = 0x81
	systemSleepUsage = 0x82
	systemWakeUsage  = 0x83
	// Used for fileShareReadWrite, openExisting, thirdIndex.
	thirdValue = 3
	// Used for errorSharingViolation, scalarBits, registrationPageOnly.
	thirtySecondValue = 32
	// Used for mouseXOffset, consumerPage.
	twelfthValue      = 12
	virtualScanPrefix = 0xff00
	wheelControlID    = "wheel"
	wheelDelta        = 120
	wheelUsage        = 0x38
	wordMask          = 0xffff

	// Scan tables list hexadecimal native codes and HID usages with human-readable labels.
	scanUsagesData = `
0001:0029 KeyEscape
0002:001e Key1
0003:001f Key2
0004:0020 Key3
0005:0021 Key4
0006:0022 Key5
0007:0023 Key6
0008:0024 Key7
0009:0025 Key8
000a:0026 Key9
000b:0027 Key0
000c:002d KeyMinus
000d:002e KeyEqual
000e:002a KeyBackspace
000f:002b KeyTab
0010:0014 KeyQ
0011:001a KeyW
0012:0008 KeyE
0013:0015 KeyR
0014:0017 KeyT
0015:001c KeyY
0016:0018 KeyU
0017:000c KeyI
0018:0012 KeyO
0019:0013 KeyP
001a:002f KeyLeftBracket
001b:0030 KeyRightBracket
001c:0028 KeyEnter
001d:00e0 KeyLeftControl
001e:0004 KeyA
001f:0016 KeyS
0020:0007 KeyD
0021:0009 KeyF
0022:000a KeyG
0023:000b KeyH
0024:000d KeyJ
0025:000e KeyK
0026:000f KeyL
0027:0033 KeySemicolon
0028:0034 KeyApostrophe
0029:0035 KeyGrave
002a:00e1 KeyLeftShift
002b:0031 KeyBackslash
002c:001d KeyZ
002d:001b KeyX
002e:0006 KeyC
002f:0019 KeyV
0030:0005 KeyB
0031:0011 KeyN
0032:0010 KeyM
0033:0036 KeyComma
0034:0037 KeyPeriod
0035:0038 KeySlash
0036:00e5 KeyRightShift
0037:0055 KeyKeypadMultiply
0038:00e2 KeyLeftAlt
0039:002c KeySpace
003a:0039 KeyCapsLock
003b:003a KeyF1
003c:003b KeyF2
003d:003c KeyF3
003e:003d KeyF4
003f:003e KeyF5
0040:003f KeyF6
0041:0040 KeyF7
0042:0041 KeyF8
0043:0042 KeyF9
0044:0043 KeyF10
0045:0053 KeyNumLock
0046:0047 KeyScrollLock
0047:005f KeyKeypad7
0048:0060 KeyKeypad8
0049:0061 KeyKeypad9
004a:0056 KeyKeypadSubtract
004b:005c KeyKeypad4
004c:005d KeyKeypad5
004d:005e KeyKeypad6
004e:0057 KeyKeypadAdd
004f:0059 KeyKeypad1
0050:005a KeyKeypad2
0051:005b KeyKeypad3
0052:0062 KeyKeypad0
0053:0063 KeyKeypadDecimal
0056:0064 KeyNonUSBackslash
0057:0044 KeyF11
0058:0045 KeyF12
0064:0068 KeyF13
0065:0069 KeyF14
0066:006a KeyF15
0067:006b KeyF16
0068:006c KeyF17
0069:006d KeyF18
006a:006e KeyF19
006b:006f KeyF20
006c:0070 KeyF21
006d:0071 KeyF22
006e:0072 KeyF23
006f:0073 KeyF24
0070:0088 keyboardInternational2Usage
0073:0087 keyboardInternational1Usage
0079:008a keyboardInternational4Usage
007b:008b keyboardInternational5Usage
007d:0089 keyboardInternational3Usage
007e:0085 keyboardKeypadCommaUsage
e01c:0058 KeyKeypadEnter
e01d:00e4 KeyRightControl
e035:0054 KeyKeypadDivide
e037:0046 KeyPrintScreen
e038:00e6 KeyRightAlt
e047:004a KeyHome
e048:0052 KeyUp
e049:004b KeyPageUp
e04b:0050 KeyLeft
e04d:004f KeyRight
e04f:004d KeyEnd
e050:0051 KeyDown
e051:004e KeyPageDown
e052:0049 KeyInsert
e053:004c KeyDelete
e05b:00e3 KeyLeftGUI
e05c:00e7 KeyRightGUI
e05d:0065 KeyApplication
e145:0048 KeyPause
`
	consumerScansData = `
e010:00b6 KeyPrevTrack
e019:00b5 KeyNextTrack
e020:00e2 KeyMute
e021:0192 consumerCalculatorUsage
e022:00cd KeyPlayPause
e024:00b7 KeyStop
e02e:00ea KeyVolumeDown
e030:00e9 KeyVolumeUp
e032:0223 consumerHomeUsage
e065:0221 consumerSearchUsage
e066:022a consumerBookmarksUsage
e067:0227 consumerRefreshUsage
e068:0226 consumerStopUsage
e069:0225 consumerForwardUsage
e06a:0224 consumerBackUsage
e06c:018a consumerEmailUsage
e06d:0183 consumerConfigurationUsage
`
	consumerVirtualKeysData = `
00a6:0224 consumerBackUsage
00a7:0225 consumerForwardUsage
00a8:0227 consumerRefreshUsage
00a9:0226 consumerStopUsage
00aa:0221 consumerSearchUsage
00ab:022a consumerBookmarksUsage
00ac:0223 consumerHomeUsage
00ad:00e2 KeyMute
00ae:00ea KeyVolumeDown
00af:00e9 KeyVolumeUp
00b0:00b5 KeyNextTrack
00b1:00b6 KeyPrevTrack
00b2:00b7 KeyStop
00b3:00cd KeyPlayPause
00b4:018a consumerEmailUsage
00b5:0183 consumerConfigurationUsage
`
)
