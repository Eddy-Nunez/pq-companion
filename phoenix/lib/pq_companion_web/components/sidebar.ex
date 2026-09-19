defmodule PQCompanionWeb.Components.Sidebar do
  @moduledoc """
  The main window's side navigation.

  Renders entirely from `PQCompanionWeb.Nav` — the same definition the
  Settings → Navigation editor renders from — so the two surfaces cannot
  disagree about which tabs exist. Sections and their order are fixed; the items
  within a section, and whether a section is collapsed, come from the user's
  preferences.

  Ported from the reference's `frontend/src/components/Sidebar.tsx` (sidebar
  region only; the fixed controls are their own components, wave 1 tasks 4.x).
  """

  use PQCompanionWeb, :html

  alias PQCompanionWeb.Components.NavIcons
  alias PQCompanionWeb.Nav

  @doc """
  Renders the sidebar.

  * `sections` — already flag-filtered, hidden-filtered, ordered, with any
    Favorites group prepended (`Nav.sidebar_sections/2`).
  * `collapsed` — per-section collapse state, keyed by section id.
  * `active_path` — the current route, for highlighting.
  """
  attr :sections, :list, required: true
  attr :collapsed, :map, default: %{}
  attr :active_path, :string, default: nil

  def sidebar(assigns) do
    ~H"""
    <nav
      id="sidebar"
      class="flex w-52 shrink-0 flex-col overflow-hidden border-r border-(--color-border) bg-(--color-surface)"
      aria-label="Main navigation"
    >
      <div class="no-drag flex-1 overflow-y-auto">
        <div :for={section <- @sections} data-section={section.id}>
          <button
            type="button"
            phx-click="sidebar:toggle_collapse"
            phx-value-section={section.id}
            aria-expanded={to_string(not collapsed?(@collapsed, section.id))}
            aria-controls={"sidebar-items-#{section.id}"}
            class="no-drag flex w-full items-center gap-1 px-3 pt-3 pb-1 text-left text-[10px] font-semibold tracking-widest text-(--color-muted) uppercase transition-colors hover:text-(--color-foreground)"
          >
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
              class={"size-3 transition-transform #{if collapsed?(@collapsed, section.id), do: "-rotate-90", else: ""}"}
            >
              <path d="m6 9 6 6 6-6" />
            </svg>
            <span>{section.label}</span>
          </button>

          <ul
            :if={not collapsed?(@collapsed, section.id)}
            id={"sidebar-items-#{section.id}"}
            class="space-y-0.5 px-2 py-1"
          >
            <li :for={item <- section.items}>
              <.link
                navigate={item.route}
                data-nav-item={item.route}
                aria-current={if Nav.active?(item, @active_path), do: "page", else: nil}
                class={link_class(Nav.active?(item, @active_path))}
              >
                <NavIcons.nav_icon name={item.icon} class="size-4 shrink-0" />
                <span>{item.label}</span>
              </.link>
            </li>
          </ul>
        </div>
      </div>
    </nav>
    """
  end

  defp collapsed?(collapsed, section_id), do: Map.get(collapsed || %{}, section_id, false) == true

  # The reference's active/inactive classes, against its own design tokens.
  defp link_class(true),
    do:
      "no-drag flex items-center gap-3 rounded px-3 py-2 text-sm font-medium bg-(--color-surface-2) text-(--color-primary)"

  defp link_class(false),
    do:
      "no-drag flex items-center gap-3 rounded px-3 py-2 text-sm text-(--color-muted-foreground) transition-colors hover:bg-(--color-surface-2) hover:text-(--color-foreground)"
end
