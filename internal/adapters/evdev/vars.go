//go:build linux && (amd64 || arm64)

package evdev

import native "github.com/holoplot/go-evdev"

// These tables describe semantic equivalents, not reconstructed HID reports.
// In particular, KEY_BACKSLASH loses the distinction between HID 0x31/0x32.
var keyboardUsages = map[native.EvCode]uint16{
	native.KEY_A: 0x04, native.KEY_B: 0x05, native.KEY_C: 0x06,
	native.KEY_D: 0x07, native.KEY_E: 0x08, native.KEY_F: 0x09,
	native.KEY_G: 0x0a, native.KEY_H: 0x0b, native.KEY_I: 0x0c,
	native.KEY_J: 0x0d, native.KEY_K: 0x0e, native.KEY_L: 0x0f,
	native.KEY_M: 0x10, native.KEY_N: 0x11, native.KEY_O: 0x12,
	native.KEY_P: 0x13, native.KEY_Q: 0x14, native.KEY_R: 0x15,
	native.KEY_S: 0x16, native.KEY_T: 0x17, native.KEY_U: 0x18,
	native.KEY_V: 0x19, native.KEY_W: 0x1a, native.KEY_X: 0x1b,
	native.KEY_Y: 0x1c, native.KEY_Z: 0x1d,
	native.KEY_1: 0x1e, native.KEY_2: 0x1f, native.KEY_3: 0x20,
	native.KEY_4: 0x21, native.KEY_5: 0x22, native.KEY_6: 0x23,
	native.KEY_7: 0x24, native.KEY_8: 0x25, native.KEY_9: 0x26, native.KEY_0: 0x27,
	native.KEY_ENTER: 0x28, native.KEY_ESC: 0x29, native.KEY_BACKSPACE: 0x2a,
	native.KEY_TAB: 0x2b, native.KEY_SPACE: 0x2c, native.KEY_MINUS: 0x2d,
	native.KEY_EQUAL: 0x2e, native.KEY_LEFTBRACE: 0x2f, native.KEY_RIGHTBRACE: 0x30,
	native.KEY_BACKSLASH: 0x31, native.KEY_SEMICOLON: 0x33, native.KEY_APOSTROPHE: 0x34,
	native.KEY_GRAVE: 0x35, native.KEY_COMMA: 0x36, native.KEY_DOT: 0x37,
	native.KEY_SLASH: 0x38, native.KEY_CAPSLOCK: 0x39,
	native.KEY_F1: 0x3a, native.KEY_F2: 0x3b, native.KEY_F3: 0x3c,
	native.KEY_F4: 0x3d, native.KEY_F5: 0x3e, native.KEY_F6: 0x3f,
	native.KEY_F7: 0x40, native.KEY_F8: 0x41, native.KEY_F9: 0x42,
	native.KEY_F10: 0x43, native.KEY_F11: 0x44, native.KEY_F12: 0x45,
	native.KEY_SYSRQ: 0x46, native.KEY_SCROLLLOCK: 0x47, native.KEY_PAUSE: 0x48,
	native.KEY_INSERT: 0x49, native.KEY_HOME: 0x4a, native.KEY_PAGEUP: 0x4b,
	native.KEY_DELETE: 0x4c, native.KEY_END: 0x4d, native.KEY_PAGEDOWN: 0x4e,
	native.KEY_RIGHT: 0x4f, native.KEY_LEFT: 0x50, native.KEY_DOWN: 0x51, native.KEY_UP: 0x52,
	native.KEY_NUMLOCK: 0x53, native.KEY_KPSLASH: 0x54, native.KEY_KPASTERISK: 0x55,
	native.KEY_KPMINUS: 0x56, native.KEY_KPPLUS: 0x57, native.KEY_KPENTER: 0x58,
	native.KEY_KP1: 0x59, native.KEY_KP2: 0x5a, native.KEY_KP3: 0x5b,
	native.KEY_KP4: 0x5c, native.KEY_KP5: 0x5d, native.KEY_KP6: 0x5e,
	native.KEY_KP7: 0x5f, native.KEY_KP8: 0x60, native.KEY_KP9: 0x61,
	native.KEY_KP0: 0x62, native.KEY_KPDOT: 0x63,
	native.KEY_102ND: 0x64, native.KEY_COMPOSE: 0x65, native.KEY_KPEQUAL: 0x67,
	native.KEY_F13: 0x68, native.KEY_F14: 0x69, native.KEY_F15: 0x6a,
	native.KEY_F16: 0x6b, native.KEY_F17: 0x6c, native.KEY_F18: 0x6d,
	native.KEY_F19: 0x6e, native.KEY_F20: 0x6f, native.KEY_F21: 0x70,
	native.KEY_F22: 0x71, native.KEY_F23: 0x72, native.KEY_F24: 0x73,
	native.KEY_HELP: 0x75, native.KEY_MENU: 0x76, native.KEY_SELECT: 0x77,
	native.KEY_STOP: 0x78, native.KEY_AGAIN: 0x79, native.KEY_UNDO: 0x7a,
	native.KEY_CUT: 0x7b, native.KEY_COPY: 0x7c, native.KEY_PASTE: 0x7d, native.KEY_FIND: 0x7e,
	native.KEY_KPCOMMA: 0x85, native.KEY_RO: 0x87, native.KEY_KATAKANAHIRAGANA: 0x88,
	native.KEY_YEN: 0x89, native.KEY_HENKAN: 0x8a, native.KEY_MUHENKAN: 0x8b,
	native.KEY_KPJPCOMMA: 0x8c, native.KEY_HANGEUL: 0x90, native.KEY_HANJA: 0x91,
	native.KEY_KATAKANA: 0x92, native.KEY_HIRAGANA: 0x93, native.KEY_ZENKAKUHANKAKU: 0x94,
	native.KEY_LEFTCTRL: 0xe0, native.KEY_LEFTSHIFT: 0xe1, native.KEY_LEFTALT: 0xe2,
	native.KEY_LEFTMETA: 0xe3, native.KEY_RIGHTCTRL: 0xe4, native.KEY_RIGHTSHIFT: 0xe5,
	native.KEY_RIGHTALT: 0xe6, native.KEY_RIGHTMETA: 0xe7,
}

var consumerUsages = map[native.EvCode]uint16{
	native.KEY_PLAY: 0xb0, native.KEY_PAUSECD: 0xb1, native.KEY_RECORD: 0xb2,
	native.KEY_FASTFORWARD: 0xb3, native.KEY_REWIND: 0xb4,
	native.KEY_NEXTSONG: 0xb5, native.KEY_PREVIOUSSONG: 0xb6,
	native.KEY_STOPCD: 0xb7, native.KEY_EJECTCD: 0xb8, native.KEY_PLAYPAUSE: 0xcd,
	native.KEY_MUTE: 0xe2, native.KEY_VOLUMEUP: 0xe9, native.KEY_VOLUMEDOWN: 0xea,
	native.KEY_CALC: 0x192, native.KEY_MAIL: 0x18a,
	native.KEY_SEARCH: 0x221, native.KEY_HOMEPAGE: 0x223,
	native.KEY_BACK: 0x224, native.KEY_FORWARD: 0x225,
	native.KEY_REFRESH: 0x227, native.KEY_BOOKMARKS: 0x22a,
}
