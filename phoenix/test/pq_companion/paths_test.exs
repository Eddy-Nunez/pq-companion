defmodule PQCompanion.PathsTest do
  @moduledoc "Wave 0 task 4.6 — the reference on-disk footprint."

  use ExUnit.Case, async: false

  alias PQCompanion.Paths

  describe "task 4.6 — the reference footprint, resolved in one place" do
    test "resolves the five reference locations" do
      paths = Paths.resolve(home: "/home/tester/.pq-companion")

      assert paths.home == "/home/tester/.pq-companion"
      assert paths.user_db == "/home/tester/.pq-companion/user.db"
      assert paths.config == "/home/tester/.pq-companion/config.yaml"
      assert paths.backups == "/home/tester/.pq-companion/backups"
      assert paths.server_log == "/home/tester/.pq-companion/logs/server.log"
    end

    test "the defaults reproduce the reference app's home" do
      expected_home = Path.join(System.user_home!(), ".pq-companion")

      assert Paths.app_home() == expected_home
      assert Paths.user_db() == Path.join(expected_home, "user.db")
      assert Paths.config() == Path.join(expected_home, "config.yaml")
      assert Paths.backups() == Path.join(expected_home, "backups")
      assert Paths.server_log() == Path.join([expected_home, "logs", "server.log"])
    end

    test "the game database lives beside the application, not under the home" do
      assert String.ends_with?(Paths.quarm_db(), "priv/data/quarm.db")
      refute String.starts_with?(Paths.quarm_db(), Paths.app_home())
    end

    test "booting against an existing install opens the existing files" do
      tmp = tmp_dir()
      File.mkdir_p!(Path.join(tmp, "logs"))
      File.mkdir_p!(Path.join(tmp, "backups"))
      File.write!(Path.join(tmp, "user.db"), "existing-user-db")
      File.write!(Path.join(tmp, "config.yaml"), "existing: config\n")

      paths = Paths.resolve(home: tmp)
      Paths.ensure!(paths)

      assert File.read!(paths.user_db) == "existing-user-db"
      assert File.read!(paths.config) == "existing: config\n"
    end

    test "a fresh machine gets a valid footprint without inventing data files" do
      paths = Paths.ensure!(home: tmp_dir())

      assert File.dir?(paths.home)
      assert File.dir?(paths.backups)
      assert File.dir?(Path.dirname(paths.server_log))
      refute File.exists?(paths.user_db)
      refute File.exists?(paths.config)
    end
  end

  defp tmp_dir do
    dir = Path.join(System.tmp_dir!(), "pq_paths_#{System.unique_integer([:positive])}")
    on_exit(fn -> File.rm_rf(dir) end)
    dir
  end
end
