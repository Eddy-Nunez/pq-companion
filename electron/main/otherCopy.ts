import { execFile } from 'child_process'
import { basename, win32 } from 'path'

// Detects a copy of PQ Companion running from a DIFFERENT folder than this
// one. Used when this launch loses the single-instance lock: normally the
// lock holder is the same install (user clicked the shortcut twice) and we
// quit silently, but a stale extracted copy (e.g. an old setup folder left
// in Downloads) also holds the lock. Then the copy the auto-updater just
// installed quits on every launch and the user keeps running the old one
// without knowing (issue #160: a 0.17.6 copy masked 0.28.2 for weeks).
//
// The running copy may be an old build that predates any handshake, so we
// can't ask it anything. Instead we list processes by exe name via
// PowerShell/CIM, which needs no cooperation from the other copy.

export interface OtherCopy {
  path: string
  version: string // "" when the exe carries no readable version
}

const QUERY_TIMEOUT_MS = 5000

function normalizePath(p: string): string {
  return p.replace(/\//g, '\\').toLowerCase()
}

// "0.17.6.0" (Windows FileVersion padding) → "0.17.6".
function cleanVersion(v: string): string {
  const parts = v.trim().split('.')
  while (parts.length > 3 && parts[parts.length - 1] === '0') parts.pop()
  return parts.join('.')
}

export function findOtherRunningCopies(): Promise<OtherCopy[]> {
  if (process.platform !== 'win32') return Promise.resolve([])
  const exeName = basename(process.execPath).replace(/'/g, "''")
  const script =
    `Get-CimInstance Win32_Process -Filter "Name='${exeName}'" | ` +
    'ForEach-Object { $p = $_.ExecutablePath; if ($p) { ' +
    '$v = (Get-Item -LiteralPath $p).VersionInfo.ProductVersion; ' +
    '"$p|$v" } }'
  return new Promise((resolve) => {
    execFile(
      'powershell.exe',
      ['-NoProfile', '-NonInteractive', '-Command', script],
      { timeout: QUERY_TIMEOUT_MS, windowsHide: true },
      (err, stdout) => {
        if (err) {
          resolve([])
          return
        }
        const own = normalizePath(process.execPath)
        const seen = new Set<string>()
        const copies: OtherCopy[] = []
        for (const line of stdout.split(/\r?\n/)) {
          const sep = line.lastIndexOf('|')
          if (sep <= 0) continue
          const path = line.slice(0, sep).trim()
          const key = normalizePath(path)
          if (key === own || seen.has(key)) continue
          seen.add(key)
          copies.push({ path, version: cleanVersion(line.slice(sep + 1)) })
        }
        resolve(copies)
      },
    )
  })
}

// isOlderVersion reports whether a is strictly older than b (dotted numeric).
// Unparseable versions compare as not-older.
export function isOlderVersion(a: string, b: string): boolean {
  if (!a || !b) return false
  const pa = a.split('.').map(Number)
  const pb = b.split('.').map(Number)
  if (pa.some(isNaN) || pb.some(isNaN)) return false
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const x = pa[i] ?? 0
    const y = pb[i] ?? 0
    if (x !== y) return x < y
  }
  return false
}

export function otherCopyMessage(
  other: OtherCopy,
  ownVersion: string,
): { message: string; detail: string } {
  const otherLabel = other.version ? `v${other.version}` : 'another version'
  const older = other.version !== '' && isOlderVersion(other.version, ownVersion)
  const message = older
    ? `An older copy of PQ Companion (${otherLabel}) is already running`
    : 'Another copy of PQ Companion is already running from a different folder'
  const detail =
    `Running copy:\n${other.path}\n\n` +
    `This copy (v${ownVersion}) can't start while that one is open:\n` +
    `${process.execPath}\n\n` +
    'Updates install to this copy, so the other one will stay on ' +
    `${otherLabel} and miss every fix.\n\n` +
    'To fix it: quit the other copy (right-click its tray icon by the ' +
    'clock → Quit, or close its window), then start PQ Companion from the ' +
    'Start menu. If the other copy is an old download you no longer need, ' +
    `you can delete its folder:\n${win32.dirname(other.path)}\n\n` +
    'Your settings, characters and triggers are shared by both copies, so ' +
    'nothing is lost.'
  return { message, detail }
}
