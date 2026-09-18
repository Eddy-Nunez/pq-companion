defmodule PQCompanion.Paths do
  @moduledoc """
  The on-disk footprint, resolved in exactly one place (wave 0 task 4.6).

  Every path the reference (Go) application uses is reproduced here, so an
  existing install is found without a migration step:

  | Artifact | Reference |
  |---|---|
  | app home | `~/.pq-companion/` |
  | user database | `~/.pq-companion/user.db` |
  | settings | `~/.pq-companion/config.yaml` |
  | backups | `~/.pq-companion/backups/` |
  | server log | `~/.pq-companion/logs/server.log` |

  Ported from (frozen reference): `appHome`/`userDBPath`/`configPath`/
  `backupsDir` in `backend/cmd/server/main.go`, and `logsDir` in
  `backend/internal/applog/applog.go`.

  The shipped game database is *not* under the app home — it lives beside the
  application — so it is resolved separately by `quarm_db/0`.

  ## Overrides

  Two application-environment keys exist so tests and tooling do not touch a
  real install:

    * `:app_home` — replace `~/.pq-companion`
    * `:quarm_db` — replace the shipped `priv/data/quarm.db`
  """

  @app_dir_name ".pq-companion"

  @type t :: %__MODULE__{
          home: String.t(),
          user_db: String.t(),
          config: String.t(),
          backups: String.t(),
          server_log: String.t(),
          quarm_db: String.t()
        }

  defstruct [:home, :user_db, :config, :backups, :server_log, :quarm_db]

  @doc """
  Resolve the whole footprint at once.

  Options `:home` and `:quarm_db` override the corresponding locations, which is
  how the tests exercise a fresh or reference-shaped install.
  """
  @spec resolve(keyword()) :: t()
  def resolve(opts \\ []) do
    home = Keyword.get(opts, :home) || app_home()

    %__MODULE__{
      home: home,
      user_db: Path.join(home, "user.db"),
      config: Path.join(home, "config.yaml"),
      backups: Path.join(home, "backups"),
      server_log: Path.join([home, "logs", "server.log"]),
      quarm_db: Keyword.get(opts, :quarm_db) || quarm_db()
    }
  end

  @doc "The application home directory."
  @spec app_home() :: String.t()
  def app_home do
    Application.get_env(:pq_companion, :app_home) ||
      Path.join(System.user_home!(), @app_dir_name)
  end

  @doc "The read-write user database."
  @spec user_db() :: String.t()
  def user_db, do: Path.join(app_home(), "user.db")

  @doc "The reference settings file."
  @spec config() :: String.t()
  def config, do: Path.join(app_home(), "config.yaml")

  @doc "The backups directory."
  @spec backups() :: String.t()
  def backups, do: Path.join(app_home(), "backups")

  @doc "The rotating server log."
  @spec server_log() :: String.t()
  def server_log, do: Path.join([app_home(), "logs", "server.log"])

  @doc """
  The shipped, read-only game database.

  `Application.app_dir/2` resolves to the app's `priv/` in dev, test and a
  release alike; in dev/test Mix symlinks `priv` into `_build`, so this points at
  the same file as `priv/data/quarm.db` in the source tree.
  """
  @spec quarm_db() :: String.t()
  def quarm_db do
    Application.get_env(:pq_companion, :quarm_db) ||
      Application.app_dir(:pq_companion, "priv/data/quarm.db")
  end

  @doc """
  Create the directories a fresh install needs: the app home, the backups
  directory and the log directory.

  Deliberately does **not** create `user.db`, `config.yaml` or the game
  database: those have owners (Ecto, the config loader, the release artifact) and
  a "valid footprint" only means the directories exist.
  """
  @spec ensure!(t() | keyword()) :: t()
  def ensure!(paths \\ resolve())

  def ensure!(opts) when is_list(opts), do: ensure!(resolve(opts))

  def ensure!(%__MODULE__{} = paths) do
    for dir <- [paths.home, paths.backups, Path.dirname(paths.server_log)] do
      File.mkdir_p!(dir)
    end

    paths
  end
end
