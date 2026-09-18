defmodule PQCompanion.QuarmDatabase do
  @moduledoc """
  The supervised Ecto repository for the shipped, read-only game database
  (wave 0 task 4.1).

  This module is deliberately *not* the one feature code talks to. Feature code
  uses the `PQCompanion.QuarmRepo` facade, which can only read. This repo is the
  connection underneath it, opened with exqlite's `mode: :readonly` so that even
  a call that bypasses the facade cannot write: SQLite refuses at the file-flag
  level.

  Two layers, on purpose:

    1. `PQCompanion.QuarmRepo` rejects writes in Elixir, before SQLite sees them.
    2. This connection is opened read-only, so a bypass still cannot write.

  The path comes from `PQCompanion.Paths.quarm_db/0` (see `init/2`), which is
  overridable with `config :pq_companion, :quarm_db`.
  """

  use Ecto.Repo,
    otp_app: :pq_companion,
    adapter: Ecto.Adapters.SQLite3

  @impl true
  def init(_context, config) do
    config =
      config
      |> Keyword.put_new(:database, PQCompanion.Paths.quarm_db())
      |> Keyword.put_new(:mode, :readonly)

    {:ok, config}
  end

  # KNOWN DEVIATION (recorded in handoff.md): the reference opens quarm.db as
  # `file:<path>?mode=ro&immutable=1`. exqlite 0.40 exposes neither `immutable`
  # nor SQLITE_OPEN_URI, and ecto_sqlite3 forces WAL, so this read-only
  # connection may create `-wal`/`-shm` siblings next to the artifact and would
  # fail on a genuinely unwritable install directory. `mode: :readonly` still
  # guarantees the database contents cannot change. Resolve before wave 11
  # (upstream an `:immutable` option to exqlite, or keep the artifact in a
  # writable per-user location).
end
