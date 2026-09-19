## 1. Canonical navigation definition

- [x] 1.1 Create `PQCompanionWeb.Nav` with compile-time sections and items, and an item struct carrying route, label, icon, optional flag and exact-match marker (design D7); verify `mix test` covers a unit test asserting four sections and the recorded item inventory (reference: `NAV_SECTIONS` in `frontend/src/lib/sidebarNav.tsx`, 152 LOC) — **VERIFIED 2026-09-18:** `PQCompanionWeb.Nav` + `PQCompanionWeb.Nav.Item` (`phoenix/lib/pq_companion_web/nav.ex`). `nav_test.exs` asserts four sections in fixed order, 34 items, unique routes, and the `Nav.Item` field shape — 21 tests, 0 failures (full suite 99).
- [x] 1.2 Implement the flag filter as a pure function, dropping items whose flag is absent or false and then dropping emptied sections; verify unit tests cover flag-off, flag-on, absent key, and a section emptied to zero items — **VERIFIED 2026-09-18:** `visible_sections/1` + `flags/1`. Tests cover flag-off, flag-on, absent key (`%{}`), and the Raids section emptied to zero items and dropped.
- [x] 1.3 Implement within-section ordering as a pure function that places listed items first and leaves unlisted items in their default relative order; verify unit tests cover partial order lists, empty order lists, and unknown keys in the order list (reference: `orderItems` / the `Number.MAX_SAFE_INTEGER` rank sentinel) — **VERIFIED 2026-09-18:** `order_items/2`. Tests cover a partial order list, an empty list, unknown keys, duplicate keys, and keys belonging to another section.
- [x] 1.4 Implement favorites resolution as a pure function over the already-flag-filtered item list; verify a unit test asserts a favorited item whose flag is disabled is not returned — **VERIFIED 2026-09-18:** `favorite_items/3`. Tests cover ranking by order, empty favorites, and the required case — a favorited item whose flag is disabled is not returned.
- [x] 1.5 Record the parity inventory: section ids, item routes in order, label text, icon name, and flag for each item; verify a test asserts the definition matches the inventory exactly, so an item added without updating the inventory fails (this is the anti-rot guard) — **VERIFIED 2026-09-18:** the `@inventory` fixture (34 rows of section, route, label, icon, flag, exact_match) is asserted equal to the definition in order. Editing `Nav` without updating the inventory fails. **Note:** every row records `exact_match: false` — see task 3.2.

## 2. Sidebar rendering

- [ ] 2.1 Implement the sidebar function component rendering sections, labels and links from `PQCompanionWeb.Nav`; verify a test asserts every inventory item renders its label and links to its route
- [ ] 2.2 Mount the sidebar in the `:root` layout and verify an overlay route renders without it (extends the Wave 0 `app-shell` layout guarantee)
- [ ] 2.3 Implement collapsible sections; verify a test asserts a collapsed section hides its items and an expanded one shows them, and that a section emptied by flags or preferences renders no label and no collapse control
- [ ] 2.4 Render the Favorites group at the top when at least one favorite exists, and render no group when none exists; verify both cases with tests
- [ ] 2.5 Collect the reference's icon paths into `PQCompanionWeb.Components.NavIcons` (design D3); verify every inventory item resolves to an icon and a test asserts no item references a missing icon

## 3. Active-item highlighting

- [ ] 3.1 Attach a `handle_params` hook recording the active path on the socket (design D6); verify a test asserts the assign updates on a live navigation, not only on mount
- [ ] 3.2 Implement exact-match highlighting for items marked exact-match; verify a test asserts the parent is unhighlighted while a child route that owns its own sidebar entry is active (reference: the `end` prop on the Raids item) — ⚠️ **the reference citation is stale:** the reference now sets `end` on **no** item. Commit `fbb09919` (*"drop duplicate Raid Editor entry from left nav"*) deleted the `/raids/editor` row and `end: true` on `/raids` in the same edit, so every item prefix-matches (`/raids/editor` keeps `/raids` lit, exactly like `/combat/log` keeps `/combat` lit). Implement the exact-match *capability*, but the test must use a synthetic `%Nav.Item{exact_match: true}`. Plan §4.5 criterion 3, the proposal's "preserved exactly" line and the spec scenario are stale pending the owner's sign-off — see the Wave 1 session report.
- [ ] 3.3 Implement prefix-match highlighting for items not marked exact-match, requiring the next character to be a path separator; verify tests cover a prefix route whose children are not sidebar rows, and a near-miss prefix that must not match

## 4. Persistent controls

- [ ] 4.1 Render the character switcher as a sticky LiveView; verify a test asserts the selection is unchanged after navigating between two tabs, and that the switcher does not re-query on navigation (reference: `CharacterSwitcher.tsx`, 181 LOC)
- [ ] 4.2 Render the log status indicator as a sticky LiveView owning a single poll loop; verify a test asserts exactly one poll loop exists after several navigations
- [ ] 4.3 Implement back/forward over in-app history; verify a test navigates several tabs and asserts back and forward restore the expected routes
- [ ] 4.4 Verify the controls are absent from every overlay route: assert zero control outlets rendered across the 16 overlay routes

## 5. Navigation preferences

- [ ] 5.1 Add the sidebar preference fields — hidden items, per-section order, favorites, and per-section collapse state — to the settings schema, additively and without renaming any existing key (design D4); verify a settings round-trip test asserts every pre-existing key is still present with the same value
- [ ] 5.2 Wire hide and order preferences into the sidebar render; verify a test asserts a hidden item disappears and a custom order is applied, and both survive a simulated restart
- [ ] 5.3 Wire favorites and implement move-up/move-down reordering server-side (design D5); verify a test asserts the order changes and persists, with no new JavaScript required for this change
- [ ] 5.4 Wire collapse state to settings with an optimistic assign; verify a test asserts collapse survives reload, and that rapid toggling converges to the last value
- [ ] 5.5 Verify rollback compatibility: after saving sidebar preferences, load the settings file with the reference application's parser and assert it parses and its own settings are unchanged

## 6. Settings navigation editor

- [ ] 6.1 Implement the settings navigation editor from the same `PQCompanionWeb.Nav` definition, offering hide, order and favorite controls per item; verify a test asserts the editor and the sidebar expose the same items for every flag combination
- [ ] 6.2 Verify a change made in the editor is reflected in the sidebar without a reload

## 7. Wave 1 verification

- [ ] 7.1 Walk the six Wave 1 acceptance criteria in `docs/phoenix-migration-plan.md` §4.5 and record the result of each
- [ ] 7.2 Replace the reference's nav assertions in `e2e/smoke/app-smoke.spec.ts` (206 LOC, 7 tests — shell render + nav + collapsible sections) with an equivalent LiveView test; verify the new test covers shell render, nav presence, and section collapse
- [ ] 7.3 Verify no file under `backend/`, `frontend/` or `electron/` was modified: `git diff --stat` against the reference trees is empty
