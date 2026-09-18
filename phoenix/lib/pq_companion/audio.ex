defmodule PQCompanion.Audio do
  @moduledoc """
  Tracks which window owns audio/alert output.

  The reference app guarantees exactly one renderer may emit trigger and alert
  audio, so a single game event cannot play the same sound twice (see
  `MainWindowLayout`'s `setAudioOwner` in `frontend/src/App.tsx`). In LiveView the
  equivalent guarantee is that the main window registers as owner and overlay
  windows never do — overlays have no alert hooks at all.
  """

  @registry __MODULE__.Registry

  def child_spec(_opts) do
    Registry.child_spec(keys: :unique, name: @registry)
  end

  @doc "Register `window_id` as the audio owner. Called from the main window only."
  @spec register(String.t()) :: {:ok, pid()} | {:error, {:already_registered, pid()}}
  def register(window_id) do
    Registry.register(@registry, :audio_owner, window_id)
  end

  @doc "How many windows currently own audio output. Must be 0 or 1."
  @spec owner_count() :: non_neg_integer()
  def owner_count do
    Registry.count(@registry)
  end
end
