defmodule PQCompanion.UserRepo do
  @moduledoc """
  The read-write repository for user data at `~/.pq-companion/user.db`
  (wave 0 task 4.1).

  This is the repository for everything the user owns — settings-adjacent
  tables, characters, combat history, backups metadata. It opens an existing
  reference-created database **in place**: no migration is applied to adopt it,
  and no schema change is required before it is usable (wave 0 task 4.4). That
  adoption is wave 2's job.

  Connection tuning:

    * `journal_mode: :wal` — the reference runs WAL, and it lets a reader and a
      writer coexist while the game is running.
    * `busy_timeout: 5_000` — the reference sidecar may briefly hold the write
      lock; wait rather than failing.
    * `:readwrite` + `:create` (exqlite's default) — a fresh machine gets a
      valid empty database rather than a boot failure.

  The path is resolved in `init/2` from `PQCompanion.Paths`, so it is overridable
  with `config :pq_companion, :app_home` (tests) or an explicit `:database`
  option (a dynamically started instance).
  """

  use Ecto.Repo,
    otp_app: :pq_companion,
    adapter: Ecto.Adapters.SQLite3

  @impl true
  def init(_context, config) do
    config =
      config
      |> Keyword.put_new(:database, PQCompanion.Paths.user_db())
      |> Keyword.put_new(:journal_mode, :wal)
      |> Keyword.put_new(:busy_timeout, 5_000)

    {:ok, config}
  end
end
