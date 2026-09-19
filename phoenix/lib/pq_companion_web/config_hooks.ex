defmodule PQCompanionWeb.ConfigHooks do
  @moduledoc """
  Attaches the sidebar's data and events to every main-window LiveView.

  The sidebar is rendered by the layout, but its state and its events belong to
  the LiveView, so this `on_mount` hook is the single place that:

  * loads the user's sidebar preferences from `PQCompanion.Config.Server`,
  * subscribes to settings changes so another window's edit shows up here
    without a reload,
  * records the active path from `handle_params` (design D6) — one code path for
    both the initial render and every subsequent live navigation, so
    highlighting cannot drift between them,
  * handles the sidebar's own events (currently section collapse).

  Applied via `live_session :main, on_mount: [{PQCompanionWeb.ConfigHooks, :sidebar}]`.
  """

  import Phoenix.Component, only: [assign: 3]
  import Phoenix.LiveView, only: [attach_hook: 4, connected?: 1]

  alias PQCompanion.Config.Server

  @doc false
  def on_mount(:sidebar, _params, _session, socket) do
    if connected?(socket), do: Server.subscribe()

    {:cont,
     socket
     |> assign(:sidebar_prefs, Server.sidebar())
     |> attach_hook(:sidebar_active_path, :handle_params, &put_active_path/3)
     |> attach_hook(:sidebar_events, :handle_event, &sidebar_event/3)
     |> attach_hook(:sidebar_config_updates, :handle_info, &config_update/2)}
  end

  defp put_active_path(_params, uri, socket) do
    {:cont, assign(socket, :active_path, path_of(uri))}
  end

  defp path_of(nil), do: nil
  defp path_of(uri), do: URI.parse(uri).path

  defp sidebar_event("sidebar:toggle_collapse", %{"section" => section_id}, socket) do
    {:halt, toggle_collapse(socket, section_id)}
  end

  defp sidebar_event(_event, _params, socket), do: {:cont, socket}

  # Optimistic: flip the assign immediately so the section responds at once, and
  # persist in the same call — `Server.update_sidebar/1` serialises and broadcasts,
  # and every subscriber (including this window) re-reads the persisted value, so
  # rapid toggling converges on the last write (design D4).
  defp toggle_collapse(socket, section_id) do
    collapsed = socket.assigns.sidebar_prefs[:collapsed] || %{}
    next = Map.update(collapsed, section_id, true, &(not &1))
    _ = Server.update_sidebar(%{"sidebar_collapsed_sections" => next})
    assign(socket, :sidebar_prefs, Map.put(socket.assigns.sidebar_prefs, :collapsed, next))
  end

  defp config_update({:config_updated, _config}, socket) do
    {:cont, assign(socket, :sidebar_prefs, Server.sidebar())}
  end

  defp config_update(_message, socket), do: {:cont, socket}
end
