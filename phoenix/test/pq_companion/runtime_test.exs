defmodule PQCompanion.RuntimeTest do
  @moduledoc """
  Wave 0 tasks 5.1–5.3 — the runtime announcement and the untouched settings file.

  `async: false` because the tests override the application home and because one
  of them binds real TCP ports.
  """

  use ExUnit.Case, async: false

  alias PQCompanion.{Paths, Runtime}

  defmodule OkPlug do
    @moduledoc false
    def init(opts), do: opts
    def call(conn, _opts), do: Plug.Conn.send_resp(conn, 200, "ok")
  end

  describe "task 5.1 — the record reflects the actually-bound port" do
    test "choose_port/1 keeps a free preferred port" do
      {:ok, free} = :gen_tcp.listen(0, [:binary, ip: {127, 0, 0, 1}, active: false])
      {:ok, {_addr, port}} = :inet.sockname(free)
      :gen_tcp.close(free)

      assert Runtime.choose_port(port) == port
    end

    test "choose_port/1 falls back to an OS-assigned port when the preferred is busy" do
      {:ok, occupied} = :gen_tcp.listen(0, [:binary, ip: {127, 0, 0, 1}, active: false])
      on_exit(fn -> :gen_tcp.close(occupied) end)
      {:ok, {_addr, preferred}} = :inet.sockname(occupied)

      assert Runtime.choose_port(preferred) == 0
    end

    test "the record carries the port the listener actually bound, not the preferred one" do
      # Occupy the preferred port, then stand in the listener the app would have
      # started on the OS-assigned port and read its address exactly as
      # Runtime.actual_port/1 does.
      {:ok, occupied} = :gen_tcp.listen(0, [:binary, ip: {127, 0, 0, 1}, active: false])
      {:ok, {_addr, preferred}} = :inet.sockname(occupied)
      on_exit(fn -> :gen_tcp.close(occupied) end)
      assert Runtime.choose_port(preferred) == 0

      server = start_supervised!({Bandit, plug: OkPlug, port: 0, ip: {127, 0, 0, 1}})

      {:ok, {_address, actual}} = ThousandIsland.listener_info(server)
      assert actual != preferred and actual > 0

      {:ok, io} = StringIO.open("")
      tmp = tmp_dir()

      assert {:ok, record} = Runtime.announce(actual, home: tmp, io: io)

      assert record["port"] == actual
      assert record["port"] != preferred
      assert is_binary(record["pid"])
      assert is_binary(record["version"])

      on_disk = Jason.decode!(File.read!(Runtime.record_path(home: tmp)))
      assert on_disk == record

      {_input, output} = StringIO.contents(io)
      assert output |> String.trim() |> Jason.decode!() == record
    end

    test "announce_from/2 reports the endpoint has no listener when the server is disabled" do
      refute Runtime.server_enabled?()
      assert {:error, :no_server_found} = Runtime.actual_port(PQCompanionWeb.Endpoint)
    end
  end

  describe "task 5.2 — no stale record survives a clean stop" do
    test "retract/1 removes the record and tolerates its absence" do
      tmp = tmp_dir()
      {:ok, io} = StringIO.open("")

      assert {:ok, _} = Runtime.announce(4321, home: tmp, io: io)
      assert File.exists?(Runtime.record_path(home: tmp))

      assert :ok = Runtime.retract(home: tmp)
      refute File.exists?(Runtime.record_path(home: tmp))

      # Removing it twice is not an error.
      assert :ok = Runtime.retract(home: tmp)
    end

    test "the application's stop/1 removes the record, as a clean shutdown does" do
      tmp = tmp_dir()

      with_home(tmp, fn ->
        {:ok, io} = StringIO.open("")
        assert {:ok, _} = Runtime.announce(4321, io: io)
        assert File.exists?(Runtime.record_path())

        assert :ok = PQCompanion.Application.stop(:normal)

        refute File.exists?(Runtime.record_path())
      end)
    end
  end

  describe "task 5.3 — the settings file is not modified by this change" do
    test "an existing config.yaml is byte-identical after boot-time side effects" do
      tmp = tmp_dir()
      config = Path.join(tmp, "config.yaml")

      original = "preferences:\n  debug_logging: false\n  theme: dark\n"
      File.write!(config, original)

      with_home(tmp, fn ->
        Paths.ensure!()

        {:ok, io} = StringIO.open("")
        assert {:ok, _} = Runtime.announce(4321, io: io)
        assert :ok = Runtime.retract()
      end)

      assert File.read!(config) == original
    end
  end

  defp tmp_dir do
    dir = Path.join(System.tmp_dir!(), "pq_runtime_#{System.unique_integer([:positive])}")
    File.mkdir_p!(dir)
    on_exit(fn -> File.rm_rf(dir) end)
    dir
  end

  defp with_home(home, fun) do
    original = Application.get_env(:pq_companion, :app_home)
    Application.put_env(:pq_companion, :app_home, home)

    try do
      fun.()
    after
      if is_nil(original) do
        Application.delete_env(:pq_companion, :app_home)
      else
        Application.put_env(:pq_companion, :app_home, original)
      end
    end
  end
end
