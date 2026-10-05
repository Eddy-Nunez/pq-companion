import { useCallback, useState } from 'react'

/**
 * useState for a string-literal value (a page's active sub-tab) that survives
 * unmount and app restarts via localStorage, so leaving a page and coming back
 * returns the user to the sub-tab they were last on.
 *
 * `allowed` guards against a stale stored value (a tab that was renamed,
 * removed or is now gated off) — anything not in the list falls back to
 * `initial`. Storage failures (private mode, blocked site data) are swallowed
 * and the state simply behaves like plain useState.
 */
export function usePersistentState<T extends string>(
  key: string,
  initial: T,
  allowed: readonly T[],
): [T, (next: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const stored = localStorage.getItem(key)
      if (stored !== null && (allowed as readonly string[]).includes(stored)) {
        return stored as T
      }
    } catch {
      /* fall through to the default */
    }
    return initial
  })

  const set = useCallback(
    (next: T) => {
      setValue(next)
      try {
        localStorage.setItem(key, next)
      } catch {
        /* noop */
      }
    },
    [key],
  )

  return [value, set]
}
