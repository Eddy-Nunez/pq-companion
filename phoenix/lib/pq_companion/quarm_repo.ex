defmodule PQCompanion.QuarmRepo do
  @moduledoc """
  The game-data repository: **read-only, structurally** (wave 0 tasks 4.1–4.3,
  4.5).

  Feature code talks to this module, never to `PQCompanion.QuarmDatabase`
  directly. Reads are delegated to that supervised Ecto repo; every write
  function is defined here solely to raise, in Elixir, before a statement can
  reach SQLite. That is the guarantee the spec asks for — "rejection shall occur
  before the statement reaches SQLite" — and the connection is *also* opened
  read-only underneath, so even a bypass cannot write.

  Why a facade rather than `use Ecto.Repo`: Ecto's generated `insert/2`,
  `update/2` and `delete/2` are not overridable, so a repo module cannot redefine
  them (the later definition is an unreachable clause). A module that owns the
  public name and delegates reads is the only way to make the rejection
  structural rather than a convention.

  The shipped `quarm.db` is resolved by `PQCompanion.Paths.quarm_db/0`. It is not
  in version control, so `verify!/0` is called at boot to fail with an actionable
  message rather than starting into a degraded state.
  """

  alias PQCompanion.{MissingGameDatabaseError, Paths, QuarmDatabase, ReadOnlyRepoError}

  # ── Reads ─────────────────────────────────────────────────────────────────

  def all(queryable, opts \\ []), do: QuarmDatabase.all(queryable, opts)
  def one(queryable, opts \\ []), do: QuarmDatabase.one(queryable, opts)
  def one!(queryable, opts \\ []), do: QuarmDatabase.one!(queryable, opts)
  def exists?(queryable, opts \\ []), do: QuarmDatabase.exists?(queryable, opts)
  def get(queryable, id, opts \\ []), do: QuarmDatabase.get(queryable, id, opts)
  def get_by(queryable, clauses, opts \\ []), do: QuarmDatabase.get_by(queryable, clauses, opts)

  # ── The four tables wave 0 reads (task 4.3) ────────────────────────────────
  # Raw queries: schemas arrive in wave 2. ecto_sqlite3 refuses a schemaless
  # `select: source` (it cannot know the columns), and the tables here have up to
  # 187 of them, so a raw `SELECT *` is the honest tool rather than a hand-written
  # column list that rots. `:limit` defaults to 1 because wave 0 only needs to
  # prove the database is readable.

  @doc "A raw query against the `items` table."
  def items(opts \\ []), do: read_table("items", opts)

  @doc "A raw query against the `spells_new` table."
  def spells(opts \\ []), do: read_table("spells_new", opts)

  @doc "A raw query against the `npc_types` table."
  def npcs(opts \\ []), do: read_table("npc_types", opts)

  @doc "A raw query against the `zone` table."
  def zones(opts \\ []), do: read_table("zone", opts)

  # Table names are literals from this module, never caller input.
  defp read_table(table, opts) do
    limit = Keyword.get(opts, :limit, 1)
    QuarmDatabase.query!("SELECT * FROM #{table} LIMIT #{limit}", [])
  end

  # ── Writes: rejected before SQLite (task 4.2) ──────────────────────────────

  def insert(_struct_or_changeset, _opts \\ []), do: read_only!(:insert)
  def insert!(_struct_or_changeset, _opts \\ []), do: read_only!(:insert!)
  def update(_changeset, _opts \\ []), do: read_only!(:update)
  def update!(_changeset, _opts \\ []), do: read_only!(:update!)
  def delete(_struct_or_changeset, _opts \\ []), do: read_only!(:delete)
  def delete!(_struct_or_changeset, _opts \\ []), do: read_only!(:delete!)

  def insert_all(_schema_or_source, _entries, _opts \\ []), do: read_only!(:insert_all)
  def insert_all!(_schema_or_source, _entries, _opts \\ []), do: read_only!(:insert_all!)
  def update_all(_queryable, _updates, _opts \\ []), do: read_only!(:update_all)
  def update_all!(_queryable, _updates, _opts \\ []), do: read_only!(:update_all!)
  def delete_all(_queryable, _opts \\ []), do: read_only!(:delete_all)
  def delete_all!(_queryable, _opts \\ []), do: read_only!(:delete_all!)

  defp read_only!(operation), do: raise(ReadOnlyRepoError, repo: __MODULE__, operation: operation)

  # ── Boot check (task 4.5) ──────────────────────────────────────────────────

  @doc """
  Is the game database present?

  "Present" means the file exists **and is non-empty**. A zero-byte file is not a
  usable database — it is what the test environment leaves behind so the
  read-only repo can start, and it must not be mistaken for the artifact.
  """
  @spec present?(String.t()) :: boolean()
  def present?(path \\ Paths.quarm_db()) do
    case File.stat(path) do
      {:ok, %{size: size}} -> size > 0
      _ -> false
    end
  end

  @doc "Whether a missing game database should abort boot (true outside test)."
  @spec required?() :: boolean()
  def required?, do: Application.get_env(:pq_companion, :quarm_db_required, true)

  @doc """
  Whether the application should start the game-database connection.

  True whenever the artifact is present — and also whenever it is required, in
  which case `verify!/0` has already raised. It is false only where the artifact
  is optional *and* absent (the test suite on a checkout that never downloaded
  `quarm.db`), so the app still boots and data-backed tests skip.
  """
  @spec available?() :: boolean()
  def available?, do: present?() or required?()

  @doc """
  Fail fast when the game database is missing.

  Called by `PQCompanion.Application.start/2` before the supervision tree comes
  up. Where the artifact is optional (the test environment) it returns `:ok` and
  `available?/0` keeps the connection out of the tree, so the suite boots and
  data-backed tests skip explicitly instead of failing.
  """
  @spec verify!() :: :ok
  def verify! do
    cond do
      present?() -> :ok
      required?() -> raise MissingGameDatabaseError, path: Paths.quarm_db()
      true -> :ok
    end
  end
end
