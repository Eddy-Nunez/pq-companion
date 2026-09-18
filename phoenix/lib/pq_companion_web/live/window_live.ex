defmodule PQCompanionWeb.WindowLive do
  @moduledoc """
  The main window.

  Placeholder for wave 0: proves the LiveView socket carries a server-side assign
  change without a page reload (task 2.1) and gives the `:window` layout
  (titlebar + sidebar + content) something to render (task 2.2). The sidebar
  itself lands in wave 1.
  """
  use PQCompanionWeb, :live_view

  alias PQCompanion.Audio
  alias PQCompanion.Windows

  @impl true
  def mount(_params, _session, socket) do
    # Only the main window owns audio. Overlay LiveViews deliberately do not.
    if connected?(socket), do: Audio.register(Windows.main().id)

    {:ok,
     socket
     |> assign(:window, Windows.main())
     |> assign(:ticks, 0)}
  end

  @impl true
  def handle_event("tick", _params, socket) do
    {:noreply, assign(socket, :ticks, socket.assigns.ticks + 1)}
  end

  # Shell round-trip events (bounds, zoom, display-only, click-through, lock).
  # Shared with the overlays so the two window classes cannot drift.
  def handle_event("pq:" <> _ = event, params, socket) do
    PQCompanionWeb.ShellEvents.handle(event, params, socket)
  end

  @impl true
  def render(assigns) do
    ~H"""
    <div id="main-window-content" class="p-6">
      <h1 class="text-lg font-semibold text-(--color-foreground)">PQ Companion</h1>
      <p class="mt-2 text-sm text-(--color-muted-foreground)">
        Wave 0 scaffold. The sidebar arrives in wave 1; feature pages in waves 4-10.
      </p>
      <p class="mt-4 text-sm text-(--color-muted-foreground)">
        server-side tick count: <span id="tick-count">{@ticks}</span>
      </p>
      <button
        id="tick-button"
        phx-click="tick"
        class="mt-3 rounded border border-(--color-border) px-3 py-1 text-sm text-(--color-foreground)"
      >
        Increment (no page reload)
      </button>
    </div>
    """
  end
end
