defmodule PQCompanionWeb.CoreComponentsTest do
  use PQCompanionWeb.ConnCase, async: true

  import Phoenix.LiveViewTest

  # Wave 0 task 2.5 — theme tokens, the daisyUI restyle, and the icon story.
  #
  # The reference app hand-rolls components against its own @theme tokens and
  # renders every icon with lucide-react (~150 files). This migration therefore
  # (a) styles against the same tokens, (b) carries no daisyUI class names at
  # all, and (c) vendors Lucide SVGs instead of heroicons. These tests pin all
  # three, so a regression back to daisyUI or heroicons fails loudly.

  @carrier "w-full rounded border border-(--color-border) bg-(--color-surface) px-3 py-1.5 text-sm text-(--color-foreground) outline-none focus:border-(--color-primary)"

  # render_component calls function components with the assigns map verbatim, so
  # a slot must be passed as its entry structure rather than a do-block.
  defp slot(content) do
    %{__slot__: :inner_block, inner_block: fn _changed, _arg -> content end}
  end

  describe "task 2.5 — theme tokens resolve in rendered output" do
    test "the main window resolves the surface and background tokens", %{conn: conn} do
      {:ok, _view, html} = live(conn, ~p"/")

      # Layout classes reference the tokens; if a token were removed from
      # app.css, the class name would silently stop resolving.
      assert html =~ "bg-(--color-background)"
      assert html =~ "bg-(--color-surface)"
      assert html =~ "text-(--color-muted-foreground)"
    end

    test "the stylesheet declares the primary and surface tokens" do
      css = File.read!(Path.join(["assets", "css", "app.css"]))
      # The durable contract is the token block, not the built artifact —
      # building is a dev toolchain concern (task 6.x), declaring is this task.
      assert css =~ "--color-primary: #c9a84c;"
      assert css =~ "--color-surface: #111111;"
    end
  end

  describe "task 2.5 — daisyUI class names are gone" do
    test "component source names no daisyUI classes" do
      # A restyle that swaps "btn" for something else but leaves "alert-info"
      # behind would look done and be broken. Grep the actual sources.
      path = Path.join(["lib", "pq_companion_web", "components", "core_components.ex"])
      src = File.read!(path)

      for legacy <- ~w(btn alert-info alert-error table-zebra list-row list-col-grow
                       checkbox-sm input-error select-error textarea-error
                       toast-top toast-end) do
        refute src =~ legacy, "daisyUI class #{inspect(legacy)} survived the restyle"
      end

      # `label` and `fieldset` are native HTML elements here (`<label for=…>`),
      # so check the daisyUI *class* usages specifically rather than the word.
      refute src =~ ~s(class="label"), "daisyUI `label` class survived the restyle"
      refute src =~ ~s(class="fieldset), "daisyUI `fieldset` class survived the restyle"
    end

    test "the stylesheet outs no heroicons or daisyUI wiring" do
      css = File.read!(Path.join(["assets", "css", "app.css"]))

      # The comments may explain the removal; the *wiring* must be gone: no
      # plugin directive, no hero- utility.
      refute css =~ "@plugin"
      refute css =~ ".hero-"

      # And the generated heroicons plugin file is deleted, not orphaned.
      refute File.exists?(Path.join(["assets", "vendor", "heroicons.js"]))
    end
  end

  describe "task 2.5 — vendored Lucide icons" do
    test "icon/1 emits inline SVG, never a hero- class" do
      html = render_component(&PQCompanionWeb.CoreComponents.icon/1, %{name: "x"})

      assert html =~ ~s(<svg)
      assert html =~ ~s(viewBox="0 0 24 24")
      assert html =~ ~s(stroke="currentColor")
      refute html =~ "hero-"
    end

    test "a rendered form error uses the vendored icon and danger token" do
      # Use the private-to-public surface: an input with errors renders `<.error>`.
      input_html =
        render_component(&PQCompanionWeb.CoreComponents.input/1,
          type: "text",
          name: "field",
          id: "field",
          value: nil,
          errors: ["is invalid"]
        )

      assert input_html =~ ~s(<svg)
      assert input_html =~ "border-(--color-danger)"
      assert input_html =~ "text-(--color-danger)"
      refute input_html =~ "hero-exclamation-circle"
    end

    test "an unknown icon name raises" do
      assert_raise ArgumentError, ~r/unknown icon/, fn ->
        render_component(&PQCompanionWeb.CoreComponents.icon/1, %{name: "not-an-icon"})
      end
    end
  end

  describe "task 2.5 — restyled components carry tokens, not daisyUI" do
    test "buttons are token-styled in both variants" do
      primary =
        render_component(&PQCompanionWeb.CoreComponents.button/1, %{
          variant: "primary",
          inner_block: slot("Go")
        })

      soft =
        render_component(&PQCompanionWeb.CoreComponents.button/1, %{
          inner_block: slot("Cancel")
        })

      assert primary =~ "bg-(--color-primary)"
      assert primary =~ "text-(--color-primary-foreground)"
      refute primary =~ " btn"
      assert soft =~ "bg-transparent"
      assert soft =~ "border-(--color-border)"
      refute soft =~ " btn"
    end

    test "text, select and textarea inputs use the carrier class" do
      for type <- ["text", "email", "password"] do
        html =
          render_component(&PQCompanionWeb.CoreComponents.input/1,
            type: type,
            name: "field",
            id: "field",
            value: nil
          )

        assert html =~ @carrier
        assert html =~ ~s(type="#{type}")
        refute html =~ "class=\"select\""
        refute html =~ "input-error"
      end

      select_html =
        render_component(&PQCompanionWeb.CoreComponents.input/1,
          type: "select",
          name: "field",
          id: "field",
          value: nil,
          options: [{"One", "1"}]
        )

      assert select_html =~ "border-(--color-border)"
      refute select_html =~ "select-error"

      textarea_html =
        render_component(&PQCompanionWeb.CoreComponents.input/1,
          type: "textarea",
          name: "field",
          id: "field",
          value: nil
        )

      assert textarea_html =~ "<textarea"
      refute textarea_html =~ "textarea-error"
    end

    test "the flash renders as a bottom-right toast styled with tokens" do
      html =
        render_component(&PQCompanionWeb.CoreComponents.flash/1, %{
          kind: :error,
          flash: %{"error" => "boom"}
        })

      assert html =~ "role=\"alert\""
      assert html =~ "fixed bottom-4 right-4"
      assert html =~ "var(--color-surface)"
      assert html =~ "text-(--color-danger)"
      assert html =~ ~s(<svg)
      refute html =~ "toast-top"
      refute html =~ "alert-error"
    end
  end
end
