defmodule PQCompanion.Shell do
  @moduledoc """
  The swappable native-shell adapter.

  The LiveView server is authoritative about *what* a window is; the shell is
  authoritative about *how* to realise it. This module is the only seam between
  them: feature code calls `PQCompanion.Shell`, never a concrete adapter.

  ## Delivery mechanism

  A window's full spec travels in the rendered document as a machine-readable
  `<meta name="pq-window">` tag (see `spec/1` and `to_json/1`), which a native
  shell scrapes on load. Live changes (bounds, zoom, display-only, click-through,
  lock) travel the other way over the LiveView event channel — the server pushes
  `"pq:window"` events and the client sends `"pq:save_bounds"` /
  `"pq:set_*"` events back (see `PQCompanionWeb.ShellEvents`).

  Putting the *declaration* in the document means a window is fully specified the
  moment it loads — no ordering handshake where a shell configures a window
  before the app knows what it wants — and every window stays testable in a plain
  browser.

  ## Adapters

  | Adapter | Status |
  |---|---|
  | `PQCompanion.Shell.Browser` | development default; opens a normal tab and reports native-only properties as unmet |
  | `PQCompanion.Shell.Electron` | candidate for wave 8 |
  | `PQCompanion.Shell.Tauri` | candidate for wave 8 |

  The adapter is chosen by configuration (`config :pq_companion, :shell_adapter`),
  so swapping it requires no change to feature code. Any new adapter must pass
  `PQCompanion.Shell.Conformance.run/1`.

  Reference (frozen): the Electron `BrowserWindow` option sets in
  `electron/main/index.ts` are the source of the native properties; the plan's
  §2.4 defines this contract.
  """

  alias PQCompanion.Windows
  alias PQCompanion.WindowState

  @token_salt "pq-window"
  @token_max_age 60 * 60 * 24

  @typedoc "A window rectangle in screen coordinates."
  @type bounds :: %{x: integer(), y: integer(), w: integer(), h: integer()}

  @typedoc """
  The complete description of a window, as the shell needs it.

  Unlike `PQCompanion.Windows.t/0` (the static declaration), this carries the
  runtime values — the session `token` and the user's current `bounds`, `zoom`,
  `display_only` and `click_through` — because that is what the shell realises.
  """
  @type window :: %{
          id: String.t(),
          slug: String.t(),
          kind: :main | :overlay,
          token: String.t(),
          route: String.t(),
          title: String.t(),
          transparent: boolean(),
          always_on_top: boolean(),
          click_through: boolean(),
          frameless: boolean(),
          resizable: boolean(),
          display_only: boolean(),
          bounds: bounds() | nil,
          zoom: float()
        }

  @doc "Open a window from a spec."
  @callback open(window()) :: {:ok, map()} | {:error, term()}

  @doc "Close a window by id."
  @callback close(String.t()) :: :ok | {:ok, map()} | {:error, term()}

  @doc "Resize/move a window by id."
  @callback resize(String.t(), bounds() | map()) :: :ok | {:ok, map()} | {:error, term()}

  @doc "Turn click-through on or off for a window."
  @callback set_click_through(String.t(), boolean()) :: :ok | {:ok, map()} | {:error, term()}

  @doc "Bring a window to the front."
  @callback focus(String.t()) :: :ok | {:ok, map()} | {:error, term()}

  @doc "Every window the shell currently has open."
  @callback list() :: [window()]

  @doc "The configured adapter module. Feature code must go through the functions below, not this."
  @spec adapter() :: module()
  def adapter do
    Application.get_env(:pq_companion, :shell_adapter, PQCompanion.Shell.Browser)
  end

  @doc """
  Build the shell spec for a window.

  Static properties come from `PQCompanion.Windows`; `bounds`, `zoom`,
  `display_only`, `click_through` and `locked` come from
  `PQCompanion.WindowState`, falling back to the declared defaults. A fresh
  session `token` is signed on every call unless one is supplied.
  """
  @spec spec(Windows.t(), keyword()) :: window()
  def spec(%Windows{} = window, opts \\ []) do
    runtime = WindowState.get(window.id)

    %{
      id: window.id,
      slug: window.slug,
      kind: window.kind,
      token: Keyword.get_lazy(opts, :token, fn -> sign_token(window.id) end),
      route: window.route,
      title: window.label,
      transparent: window.transparent,
      always_on_top: window.always_on_top,
      click_through: Map.get(runtime, :click_through, window.click_through),
      frameless: window.frameless,
      resizable: window.resizable,
      display_only: Map.get(runtime, :display_only, window.display_only),
      bounds: Map.get(runtime, :bounds),
      zoom: Map.get(runtime, :zoom, window.zoom)
    }
  end

  @doc """
  Encode a spec for the `pq-window` meta tag.

  Keys are camelCase because the consumer is a shell's JavaScript, not Elixir.
  """
  @spec to_json(window()) :: String.t()
  def to_json(spec) do
    %{
      "id" => spec.id,
      "slug" => spec.slug,
      "kind" => to_string(spec.kind),
      "token" => spec.token,
      "route" => spec.route,
      "title" => spec.title,
      "transparent" => spec.transparent,
      "alwaysOnTop" => spec.always_on_top,
      "clickThrough" => spec.click_through,
      "frameless" => spec.frameless,
      "resizable" => spec.resizable,
      "displayOnly" => spec.display_only,
      "bounds" => spec.bounds,
      "zoom" => spec.zoom
    }
    |> Jason.encode!()
  end

  @doc "Sign a per-window session token the shell can present back."
  @spec sign_token(String.t()) :: String.t()
  def sign_token(id) do
    nonce = Base.url_encode64(:crypto.strong_rand_bytes(9), padding: false)
    Phoenix.Token.sign(PQCompanionWeb.Endpoint, @token_salt, %{"id" => id, "nonce" => nonce})
  end

  @doc "Verify a token came from `sign_token/1` for `id`."
  @spec verify_token(String.t(), String.t()) :: {:ok, :verified} | {:error, term()}
  def verify_token(token, id) do
    case Phoenix.Token.verify(PQCompanionWeb.Endpoint, @token_salt, token,
           max_age: @token_max_age
         ) do
      {:ok, %{"id" => ^id}} -> {:ok, :verified}
      {:ok, %{"id" => _other}} -> {:error, :wrong_window}
      {:error, reason} -> {:error, reason}
    end
  end

  # ── Dispatch to the configured adapter ────────────────────────────────────
  # Feature code calls these. Only these functions know about `adapter/0`.

  @spec open(window()) :: {:ok, map()} | {:error, term()}
  def open(spec), do: adapter().open(spec)

  @spec close(String.t()) :: :ok | {:ok, map()} | {:error, term()}
  def close(id), do: adapter().close(id)

  @spec resize(String.t(), bounds() | map()) :: :ok | {:ok, map()} | {:error, term()}
  def resize(id, bounds), do: adapter().resize(id, bounds)

  @spec set_click_through(String.t(), boolean()) :: :ok | {:ok, map()} | {:error, term()}
  def set_click_through(id, value), do: adapter().set_click_through(id, value)

  @spec focus(String.t()) :: :ok | {:ok, map()} | {:error, term()}
  def focus(id), do: adapter().focus(id)

  @spec list() :: [window()]
  def list, do: adapter().list()
end
