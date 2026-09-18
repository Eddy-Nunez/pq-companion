defmodule PQCompanion.Shell.Conformance do
  @moduledoc """
  The adapter conformance suite.

  Any module claiming to be a `PQCompanion.Shell` adapter must pass
  `run/1`. Wave 8's Electron/Tauri adapter is not "done" until this is green
  against it, which is what makes that work a bounded task rather than an
  open-ended one.

  `run/1` returns `:ok` or `{:error, reasons}`. Each reason names the operation
  that failed, so a stub missing an operation produces a message that says
  exactly which one — the spec's "fails and names the missing operation".

  It is a function rather than an ExUnit-generated test module so that tests can
  assert the *failure* path against a deliberately incomplete adapter, which a
  test-generating macro cannot easily do.
  """

  alias PQCompanion.Shell
  alias PQCompanion.Windows

  @operations [open: 1, close: 1, resize: 2, set_click_through: 2, focus: 1, list: 0]

  @doc "The operations every adapter must export, as `{name, arity}`."
  @spec operations() :: [{atom(), non_neg_integer()}]
  def operations, do: @operations

  @doc """
  Run the suite against `adapter`.

  Returns `:ok` when the adapter exports every operation and each one honours
  the contract, otherwise `{:error, reasons}` with one string per violation.
  """
  @spec run(module()) :: :ok | {:error, [String.t()]}
  def run(adapter) do
    Code.ensure_loaded(adapter)

    missing =
      for {name, arity} <- @operations, not function_exported?(adapter, name, arity) do
        "#{inspect(adapter)} is missing required operation #{name}/#{arity}"
      end

    case missing do
      [] -> exercise(adapter)
      _ -> {:error, missing}
    end
  end

  defp exercise(adapter) do
    spec = Shell.spec(Windows.fetch("dps"))

    violations =
      []
      |> check("open/1", fn -> adapter.open(spec) end, &ok?/1)
      |> check("close/1", fn -> adapter.close(spec.id) end, &ok?/1)
      |> check("resize/2", fn -> adapter.resize(spec.id, %{x: 1, y: 2, w: 3, h: 4}) end, &ok?/1)
      |> check(
        "set_click_through/2",
        fn -> adapter.set_click_through(spec.id, true) end,
        &ok?/1
      )
      |> check("focus/1", fn -> adapter.focus(spec.id) end, &ok?/1)
      |> check("list/0", fn -> adapter.list() end, &is_list/1)

    if violations == [], do: :ok, else: {:error, violations}
  end

  defp ok?(:ok), do: true
  defp ok?({:ok, _}), do: true
  defp ok?({:error, _}), do: true
  defp ok?(_), do: false

  defp check(violations, operation, fun, predicate) do
    result =
      try do
        fun.()
      rescue
        error -> {:raised, error}
      end

    case result do
      {:raised, error} ->
        violations ++
          ["#{operation} raised #{inspect(error.__struct__)}: #{Exception.message(error)}"]

      value ->
        if predicate.(value) do
          violations
        else
          violations ++
            ["#{operation} returned #{inspect(value)}, which does not satisfy the contract"]
        end
    end
  end
end
