defmodule PQCompanion.UserRepoTest do
  @moduledoc """
  Wave 0 tasks 4.1 (user side) and 4.4 — the read-write user repository.

  `DataCase` checks out a sandbox connection for the supervised repo. The
  durability test deliberately starts its *own* repo instance on a plain pool,
  pointed at a scratch file, so its writes commit and can be read back after the
  instance is stopped and reopened — which is what "visible after a restart"
  means without restarting the whole application.
  """

  use PQCompanion.DataCase, async: false

  alias PQCompanion.{Paths, UserRepo}

  describe "task 4.1 — the user repo is supervised and healthy" do
    test "is in the supervision tree and answers a query" do
      assert Process.whereis(UserRepo)
      assert %{rows: [[1]]} = UserRepo.query!("SELECT 1")
    end

    test "opens read-write with WAL and a busy timeout" do
      config = UserRepo.config()
      assert config[:journal_mode] == :wal
      assert config[:busy_timeout] == 5_000
      assert %{rows: [["wal"]]} = UserRepo.query!("PRAGMA journal_mode")
    end

    test "the resolver points at the reference footprint" do
      assert Paths.user_db() == Path.join(Paths.app_home(), "user.db")
    end
  end

  describe "task 4.4 — an existing reference-created user database" do
    test "a value written is visible after the database is reopened" do
      tmp = Path.join(System.tmp_dir!(), "pq_user_#{System.unique_integer([:positive])}.db")
      on_exit(fn -> File.rm_rf(tmp) end)

      {:ok, pid} = start_scratch_repo(tmp)

      # Simulate a database the reference app created: a table we did not
      # migrate into existence.
      UserRepo.query!("CREATE TABLE ref_probe (id INTEGER PRIMARY KEY, note TEXT NOT NULL)")
      UserRepo.query!("INSERT INTO ref_probe (id, note) VALUES (1, 'durable')")
      assert %{rows: [["durable"]]} = UserRepo.query!("SELECT note FROM ref_probe WHERE id = 1")

      :ok = Supervisor.stop(pid)

      {:ok, pid2} = start_scratch_repo(tmp)

      assert %{rows: [["durable"]]} = UserRepo.query!("SELECT note FROM ref_probe WHERE id = 1")

      :ok = Supervisor.stop(pid2)
    end
  end

  defp start_scratch_repo(path) do
    {:ok, pid} =
      UserRepo.start_link(name: nil, database: path, pool: DBConnection.ConnectionPool)

    UserRepo.put_dynamic_repo(pid)
    {:ok, pid}
  end
end
