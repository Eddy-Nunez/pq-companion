# This file is responsible for configuring your application
# and its dependencies with the aid of the Config module.
#
# This configuration file is loaded before any dependency and
# is restricted to this project.

# General application configuration
import Config

config :pq_companion,
  namespace: PQCompanion,
  ecto_repos: [PQCompanion.UserRepo],
  generators: [timestamp_type: :utc_datetime]

# Migrations stay in the generator's `priv/repo/migrations`; Ecto would otherwise
# look under `priv/user_repo/migrations` for a repo named `UserRepo`.
config :pq_companion, PQCompanion.UserRepo, priv: "priv/repo"

# The shipped game database (quarm.db) is read-only and lives beside the app,
# not under the app home. `PQCompanion.QuarmRepo` refuses writes in Elixir and
# the connection is opened read-only underneath. In `:test` this is false so the
# suite can boot without the artifact (which is not in version control) and
# data-backed tests skip explicitly instead of failing.
config :pq_companion, :quarm_db_required, true

# Which native-shell adapter window operations are dispatched to. The browser
# adapter is the development default so no wave before 8 is blocked on the
# deferred Electron-vs-Tauri decision (see PQCompanion.Shell).
config :pq_companion, :shell_adapter, PQCompanion.Shell.Browser

# Configure the endpoint
config :pq_companion, PQCompanionWeb.Endpoint,
  url: [host: "localhost"],
  adapter: Bandit.PhoenixAdapter,
  render_errors: [
    formats: [html: PQCompanionWeb.ErrorHTML, json: PQCompanionWeb.ErrorJSON],
    layout: false
  ],
  pubsub_server: PQCompanion.PubSub,
  live_view: [signing_salt: "6HVevmxZ"]

# Configure LiveView
config :phoenix_live_view,
  # the attribute set on all root tags. Used for Phoenix.LiveView.ColocatedCSS.
  root_tag_attribute: "phx-r"

# This app has no npm and no `assets/node_modules`, and its only client hooks are
# plain ones in `assets/js/app.js`, so the colocated-assets compiler has nothing
# to import. Silencing the symlink warning keeps a Windows `mix compile` clean
# (without Developer Mode the symlink is refused with `:eperm`; nothing is lost).
config :phoenix_live_view, :colocated_assets, disable_symlink_warning: true

# Configure esbuild (the version is required)
config :esbuild,
  version: "0.25.4",
  pq_companion: [
    args:
      ~w(js/app.js --bundle --target=es2022 --outdir=../priv/static/assets/js --external:/fonts/* --external:/images/* --alias:@=.),
    cd: Path.expand("../assets", __DIR__),
    env: %{"NODE_PATH" => [Path.expand("../deps", __DIR__), Mix.Project.build_path()]}
  ]

# Configure tailwind (the version is required)
config :tailwind,
  version: "4.3.0",
  pq_companion: [
    args: ~w(
      --input=assets/css/app.css
      --output=priv/static/assets/css/app.css
    ),
    cd: Path.expand("..", __DIR__),
    env: %{"NODE_PATH" => [Path.expand("../deps", __DIR__), Mix.Project.build_path()]}
  ]

# Configure Elixir's Logger
config :logger, :default_formatter,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id]

# Use Jason for JSON parsing in Phoenix
config :phoenix, :json_library, Jason

# Import environment specific config. This must remain at the bottom
# of this file so it overrides the configuration defined above.
import_config "#{config_env()}.exs"
