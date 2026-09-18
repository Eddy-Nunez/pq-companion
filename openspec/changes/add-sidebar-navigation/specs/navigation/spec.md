## Purpose

Provides the application's primary side navigation: a single canonical
definition of the tabs, the section and feature-flag rules that decide which are
visible, correct active-item highlighting, the controls that persist across
navigation, and the user preferences that shape the sidebar's contents and order.

## ADDED Requirements

### Requirement: One canonical navigation definition serves every consumer

The application SHALL define the sidebar's sections and items in exactly one
module, and SHALL render the sidebar and the settings navigation editor from that
same module, so the two surfaces cannot disagree about which tabs exist.

#### Scenario: Both consumers agree

- **WHEN** the sidebar and the settings navigation editor are rendered in the same
  session
- **THEN** they present the same set of sections and items, differing only in
  presentation

#### Scenario: Adding a tab requires one edit

- **WHEN** an item is added to the canonical definition
- **THEN** it appears in both the sidebar and the settings editor without any
  second registration

#### Scenario: Definition is shared, not copied

- **WHEN** the test suite runs
- **THEN** a test enumerates the canonical definition and asserts both consumer
  surfaces account for every item

### Requirement: Navigation is organized into labelled sections with stable item keys

The navigation SHALL present items grouped under labelled sections, and each item
SHALL carry a stable key that is independent of its label and position, so that
renaming a label cannot break a user's saved order or favorites.

#### Scenario: Item keys survive a label change

- **WHEN** an item's display label changes
- **THEN** its saved order position and favorite status are preserved

#### Scenario: Sections are ordered deterministically

- **WHEN** the sidebar renders
- **THEN** sections appear in a fixed order that user preferences cannot reorder

### Requirement: Feature-gated items are hidden unless their flag is enabled

Items MAY declare a feature flag. An item whose flag is disabled SHALL NOT appear
in the sidebar or in the settings navigation editor. A flag SHALL be read from
persisted user preferences.

#### Scenario: Disabled flag hides the item

- **WHEN** a gated item's flag is disabled
- **THEN** the item appears in neither the sidebar nor the settings editor

#### Scenario: Enabled flag reveals the item

- **WHEN** the flag is enabled and settings are saved
- **THEN** the item appears in both surfaces without a restart

#### Scenario: Unknown flag is treated as disabled

- **WHEN** a preference key is absent
- **THEN** the item it gates is hidden

### Requirement: A section with no visible items is not rendered

A section whose items are all hidden — by flag or by user preference — SHALL NOT
be rendered at all, including its label and its collapse control.

#### Scenario: All items hidden

- **WHEN** every item in a section is hidden
- **THEN** the section and its label do not appear in the sidebar

#### Scenario: One item remains

- **WHEN** exactly one item in a section is visible
- **THEN** the section renders with its label and that item only

### Requirement: Active-item highlighting distinguishes parent routes from prefix routes

An item MAY be marked for exact-match highlighting. An item so marked SHALL be
highlighted only when the current route matches it exactly. An item not so marked
SHALL be highlighted when the current route matches it or begins with it followed
by a path separator.

#### Scenario: Parent route with its own child tab

- **WHEN** a child route that has its own sidebar entry is active
- **THEN** the parent item is not highlighted

#### Scenario: Parent route whose children are not tabs

- **WHEN** a child route that has no sidebar entry of its own is active
- **THEN** the parent item is highlighted

#### Scenario: Exact match

- **WHEN** the current route equals an exact-match item's route
- **THEN** that item is highlighted

#### Scenario: Similar prefix is not a match

- **WHEN** the current route begins with an item's route but the next character is
  not a path separator
- **THEN** that item is not highlighted

### Requirement: Persistent controls survive navigation without remounting

The controls outside the tab list — at minimum the character switcher, the log
status indicator, and back/forward history — SHALL persist across in-app
navigation without remounting and without refetching the data they display.

#### Scenario: Character selection is stable across tabs

- **WHEN** the user selects a character and then navigates between tabs
- **THEN** the selection is unchanged and the switcher does not refetch on each
  navigation

#### Scenario: Log status does not restart its polling

- **WHEN** the user navigates between tabs
- **THEN** the log status indicator keeps a single polling loop rather than
  starting a new one per navigation

#### Scenario: Back and forward navigate in-app history

- **WHEN** the user navigates several tabs and then goes back and forward
- **THEN** the previously visited tabs are restored

#### Scenario: Controls never appear in overlay windows

- **WHEN** an overlay route renders
- **THEN** the persistent controls are absent

### Requirement: Users can hide, reorder and favorite navigation items

The application SHALL let users hide items, order the items within a section, and
mark favorites. All three SHALL persist across restarts and SHALL be applied
consistently in the sidebar and the settings navigation editor.

#### Scenario: Hidden item is hidden everywhere

- **WHEN** an item is hidden
- **THEN** it is absent from the sidebar and marked as hidden in the settings
  editor, and remains hidden after a restart

#### Scenario: Custom order persists

- **WHEN** items within a section are reordered
- **THEN** the sidebar reflects that order and it survives a restart

#### Scenario: Ordering is stable for unlisted items

- **WHEN** an order preference lists only some of a section's items
- **THEN** the listed items appear first in the specified order and the remaining
  items follow in their default relative order

#### Scenario: Favorites appear in a dedicated group

- **WHEN** at least one item is marked favorite
- **THEN** a Favorites group appears at the top of the sidebar containing exactly
  the favorited items

#### Scenario: Favorites cannot resurrect a hidden or gated item

- **WHEN** an item is marked favorite and then becomes hidden by preference or by
  its flag being disabled
- **THEN** it does not appear in the Favorites group

#### Scenario: No favorites means no group

- **WHEN** no items are favorited
- **THEN** the Favorites group is not rendered

### Requirement: Section collapse state persists in user settings

Users SHALL be able to collapse and expand each section, and the state SHALL be
persisted in the application's user settings rather than browser-local storage,
so it survives reinstall and applies to every window.

#### Scenario: Collapse persists across restart

- **WHEN** a section is collapsed and the application restarts
- **THEN** the section is still collapsed

#### Scenario: Collapse state is consistent across windows

- **WHEN** the same user settings are loaded in more than one window
- **THEN** a section's collapsed state is the same in both

### Requirement: Sidebar preferences do not disturb settings the reference app reads

Sidebar preferences SHALL be stored in the existing user settings file without
renaming or removing any key the reference application reads, so the settings
file remains valid for rollback.

#### Scenario: Reference app can still read settings

- **WHEN** the migrated application saves sidebar preferences and the reference
  application then loads the settings file
- **THEN** the reference application loads it successfully and its own settings
  are unchanged

#### Scenario: No existing key is renamed or dropped

- **WHEN** the settings file is written by the migrated application
- **THEN** every key present before the write is still present with the same
  meaning
