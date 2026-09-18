defmodule PQCompanion.Application do
  # See https://elixir.hexdocs.pm/Application.html
  # for more information on OTP Applications
  @moduledoc false

  use Application

  @impl true
  def start(_type, _args) do
    # Resolve the on-disk footprint and fail fast if the shipped game database is
    # missing, before the supervision tree (and the HTTP listener) comes up.
    PQCompanion.Paths.ensure!()
    PQCompanion.QuarmRepo.verify!()

    children =
      [
        PQCompanionWeb.Telemetry,
        PQCompanion.UserRepo
      ] ++
        quarm_children() ++
        [
          PQCompanion.Audio,
          PQCompanion.WindowState,
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
    Supervisor.start_link(children, opts)
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

  defp skip_migrations?() do
    # By default, sqlite migrations are run when using a release
    System.get_env("RELEASE_NAME") == nil
  end
end
