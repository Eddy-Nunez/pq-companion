defmodule PQCompanionWeb.ShellEvents do
  @moduledoc """
  Handles the shell round-trip events shared by every window class.

  Property *changes* travel over the LiveView event channel rather than the
  `pq-window` meta tag (which is read once, on load):

    * the server pushes `"pq:window"` with a patch when a property changes, and a
      native shell's preload bridge applies it — no navigation, no reload;
    * the client sends `"pq:save_bounds"` (from a resize/move hook) and
      `"pq:set_<property>"` back, and this module persists them in
      `PQCompanion.WindowState` so the next `open/1` re-applies them.

  Both `PQCompanionWeb.WindowLive` and `PQCompanionWeb.OverlayLive` delegate their
  `"pq:"`-prefixed events here, so the main window and the overlays cannot drift
  apart.
  """

  import Phoenix.LiveView, only: [push_event: 3]

  alias PQCompanion.WindowState

  # The properties a live update may change, and the stored key each maps to.
  # An explicit map rather than `String.to_existing_atom/1` so an unknown event
  # name is a no-op rather than a crash.
  @properties %{
    "display_only" => :display_only,
    "click_through" => :click_through,
    "zoom" => :zoom,
    "locked" => :locked
  }

  @doc """
  Handle a shell event. Returns `{:noreply, socket}`, like `handle_event/3`.

  Called from each LiveView's `handle_event("pq:" <> _, ...)` clause.
  """
  @spec handle(String.t(), map(), Phoenix.LiveView.Socket.t()) ::
          {:noreply, Phoenix.LiveView.Socket.t()}
  def handle("pq:save_bounds", %{"bounds" => bounds}, socket) do
    id = socket.assigns.window.id
    state = WindowState.put(id, %{bounds: normalize_bounds(bounds)})
    {:noreply, push_patch(socket, %{bounds: state.bounds})}
  end

  def handle("pq:set_" <> property, %{"value" => value}, socket) do
    case Map.fetch(@properties, property) do
      {:ok, key} ->
        id = socket.assigns.window.id
        state = WindowState.put(id, %{key => coerce(key, value)})
        {:noreply, push_patch(socket, %{key => Map.get(state, key)})}

      :error ->
        {:noreply, socket}
    end
  end

  def handle(_event, _params, socket), do: {:noreply, socket}

  defp push_patch(socket, patch) do
    push_event(socket, "pq:window", %{id: socket.assigns.window.id, patch: patch})
  end

  @doc false
  def normalize_bounds(bounds) do
    %{
      x: number(bounds, :x),
      y: number(bounds, :y),
      w: number(bounds, :w),
      h: number(bounds, :h)
    }
  end

  defp number(map, key) do
    case Map.get(map, key) || Map.get(map, to_string(key)) do
      nil -> nil
      value when is_integer(value) -> value
      value when is_float(value) -> trunc(value)
      value when is_binary(value) -> String.to_integer(value)
    end
  end

  defp coerce(:zoom, value) when is_binary(value), do: String.to_float(normalize_float(value))
  defp coerce(:zoom, value) when is_integer(value), do: value / 1
  defp coerce(:zoom, value) when is_float(value), do: value
  defp coerce(_key, value) when value in [true, "true"], do: true
  defp coerce(_key, _value), do: false

  defp normalize_float(value) do
    if String.contains?(value, "."), do: value, else: value <> ".0"
  end
end
