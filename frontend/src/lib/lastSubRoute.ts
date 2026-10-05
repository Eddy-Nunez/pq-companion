// Remembers which child route the user last had open inside a section whose
// sub-pages are real routes (Combat, Raids), so clicking the section's sidebar
// button returns them to that sub-page instead of always the first one.
//
// Per-viewer UI preference, kept in localStorage; every access is guarded so a
// blocked/cleared store just means "no memory" and the plain section path is
// used.

const SECTION_CHILDREN: Record<string, readonly string[]> = {
  '/combat': ['/combat/log', '/combat/history'],
  '/raids': ['/raids', '/raids/check', '/raids/editor'],
}

const keyFor = (section: string): string => `pq-subroute:${section}`

// rememberSubRoute records pathname when it is a known child of a tracked
// section. Call on every navigation inside that section's layout.
export function rememberSubRoute(pathname: string): void {
  for (const [section, children] of Object.entries(SECTION_CHILDREN)) {
    if (children.includes(pathname)) {
      try {
        localStorage.setItem(keyFor(section), pathname)
      } catch {
        /* noop */
      }
      return
    }
  }
}

// resolveRememberedRoute maps a sidebar target to the section's last-used child
// route when one is stored; any other target passes through unchanged.
export function resolveRememberedRoute(to: string): string {
  const children = SECTION_CHILDREN[to]
  if (!children) return to
  try {
    const stored = localStorage.getItem(keyFor(to))
    if (stored && children.includes(stored)) return stored
  } catch {
    /* fall through */
  }
  return to
}
