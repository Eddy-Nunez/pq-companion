defmodule PQCompanionWeb.ShellEventsTest do
  @moduledoc """
  Wave 0 task 3.5 — property changes propagate to the shell without a reload, and
  reported bounds are persisted and re-applied on the next open.

  `async: false`: these tests write the global window-state store.
  """

  use PQCompanionWeb.ConnCase, async: false

  import Phoenix.LiveViewTest

  alias PQCompanion.Shell
  alias PQCompanion.Shell.Browser
  alias PQCompanion.{WindowState, Windows}

  setup do
    Browser.reset()
    WindowState.reset()
    :ok
  end

  describe "task 3.5 — live property propagation" do
    test "a display-only toggle reaches the shell as an update, with no reload", %{conn: conn} do
      {:ok, view, _html} = live(conn, Windows.fetch("npc").route)

      render_hook(view, "pq:set_display_only", %{"value" => true})

      assert_push_event(view, "pq:window", %{id: "npc", patch: %{display_only: true}})
      assert WindowState.get("npc").display_only == true

      # No navigation happened: the same socket still renders the same overlay.
      assert render(view) =~ "overlay-npc"
    end

    test "zoom, click-through and lock propagate too", %{conn: conn} do
      {:ok, view, _html} = live(conn, Windows.fetch("npc").route)

      render_hook(view, "pq:set_zoom", %{"value" => "1.5"})
      assert_push_event(view, "pq:window", %{id: "npc", patch: %{zoom: 1.5}})

      render_hook(view, "pq:set_click_through", %{"value" => true})
      assert_push_event(view, "pq:window", %{id: "npc", patch: %{click_through: true}})

      render_hook(view, "pq:set_locked", %{"value" => true})
      assert_push_event(view, "pq:window", %{id: "npc", patch: %{locked: true}})

      state = WindowState.get("npc")
      assert state.zoom == 1.5
      assert state.click_through == true
      assert state.locked == true
    end

    test "reported bounds are persisted and re-applied on the next open", %{conn: conn} do
      {:ok, view, _html} = live(conn, Windows.fetch("npc").route)

      bounds = %{"x" => 120, "y" => 40, "w" => 320, "h" => 200}
      render_hook(view, "pq:save_bounds", %{"bounds" => bounds})

      assert_push_event(view, "pq:window", %{
        id: "npc",
        patch: %{bounds: %{x: 120, y: 40, w: 320, h: 200}}
      })

      assert WindowState.get("npc").bounds == %{x: 120, y: 40, w: 320, h: 200}

      # The next spec a shell receives already carries the stored bounds, so
      # opening the window re-applies them.
      spec = Shell.spec(Windows.fetch("npc"))
      assert spec.bounds == %{x: 120, y: 40, w: 320, h: 200}

      assert {:ok, _} = Browser.open(spec)
      assert Browser.recorded("npc").spec.bounds == %{x: 120, y: 40, w: 320, h: 200}
      assert :bounds in Browser.unmet("npc")
    end

    test "the main window handles the same events as an overlay", %{conn: conn} do
      {:ok, view, _html} = live(conn, ~p"/")

      render_hook(view, "pq:set_locked", %{"value" => true})
      assert_push_event(view, "pq:window", %{id: "main", patch: %{locked: true}})
    end

    test "an unknown property event is a no-op, not a crash", %{conn: conn} do
      {:ok, view, _html} = live(conn, Windows.fetch("npc").route)

      render_hook(view, "pq:set_nonsense", %{"value" => true})

      assert render(view) =~ "overlay-npc"
    end
  end
end
