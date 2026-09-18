defmodule PQCompanion.QuarmRepoTest do
  @moduledoc """
  Wave 0 tasks 4.1, 4.2, 4.3 and 4.5 — the game-data repository.

  `async: false` because one describe block temporarily overrides the game
  database path in the application environment.
  """

  use ExUnit.Case, async: false

  alias PQCompanion.{
    MissingGameDatabaseError,
    Paths,
    QuarmDatabase,
    QuarmRepo,
    ReadOnlyRepoError
  }

  describe "task 4.1 — the game repo is supervised and healthy" do
    test "is in the supervision tree and answers a query" do
      if QuarmRepo.present?() do
        assert Process.whereis(QuarmDatabase)
        assert %{rows: [[1]]} = QuarmDatabase.query!("SELECT 1")
      else
        warn_skip()
      end
    end

    test "is opened read-only at the connection level" do
      if QuarmRepo.present?() do
        config = QuarmDatabase.config()
        assert config[:mode] == :readonly
        assert String.ends_with?(config[:database], "quarm.db")
      else
        warn_skip()
      end
    end
  end

  describe "task 4.2 — writes are rejected before SQLite" do
    test "insert, update and delete raise and name the repository" do
      for operation <- [:insert, :update, :delete] do
        error = assert_raise(ReadOnlyRepoError, fn -> apply(QuarmRepo, operation, [%{}]) end)

        assert Exception.message(error) =~ "PQCompanion.QuarmRepo is read-only"
        assert error.repo == QuarmRepo
        assert error.operation == operation
      end
    end

    test "the bulk writes are guarded too" do
      assert_raise ReadOnlyRepoError, fn -> QuarmRepo.insert_all("items", [%{}]) end
      assert_raise ReadOnlyRepoError, fn -> QuarmRepo.update_all("items", set: [id: 1]) end
      assert_raise ReadOnlyRepoError, fn -> QuarmRepo.delete_all("items") end
    end

    test "the exception is ours, not SQLite's — the guard runs in Elixir" do
      # An attempt that reached SQLite would surface as an Exqlite error; a
      # ReadOnlyRepoError can only come from `QuarmRepo` itself. That is the
      # "before the statement reaches SQLite" guarantee.
      assert_raise ReadOnlyRepoError, fn -> QuarmRepo.insert(%{}) end
    end
  end

  describe "task 4.3 — reading the four game tables" do
    test "reads rows from items, spells_new, npc_types and zone" do
      if QuarmRepo.present?() do
        items = QuarmRepo.items()
        assert [_ | _] = items.rows
        assert "id" in items.columns
        assert "Name" in items.columns

        spells = QuarmRepo.spells()
        assert [_ | _] = spells.rows
        assert "name" in spells.columns

        npcs = QuarmRepo.npcs()
        assert [_ | _] = npcs.rows
        assert "name" in npcs.columns

        zones = QuarmRepo.zones()
        assert [_ | _] = zones.rows
        assert "short_name" in zones.columns
      else
        warn_skip()
      end
    end
  end

  describe "task 4.5 — a missing game database is actionable, never silent" do
    test "present?/1 is false for a missing or empty file" do
      dir = tmp_dir()

      refute QuarmRepo.present?(Path.join(dir, "absent.db"))

      empty = Path.join(dir, "empty.db")
      File.touch!(empty)
      refute QuarmRepo.present?(empty), "a zero-byte scratch file must not count as the artifact"
    end

    test "verify!/0 raises an error naming the path and how to obtain the file" do
      missing = Path.join(tmp_dir(), "quarm.db")

      with_env(%{quarm_db: missing, quarm_db_required: true}, fn ->
        error = assert_raise(MissingGameDatabaseError, fn -> QuarmRepo.verify!() end)
        message = Exception.message(error)

        assert message =~ missing
        assert message =~ "quarm.db"
        assert message =~ "curl -L -o"
        assert message =~ "data-latest"
      end)
    end

    test "verify!/0 tolerates absence where the artifact is optional" do
      missing = Path.join(tmp_dir(), "quarm.db")

      with_env(%{quarm_db: missing, quarm_db_required: false}, fn ->
        assert :ok = QuarmRepo.verify!()
        refute QuarmRepo.present?(missing)
        # ...and the connection is kept out of the supervision tree entirely, so
        # the app still boots and data-backed tests skip.
        refute QuarmRepo.available?()
      end)
    end
  end

  defp warn_skip do
    IO.warn(
      "quarm.db is not present at #{Paths.quarm_db()} — skipping the game-data assertions. " <>
        "See PQCompanion.QuarmRepo.verify!/0 for the download command."
    )
  end

  defp tmp_dir do
    dir = Path.join(System.tmp_dir!(), "pq_quarm_#{System.unique_integer([:positive])}")
    File.mkdir_p!(dir)
    on_exit(fn -> File.rm_rf(dir) end)
    dir
  end

  defp with_env(overrides, fun) do
    original =
      Map.new(overrides, fn {key, _} -> {key, Application.get_env(:pq_companion, key)} end)

    for {key, value} <- overrides, do: Application.put_env(:pq_companion, key, value)

    try do
      fun.()
    after
      for {key, value} <- original do
        if is_nil(value) do
          Application.delete_env(:pq_companion, key)
        else
          Application.put_env(:pq_companion, key, value)
        end
      end
    end
  end
end
