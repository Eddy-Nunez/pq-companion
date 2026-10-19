import type React from 'react'

// EverQuest keybind strings, as eqclient.ini spells them.
//
// Confirmed from a real file (Riz, 2026-10-19): one modifier is written
// SHIFT+, CONTROL+ or ALT+ before the key; letters are lowercase ("l"); other
// keys seen so far are digits, F1..F12, TAB, SPACE, INSERT and "\". How EQ
// orders two modifiers together has not been seen, so capture refuses combos
// rather than guess — a spelling EQ cannot parse is dropped silently in game.
// Mirrors bindKeyRe in backend/internal/eqconfig/keymaps.go, which re-checks.

const MOD_LABEL: Record<string, string> = {
  SHIFT: 'Shift',
  CONTROL: 'Ctrl',
  ALT: 'Alt',
}

const KEY_LABEL: Record<string, string> = {
  TAB: 'Tab',
  SPACE: 'Space',
  INSERT: 'Insert',
}

// formatKeybind turns "SHIFT+0" into "Shift+0" and "l" into "L". Anything
// unrecognised is shown as written, so a bind set in game always displays.
export function formatKeybind(raw: string): string {
  if (!raw) return ''
  return raw
    .split('+')
    .map((part) => {
      const up = part.toUpperCase()
      if (MOD_LABEL[up]) return MOD_LABEL[up]
      if (KEY_LABEL[up]) return KEY_LABEL[up]
      return part.length === 1 ? part.toUpperCase() : part
    })
    .join('+')
}

export type CaptureResult =
  | { ok: true; value: string }
  | { ok: false; reason: string }

// keybindFromEvent converts a keydown into EQ's spelling. Returns null while
// only modifiers are held, so the caller keeps listening for the real key.
export function keybindFromEvent(e: KeyboardEvent | React.KeyboardEvent): CaptureResult | null {
  const code = e.code
  if (/^(Shift|Control|Alt|Meta)(Left|Right)$/.test(code)) return null

  let key: string | null = null
  let m: RegExpMatchArray | null
  if ((m = code.match(/^Key([A-Z])$/))) key = m[1].toLowerCase()
  else if ((m = code.match(/^Digit([0-9])$/))) key = m[1]
  else if ((m = code.match(/^F([1-9]|1[0-2])$/))) key = `F${m[1]}`
  else if (code === 'Tab') key = 'TAB'
  else if (code === 'Space') key = 'SPACE'
  else if (code === 'Insert') key = 'INSERT'
  else if (code === 'Backslash') key = '\\'

  if (!key) {
    return {
      ok: false,
      reason:
        'EverQuest’s name for that key isn’t confirmed yet. Use a letter, ' +
        'number, F1–F12, Tab, Space, Insert or \\, or set it in game.',
    }
  }

  const mods = [e.shiftKey && 'SHIFT', e.ctrlKey && 'CONTROL', e.altKey && 'ALT']
    .filter(Boolean) as string[]
  if (e.metaKey) {
    return { ok: false, reason: 'EverQuest has no Windows/Cmd-key binds.' }
  }
  if (mods.length > 1) {
    return {
      ok: false,
      reason:
        'Only one modifier (Shift, Ctrl or Alt) can be set from here. Set ' +
        'combinations in game.',
    }
  }
  return { ok: true, value: mods.length ? `${mods[0]}+${key}` : key }
}
