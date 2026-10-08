// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build windows && (amd64 || arm64)

package win32

const (
	angularUnits   = 0x14
	buttonIDFormat = "button:%d"
	buttonPage     = 9
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
	// Used for desktopXUsage, registrationPageMask.
	fortyEighthValue = 0x30
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
)
