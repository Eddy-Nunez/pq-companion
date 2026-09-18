defmodule PQCompanion.Runtime do
  @moduledoc """
  The runtime announcement a native shell consumes (wave 0 tasks 5.1–5.2,
  design D6).

  The reference is spawned by Electron with piped stdio and prints
  `BACKEND_PORT=N` so its parent learns the actual port; it also writes
  `~/.pq-companion/server-port` for a parent that did not spawn it. This module
  keeps the *intent* — a supervisor must be able to discover where to connect —
  but emits one richer record instead of two ad-hoc channels:

    * `~/.pq-companion/runtime.json` — `{"port", "pid", "version"}`, written
      atomically and removed on a clean shutdown, for a shell that starts the
      app detached;
    * the same record as a single JSON line on stdout, for a shell that spawns
      and supervises it.

  The record is written **after** the listener is bound and carries the port the
  listener actually bound, which is not necessarily the configured one: when the
  preferred port is taken, `choose_port/1` asks the OS for a free one.

  Ported from (frozen reference): the `BACKEND_PORT=N` stdout contract and the
  `server-port` file in `backend/cmd/server/main.go`.
  """

  require Logger

  alias PQCompanion.Paths

  @record_file "runtime.json"
  @default_port 4000

  @doc "The path of the runtime record."
  @spec record_path(keyword()) :: String.t()
  def record_path(opts \\ []) do
    home = Keyword.get(opts, :home) || Paths.app_home()
    Path.join(home, @record_file)
  end

  @doc "The application version as a string, or `\"0.0.0\"` when unavailable."
  @spec version() :: String.t()
  def version do
    case Application.spec(:pq_companion, :vsn) do
      nil -> "0.0.0"
      vsn -> to_string(vsn)
    end
  end

  @doc "The OS process id, as the reference records it."
  @spec os_pid() :: String.t()
  def os_pid, do: System.pid()

  @doc "Build the runtime record for an actually-bound port."
  @spec build_record(integer()) :: map()
  def build_record(port) do
    %{"port" => port, "pid" => os_pid(), "version" => version()}
  end

  @doc """
  Pick the port to configure the listener with.

  Returns `preferred` when a listener can bind it, otherwise `0` so the OS
  assigns a free one — the reference's behaviour when the preferred port is busy
  (e.g. another local app on the same port).

  The probe is a short-lived listener, so a race exists between it and Bandit's
  own bind. The reference avoids that by holding its listener across the
  fallback; replicating that here would mean reaching into the adapter. This
  narrow race is accepted for wave 0 and noted in the handoff.
  """
  @spec choose_port(integer()) :: non_neg_integer()
  def choose_port(preferred) do
    case :gen_tcp.listen(preferred, [:binary, ip: {127, 0, 0, 1}, active: false]) do
      {:ok, socket} ->
        :gen_tcp.close(socket)
        preferred

      {:error, _reason} ->
        0
    end
  end

  @doc """
  Resolve and set the endpoint's HTTP port before the listener starts.

  A no-op when the server is disabled (the test environment), so it never
  rewrites the configured port in tests.
  """
  @spec configure_port!() :: non_neg_integer() | nil
  def configure_port! do
    if server_enabled?() do
      config = Application.get_env(:pq_companion, PQCompanionWeb.Endpoint, [])
      http = Keyword.get(config, :http, [])
      chosen = choose_port(Keyword.get(http, :port, @default_port))

      config = Keyword.put(config, :http, Keyword.put(http, :port, chosen))
      Application.put_env(:pq_companion, PQCompanionWeb.Endpoint, config)
      chosen
    end
  end

  @doc "Whether the endpoint starts a listener in this environment."
  @spec server_enabled?() :: boolean()
  def server_enabled? do
    Application.get_env(:pq_companion, PQCompanionWeb.Endpoint, [])[:server] != false
  end

  @doc """
  The port the running listener actually bound.

  `Bandit.PhoenixAdapter.server_info/2` resolves the Bandit process under the
  endpoint supervisor and asks Thousand Island for the socket's real address —
  the same source the reference's `listener.Addr()` provides.
  """
  @spec actual_port(module()) :: {:ok, integer()} | {:error, term()}
  def actual_port(endpoint \\ PQCompanionWeb.Endpoint) do
    case Bandit.PhoenixAdapter.server_info(endpoint, :http) do
      {:ok, {_address, port}} -> {:ok, port}
      {:error, reason} -> {:error, reason}
    end
  end

  @doc "Announce the port the configured endpoint actually bound."
  @spec announce_from(module(), keyword()) :: {:ok, map()} | {:error, term()}
  def announce_from(endpoint \\ PQCompanionWeb.Endpoint, opts \\ []) do
    case actual_port(endpoint) do
      {:ok, port} -> announce(port, opts)
      {:error, reason} -> {:error, reason}
    end
  end

  @doc """
  Write and emit the record for a bound port.

  Options: `:home` (override the app home, for tests) and `:io` (override the
  output device, default `:stdio`).
  """
  @spec announce(integer(), keyword()) :: {:ok, map()} | {:error, term()}
  def announce(port, opts \\ []) do
    record = build_record(port)

    with :ok <- write(record, opts) do
      emit(record, opts)
      {:ok, record}
    end
  end

  @doc "Write the record atomically (temp file plus rename)."
  @spec write(map(), keyword()) :: :ok | {:error, term()}
  def write(record, opts \\ []) do
    path = record_path(opts)
    File.mkdir_p!(Path.dirname(path))
    tmp = path <> ".tmp"
    File.write!(tmp, Jason.encode!(record))
    File.rename!(tmp, path)
    :ok
  rescue
    error -> {:error, error}
  end

  @doc "Emit the record as one line on the output device."
  @spec emit(map(), keyword()) :: :ok
  def emit(record, opts \\ []) do
    IO.puts(Keyword.get(opts, :io, :stdio), Jason.encode!(record))
    :ok
  end

  @doc "Remove the record; a missing record is not an error."
  @spec retract(keyword()) :: :ok | {:error, term()}
  def retract(opts \\ []) do
    case File.rm(record_path(opts)) do
      :ok -> :ok
      {:error, :enoent} -> :ok
      {:error, reason} -> {:error, reason}
    end
  end
end
