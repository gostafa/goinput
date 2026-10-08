// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

//go:build linux && (amd64 || arm64)

package evdev

import (
	"github.com/gostafa/goinput/internal/domain"
	native "github.com/holoplot/go-evdev"
)

func keyboardUsage(code native.EvCode) domain.Usage {
	lookups := keyboardLookups()
	for idx := range lookups {
		usage := lookups[idx](code)
		if usage != domain.UsageUnknown {
			return usage
		}
	}

	return domain.UsageUnknown
}

func keyboardLookups() []usageLookup { return append(keyboardFirstLookups(), keyboardLastLookups()...) }

func keyboardFirstLookups() []usageLookup {
	return []usageLookup{
		keyboardUsage0,
		keyboardUsage1,
		keyboardUsage2,
		keyboardUsage3,
		keyboardUsage4,
		keyboardUsage5,
		keyboardUsage6,
		keyboardUsage7,
		keyboardUsage8,
		keyboardUsage9,
		keyboardUsage10,
		keyboardUsage11,
		keyboardUsage12,
		keyboardUsage13,
		keyboardUsage14,
		keyboardUsage15,
		keyboardUsage16,
	}
}

func keyboardLastLookups() []usageLookup {
	return []usageLookup{
		keyboardUsage17,
		keyboardUsage18,
		keyboardUsage19,
		keyboardUsage20,
		keyboardUsage21,
		keyboardUsage22,
		keyboardUsage23,
		keyboardUsage24,
		keyboardUsage25,
		keyboardUsage26,
		keyboardUsage27,
		keyboardUsage28,
		keyboardUsage29,
		keyboardUsage30,
		keyboardUsage31,
		keyboardUsage32,
		keyboardUsage33,
		keyboardUsage34,
	}
}

func keyboardUsage0(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_A: nativeFour,
		native.KEY_B: nativeFive,
		native.KEY_C: nativeSix,
		native.KEY_D: nativeSeven,
	}[code])
}

func keyboardUsage1(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_E: usage8,
		native.KEY_F: nativeNine,
		native.KEY_G: usageA,
		native.KEY_H: usageB,
	}[code])
}

func keyboardUsage2(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_I: nativeTwelve,
		native.KEY_J: usageD,
		native.KEY_K: usageE,
		native.KEY_L: usageF,
	}[code])
}

func keyboardUsage3(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_M: usage10,
		native.KEY_N: nativeSeventeen,
		native.KEY_O: usage12,
		native.KEY_P: usage13,
	}[code])
}

func keyboardUsage4(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_Q: usage14,
		native.KEY_R: usage15,
		native.KEY_S: usage16,
		native.KEY_T: usage17,
	}[code])
}

func keyboardUsage5(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_U: nativeTwentyFour,
		native.KEY_V: usage19,
		native.KEY_W: usage1A,
		native.KEY_X: usage1B,
	}[code])
}

func keyboardUsage6(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_Y: usage1C,
		native.KEY_Z: usage1D,
		native.KEY_1: usage1E,
		native.KEY_2: usage1F,
	}[code])
}

func keyboardUsage7(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_3: usage20,
		native.KEY_4: usage21,
		native.KEY_5: usage22,
		native.KEY_6: usage23,
	}[code])
}

func keyboardUsage8(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_7: usage24,
		native.KEY_8: usage25,
		native.KEY_9: usage26,
		native.KEY_0: usage27,
	}[code])
}

func keyboardUsage9(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_ENTER:     usage28,
		native.KEY_ESC:       usage29,
		native.KEY_BACKSPACE: usage2A,
		native.KEY_TAB:       usage2B,
	}[code])
}

func keyboardUsage10(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_SPACE:     usage2C,
		native.KEY_MINUS:     usage2D,
		native.KEY_EQUAL:     usage2E,
		native.KEY_LEFTBRACE: usage2F,
	}[code])
}

func keyboardUsage11(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_RIGHTBRACE: axisUsageBase,
		native.KEY_BACKSLASH:  usage31,
		native.KEY_SEMICOLON:  usage33,
		native.KEY_APOSTROPHE: usage34,
	}[code])
}

func keyboardUsage12(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_GRAVE: usage35,
		native.KEY_COMMA: usage36,
		native.KEY_DOT:   dialUsage,
		native.KEY_SLASH: wheelUsage,
	}[code])
}

func keyboardUsage13(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_CAPSLOCK: hatUsage,
		native.KEY_F1:       usage3A,
		native.KEY_F2:       usage3B,
		native.KEY_F3:       usage3C,
	}[code])
}

func keyboardUsage14(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_F4: usage3D,
		native.KEY_F5: usage3E,
		native.KEY_F6: usage3F,
		native.KEY_F7: usage40,
	}[code])
}

func keyboardUsage15(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_F8:  usage41,
		native.KEY_F9:  usage42,
		native.KEY_F10: usage43,
		native.KEY_F11: usage44,
	}[code])
}

func keyboardUsage16(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_F12:        usage45,
		native.KEY_SYSRQ:      usage46,
		native.KEY_SCROLLLOCK: usage47,
		native.KEY_PAUSE:      usage48,
	}[code])
}

func keyboardUsage17(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_INSERT: usage49,
		native.KEY_HOME:   usage4A,
		native.KEY_PAGEUP: usage4B,
		native.KEY_DELETE: usage4C,
	}[code])
}

func keyboardUsage18(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_END:      usage4D,
		native.KEY_PAGEDOWN: usage4E,
		native.KEY_RIGHT:    usage4F,
		native.KEY_LEFT:     usage50,
	}[code])
}

func keyboardUsage19(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_DOWN:    usage51,
		native.KEY_UP:      usage52,
		native.KEY_NUMLOCK: usage53,
		native.KEY_KPSLASH: usage54,
	}[code])
}

func keyboardUsage20(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_KPASTERISK: usage55,
		native.KEY_KPMINUS:    usage56,
		native.KEY_KPPLUS:     usage57,
		native.KEY_KPENTER:    usage58,
	}[code])
}

func keyboardUsage21(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_KP1: usage59,
		native.KEY_KP2: usage5A,
		native.KEY_KP3: usage5B,
		native.KEY_KP4: usage5C,
	}[code])
}

func keyboardUsage22(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_KP5: usage5D,
		native.KEY_KP6: usage5E,
		native.KEY_KP7: usage5F,
		native.KEY_KP8: usage60,
	}[code])
}

func keyboardUsage23(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_KP9:   usage61,
		native.KEY_KP0:   usage62,
		native.KEY_KPDOT: usage63,
		native.KEY_102ND: usage64,
	}[code])
}

func keyboardUsage24(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_COMPOSE: usage65,
		native.KEY_KPEQUAL: usage67,
		native.KEY_F13:     usage68,
		native.KEY_F14:     usage69,
	}[code])
}

func keyboardUsage25(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_F15: usage6A,
		native.KEY_F16: usage6B,
		native.KEY_F17: usage6C,
		native.KEY_F18: usage6D,
	}[code])
}

func keyboardUsage26(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_F19: usage6E,
		native.KEY_F20: usage6F,
		native.KEY_F21: usage70,
		native.KEY_F22: usage71,
	}[code])
}

func keyboardUsage27(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_F23:  usage72,
		native.KEY_F24:  usage73,
		native.KEY_HELP: usage75,
		native.KEY_MENU: usage76,
	}[code])
}

func keyboardUsage28(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_SELECT: usage77,
		native.KEY_STOP:   wheelDetent,
		native.KEY_AGAIN:  usage79,
		native.KEY_UNDO:   usage7A,
	}[code])
}

func keyboardUsage29(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_CUT:   usage7B,
		native.KEY_COPY:  usage7C,
		native.KEY_PASTE: usage7D,
		native.KEY_FIND:  usage7E,
	}[code])
}

func keyboardUsage30(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_KPCOMMA:          usage85,
		native.KEY_RO:               usage87,
		native.KEY_KATAKANAHIRAGANA: usage88,
		native.KEY_YEN:              usage89,
	}[code])
}

func keyboardUsage31(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_HENKAN:    usage8A,
		native.KEY_MUHENKAN:  usage8B,
		native.KEY_KPJPCOMMA: usage8C,
		native.KEY_HANGEUL:   usage90,
	}[code])
}

func keyboardUsage32(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_HANJA:          usage91,
		native.KEY_KATAKANA:       usage92,
		native.KEY_HIRAGANA:       usage93,
		native.KEY_ZENKAKUHANKAKU: usage94,
	}[code])
}

func keyboardUsage33(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_LEFTCTRL:  usageE0,
		native.KEY_LEFTSHIFT: usageE1,
		native.KEY_LEFTALT:   usageE2,
		native.KEY_LEFTMETA:  usageE3,
	}[code])
}

func keyboardUsage34(code native.EvCode) domain.Usage {
	return hidUsage(nativeSeven, map[native.EvCode]uint16{
		native.KEY_RIGHTCTRL:  usageE4,
		native.KEY_RIGHTSHIFT: usageE5,
		native.KEY_RIGHTALT:   usageE6,
		native.KEY_RIGHTMETA:  usageE7,
	}[code])
}

func consumerUsage(code native.EvCode) domain.Usage {
	lookups := consumerLookups()
	for idx := range lookups {
		usage := lookups[idx](code)
		if usage != domain.UsageUnknown {
			return usage
		}
	}

	return domain.UsageUnknown
}

func consumerLookups() []usageLookup {
	return []usageLookup{
		consumerUsage0,
		consumerUsage1,
		consumerUsage2,
		consumerUsage3,
		consumerUsage4,
		consumerUsage5,
	}
}

func consumerUsage0(code native.EvCode) domain.Usage {
	return hidUsage(nativeTwelve, map[native.EvCode]uint16{
		native.KEY_PLAY:        usageB0,
		native.KEY_PAUSECD:     usageB1,
		native.KEY_RECORD:      usageB2,
		native.KEY_FASTFORWARD: usageB3,
	}[code])
}

func consumerUsage1(code native.EvCode) domain.Usage {
	return hidUsage(nativeTwelve, map[native.EvCode]uint16{
		native.KEY_REWIND:       usageB4,
		native.KEY_NEXTSONG:     usageB5,
		native.KEY_PREVIOUSSONG: usageB6,
		native.KEY_STOPCD:       usageB7,
	}[code])
}

func consumerUsage2(code native.EvCode) domain.Usage {
	return hidUsage(nativeTwelve, map[native.EvCode]uint16{
		native.KEY_EJECTCD:   usageB8,
		native.KEY_PLAYPAUSE: usageCD,
		native.KEY_MUTE:      usageE2,
		native.KEY_VOLUMEUP:  usageE9,
	}[code])
}

func consumerUsage3(code native.EvCode) domain.Usage {
	return hidUsage(nativeTwelve, map[native.EvCode]uint16{
		native.KEY_VOLUMEDOWN: usageEA,
		native.KEY_CALC:       usage192,
		native.KEY_MAIL:       usage18A,
		native.KEY_SEARCH:     usage221,
	}[code])
}

func consumerUsage4(code native.EvCode) domain.Usage {
	return hidUsage(nativeTwelve, map[native.EvCode]uint16{
		native.KEY_HOMEPAGE: usage223,
		native.KEY_BACK:     usage224,
		native.KEY_FORWARD:  usage225,
		native.KEY_REFRESH:  usage227,
	}[code])
}

func consumerUsage5(code native.EvCode) domain.Usage {
	return hidUsage(nativeTwelve, map[native.EvCode]uint16{
		native.KEY_BOOKMARKS: usage22A,
	}[code])
}
