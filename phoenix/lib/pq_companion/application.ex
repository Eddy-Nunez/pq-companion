defmodule PQCompanion.Application do
  # See https://elixir.hexdocs.pm/Application.html
  # for more information on OTP Applications
  @moduledoc false

  require Logger

  use Application

  @impl true
  def start(_type, _args) do
    # Resolve the on-disk footprint and fail fast if the shipped game database is
    # missing, before the supervision tree (and the HTTP listener) comes up.
    PQCompanion.Paths.ensure!()
    PQCompanion.QuarmRepo.verify!()
    # Pick the listener port now, before the endpoint child reads it.
    PQCompanion.Runtime.configure_port!()

    children =
      [
        PQCompanionWeb.Telemetry,
        PQCompanion.UserRepo
      ] ++
        quarm_children() ++
        [
          PQCompanion.Audio,
          PQCompanion.WindowState,
          # The single owner of config.yaml (wave 1 task 5.1). The path is
          # injectable so tests never read or write the real user's settings.
          {PQCompanion.Config.Server, path: Application.get_env(:pq_companion, :config_path)},
          PQCompanion.Shell.Browser,
          {Ecto.Migrator,
           repos: Application.fetch_env!(:pq_companion, :ecto_repos), skip: skip_migrations?()},
          {DNSCluster, query: Application.get_env(:pq_companion, :dns_cluster_query) || :ignore},
          {Phoenix.PubSub, name: PQCompanion.PubSub},
          # Start a worker by calling: PQCompanion.Worker.start_link(arg)
          # {PQCompanion.Worker, arg},
          # Start to serve requests, typically the last entry
          PQCompanionWeb.Endpoint
        ]

    # See https://elixir.hexdocs.pm/Supervisor.html
    # for other strategies and supported options
    opts = [strategy: :one_for_one, name: PQCompanion.Supervisor]

    with {:ok, supervisor} <- Supervisor.start_link(children, opts) do
      announce_runtime()
      {:ok, supervisor}
    end
  end

  # The listener is bound by the time the supervisor returns, so the record can
  # carry the port the socket actually took.
  defp announce_runtime do
    if PQCompanion.Runtime.server_enabled?() do
      case PQCompanion.Runtime.announce_from() do
        {:ok, record} ->
          Logger.info(
            "runtime announced at #{PQCompanion.Runtime.record_path()} port=#{record["port"]}"
          )

        # The app was started without serving (e.g. `mix run`, `mix compile`):
        # there is no listener to announce, and that is not a failure.
        {:error, :no_server_found} ->
          :ok

        {:error, reason} ->
          Logger.warning("could not announce the runtime record: #{inspect(reason)}")
      end
    end

    :ok
  end

  # The shipped game database is required outside test; `verify!/0` has already
  # aborted boot if it is missing there. In test it is optional (the artifact is
  # not in version control), so where it is absent the connection is left out of
  # the tree and data-backed tests skip rather than the whole suite failing.
  defp quarm_children do
    if PQCompanion.QuarmRepo.available?(), do: [PQCompanion.QuarmDatabase], else: []
  end

  # Tell Phoenix to update the endpoint configuration
  # whenever the application is updated.
  @impl true
  def config_change(changed, _new, removed) do
    PQCompanionWeb.Endpoint.config_change(changed, removed)
    :ok
  end

  # A stale runtime record must not outlive the process (wave 0 task 5.2): a
  # native shell that reads it after a clean stop would otherwise connect to a
  # dead port.
  @impl true
  def stop(_state) do
    PQCompanion.Runtime.retract()
    :ok
  end

  defp skip_migrations?() do
    # By default, sqlite migrations are run when using a release
    System.get_env("RELEASE_NAME") == nil
  end
end
