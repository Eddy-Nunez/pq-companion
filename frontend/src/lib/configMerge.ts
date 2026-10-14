// Three-way merge for config saves. Settings components build their next
// config as `{ ...config, ...change }` from a copy fetched when they mounted,
// so a whole-object PUT also writes back every other field as it was then —
// silently reverting anything changed since by another window or by the
// backend itself (e.g. the active character cleared on login). Merging only
// the fields that differ between the caller's base and next onto a fresh
// server copy keeps the caller's edits without the stale rest.

type Plain = Record<string, unknown>

function isPlainObject(v: unknown): v is Plain {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function sameValue(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}

/**
 * mergeConfigEdits applies the edits that turn base into next onto fresh.
 * Nested objects merge key by key; arrays and scalars are replaced whole. A
 * key the caller removed (present in base, absent in next) is removed from
 * the result. Keys the caller didn't touch keep fresh's value.
 */
export function mergeConfigEdits<T>(fresh: T, base: T, next: T): T {
  return mergeValue(fresh, base, next) as T
}

function mergeValue(fresh: unknown, base: unknown, next: unknown): unknown {
  if (sameValue(base, next)) return fresh
  if (!isPlainObject(fresh) || !isPlainObject(base) || !isPlainObject(next)) {
    return next
  }
  const out: Plain = { ...fresh }
  for (const key of new Set([...Object.keys(base), ...Object.keys(next)])) {
    if (!(key in next)) {
      delete out[key]
    } else {
      out[key] = mergeValue(fresh[key], base[key], next[key])
    }
  }
  return out
}
