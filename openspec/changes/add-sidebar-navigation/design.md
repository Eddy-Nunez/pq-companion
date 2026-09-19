## Context

See proposal.md — Why. Current state that shapes this design:

- The reference keeps its sidebar state in three different places: hide/order/
  favorites in `config.yaml` preferences (via `useSidebarPrefs`), section collapse
  in `localStorage['sidebar_collapsed_sections']`, and scroll/hover ephemera in
  component state.
- The reference's `Layout.tsx` mounts the sidebar plus a set of hooks
  (`useAudioEngine`, `useTriggerClipboard`, `useTimerAlerts`, `useRespawnAlerts`,
  `useMetronomeAlerts`, `useLogFeedSubscriber`) that are deliberately only mounted
  for the main window, so overlay windows never fire duplicate alerts.
- The reference works around a React Router v7 scheduling bug with
  `useTransitions={false}`; the comment in `App.tsx` records that concurrent
  commit made navigation land one click late for external-store pages.
- `app-shell` (Wave 0) already provides the three layouts and the `:root`
  shell; this change fills the sidebar region.

## Goals / Non-Goals

**Goals:**

- The sidebar renders from one definition module shared with the settings editor.
- Active highlighting matches the reference's exact-vs-prefix semantics,
  including the two cases that look inconsistent but are deliberate.
- Persistent controls (character switcher, log status, back/forward) survive
  navigation without remounting or refetching.
- Hide / order / favorites / collapse all persist in user settings.
- Visually identical to the reference sidebar.

**Non-Goals:**

- Building the pages the items point at — they render placeholders until
  Waves 4–10.
- Overlay-window navigation.
- Drag-and-drop reordering. Server-side move controls ship here; a shared
  sortable hook lands later and is reused by trigger categories and wishlist
  slots.
- The global search dialog (⌘K / Ctrl+K). It is a sibling of the sidebar, not
  part of this capability; it ports with the game-database change.

## Decisions

### D1. One LiveView per route, with sticky children for the persistent controls

**Decision:** each route is its own `Phoenix.LiveView`, composed by the `:root`
layout. The controls that must outlive navigation are rendered with
`live_render(..., sticky: true)` as separate LiveViews.

**Why:** the alternative — one root LiveView that swaps content by `live_action`
— keeps everything mounted but grows into a god-module, and forces every feature
page to be a component of it rather than an independently testable LiveView.
Sticky children give exactly the property that matters (the sidebar and its
controls do not remount) without collapsing the page structure.

**Alternative considered:** a single root LiveView with `live_component` per page.
Rejected — it makes each page's `mount`/`handle_params` lifecycle the root's
concern, and the reference already has 62 independent pages.

**Note:** this also removes the `useTransitions={false}` workaround entirely. That
bug was an artifact of client-side concurrent routing between React's commit phase
and external stores; with server-rendered LiveView navigation there is no
equivalent hazard.

### D2. Plain path routing, not the reference's hash routing

**Decision:** plain paths (`/items`, `/combat/log`), not `/#/items`.

**Why:** the reference uses `HashRouter` because Electron loads a single
`file://` document and needs client-side routing to survive a reload without a
server. The LiveView app has a real HTTP server in every window, including
overlay windows, so hash routing buys nothing and costs the deep-link indirection
the reference needs (`window.electron.app.onNavigate` re-pushes a hash route into
the main window).

**Trade-off:** any externally recorded deep link using a hash form breaks. There
are none — the reference generates these links internally at runtime only.

### D3. Vendor the reference's icon paths instead of switching icon sets

**Decision:** port the reference's icon set into a `PQCompanionWeb.Components.NavIcons`
function component, one function per icon, using the same SVG paths.

**Why:** swapping to a different icon set (e.g. the `heroicons` package) means the
sidebar looks subtly different everywhere, and the sidebar is the surface users
stare at all day. Vendoring is ~32 small functions of static markup, has no
runtime cost, and needs no JS build step.

**Alternative considered:** a JS hook driving an icon library. Rejected —
reintroduces a client dependency for static art, which is the thing this
migration is removing.

### D4. Section collapse state moves from browser storage into user settings

**Decision:** collapse state becomes a persisted preference alongside hide/order/
favorites.

**Why:** it is genuine user state, not ephemera. `localStorage` is per-window and
per-origin, so in a multi-window desktop app the same user gets different
collapse states in different windows — and it does not survive a reinstall. The
reference already persists the *other* three sidebar preferences properly; this
makes the fourth consistent.

**Trade-off:** a round-trip per toggle. Mitigated by keeping the toggle
optimistic in the LiveView assign and persisting asynchronously.

### D5. Move controls first, drag-and-drop later

**Decision:** favorites reordering ships with explicit move-up/move-down controls
handled server-side. A `phx-hook` sortable is added later.

**Why:** it keeps this change at zero new JavaScript while still delivering
working reordering. The sortable hook is then written once and reused by trigger
categories and wishlist slots, which the reference implements with three separate
dnd-kit usage sites.

### D6. Active path is derived from `handle_params`, not from the request path

**Decision:** an `attach_hook(:active_path, :handle_params, ...)` records the
current path on the socket; the sidebar matches against that.

**Why:** it gives one code path for mount and for every subsequent navigation, so
highlighting cannot drift between initial render and a live navigation. It also
keeps the matching rule in one place rather than spreading route knowledge
through templates.

### D7. The definition is compile-time data in a module attribute

**Decision:** sections and items live in a module attribute in `PQCompanionWeb.Nav`, with
pure functions for the flag filter, ordering and favorites.

**Why:** the definition is static, so there is no reason to store or fetch it, and
compile-time data makes it inspectable in tests and safe in a release. The pure
functions mirror the reference's `visibleNavSections` / `favoriteItems` /
`orderItems` one-for-one, which makes the parity test a direct translation.

## Risks / Trade-offs

- **[Sticky children are re-mounted if their session changes]** → Keep the sticky
  children's assigns free of per-page state so their identity is stable; the
  character switcher's selection already lives in settings, not in the child.
- **[Collapse persisted server-side makes rapid toggling write-heavy]** → Toggle
  the assign immediately and debounce the persist; the reference writes settings
  on every change already, so this is no worse.
- **[Parity test could rot as items are added in later waves]** → The test asserts
  the definition and both consumers agree against a recorded inventory, so adding
  an item without updating the inventory fails loudly. Update the inventory in the
  same change that adds the item.
- **[Vendored icons add ~32 static functions]** → Bounded, no runtime cost, and it
  removes a dependency rather than adding one.
- **[Back/forward depends on LiveView's navigation history having the same
  granularity as the reference's]** → Covered by an explicit scenario; if
  granularity differs (e.g. `patch` vs `navigate`), resolve in the task that
  implements it rather than changing the spec.

## Migration Plan

1. Land `PQCompanionWeb.Nav` plus its unit tests; no UI yet. Both consumers can be pointed
   at it independently.
2. Add the sidebar to the `:root` layout and the settings editor to the settings
   page.
3. Add the sticky controls, replacing `useSidebarPrefs` and `useHistoryNav`.
4. Add the sidebar preference fields to the settings schema — additive only, no
   renames — and verify the reference app still loads the file.
5. Replace the reference's nav/shell Playwright smoke assertions with the
   LiveView parity test.

**Rollback:** the sidebar is additive to Wave 0's shell; reverting this change
leaves a shell with placeholder content. The only shared persistent artifact is
`config.yaml`, and the new keys are ignored by the reference app.

## Open Questions

- Should the log feed keep buffering while the user is on another tab? The
  reference deliberately keeps it subscribed in the main-window layout so the
  feed persists across navigation. In LiveView that suggests a background process
  owning the buffer rather than a LiveView. This does not change the navigation
  contract, so it is deferred to the log-pipeline change — but it is the reason
  the persistent controls are specced as "without refetching" rather than "in the
  sidebar".
- Should the settings navigation editor support reordering across sections, or
  only within a section? The reference only allows within-section ordering.
  Deferred; the parity inventory will record whichever the port implements.
