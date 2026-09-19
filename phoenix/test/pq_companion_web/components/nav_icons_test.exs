defmodule PQCompanionWeb.Components.NavIconsTest do
  @moduledoc """
  Wave 1 task 2.5 — the vendored Lucide icon set resolves every icon the
  canonical navigation definition references.
  """

  use ExUnit.Case, async: true

  import Phoenix.LiveViewTest

  alias PQCompanionWeb.Components.NavIcons
  alias PQCompanionWeb.Nav

  describe "task 2.5 — every inventory item resolves to an icon" do
    test "the definition references no icon the module cannot render" do
      missing =
        Nav.items()
        |> Enum.map(& &1.icon)
        |> Enum.uniq()
        |> Enum.reject(&(&1 in NavIcons.names()))

      assert missing == [], "navigation items reference unvendored icons: #{inspect(missing)}"
    end

    test "the set is the reference's 32 distinct icons, with no duplicates" do
      assert length(NavIcons.names()) == 32
      assert NavIcons.names() == Enum.uniq(NavIcons.names())
      assert NavIcons.names() == Enum.sort(NavIcons.names())
    end
  end

  describe "rendering" do
    test "each name renders a complete inline svg with lucide's defaults" do
      for name <- NavIcons.names() do
        html = render_component(&NavIcons.nav_icon/1, %{name: name})

        assert html =~ "<svg", "#{name} did not render an svg"
        assert html =~ "</svg>", "#{name} did not close its svg"
        assert html =~ ~s(stroke="currentColor")
        assert html =~ ~s(stroke-width="2")
        assert html =~ ~s(viewBox="0 0 24 24")
      end
    end

    test "the default size is 16px, matching the reference's `size={16}`" do
      assert render_component(&NavIcons.nav_icon/1, %{name: "sword"}) =~ ~s(class="size-4")
    end

    test "an unknown icon name raises rather than rendering a blank" do
      assert_raise ArgumentError, ~r/unknown nav icon/, fn ->
        render_component(&NavIcons.nav_icon/1, %{name: "definitely-not-an-icon"})
      end
    end

    test "an aliased name renders the aliased art (`wand-2` -> `wand-sparkles`)" do
      # lucide keeps `Wand2` as a re-export of `wand-sparkles`; the vendored body
      # must be the target's, not an empty alias shim.
      html = render_component(&NavIcons.nav_icon/1, %{name: "wand-2"})
      assert html =~ "<path"
    end
  end
end
