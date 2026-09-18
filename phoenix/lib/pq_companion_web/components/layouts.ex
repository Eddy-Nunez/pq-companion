defmodule PQCompanionWeb.Layouts do
  @moduledoc """
  Window layouts.

  Three window classes, expressed as an HTML shell plus an inner layout:

  | window class | shell             | inner            |
  |--------------|-------------------|------------------|
  | main window  | `root/1` (opaque) | `window/1`       |
  | overlay      | `overlay/1`       | `bare/1`         |
  | bare         | `root/1` (opaque) | `bare/1`         |

  Note on naming: `add-phoenix-scaffold` originally called the main-window class
  `:root`. That collides with Phoenix's own "root layout" concept (the outermost
  HTML document, set via `put_root_layout`), so the window class is `:window` and
  `root/1` keeps its Phoenix meaning. The spec was updated to match.
  """
  use PQCompanionWeb, :html

  embed_templates "layouts/*"

  @doc """
  The main window: titlebar, sidebar and content.

  The sidebar itself is wave 1; this is its slot plus the titlebar region.

  Note: as a LiveView layout this receives `@inner_content`, not a named slot.
  The generator's `app/1` used `render_slot(@inner_block)` and worked only
  because `config :phoenix_live_view, layout: false` means it was always called
  explicitly as `<Layouts.app>` from a template. A real layout is invoked by
  Phoenix with `inner_content`.
  """
  attr :flash, :map, default: %{}
  attr :window, :map, default: nil
  attr :inner_content, :any, default: nil

  def window(assigns) do
    ~H"""
    <div id="main-window" phx-hook="PqWindow" class="flex h-screen flex-col bg-(--color-background)">
      <header
        id="titlebar"
        class="flex h-9 shrink-0 items-center border-b border-(--color-border) px-3"
      >
        <span class="text-xs font-medium text-(--color-muted-foreground)">
          {@window && @window.label}
        </span>
      </header>
      <div class="flex min-h-0 flex-1">
        <nav
          id="sidebar"
          class="w-52 shrink-0 overflow-y-auto border-r border-(--color-border) bg-(--color-surface)"
          aria-label="Main navigation"
        >
          <%!-- wave 1 (add-sidebar-navigation) renders PQWeb.Nav here --%>
          <p class="p-3 text-xs text-(--color-muted)">navigation arrives in wave 1</p>
        </nav>
        <main id="content" class="min-w-0 flex-1 overflow-y-auto">
          {@inner_content}
        </main>
      </div>
    </div>
    """
  end

  @doc """
  Content only — no titlebar, no sidebar.

  Used by overlay windows (which must have no chrome) and by bare routes such as
  the onboarding wizard and modals. Receives `@inner_content` as a layout.
  """
  attr :flash, :map, default: %{}
  attr :window, :map, default: nil
  attr :inner_content, :any, default: nil

  def bare(assigns) do
    ~H"""
    <div id="bare-window" class="min-h-0">
      {@inner_content}
    </div>
    """
  end

  @doc """
  Shows the flash group with standard titles and content.
  """
  attr :flash, :map, required: true
  attr :id, :string, default: "flash-group"

  def flash_group(assigns) do
    ~H"""
    <div id={@id} aria-live="polite">
      <.flash kind={:info} flash={@flash} />
      <.flash kind={:error} flash={@flash} />
    </div>
    """
  end
end
