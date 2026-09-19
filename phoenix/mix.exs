defmodule PQCompanion.MixProject do
  use Mix.Project

  def project do
    [
      app: :pq_companion,
      version: "0.1.0",
      elixir: "~> 1.17",
      elixirc_paths: elixirc_paths(Mix.env()),
      start_permanent: Mix.env() == :prod,
      aliases: aliases(),
      deps: deps(),
      compilers: [:phoenix_live_view] ++ Mix.compilers(),
      listeners: [Phoenix.CodeReloader]
    ]
  end

  # Configuration for the OTP application.
  #
  # Type `mix help compile.app` for more information.
  def application do
    [
      mod: {PQCompanion.Application, []},
      extra_applications: [:logger, :runtime_tools]
    ]
  end

  def cli do
    [
      preferred_envs: [precommit: :test]
    ]
  end

  # Specifies which paths to compile per environment.
  defp elixirc_paths(:test), do: ["lib", "test/support"]
  defp elixirc_paths(_), do: ["lib"]

  # Specifies your project dependencies.
  #
  # Type `mix help deps` for examples and options.
  defp deps do
    [
      {:phoenix, "~> 1.8.14"},
      {:phoenix_ecto, "~> 4.5"},
      {:ecto_sql, "~> 3.13"},
      {:ecto_sqlite3, ">= 0.0.0"},
      {:phoenix_html, "~> 4.1"},
      {:phoenix_live_reload, "~> 1.2", only: :dev},
      {:phoenix_live_view, "~> 1.2.0"},
      {:lazy_html, ">= 0.1.0", only: :test},
      {:phoenix_live_dashboard, "~> 0.8.3"},
      {:esbuild, "~> 0.10", runtime: Mix.env() == :dev},
      {:tailwind, "~> 0.5", runtime: Mix.env() == :dev},
      {:telemetry_metrics, "~> 1.0"},
      {:telemetry_poller, "~> 1.0"},
      {:jason, "~> 1.2"},
      {:dns_cluster, "~> 0.2.0"},
      {:bandit, "~> 1.5"},

      # ── Added for the PQ Companion migration (wave 0 task 1.3) ──────────────
      # daisyUI was removed: the reference hand-rolls components against its own
      # `@theme` tokens, and daisyUI's theme variables collide with them by name.
      # See openspec/changes/add-phoenix-scaffold design D7.

      # Watching the EverQuest directory (log, Zeal exports, .ini files).
      # NOTE: Windows backends are unverified — wave 0 task 1.5 settles it, and
      # the fallback is a poll-based GenServer. Only wave 3 depends on this.
      {:file_system, "~> 1.1"},

      # `config.yaml` (wave 1 task 5.1). `yaml_elixir` reads; it cannot write, so
      # `ymlr` (pure Elixir, MIT, no deps — deliberately not `fast_yaml`, which
      # is a NIF and would add a Windows build step) writes. Both are runtime
      # deps: the settings file is read and written in dev, test and a release.
      {:yaml_elixir, "~> 2.12"},
      {:ymlr, "~> 5.0"},

      # Validating config load options (settings changeset work, wave 2).
      {:nimble_options, "~> 1.1"},

      # Quality gates — dev/test only, not in a release.
      {:credo, "~> 1.7", only: [:dev, :test], runtime: false},
      {:dialyxir, "~> 1.4", only: [:dev, :test], runtime: false},
      {:mix_audit, "~> 2.1", only: [:dev, :test], runtime: false}
    ]
  end

  # Aliases are shortcuts or tasks specific to the current project.
  # For example, to install project dependencies and perform other setup tasks, run:
  #
  #     $ mix setup
  #
  # See the documentation for `Mix` for more info on aliases.
  defp aliases do
    [
      # `mix dev` is the one-command development startup (wave 0 task 6.1).
      # It deliberately does not chain `setup`: `setup` ends with
      # `run priv/repo/seeds.exs`, which starts the whole application before
      # `phx.server` can set `serve_endpoints`, so the endpoint would never bind.
      dev: [
        "deps.get",
        "ecto.create --quiet",
        "ecto.migrate --quiet",
        "assets.setup",
        "assets.build",
        "phx.server"
      ],
      setup: ["deps.get", "ecto.setup", "assets.setup", "assets.build"],
      "ecto.setup": ["ecto.create", "ecto.migrate", "run priv/repo/seeds.exs"],
      "ecto.reset": ["ecto.drop", "ecto.setup"],
      test: ["ecto.create --quiet", "ecto.migrate --quiet", "test"],
      "assets.setup": ["tailwind.install --if-missing", "esbuild.install --if-missing"],
      "assets.build": ["compile", "tailwind pq_companion", "esbuild pq_companion"],
      "assets.deploy": [
        "tailwind pq_companion --minify",
        "esbuild pq_companion --minify",
        "phx.digest"
      ],
      precommit: ["compile --warnings-as-errors", "deps.unlock --unused", "format", "test"]
    ]
  end
end
