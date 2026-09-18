defmodule PQCompanion.Windows do
  @moduledoc """
  The canonical registry of PQ Companion's windows.

  Single source of truth for: which windows exist, their route, and the native
  properties each requires. Mirrors the reference app's 16 `OverlayPage` routes
  in `frontend/src/App.tsx` plus the main window.

  Ported from (frozen reference): `frontend/src/App.tsx`, and the `BrowserWindow`
  option sets in `electron/main/index.ts` (the source of the native properties).
  """

  @type t :: %__MODULE__{}

  defstruct [
    :id,
    :slug,
    :route,
    :label,
    :kind,
    :transparent,
    :always_on_top,
    :click_through,
    :frameless,
    :resizable,
    :display_only,
    :zoom
  ]

  @main_id "main"

  # Defaults for an overlay window, from the reference's overlay BrowserWindow
  # options: transparent, always-on-top, frameless, not resizable, no chrome.
  # `click_through` is false here because the reference toggles it at runtime
  # via an explicit lock control rather than defaulting to pass-through.
  @overlay_defaults [
    kind: :overlay,
    transparent: true,
    always_on_top: true,
    click_through: false,
    frameless: true,
    resizable: false,
    display_only: false,
    zoom: 1.0
  ]

  # The 16 overlays, in the reference's route order. `id` matches the
  # reference's `overlayKey` so per-overlay preferences (zoom, position, lock)
  # keep working after the port.
  # The 16 overlays. `id` is the reference's `overlayKey`, kept verbatim so
  # per-overlay preferences (zoom, position, lock) survive the port. `slug` is
  # the kebab-case URL segment — routes and ids deliberately differ, which is
  # why `fetch/1` and `fetch_by_slug/1` both exist. Conflating them was a bug:
  # mounting /w/buff-timer looked up "buff-timer" and found nothing.
  @overlays [
    {"dps", "dps", "DPS Meter"},
    {"hps", "hps", "HPS Meter"},
    {"buffTimer", "buff-timer", "Buff Timers"},
    {"detrimTimer", "detrim-timer", "Detrimental Timers"},
    {"customTimer", "custom-timer", "Custom Timers"},
    {"trigger", "trigger", "Trigger Alerts"},
    {"npc", "npc", "NPC Info"},
    {"discordVoice", "discord-voice", "Discord Voice"},
    {"threat", "threat", "Threat Meter"},
    {"rollTracker", "roll-tracker", "Roll Tracker"},
    {"respawnTimer", "respawn-timer", "Respawn Timers"},
    {"zoneLockouts", "zone-lockouts", "Zone Lockouts"},
    {"raidReadiness", "raid-readiness", "Raid Readiness"},
    {"liveMap", "live-map", "Live Map"},
    {"chChain", "ch-chain", "CH Chain"},
    {"chMetronome", "ch-metronome", "CH Metronome"}
  ]

  @doc "Every window, main window first."
  @spec all() :: [t()]
  def all, do: [main() | overlays()]

  @doc "The main window."
  @spec main() :: t()
  def main do
    struct!(
      %__MODULE__{},
      id: @main_id,
      slug: @main_id,
      route: "/",
      label: "PQ Companion",
      kind: :main,
      transparent: false,
      always_on_top: false,
      click_through: false,
      frameless: false,
      resizable: true,
      display_only: false,
      zoom: 1.0
    )
  end

  @doc "The 16 overlay windows."
  @spec overlays() :: [t()]
  def overlays do
    Enum.map(@overlays, fn {id, slug, label} ->
      struct!(
        %__MODULE__{},
        [id: id, slug: slug, route: "/w/" <> slug, label: label] ++ @overlay_defaults
      )
    end)
  end

  @doc "Look a window up by id, or nil."
  @spec fetch(String.t()) :: t() | nil
  def fetch(id), do: Enum.find(all(), &(&1.id == id))

  @doc """
  Look a window up by URL slug, or nil.

  Distinct from `fetch/1` on purpose: the URL segment is kebab-case, the id is the
  reference's camelCase overlayKey.
  """
  @spec fetch_by_slug(String.t()) :: t() | nil
  def fetch_by_slug(slug), do: Enum.find(all(), &(&1.slug == slug))

  @doc "Look a window up by full route, or nil."
  @spec fetch_by_route(String.t()) :: t() | nil
  def fetch_by_route(route), do: Enum.find(all(), &(&1.route == route))

  @doc "True when the window is an overlay."
  @spec overlay?(t()) :: boolean()
  def overlay?(%__MODULE__{kind: :overlay}), do: true
  def overlay?(%__MODULE__{}), do: false
end
