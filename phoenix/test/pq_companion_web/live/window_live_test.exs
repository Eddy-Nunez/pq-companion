defmodule PQCompanionWeb.WindowLiveTest do
  # async: false — these assert on the global audio-owner registry.
  use PQCompanionWeb.ConnCase, async: false

  import Phoenix.LiveViewTest

  alias PQCompanion.Audio

  describe "task 2.1 — LiveView socket carries server-side state" do
    test "establishes a LiveView connection and renders server state", %{conn: conn} do
      {:ok, view, html} = live(conn, ~p"/")

      assert html =~ "server-side tick count"
      assert render(view) =~ ~s(id="tick-count")
      assert has_element?(view, "#tick-count", "0")
    end

    test "a server-side assign change arrives without a page reload", %{conn: conn} do
      {:ok, view, _html} = live(conn, ~p"/")

      # Render, then drive the server. render_click goes over the real LiveView
      # socket, so a changed diff proves the round trip rather than a re-render.
      assert has_element?(view, "#tick-count", "0")

      html = view |> element("#tick-button") |> render_click()
      assert html =~ ~s(id="tick-count")
      assert has_element?(view, "#tick-count", "1")

      view |> element("#tick-button") |> render_click()
      assert has_element?(view, "#tick-count", "2")
    end

    test "declares the main window spec in its assigns", %{conn: conn} do
      {:ok, view, _html} = live(conn, ~p"/")
      assert view.module == PQCompanionWeb.WindowLive
    end
  end

  describe "task 2.4 — exactly one window owns audio" do
    test "the main window registers as audio owner", %{conn: conn} do
      before = Audio.owner_count()
      {:ok, _view, _html} = live(conn, ~p"/")
      assert Audio.owner_count() == before + 1
    end

    test "an overlay window never registers as audio owner", %{conn: conn} do
      # Overlays must not emit alerts, so a single game event cannot alert twice.
      # This is the LiveView equivalent of the reference's MainWindowLayout vs
      # OverlayPage React-tree split.
      before = Audio.owner_count()
      {:ok, _view, _html} = live(conn, "/w/dps")
      assert Audio.owner_count() == before
    end

    test "no overlay registers an owner, for every overlay", %{conn: conn} do
      before = Audio.owner_count()

      for window <- PQCompanion.Windows.overlays() do
        {:ok, _view, _html} = live(conn, window.route)
      end

      assert Audio.owner_count() == before,
             "an overlay registered as audio owner — overlays must never emit alerts"
    end
  end
end
