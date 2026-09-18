defmodule PQCompanionWeb.OverlayLiveTest do
  use PQCompanionWeb.ConnCase, async: true

  import Phoenix.LiveViewTest

  alias PQCompanion.Windows

  describe "task 2.3 — every reference overlay route renders" do
    test "all 16 overlay routes render, enumerated from the registry", %{conn: conn} do
      overlays = Windows.overlays()
      assert length(overlays) == 16

      for window <- overlays do
        {:ok, view, html} = live(conn, window.route)
        assert html =~ "overlay-#{window.id}"
        assert has_element?(view, "#overlay-#{window.id}")
      end
    end

    test "each overlay renders its own label, not a shared one", %{conn: conn} do
      {:ok, _v, dps} = live(conn, Windows.fetch("dps").route)
      {:ok, _v, npc} = live(conn, Windows.fetch("npc").route)

      assert dps =~ "DPS Meter"
      assert npc =~ "NPC Info"
      refute npc =~ "DPS Meter"
    end

    test "an unknown window id redirects to the main window", %{conn: conn} do
      assert {:error, {:live_redirect, %{to: "/"}}} = live(conn, "/w/not-a-window")
    end
  end

  describe "window spec in the document (task 3.2 groundwork)" do
    test "the overlay emits a parseable pq-window meta tag with every key", %{conn: conn} do
      {:ok, _view, html} = live(conn, Windows.fetch("npc").route)

      # Parse rather than regex: HEEx HTML-escapes the attribute, so a regex
      # captures `&quot;` and breaks Jason. An HTML parser unescapes it — which
      # is exactly what a native shell scraping the tag will do, so parsing is
      # also the more faithful test.
      doc = LazyHTML.from_document(html)

      [content] =
        doc
        |> LazyHTML.query(~s(meta[name="pq-window"]))
        |> LazyHTML.attribute("content")
      spec = Jason.decode!(content)

      assert spec["id"] == "npc"
      assert spec["slug"] == "npc"
      assert spec["route"] == "/w/npc"
      assert spec["title"] == "NPC Info"
      assert spec["kind"] == "overlay"
      assert spec["transparent"] == true
      assert spec["alwaysOnTop"] == true
      assert spec["frameless"] == true
      assert spec["resizable"] == false
      assert spec["clickThrough"] == false
      assert spec["displayOnly"] == false
      assert is_float(spec["zoom"])
    end

    test "the main window does not emit a pq-window meta tag", %{conn: conn} do
      # Not an omission to fix: the main window is declared by the shell, and the
      # overlay shell is the only place the spec currently travels. If the shell
      # ends up needing a main-window spec too, add it to root.html.heex and
      # update this test — do not let it silently drift.
      {:ok, _view, html} = live(conn, ~p"/")
      refute html =~ ~s(name="pq-window")
    end
  end
end
