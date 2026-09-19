defmodule PQCompanion.Config do
  @moduledoc """
  `~/.pq-companion/config.yaml` — the user's settings file.

  The reference app owns this file (`backend/internal/config/config.go`), and its
  key names are a **public contract**: they are the user's data, and the Go app
  must still be able to read the file for rollback. So this module never renames
  or drops a key it does not understand — it loads the file as a plain
  string-keyed map, edits only the keys it owns, and writes the whole map back.

  **Wave 1 scope.** Only the sidebar preference keys are interpreted here:
  `preferences.sidebar_hidden`, `preferences.sidebar_order`,
  `preferences.sidebar_favorites`, the new `preferences.sidebar_collapsed_sections`,
  and the three feature flags (`pop_flags_enabled`, `faction_tracker_enabled`,
  `raids_enabled`). Wave 2 (`add-data-model`) extends this into a typed embedded
  schema over all ~186 fields; it does not replace the reader/writer or the
  atomic-save discipline established here.

  Reads use `yaml_elixir`; writes use `ymlr` (`yaml_elixir` has no encoder, and
  `ymlr` is pure Elixir with no NIF, unlike `fast_yaml`). The write is atomic —
  temp file in the same directory, then rename — mirroring the Go
  `WriteFile` so an interrupted save can never leave a truncated settings file.

  ## Fidelity note

  Output is **value-faithful**, not byte-identical to the Go writer: the document
  marker is stripped and key order is not Go's struct order. Every key and value
  survives, which is what rollback requires; byte-identical output is a Wave 2
  concern, and this note records that it is not yet achieved.
  """

  alias PQCompanion.Paths

  @preferences "preferences"

  @flags ~w(pop_flags_enabled faction_tracker_enabled raids_enabled)

  @sidebar_list_keys ~w(sidebar_hidden sidebar_order sidebar_favorites)
  @collapse_key "sidebar_collapsed_sections"

  @type t :: map()

  @doc "The settings file path (`~/.pq-companion/config.yaml`)."
  @spec path() :: String.t()
  def path, do: Paths.config()

  @doc """
  Read the settings file.

  A missing file is not an error — a fresh install has none — and yields an empty
  map. Unknown keys are preserved verbatim in the returned map.
  """
  @spec load(String.t()) :: {:ok, t()} | {:error, term()}
  def load(path \\ path()) do
    case File.read(path) do
      {:ok, body} -> decode(body)
      {:error, :enoent} -> {:ok, %{}}
      {:error, reason} -> {:error, reason}
    end
  end

  @doc "Like `load/1` but raises `File.Error`/`RuntimeError` on failure."
  @spec load!(String.t()) :: t()
  def load!(path \\ path()) do
    case load(path) do
      {:ok, config} -> config
      {:error, reason} -> raise "could not read #{path}: #{inspect(reason)}"
    end
  end

  @doc """
  Atomically write `config` to `path`.

  Writes a temp file in the target directory, then renames over the target, so an
  interrupted save leaves the previous file intact rather than a truncated one.
  """
  @spec save(t(), String.t()) :: :ok | {:error, term()}
  def save(config, path \\ path()) when is_map(config) do
    dir = Path.dirname(path)

    with :ok <- File.mkdir_p(dir),
         {:ok, tmp} <- write_temp(dir, config) do
      case File.rename(tmp, path) do
        :ok ->
          :ok

        {:error, reason} ->
          _ = File.rm(tmp)
          {:error, reason}
      end
    end
  end

  defp write_temp(dir, config) do
    tmp = Path.join(dir, ".config-#{System.unique_integer([:positive])}.yaml.tmp")
    body = config |> Ymlr.document!() |> String.replace_prefix("---\n", "")

    case File.write(tmp, body) do
      :ok ->
        # The Go writer chmods the temp file to 0644 before the rename.
        _ = File.chmod(tmp, 0o644)
        {:ok, tmp}

      {:error, reason} ->
        {:error, reason}
    end
  end

  defp decode(body) do
    case YamlElixir.read_from_string(body) do
      {:ok, nil} -> {:ok, %{}}
      {:ok, config} when is_map(config) -> {:ok, config}
      {:ok, other} -> {:error, {:not_a_map, other}}
      {:error, error} -> {:error, {:invalid_yaml, error}}
    end
  end

  ## Preferences

  @doc "The `preferences` map, or an empty map when absent or malformed."
  @spec preferences(t()) :: map()
  def preferences(config) do
    case config[@preferences] do
      prefs when is_map(prefs) -> prefs
      _ -> %{}
    end
  end

  @doc "The feature-flag map consumed by `PQCompanionWeb.Nav.flags/1`."
  @spec flags(t()) :: map()
  def flags(config), do: Map.take(preferences(config), @flags)

  @doc "The sidebar preference keys this module owns."
  @spec sidebar_keys() :: [String.t()]
  def sidebar_keys, do: @sidebar_list_keys ++ [@collapse_key]

  @doc "Hidden sidebar routes."
  @spec sidebar_hidden(t()) :: [String.t()]
  def sidebar_hidden(config), do: preference_list(config, "sidebar_hidden")

  @doc "The user's within-section sidebar order (a flat list of routes)."
  @spec sidebar_order(t()) :: [String.t()]
  def sidebar_order(config), do: preference_list(config, "sidebar_order")

  @doc "Starred sidebar routes."
  @spec sidebar_favorites(t()) :: [String.t()]
  def sidebar_favorites(config), do: preference_list(config, "sidebar_favorites")

  @doc "Per-section collapse state, keyed by section id."
  @spec sidebar_collapsed(t()) :: %{String.t() => boolean()}
  def sidebar_collapsed(config) do
    case preferences(config)[@collapse_key] do
      collapsed when is_map(collapsed) -> collapsed
      _ -> %{}
    end
  end

  @doc """
  Merge `changes` into `preferences`, leaving every other key — and every other
  top-level key — untouched. Additive only: this never removes a key the caller
  did not name.
  """
  @spec put_preferences(t(), map()) :: t()
  def put_preferences(config, changes) when is_map(changes) do
    Map.put(config, @preferences, Map.merge(preferences(config), changes))
  end

  @doc "Set the hidden sidebar routes."
  @spec put_sidebar_hidden(t(), [String.t()]) :: t()
  def put_sidebar_hidden(config, routes),
    do: put_preferences(config, %{"sidebar_hidden" => routes})

  @doc "Set the sidebar order."
  @spec put_sidebar_order(t(), [String.t()]) :: t()
  def put_sidebar_order(config, routes), do: put_preferences(config, %{"sidebar_order" => routes})

  @doc "Set the starred sidebar routes."
  @spec put_sidebar_favorites(t(), [String.t()]) :: t()
  def put_sidebar_favorites(config, routes),
    do: put_preferences(config, %{"sidebar_favorites" => routes})

  @doc "Set the per-section collapse state."
  @spec put_sidebar_collapsed(t(), map()) :: t()
  def put_sidebar_collapsed(config, collapsed) when is_map(collapsed),
    do: put_preferences(config, %{@collapse_key => collapsed})

  defp preference_list(config, key) do
    case preferences(config)[key] do
      list when is_list(list) -> list
      _ -> []
    end
  end
end
