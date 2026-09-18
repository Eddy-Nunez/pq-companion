defmodule PQCompanionWeb.OverlayLive do
  @moduledoc """
  A placeholder overlay window.

  One parametrised LiveView serves all 16 overlays in wave 0. The design note in
  `add-phoenix-scaffold` suggested one LiveView per overlay; that is the right
  shape once each has real behaviour (wave 8), but 16 placeholder modules would be
  noise now. The guard against silently losing a window is the test that
  enumerates `PQCompanion.Windows.overlays/0`, not the module count.

  Critically: this LiveView mounts **no alert hooks**. Overlays must never emit
  audio or TTS, so that a single game event cannot alert twice (task 2.4).
  """
  use PQCompanionWeb, :overlay_view

  alias PQCompanion.Windows

  @impl true
  def mount(%{"slug" => slug}, _session, socket) do
    case Windows.fetch_by_slug(slug) do
      nil ->
        {:ok, push_navigate(socket, to: "/")}

      window ->
        # Deliberately NO Audio.register/1 here — overlays never own audio.
        {:ok, assign(socket, :window, window)}
    end
  end

  @impl true
  def render(assigns) do
    ~H"""
    <div id={"overlay-#{@window.id}"} class="p-2 text-(--color-foreground)">
      <span class="text-xs">{@window.label}</span>
    </div>
    """
  end
end
