defmodule PQCompanion.Shell.Browser do
  @moduledoc """
  The development-default shell adapter: a plain browser tab.

  It exists so that no wave before 8 is blocked on the deferred
  Electron-vs-Tauri decision. It accepts every window spec, *records* the specs
  it received so tests can assert on them, and applies the property updates it
  can (which it then keeps for `list/0`).

  What it deliberately does **not** do is pretend. A browser tab cannot be
  transparent, always-on-top, frameless, click-through or display-only, and it
  cannot be positioned by the server. Every operation that asked for one of those
  returns the list of properties it could not honour (`:unmet`) rather than a bare
  success, so an overlay bug cannot hide until wave 8. The one property it does
  honour is persistence of what it was told, which is what makes task 3.5's
  "bounds are re-applied on next open" assertable without a native shell.

  State is a map of window id => `%{spec: spec, unmet: [...], focused: bool}`
  held by this `GenServer`. `reset/0` exists for tests.
  """

  @behaviour PQCompanion.Shell

  use GenServer

  # Properties a browser tab cannot provide. `bounds` is included dynamically
  # only when a spec actually asks for one.
  @native_only [:transparent, :always_on_top, :click_through, :frameless, :display_only]

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  # ── Behaviour callbacks ───────────────────────────────────────────────────

  @impl PQCompanion.Shell
  def open(spec), do: GenServer.call(__MODULE__, {:open, spec})

  @impl PQCompanion.Shell
  def close(id), do: GenServer.call(__MODULE__, {:close, id})

  @impl PQCompanion.Shell
  def resize(id, bounds), do: GenServer.call(__MODULE__, {:resize, id, bounds})

  @impl PQCompanion.Shell
  def set_click_through(id, value),
    do: GenServer.call(__MODULE__, {:set_click_through, id, value})

  @impl PQCompanion.Shell
  def focus(id), do: GenServer.call(__MODULE__, {:focus, id})

  @impl PQCompanion.Shell
  def list, do: GenServer.call(__MODULE__, :list)

  # ── Test/introspection helpers (not part of the behaviour) ─────────────────

  @doc "The recorded entry for a window id, or nil."
  def recorded(id), do: GenServer.call(__MODULE__, {:recorded, id})

  @doc "The properties the adapter could not honour for a window id."
  def unmet(id) do
    case recorded(id) do
      %{unmet: unmet} -> unmet
      nil -> []
    end
  end

  @doc "Drop all recorded windows. Used by tests."
  def reset, do: GenServer.call(__MODULE__, :reset)

  # ── GenServer ─────────────────────────────────────────────────────────────

  @impl true
  def init(_opts), do: {:ok, %{windows: %{}}}

  @impl true
  def handle_call({:open, spec}, _from, state) do
    unmet = unmet_properties(spec)
    entry = %{spec: spec, unmet: unmet, focused: false}
    state = put_in(state.windows[spec.id], entry)
    {:reply, {:ok, %{id: spec.id, route: spec.route, unmet: unmet}}, state}
  end

  def handle_call({:close, id}, _from, state) do
    {:reply, :ok, %{state | windows: Map.delete(state.windows, id)}}
  end

  def handle_call({:resize, id, bounds}, _from, state) do
    {reply, state} =
      update_window(state, id, fn entry ->
        spec = Map.put(entry.spec, :bounds, normalize_bounds(bounds))
        entry = %{entry | spec: spec, unmet: Enum.uniq([:bounds | entry.unmet])}
        {{:ok, %{id: id, unmet: [:bounds]}}, entry}
      end)

    {:reply, reply, state}
  end

  def handle_call({:set_click_through, id, value}, _from, state) do
    {reply, state} =
      update_window(state, id, fn entry ->
        spec = Map.put(entry.spec, :click_through, value)
        entry = %{entry | spec: spec, unmet: Enum.uniq([:click_through | entry.unmet])}
        {{:ok, %{id: id, unmet: [:click_through]}}, entry}
      end)

    {:reply, reply, state}
  end

  def handle_call({:focus, id}, _from, state) do
    {reply, state} =
      update_window(state, id, fn entry ->
        {:ok, %{entry | focused: true}}
      end)

    {:reply, reply, state}
  end

  def handle_call(:list, _from, state) do
    {:reply, state.windows |> Map.values() |> Enum.map(& &1.spec), state}
  end

  def handle_call({:recorded, id}, _from, state) do
    {:reply, Map.get(state.windows, id), state}
  end

  def handle_call(:reset, _from, state) do
    {:reply, :ok, %{state | windows: %{}}}
  end

  # ── Internals ─────────────────────────────────────────────────────────────

  defp update_window(state, id, fun) do
    case Map.fetch(state.windows, id) do
      {:ok, entry} ->
        {reply, entry} = fun.(entry)
        {reply, put_in(state.windows[id], entry)}

      :error ->
        {{:error, {:unknown_window, id}}, state}
    end
  end

  defp unmet_properties(spec) do
    requested = for key <- @native_only, Map.get(spec, key) in [true], do: key
    requested ++ if(is_nil(Map.get(spec, :bounds)), do: [], else: [:bounds])
  end

  defp normalize_bounds(bounds) do
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
end
