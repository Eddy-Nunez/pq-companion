## Why

The sidebar is the app's primary navigation and the surface users interact with
most often, which makes it the right first real port: it is small, fully
specified by an existing single source of truth, and it exercises the
layout/component/live-navigation model before any domain porting depends on it
(`docs/phoenix-migration-plan.md`, **Wave 1**).

The reference already solved the hard part: `sidebarNav.tsx` is the canonical tab
definition shared by the live sidebar and the Settings → Navigation editor, which
is why the two never drift. This change preserves that property, and ports the
user's hide/order/favorites/collapse preferences with it.

## What Changes

- Port the canonical navigation definition to `PQCompanionWeb.Nav`, with the same four
  sections and the same items, section rules and flag gating.
- Render the sidebar from that definition, with active-item highlighting that
  preserves the reference's exact-vs-prefix semantics.
- Make the persistent controls — character switcher, log-status indicator,
  back/forward history — survive navigation without remounting or refetching.
- Port hide / order / favorites, and move section collapse state from
  browser-local storage into the persisted user settings so it survives a
  reinstall and behaves consistently across windows.
- Render favorites at the top of the sidebar, reorderable.
- Point the Settings → Navigation editor at the same definition module.
- Vendor the reference's icon set so the sidebar is visually identical.

Not in this change: any page behind a nav item (Waves 4–10 render placeholders),
overlay navigation, and drag-and-drop reordering (server-side move controls
first; a shared sortable hook lands later and is reused by trigger categories and
wishlist slots).

## Capabilities

### New Capabilities

- `navigation`: The application's primary side navigation — its canonical
  definition, section and flag rules, active-item highlighting, the persistent
  controls that outlive navigation, and the user preferences that shape it.

### Modified Capabilities

None.

## Impact

- **Affected specs:** `navigation`
- **Affected code:** new tree `phoenix/`; new modules `PQCompanionWeb.Nav`,
  `PQCompanionWeb.Components.Sidebar`, `PQCompanionWeb.Components.NavIcons`,
  `PQCompanionWeb.SidebarControlsLive`, `PQCompanionWeb.Settings.NavSettings`; extends
  `PQCompanion.Config` with the sidebar preference fields
- **Ported from (frozen reference, read-only):**
  - `frontend/src/lib/sidebarNav.tsx` — 152 LOC, 0 test LOC; the canonical
    definition, `visibleNavSections`, `favoriteItems`, `orderItems`
  - `frontend/src/components/Sidebar.tsx` — 219 LOC, 0 test LOC; rendering,
    collapse state, favorites, fixed controls
  - `frontend/src/components/settings/SidebarNavSettings.tsx` — 208 LOC, 0 test
    LOC; the second consumer of the canonical definition
  - `frontend/src/components/CharacterSwitcher.tsx` — 181 LOC, 0 test LOC;
    the sticky control
  - `frontend/src/components/Layout.tsx` — 61 LOC; sidebar/content composition
  - `frontend/src/hooks/useSidebarPrefs.ts` — 45 LOC; preference wiring
  - `frontend/src/hooks/useHistoryNav.ts` — 63 LOC; back/forward, replaced by
    LiveView navigation history
  - `e2e/smoke/app-smoke.spec.ts` — 206 LOC, 7 tests; its nav/shell assertions
    are the seed for this change's parity test
- **Reference behavior that must be preserved exactly:** three flag-gated items
  (`pop_flags_enabled`, `faction_tracker_enabled`, `raids_enabled`); sections with
  no visible items disappear entirely; **no item uses exact-match highlighting** —
  reference commit `fbb09919` dropped the `/raids/editor` sidebar row and the
  `end: true` on `/raids` in the same edit, so `/raids/editor` keeps `/raids` lit
  exactly as `/combat/log` keeps `/combat` lit. The exact-match capability is
  retained and tested with a synthetic item, but no current item sets it.
- **User-data impact:** the reference stores hide/order/favorites in
  `config.yaml` and collapse state in browser-local storage. Collapse moving into
  `config.yaml` adds keys; it does not rename or remove any existing key, so the
  Go app can still read the file.
- **Risk:** low. No native code, no game logic, no database schema beyond the
  added preference keys.
