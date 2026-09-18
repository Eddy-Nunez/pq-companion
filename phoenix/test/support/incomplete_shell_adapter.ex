defmodule PQCompanion.Shell.IncompleteAdapter do
  @moduledoc """
  A deliberately incomplete shell adapter, used only to prove that
  `PQCompanion.Shell.Conformance.run/1` fails and *names* the operation an
  adapter is missing.

  It is intentionally not annotated with `@behaviour PQCompanion.Shell` — that
  would emit compile-time warnings for the missing callbacks, and the conformance
  suite is supposed to catch the gap at runtime, not the compiler.
  """

  # Only two of the six required operations. close/1, resize/2,
  # set_click_through/2 and focus/1 are deliberately absent.
  def open(_spec), do: {:ok, %{unmet: []}}
  def list, do: []
end
