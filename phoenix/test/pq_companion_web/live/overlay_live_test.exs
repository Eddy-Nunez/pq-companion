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

  describe "window spec in the document (task 3.2)" do
    test "the overlay emits a parseable pq-window meta tag with every key", %{conn: conn} do
      {:ok, _view, html} = live(conn, Windows.fetch("npc").route)

      # Parse rather than regex: HEEx HTML-escapes the attribute, so a regex
      # captures `&quot;` and breaks Jason. An HTML parser unescapes it — which
      # is exactly what a native shell scraping the tag will do, so parsing is
      # also the more faithful test.
      spec = pq_window_spec(html)

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

      # The session token and bounds the shell contract requires (plan §2.4).
      assert is_binary(spec["token"]) and spec["token"] != ""
      assert Map.has_key?(spec, "bounds")
    end

    test "the main window also declares a spec, so a shell needs no special case", %{conn: conn} do
      {:ok, _view, html} = live(conn, ~p"/")
      spec = pq_window_spec(html)

      assert spec["id"] == "main"
      assert spec["kind"] == "main"
      assert spec["transparent"] == false
      assert spec["clickThrough"] == false
      assert spec["resizable"] == true
      assert is_binary(spec["token"]) and spec["token"] != ""
    end

    test "both window classes declare every key the shell contract requires", %{conn: conn} do
      required = ~w(id slug kind token route title transparent alwaysOnTop clickThrough
                    frameless resizable displayOnly bounds zoom)

      for {route, _kind} <- [{~p"/", :main}, {Windows.fetch("dps").route, :overlay}] do
        {:ok, _view, html} = live(conn, route)
        spec = pq_window_spec(html)

        for key <- required do
          assert Map.has_key?(spec, key), "#{route} is missing #{key}"
        end
      end
    end
  end

  defp pq_window_spec(html) do
    [content] =
      html
      |> LazyHTML.from_document()
      |> LazyHTML.query(~s(meta[name="pq-window"]))
      |> LazyHTML.attribute("content")

    Jason.decode!(content)
  end
end
