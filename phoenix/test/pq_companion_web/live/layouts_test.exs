defmodule PQCompanionWeb.LayoutsTest do
  use PQCompanionWeb.ConnCase, async: true

  import Phoenix.LiveViewTest

  alias PQCompanion.Windows

  describe "main window layout" do
    test "renders the titlebar and sidebar regions", %{conn: conn} do
      {:ok, _view, html} = live(conn, ~p"/")

      assert html =~ ~s(id="titlebar")
      assert html =~ ~s(id="sidebar")
      assert html =~ ~s(id="content")
      assert html =~ ~s(id="main-window")
    end

    test "does not apply the transparent body class", %{conn: conn} do
      {:ok, _view, html} = live(conn, ~p"/")
      refute html =~ "body-transparent"
    end
  end

  describe "overlay layout" do
    test "renders no titlebar and no sidebar", %{conn: conn} do
      {:ok, _view, html} = live(conn, Windows.fetch("dps").route)

      refute html =~ ~s(id="titlebar")
      refute html =~ ~s(id="sidebar")
    end

    test "applies the transparent body class", %{conn: conn} do
      {:ok, _view, html} = live(conn, Windows.fetch("dps").route)
      assert html =~ "body-transparent"
    end
  end

  describe "bare layout" do
    test "renders neither chrome nor transparency", %{conn: _conn} do
      html =
        render_component(&PQCompanionWeb.Layouts.bare/1, %{inner_content: "x", window: nil})

      refute html =~ ~s(id="titlebar")
      refute html =~ ~s(id="sidebar")
      refute html =~ "body-transparent"
    end
  end
end
