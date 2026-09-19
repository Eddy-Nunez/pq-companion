defmodule PQCompanion.Config.Server do
  @moduledoc """
  The single owner of `~/.pq-companion/config.yaml`.

  Wave 1 introduces this process because the sidebar's preferences are persisted
  state (design D4) and there must be exactly **one** writer: two processes
  writing temp files and renaming concurrently can interleave and produce a file
  that mixes both. Every settings write in the application goes through this
  GenServer, and every settings change is broadcast so open windows update
  without a reload (the reference's `config:updated` event, preserved).

  Wave 2 (`add-data-model`) replaces the untyped map with an embedded-schema
  struct and adds validation, defaults and external-file watching. It does not
  add a second writer.

  The path is injectable so tests never touch the real user's settings file:
  `start_link(path: ...)`, and the application passes
  `config :pq_companion, :config_path` when it is set.
  """

  use GenServer

  alias PQCompanion.Config

  @topic "config:updated"

  @doc "Start the settings owner. Options: `:path`, `:name`."
  @spec start_link(keyword()) :: GenServer.on_start()
  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @doc "The whole settings map."
  @spec get(GenServer.server()) :: Config.t()
  def get(server \\ __MODULE__), do: GenServer.call(server, :get)

  @doc "The `preferences` map."
  @spec preferences(GenServer.server()) :: map()
  def preferences(server \\ __MODULE__), do: GenServer.call(server, :preferences)

  @doc "The feature-flag map."
  @spec flags(GenServer.server()) :: map()
  def flags(server \\ __MODULE__), do: GenServer.call(server, :flags)

  @doc """
  The sidebar preferences the UI needs, already resolved to their defaults:

  `%{hidden: [...], order: [...], favorites: [...], collapsed: %{}, flags: %{}}`
  """
  @spec sidebar(GenServer.server()) :: map()
  def sidebar(server \\ __MODULE__), do: GenServer.call(server, :sidebar)

  @doc """
  Merge `changes` into `preferences`, save atomically, and broadcast.

  Returns `{:ok, config}` or `{:error, reason}`; on failure nothing is broadcast
  and the in-memory value is left unchanged, so the process and the file never
  disagree.
  """
  @spec update_sidebar(map(), GenServer.server()) :: {:ok, Config.t()} | {:error, term()}
  def update_sidebar(changes, server \\ __MODULE__),
    do: GenServer.call(server, {:update_sidebar, changes})

  @doc "Re-read the file, discarding in-memory state (used by tests and, later, external-edit watching)."
  @spec reload(GenServer.server()) :: :ok
  def reload(server \\ __MODULE__), do: GenServer.call(server, :reload)

  @doc "The PubSub topic settings changes are broadcast on."
  @spec topic() :: String.t()
  def topic, do: @topic

  @doc "Subscribe the calling process to settings changes."
  @spec subscribe() :: :ok
  def subscribe, do: Phoenix.PubSub.subscribe(PQCompanion.PubSub, @topic)

  @impl true
  def init(opts) do
    path = Keyword.get(opts, :path) || Config.path()
    {:ok, %{path: path, config: Config.load!(path)}}
  end

  @impl true
  def handle_call(:get, _from, state), do: {:reply, state.config, state}

  def handle_call(:preferences, _from, state),
    do: {:reply, Config.preferences(state.config), state}

  def handle_call(:flags, _from, state), do: {:reply, Config.flags(state.config), state}

  def handle_call(:sidebar, _from, state), do: {:reply, sidebar_prefs(state.config), state}

  def handle_call({:update_sidebar, changes}, _from, state) when is_map(changes) do
    config = Config.put_preferences(state.config, changes)

    case Config.save(config, state.path) do
      :ok ->
        broadcast(config)
        {:reply, {:ok, config}, %{state | config: config}}

      {:error, reason} ->
        {:reply, {:error, reason}, state}
    end
  end

  def handle_call(:reload, _from, state) do
    {:reply, :ok, %{state | config: Config.load!(state.path)}}
  end

  @doc false
  def sidebar_prefs(config) do
    %{
      hidden: Config.sidebar_hidden(config),
      order: Config.sidebar_order(config),
      favorites: Config.sidebar_favorites(config),
      collapsed: Config.sidebar_collapsed(config),
      flags: Config.flags(config)
    }
  end

  defp broadcast(config) do
    Phoenix.PubSub.broadcast(PQCompanion.PubSub, @topic, {:config_updated, config})
  end
end
