defmodule PQCompanionWeb.PlaceholderLive do
  @moduledoc """
  A stand-in page behind a navigation item.

  Wave 1 owns the sidebar, so every item it links to must resolve — otherwise the
  links are dead, active highlighting cannot be exercised by real navigation, and
  back/forward has nothing to move between. Each placeholder is replaced by the
  real page in waves 4–10.

  The main window's chrome (titlebar, sidebar) comes from the `:window` layout;
  this LiveView renders only the content region.
  """
  use PQCompanionWeb, :live_view

  alias PQCompanion.Windows
  alias PQCompanionWeb.Nav

  @impl true
  def mount(_params, _session, socket) do
    {:ok, assign(socket, :window, Windows.main())}
  end

  # Navigation between nav items stays inside this LiveView, so it arrives as a
  # patch; the active path itself is recorded by the `ConfigHooks` `handle_params`
  # hook, which runs before this callback.
  @impl true
  def handle_params(_params, _uri, socket), do: {:noreply, socket}

  @impl true
  def render(assigns) do
    assigns = assign(assigns, :item, item_for(assigns[:active_path]))

    ~H"""
    <div id="placeholder-page" class="p-6">
      <h1 class="text-lg font-semibold text-(--color-foreground)">
        {@item && @item.label}
      </h1>
      <p class="mt-2 text-sm text-(--color-muted-foreground)">
        This page is a wave 1 placeholder. Its real content arrives in a later wave.
      </p>
      <p class="mt-4 text-xs text-(--color-muted)">
        route: <code id="placeholder-route">{@active_path}</code>
      </p>
    </div>
    """
  end

  defp item_for(nil), do: nil
  defp item_for(path), do: Enum.find(Nav.items(), &(&1.route == path))
end
