defmodule PQCompanionWeb.Nav.Item do
  @moduledoc """
  A single navigable side-tab.

  Ported from the reference's `NavItem` (`frontend/src/lib/sidebarNav.tsx`).
  `route` doubles as the **stable key** used by the hide / order / favorites
  preferences: the reference persists route strings (its `to`) in
  `config.yaml`, so a route is a compatibility key and must not be renamed
  casually. `label` is free to change without disturbing saved preferences.
  """

  @enforce_keys [:route, :label, :icon]
  defstruct [:route, :label, :icon, flag: nil, exact_match: false]

  @typedoc """
  * `route` — the item's path, and its stable preference key.
  * `label` — display text; may change without breaking saved preferences.
  * `icon` — vendored icon name (resolved by `PQCompanionWeb.Components.NavIcons`).
  * `flag` — a `config.yaml` preference key that gates the item, or `nil`.
  * `exact_match` — highlight on an exact route match only (the reference's
    React Router `end` prop). `false` — prefix match — is the default.
  """
  @type t :: %__MODULE__{
          route: String.t(),
          label: String.t(),
          icon: String.t(),
          flag: String.t() | nil,
          exact_match: boolean()
        }
end

defmodule PQCompanionWeb.Nav do
  @moduledoc """
  The canonical sidebar definition and the pure transforms over it.

  Single source of truth for the sidebar's sections and items, shared by the
  sidebar and the Settings → Navigation editor so the two cannot disagree about
  which tabs exist. The reference keeps the same single source in `NAV_SECTIONS`
  (`frontend/src/lib/sidebarNav.tsx`), which is why its sidebar and settings
  editor never drift.

  Sections and their order are **fixed**; only the items within a section are
  subject to feature flags and to the user's hide / order / favorites
  preferences.

  Nothing currently sets `exact_match: true`, and that is faithful to the
  reference rather than an omission: commit `fbb09919` (*"drop duplicate Raid
  Editor entry from left nav"*) removed the `/raids/editor` sidebar row **and**
  the `end: true` on `/raids` in the same edit, because the editor is now a tab
  *inside* Raid Composition. Every item therefore prefix-matches — `/raids`
  behaves exactly like `/combat`, whose `/combat/log` and `/combat/history`
  children are likewise not sidebar rows. The exact-match capability is retained
  because the behaviour contract requires it (task 3.2); the migration artifacts
  that still describe `/raids` as exact-match predate `fbb09919`.
  """

  alias PQCompanionWeb.Nav.Item

  @typedoc "A labelled group of items. `id` and section order are fixed."
  @type section :: %{id: String.t(), label: String.t(), items: [Item.t()]}

  # The three feature flags, in the reference's `navFlags` order.
  @flag_keys ~w(pop_flags_enabled faction_tracker_enabled raids_enabled)

  # Unranked items sort after every ranked one. `:infinity` is an atom and every
  # atom sorts after every integer in Elixir term order — the equivalent of the
  # reference's `Number.MAX_SAFE_INTEGER` sentinel in `orderItems`.
  @unranked :infinity

  # The four sections and 34 items, verbatim from the reference's NAV_SECTIONS.
  @sections [
    %{
      id: "database",
      label: "Database",
      items: [
        %Item{route: "/items", label: "Items", icon: "sword"},
        %Item{route: "/spells", label: "Spells", icon: "sparkles"},
        %Item{route: "/npcs", label: "NPCs", icon: "skull"},
        %Item{route: "/zones", label: "Zones", icon: "map"},
        %Item{route: "/recipes", label: "Recipes", icon: "hammer"},
        %Item{route: "/tradeskill-leveling", label: "Tradeskill Leveling", icon: "route"},
        %Item{route: "/quests", label: "Quests", icon: "scroll-text"},
        %Item{route: "/charm-pet-finder", label: "Charm Pet Finder", icon: "paw-print"},
        %Item{route: "/resist-calc", label: "Resist Calculator", icon: "percent"}
      ]
    },
    %{
      id: "characters",
      label: "Characters",
      items: [
        %Item{route: "/characters/overview", label: "Active Character", icon: "users"},
        %Item{route: "/characters/progress", label: "Character Info", icon: "trending-up"},
        %Item{route: "/characters/inventory", label: "Inventory", icon: "package"},
        %Item{route: "/characters/spells", label: "Spellbook", icon: "book-open"},
        %Item{route: "/characters/spellsets", label: "Spellsets", icon: "library"},
        %Item{route: "/characters/bandolier", label: "Bandolier", icon: "swords"},
        %Item{route: "/characters/macros", label: "Macros", icon: "keyboard"},
        %Item{route: "/characters/keys", label: "Keys", icon: "key-round"},
        %Item{route: "/characters/lockouts", label: "Lockouts", icon: "hourglass"},
        %Item{route: "/characters/wishlist", label: "Wishlist", icon: "star"},
        %Item{route: "/characters/upgrades", label: "Gear Upgrades", icon: "wand-2"},
        %Item{route: "/characters/tasks", label: "Tasks", icon: "list-checks"},
        %Item{route: "/pop-flags", label: "PoP Flags", icon: "flag", flag: "pop_flags_enabled"},
        %Item{route: "/trader-tracker", label: "Trader Tracker", icon: "store"},
        %Item{
          route: "/characters/factions",
          label: "Factions",
          icon: "scale",
          flag: "faction_tracker_enabled"
        }
      ]
    },
    %{
      id: "raids",
      label: "Raids",
      items: [
        %Item{
          route: "/raids",
          label: "Raid Composition",
          icon: "shield-check",
          flag: "raids_enabled"
        }
      ]
    },
    %{
      id: "parsing",
      label: "Parsing",
      items: [
        %Item{route: "/log-feed", label: "Log Feed", icon: "activity"},
        %Item{route: "/live-map", label: "Live Map", icon: "navigation"},
        %Item{route: "/overlays", label: "Overlays", icon: "layers"},
        %Item{route: "/combat", label: "Combat Log", icon: "scroll-text"},
        %Item{route: "/triggers", label: "Triggers", icon: "zap"},
        %Item{route: "/rolls", label: "Roll Tracker", icon: "dice-5"},
        %Item{route: "/players", label: "Player Tracker", icon: "user-search"},
        %Item{route: "/chat", label: "Chat History", icon: "message-square"},
        %Item{route: "/loot", label: "Loot Tracker", icon: "package"}
      ]
    }
  ]

  @doc """
  Every section and item, unfiltered — the canonical definition.

  Iterate this (rather than re-listing tabs) to prove the sidebar and the
  settings editor agree, and to drive the parity inventory test.
  """
  @spec sections() :: [section()]
  def sections, do: @sections

  @doc "Every item, flattened in section then definition order."
  @spec items() :: [Item.t()]
  def items, do: Enum.flat_map(@sections, & &1.items)

  @doc "The feature-flag keys, in order."
  @spec flag_keys() :: [String.t()]
  def flag_keys, do: @flag_keys

  @doc """
  Build the flag map consumed by `visible_sections/1` from `config.yaml`
  preferences.

  Mirrors the reference's `navFlags`: a flag is off unless its preference is
  explicitly truthy. Accepts string keys (how `config.yaml` reads) or atom keys.
  """
  @spec flags(map() | nil) :: %{String.t() => boolean()}
  def flags(prefs), do: Map.new(@flag_keys, &{&1, enabled?(prefs, &1)})

  defp enabled?(prefs, key) when is_map(prefs) do
    case Map.fetch(prefs, key) do
      {:ok, value} -> truthy?(value)
      :error -> truthy?(Map.get(prefs, String.to_atom(key), false))
    end
  end

  defp enabled?(_prefs, _key), do: false

  defp truthy?(value), do: value in [true, "true", 1]

  @doc """
  The definition with flag-gated items removed, then any emptied section dropped.

  Mirrors the reference's `visibleNavSections`. A section emptied by flags is
  not rendered at all — no label, no collapse control.
  """
  @spec visible_sections(%{String.t() => boolean()}) :: [section()]
  def visible_sections(flags) when is_map(flags) do
    @sections
    |> Enum.map(fn section ->
      %{section | items: Enum.filter(section.items, &visible?(&1, flags))}
    end)
    |> Enum.reject(&(&1.items == []))
  end

  defp visible?(%Item{flag: nil}, _flags), do: true
  defp visible?(%Item{flag: flag}, flags), do: Map.get(flags, flag, false) == true

  @doc """
  Order a section's items by their position in `order` (a list of routes).

  Items absent from `order` keep their default relative position, after the
  ranked ones — `Enum.sort_by/2` is stable, matching the reference's
  `orderItems` index tie-break. Duplicate routes keep their first position.
  """
  @spec order_items([Item.t()], [String.t()]) :: [Item.t()]
  def order_items(items, order) do
    rank =
      order
      |> Enum.with_index()
      |> Enum.reduce(%{}, fn {route, index}, acc -> Map.put_new(acc, route, index) end)

    Enum.sort_by(items, &Map.get(rank, &1.route, @unranked))
  end

  @doc """
  The favorited items, flattened from `sections` and ranked by `order`.

  Pass the output of `visible_sections/1` so flag-gated items are already
  excluded — a favorite must not resurrect a gated or hidden item. Hidden items
  are the caller's responsibility to remove, exactly as in the reference's
  `favoriteItems`.
  """
  @spec favorite_items([section()], [String.t()], [String.t()]) :: [Item.t()]
  def favorite_items(sections, favorites, order) do
    favorite_set = MapSet.new(favorites)

    sections
    |> Enum.flat_map(& &1.items)
    |> Enum.filter(&MapSet.member?(favorite_set, &1.route))
    |> order_items(order)
  end
end
