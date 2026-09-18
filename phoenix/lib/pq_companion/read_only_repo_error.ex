defmodule PQCompanion.ReadOnlyRepoError do
  @moduledoc """
  Raised when a write is attempted against the game-data repository.

  This is raised by `PQCompanion.QuarmRepo` itself — in Elixir, before any
  statement reaches SQLite — not by the database. That distinction is the point:
  the read-only guarantee is structural, so a mistake fails immediately and
  loudly instead of depending on the database engine to refuse.
  """

  defexception [:repo, :operation]

  @impl true
  def message(%{repo: repo, operation: operation}) do
    """
    #{inspect(repo)} is read-only: #{operation} is not permitted.

    The game database (quarm.db) is shipped read-only and is never written.
    User data belongs in PQCompanion.UserRepo.
    """
  end

  def message(%{operation: operation}) do
    "the game-data repository is read-only: #{operation} is not permitted"
  end
end
