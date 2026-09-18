defmodule PQCompanion.Repo do
  use Ecto.Repo,
    otp_app: :pq_companion,
    adapter: Ecto.Adapters.SQLite3
end
