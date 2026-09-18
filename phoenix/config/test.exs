import Config

# Configure your database
#
# Tests never touch the real `~/.pq-companion/user.db`: the explicit `database`
# here wins over `PQCompanion.UserRepo.init/2`.
#
# The MIX_TEST_PARTITION environment variable can be used
# to provide built-in test partitioning in CI environment.
# Run `mix help test` for more information.
config :pq_companion, PQCompanion.UserRepo,
  database: Path.expand("../pq_companion_test.db", __DIR__),
  pool_size: 5,
  pool: Ecto.Adapters.SQL.Sandbox

# The game database artifact is not in version control; the suite must boot
# without it, and data-backed tests skip with an explicit warning (task 4.3).
config :pq_companion, :quarm_db_required, false

# We don't run a server during test. If one is required,
# you can enable the server option below.
config :pq_companion, PQCompanionWeb.Endpoint,
  http: [ip: {127, 0, 0, 1}, port: 4002],
  secret_key_base: "54E5OOUXQHL0RyfnfipHkdzNu60fYoQ0+sHglytmaw04jYts6yrt2Aep/ENMCUBo",
  server: false

# Print only warnings and errors during test
config :logger, level: :warning

# Initialize plugs at runtime for faster test compilation
config :phoenix, :plug_init_mode, :runtime

# Enable helpful, but potentially expensive runtime checks
config :phoenix_live_view,
  enable_expensive_runtime_checks: true

# Sort query params output of verified routes for robust url comparisons
config :phoenix,
  sort_verified_routes_query_params: true
