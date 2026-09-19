defmodule PQCompanion.Config.ServerTest do
  @moduledoc """
  Wave 1 task 5.1 — the single owning process for settings writes: serialised
  updates, atomic saves, and the change broadcast every window listens for.
  """

  use ExUnit.Case, async: true

  alias PQCompanion.Config
  alias PQCompanion.Config.Server

  setup do
    dir = Path.join(System.tmp_dir!(), "pq_cfg_server_#{System.unique_integer([:positive])}")
    File.mkdir_p!(dir)
    path = Path.join(dir, "config.yaml")
    File.write!(path, "eq_path: /eq\npreferences:\n  map_style: outline\n")
    name = :"cfg_server_#{System.unique_integer([:positive])}"
    start_supervised!({Server, path: path, name: name})
    %{dir: dir, path: path, server: name}
  end

  describe "task 5.1 — one writer owns the file" do
    test "reads the file at boot", %{server: server} do
      assert Server.get(server)["eq_path"] == "/eq"
      assert Server.preferences(server)["map_style"] == "outline"
    end

    test "sidebar/1 resolves defaults for absent keys", %{server: server} do
      assert Server.sidebar(server) == %{
               hidden: [],
               order: [],
               favorites: [],
               collapsed: %{},
               flags: %{}
             }
    end

    test "update_sidebar/2 persists and broadcasts", %{server: server, path: path} do
      Server.subscribe()

      assert {:ok, _} = Server.update_sidebar(%{"sidebar_hidden" => ["/loot"]}, server)

      assert_receive {:config_updated, config}
      assert Config.sidebar_hidden(config) == ["/loot"]
      assert Config.sidebar_hidden(Config.load!(path)) == ["/loot"]
      # and everything we do not own is untouched
      assert Config.load!(path)["eq_path"] == "/eq"
      assert Config.load!(path) |> Config.preferences() |> Map.fetch!("map_style") == "outline"
    end

    test "concurrent updates serialise and always leave a parseable file", %{
      server: server,
      path: path,
      dir: dir
    } do
      tasks =
        for i <- 1..12 do
          Task.async(fn ->
            Server.update_sidebar(%{"sidebar_order" => ["/route-#{i}"]}, server)
          end)
        end

      assert Enum.all?(Task.await_many(tasks), &match?({:ok, _}, &1))

      # The file parses, and its value is exactly one of the accepted writes.
      assert %{"preferences" => %{"sidebar_order" => [route]}} = Config.load!(path)
      assert route =~ ~r|^/route-\d+$|

      # No temp file survives.
      assert dir |> File.ls!() |> Enum.filter(&String.ends_with?(&1, ".tmp")) == []
    end

    test "a failed save is reported and leaves memory and disk unchanged", %{
      server: server,
      path: path
    } do
      # Replace the file with a directory so the rename cannot succeed.
      File.rm!(path)
      File.mkdir_p!(path)

      assert {:error, _} = Server.update_sidebar(%{"sidebar_hidden" => ["/loot"]}, server)
      assert Server.sidebar(server).hidden == []
    end

    test "reload/1 picks up an external edit", %{server: server, path: path} do
      File.write!(path, "preferences:\n  sidebar_favorites:\n    - /items\n")
      assert :ok = Server.reload(server)
      assert Server.sidebar(server).favorites == ["/items"]
    end
  end
end
