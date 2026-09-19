defmodule PQCompanionWeb.NavTest do
  @moduledoc """
  Wave 1 tasks 1.1–1.5 — the canonical navigation definition and its pure
  transforms (flag filter, ordering, favorites), plus the parity inventory that
  is the anti-rot guard.

  Reference: `frontend/src/lib/sidebarNav.tsx` (frozen, read-only) at the
  branch's pinned commit.
  """

  use ExUnit.Case, async: true

  alias PQCompanionWeb.Nav
  alias PQCompanionWeb.Nav.Item

  # Task 1.5 — the recorded parity inventory, in definition order:
  # {section_id, route, label, icon, flag, exact_match}. If an item is added,
  # removed, reordered or relabelled in `Nav` without updating this list, the
  # parity test below fails — which is the point: it forces the change to be a
  # conscious one that also updates the persisted-key contract.
  @inventory [
    # Database (9)
    {"database", "/items", "Items", "sword", nil, false},
    {"database", "/spells", "Spells", "sparkles", nil, false},
    {"database", "/npcs", "NPCs", "skull", nil, false},
    {"database", "/zones", "Zones", "map", nil, false},
    {"database", "/recipes", "Recipes", "hammer", nil, false},
    {"database", "/tradeskill-leveling", "Tradeskill Leveling", "route", nil, false},
    {"database", "/quests", "Quests", "scroll-text", nil, false},
    {"database", "/charm-pet-finder", "Charm Pet Finder", "paw-print", nil, false},
    {"database", "/resist-calc", "Resist Calculator", "percent", nil, false},
    # Characters (15)
    {"characters", "/characters/overview", "Active Character", "users", nil, false},
    {"characters", "/characters/progress", "Character Info", "trending-up", nil, false},
    {"characters", "/characters/inventory", "Inventory", "package", nil, false},
    {"characters", "/characters/spells", "Spellbook", "book-open", nil, false},
    {"characters", "/characters/spellsets", "Spellsets", "library", nil, false},
    {"characters", "/characters/bandolier", "Bandolier", "swords", nil, false},
    {"characters", "/characters/macros", "Macros", "keyboard", nil, false},
    {"characters", "/characters/keys", "Keys", "key-round", nil, false},
    {"characters", "/characters/lockouts", "Lockouts", "hourglass", nil, false},
    {"characters", "/characters/wishlist", "Wishlist", "star", nil, false},
    {"characters", "/characters/upgrades", "Gear Upgrades", "wand-2", nil, false},
    {"characters", "/characters/tasks", "Tasks", "list-checks", nil, false},
    {"characters", "/pop-flags", "PoP Flags", "flag", "pop_flags_enabled", false},
    {"characters", "/trader-tracker", "Trader Tracker", "store", nil, false},
    {"characters", "/characters/factions", "Factions", "scale", "faction_tracker_enabled", false},
    # Raids (1)
    {"raids", "/raids", "Raid Composition", "shield-check", "raids_enabled", false},
    # Parsing (9)
    {"parsing", "/log-feed", "Log Feed", "activity", nil, false},
    {"parsing", "/live-map", "Live Map", "navigation", nil, false},
    {"parsing", "/overlays", "Overlays", "layers", nil, false},
    {"parsing", "/combat", "Combat Log", "scroll-text", nil, false},
    {"parsing", "/triggers", "Triggers", "zap", nil, false},
    {"parsing", "/rolls", "Roll Tracker", "dice-5", nil, false},
    {"parsing", "/players", "Player Tracker", "user-search", nil, false},
    {"parsing", "/chat", "Chat History", "message-square", nil, false},
    {"parsing", "/loot", "Loot Tracker", "package", nil, false}
  ]

  # The reference's `flags` for a fully-enabled Developer tab.
  @all_flags %{
    "pop_flags_enabled" => true,
    "faction_tracker_enabled" => true,
    "raids_enabled" => true
  }

  defp inventory_rows do
    for section <- Nav.sections(), item <- section.items do
      {section.id, item.route, item.label, item.icon, item.flag, item.exact_match}
    end
  end

  defp routes(sections), do: Enum.map(sections, & &1.id)

  defp database_items do
    Enum.find(Nav.sections(), &(&1.id == "database")).items
  end

  describe "task 1.1 — the definition" do
    test "has four sections, in fixed order, and 34 items" do
      assert routes(Nav.sections()) == ["database", "characters", "raids", "parsing"]
      assert length(Nav.items()) == 34

      assert Enum.map(Nav.sections(), & &1.label) == [
               "Database",
               "Characters",
               "Raids",
               "Parsing"
             ]
    end

    test "every item is a Nav.Item with the required fields" do
      assert Enum.all?(
               Nav.items(),
               &match?(
                 %Item{route: r, label: l, icon: i}
                 when is_binary(r) and is_binary(l) and is_binary(i),
                 &1
               )
             )

      assert Enum.all?(
               Nav.items(),
               &(&1.flag in [nil, "pop_flags_enabled", "faction_tracker_enabled", "raids_enabled"])
             )
    end

    test "routes are unique, since a route is the persisted preference key" do
      all = Enum.map(Nav.items(), & &1.route)
      assert all == Enum.uniq(all)
    end
  end

  describe "task 1.5 — parity inventory (anti-rot guard)" do
    test "the definition matches the recorded inventory exactly, in order" do
      assert inventory_rows() == @inventory
    end

    test "the inventory itself has the expected shape" do
      assert length(@inventory) == 34

      assert Enum.map(@inventory, fn {section, _, _, _, _, _} -> section end) |> Enum.uniq() ==
               ["database", "characters", "raids", "parsing"]
    end

    test "no item uses exact match — faithful to the reference after fbb09919" do
      # fbb09919 dropped the /raids/editor row and `end: true` on /raids together.
      assert Enum.all?(@inventory, fn {_s, _r, _l, _i, _f, exact} -> exact == false end)
    end
  end

  describe "task 1.2 — the flag filter" do
    test "a disabled flag hides the item" do
      sections = Nav.visible_sections(Nav.flags(%{}))

      assert Enum.flat_map(sections, & &1.items)
             |> Enum.map(& &1.route)
             |> Enum.member?("/pop-flags") == false

      assert Enum.all?(sections, &(&1.id != "raids"))
    end

    test "an enabled flag reveals the item, with no restart" do
      sections = Nav.visible_sections(Nav.flags(@all_flags))
      assert routes(sections) == ["database", "characters", "raids", "parsing"]

      assert Enum.flat_map(sections, & &1.items)
             |> Enum.map(& &1.route)
             |> Enum.member?("/pop-flags")

      assert Enum.flat_map(sections, & &1.items)
             |> Enum.map(& &1.route)
             |> Enum.member?("/characters/factions")
    end

    test "an absent flag key is treated as disabled" do
      sections = Nav.visible_sections(%{})
      refute Enum.flat_map(sections, & &1.items) |> Enum.map(& &1.route) |> Enum.member?("/raids")
    end

    test "a section emptied by flags is dropped entirely" do
      # raids_enabled is the only item in the Raids section.
      sections = Nav.visible_sections(Nav.flags(%{"raids_enabled" => false}))
      refute "raids" in routes(sections)

      # With it on, the section is back.
      assert "raids" in routes(Nav.visible_sections(Nav.flags(%{"raids_enabled" => true})))
    end

    test "an ungated section is unaffected by flags" do
      assert "database" in routes(Nav.visible_sections(%{}))
    end

    test "flags/1 accepts atom keys too and reads truthiness like the reference" do
      assert Nav.flags(%{raids_enabled: true})["raids_enabled"] == true
      assert Nav.flags(%{"raids_enabled" => false})["raids_enabled"] == false

      assert Nav.flags(nil) == %{
               "pop_flags_enabled" => false,
               "faction_tracker_enabled" => false,
               "raids_enabled" => false
             }
    end
  end

  describe "task 1.3 — within-section ordering" do
    test "a partial order list puts listed items first and leaves the rest in default order" do
      ordered = Nav.order_items(database_items(), ["/npcs", "/items"])

      assert Enum.map(ordered, & &1.route) ==
               [
                 "/npcs",
                 "/items",
                 "/spells",
                 "/zones",
                 "/recipes",
                 "/tradeskill-leveling",
                 "/quests",
                 "/charm-pet-finder",
                 "/resist-calc"
               ]
    end

    test "an empty order list is the identity" do
      assert Nav.order_items(database_items(), []) == database_items()
    end

    test "unknown keys in the order list are ignored" do
      assert Nav.order_items(database_items(), ["/does-not-exist", "/zones"])
             |> Enum.map(& &1.route) ==
               [
                 "/zones",
                 "/items",
                 "/spells",
                 "/npcs",
                 "/recipes",
                 "/tradeskill-leveling",
                 "/quests",
                 "/charm-pet-finder",
                 "/resist-calc"
               ]
    end

    test "ordering one section is unaffected by routes from another" do
      assert Nav.order_items(database_items(), ["/raids"]) == database_items()
    end

    test "a duplicate route keeps its first position" do
      assert Nav.order_items(database_items(), ["/zones", "/zones"]) |> hd() |> Map.fetch!(:route) ==
               "/zones"
    end
  end

  describe "task 1.4 — favorites resolution" do
    test "resolves favorites from the flag-filtered list, ranked by order" do
      visible = Nav.visible_sections(Nav.flags(@all_flags))
      favs = Nav.favorite_items(visible, ["/raids", "/items"], [])
      assert Enum.map(favs, & &1.route) == ["/items", "/raids"]
    end

    test "a favorited item whose flag is disabled is not returned" do
      # Pass the visible sections, as the reference's favoriteItems does.
      visible = Nav.visible_sections(Nav.flags(%{"raids_enabled" => false}))

      assert Nav.favorite_items(visible, ["/raids", "/items"], []) |> Enum.map(& &1.route) == [
               "/items"
             ]
    end

    test "no favorites yields an empty list, so no group renders" do
      assert Nav.favorite_items(Nav.visible_sections(Nav.flags(@all_flags)), [], []) == []
    end

    test "favorites honour the order preference" do
      visible = Nav.visible_sections(Nav.flags(@all_flags))

      assert Nav.favorite_items(visible, ["/items", "/raids"], ["/raids"]) |> Enum.map(& &1.route) ==
               ["/raids", "/items"]
    end
  end
end
