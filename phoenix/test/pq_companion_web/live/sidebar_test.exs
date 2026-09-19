defmodule PQCompanionWeb.SidebarTest do
  @moduledoc """
  Wave 1 tasks 2.1–2.4 and 3.1 — the sidebar as it actually renders in the main
  window: every item, the overlay exclusion, collapse, the Favorites group, and
  highlighting that follows a live navigation.

  `async: false`: these tests drive the application-supervised
  `PQCompanion.Config.Server`, which is a single global writer.
  """

  use PQCompanionWeb.ConnCase, async: false

  import Phoenix.LiveViewTest

  alias PQCompanion.Config.Server
  alias PQCompanion.Windows
  alias PQCompanionWeb.Nav

  setup do
    reset_prefs()
    :ok
  end

  defp reset_prefs do
    {:ok, _} =
      Server.update_sidebar(%{
        "sidebar_hidden" => [],
        "sidebar_order" => [],
        "sidebar_favorites" => [],
        "sidebar_collapsed_sections" => %{},
        "pop_flags_enabled" => false,
        "faction_tracker_enabled" => false,
        "raids_enabled" => false
      })
  end

  defp flags_on do
    {:ok, _} =
      Server.update_sidebar(%{
        "pop_flags_enabled" => true,
        "faction_tracker_enabled" => true,
        "raids_enabled" => true
      })
  end

  describe "task 2.1 — rendering from the canonical definition" do
    test "every item renders its label and links to its route", %{conn: conn} do
      flags_on()
      {:ok, _view, html} = live(conn, ~p"/")

      for item <- Nav.items() do
        assert html =~ ~s(data-nav-item="#{item.route}"), "missing link for #{item.route}"
        assert html =~ item.label, "missing label #{item.label}"
      end
    end

    test "sections render in the definition's fixed order", %{conn: conn} do
      flags_on()
      {:ok, _view, html} = live(conn, ~p"/")

      positions =
        ["Database", "Characters", "Raids", "Parsing"]
        |> Enum.map(&:binary.match(html, &1))
        |> Enum.map(fn {pos, _} -> pos end)

      assert positions == Enum.sort(positions)
    end
  end

  describe "task 2.2 — overlays have no sidebar" do
    test "an overlay route renders neither the sidebar nor its items", %{conn: conn} do
      {:ok, _view, html} = live(conn, Windows.fetch("dps").route)

      refute html =~ ~s(id="sidebar")
      refute html =~ "data-nav-item"
    end
  end

  describe "task 2.3 — collapsible sections" do
    test "collapsing hides a section's items and expanding shows them again", %{conn: conn} do
      {:ok, view, _html} = live(conn, ~p"/")

      assert has_element?(view, "#sidebar-items-database")

      view |> element("button[phx-value-section='database']") |> render_click()
      refute has_element?(view, "#sidebar-items-database")

      view |> element("button[phx-value-section='database']") |> render_click()
      assert has_element?(view, "#sidebar-items-database")
    end

    test "a section emptied by flags renders no label and no collapse control", %{conn: conn} do
      # raids_enabled is off by default, so the Raids section is empty.
      {:ok, _view, html} = live(conn, ~p"/")

      refute html =~ ~s(phx-value-section="raids")
      refute html =~ ">Raids<"
    end

    test "a section emptied by prefs renders no label and no collapse control", %{conn: conn} do
      # Hide every Parsing item.
      parsing = Enum.find(Nav.sections(), &(&1.id == "parsing")).items |> Enum.map(& &1.route)
      {:ok, _} = Server.update_sidebar(%{"sidebar_hidden" => parsing})

      {:ok, _view, html} = live(conn, ~p"/")
      refute html =~ ~s(phx-value-section="parsing")
      refute html =~ ">Parsing<"
    end
  end

  describe "task 2.4 — the Favorites group" do
    test "no favorites means no Favorites group", %{conn: conn} do
      {:ok, _view, html} = live(conn, ~p"/")
      refute html =~ ">Favorites<"
    end

    test "a favorite appears in a Favorites group at the top", %{conn: conn} do
      {:ok, _} = Server.update_sidebar(%{"sidebar_favorites" => ["/items"]})
      {:ok, _view, html} = live(conn, ~p"/")

      assert html =~ ">Favorites<"

      # It is at the top: the Favorites label precedes the Database label.
      {fav, _} = :binary.match(html, ">Favorites<")
      {db, _} = :binary.match(html, ">Database<")
      assert fav < db

      # And it is a shortcut, not a move: the item is still in its own section.
      assert html =~ ~s(id="sidebar-items-database")
    end

    test "a gated favorite does not appear", %{conn: conn} do
      # raids_enabled is off, so /raids is not visible and cannot be favorited in.
      {:ok, _} = Server.update_sidebar(%{"sidebar_favorites" => ["/raids"]})
      {:ok, _view, html} = live(conn, ~p"/")

      refute html =~ ">Favorites<"
    end
  end

  describe "task 3.1 — active path comes from handle_params" do
    test "highlighting follows a live navigation, not only the initial mount", %{conn: conn} do
      {:ok, view, _html} = live(conn, ~p"/items")
      assert has_element?(view, ~s([data-nav-item="/items"][aria-current="page"]))

      # A different route served by the same LiveView is a patch, so this is a
      # live navigation rather than a fresh mount.
      render_patch(view, ~p"/spells")

      assert has_element?(view, ~s([data-nav-item="/spells"][aria-current="page"]))
      refute has_element?(view, ~s([data-nav-item="/items"][aria-current="page"]))
    end

    test "the placeholder content follows the patched route", %{conn: conn} do
      {:ok, view, _html} = live(conn, ~p"/items")
      assert has_element?(view, "#placeholder-route", "/items")

      render_patch(view, ~p"/zones")
      assert has_element?(view, "#placeholder-route", "/zones")
    end
  end
end
