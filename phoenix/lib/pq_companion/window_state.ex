defmodule PQCompanion.WindowState do
  @moduledoc """
  Runtime (mutable) state for each window, as opposed to the static declaration
  in `PQCompanion.Windows`.

  `PQCompanion.Windows` says what a window *is* — id, route, default native
  properties. This module holds what changes while the app runs: where the user
  dragged the window, the current zoom, whether display-only or click-through is
  on, whether the window is locked.

  The distinction matters because the static declaration is compile-time data and
  the runtime state is per-install user data. Wave 2 (`add-data-model`) will back
  this with the real settings store; for the wave 0 scaffold it is an ETS table
  owned by a supervised `GenServer`, which is enough to prove the shell contract
  (task 3.5: bounds are persisted and re-applied on the next open).

  Stored maps are *partial*: only keys the user has actually changed are present,
  so `PQCompanion.Shell.spec/1` can fall back to the window's declared default.
  """

  use GenServer

  @table :pq_window_state

  @type state :: %{
          optional(:bounds) => map() | nil,
          optional(:zoom) => float(),
          optional(:display_only) => boolean(),
          optional(:click_through) => boolean(),
          optional(:locked) => boolean()
        }

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @doc """
  The stored overrides for a window id, or `%{}` when nothing has changed.

  Reads the ETS table directly so a render path never serialises behind the
  GenServer.
  """
  @spec get(String.t()) :: state()
  def get(id) do
    case :ets.lookup(@table, id) do
      [{^id, state}] -> state
      [] -> %{}
    end
  end

  @doc "Merge `attrs` into the stored state for `id` and return the result."
  @spec put(String.t(), state()) :: state()
  def put(id, attrs) when is_map(attrs) do
    GenServer.call(__MODULE__, {:put, id, attrs})
  end

  @doc "Drop all stored state. Used by tests; not called in application code."
  @spec reset() :: :ok
  def reset, do: GenServer.call(__MODULE__, :reset)

  @impl true
  def init(_opts) do
    :ets.new(@table, [:named_table, :public, :set, read_concurrency: true])
    {:ok, %{}}
  end

  @impl true
  def handle_call({:put, id, attrs}, _from, state) do
    merged = Map.merge(get(id), attrs)
    :ets.insert(@table, {id, merged})
    {:reply, merged, state}
  end

  def handle_call(:reset, _from, state) do
    :ets.delete_all_objects(@table)
    {:reply, :ok, state}
  end
end
