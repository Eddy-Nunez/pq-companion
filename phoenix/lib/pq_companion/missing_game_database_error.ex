defmodule PQCompanion.MissingGameDatabaseError do
  @moduledoc """
  Raised at boot when the shipped game database is absent.

  The application refuses to start rather than serving an empty result set in
  its place, which is the failure mode the reference app is careful to avoid.
  The message names the expected path *and* where to obtain the file, because the
  artifact is not in version control (`*.db` is gitignored) and a fresh checkout
  legitimately does not have it.
  """

  defexception [:path]

  @download_url "https://github.com/jasonsoprovich/pq-companion/releases/download/data-latest/quarm.db"

  @impl true
  def message(%{path: path}) do
    """
    The game database is missing.

      expected path: #{path}

    Download it with:

      curl -L -o "#{path}" #{@download_url}

    The file ships as a release artifact and is not in version control.
    """
  end
end
