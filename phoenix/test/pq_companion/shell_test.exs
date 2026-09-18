defmodule PQCompanion.ShellTest do
  @moduledoc """
  Wave 0 tasks 3.1, 3.3 and 3.4 — the shell behaviour, the browser adapter, and
  the conformance suite.

  `async: false` because the browser adapter records state and one test swaps the
  configured adapter.
  """

  use ExUnit.Case, async: false

  alias PQCompanion.Shell
  alias PQCompanion.Shell.{Browser, Conformance, IncompleteAdapter}
  alias PQCompanion.{WindowState, Windows}

  setup do
    Browser.reset()
    WindowState.reset()
    :ok
  end

  describe "task 3.1 — the behaviour" do
    test "declares the six required operations" do
      callbacks = Shell.behaviour_info(:callbacks)

      for operation <- [open: 1, close: 1, resize: 2, set_click_through: 2, focus: 1, list: 0] do
        assert operation in callbacks, "the behaviour does not declare #{inspect(operation)}"
      end
    end

    test "feature code never names a concrete adapter" do
      offenders =
        "lib/pq_companion_web/**/*.ex"
        |> Path.wildcard()
        |> Enum.filter(&(File.read!(&1) =~ "Shell.Browser"))

      assert offenders == [], "these modules name a concrete shell adapter: #{inspect(offenders)}"
    end

    test "the adapter is selected by configuration, with no feature-code change" do
      original = Application.get_env(:pq_companion, :shell_adapter)
      on_exit(fn -> Application.put_env(:pq_companion, :shell_adapter, original) end)

      assert Shell.adapter() == Browser

      Application.put_env(:pq_companion, :shell_adapter, IncompleteAdapter)
      assert Shell.adapter() == IncompleteAdapter

      # Dispatch goes to the configured adapter, not a hardcoded one.
      assert {:ok, _} = Shell.open(Shell.spec(Windows.fetch("dps")))
    end
  end

  describe "task 3.3 — the browser adapter" do
    test "records the specs it received" do
      spec = Shell.spec(Windows.fetch("dps"))
      assert {:ok, %{id: "dps", route: "/w/dps"}} = Browser.open(spec)
      assert [recorded] = Browser.list()
      assert recorded.id == "dps"
      assert Browser.recorded("dps").spec.id == "dps"
    end

    test "reports transparency and the other native-only properties as unmet" do
      spec = Shell.spec(Windows.fetch("npc"))
      assert spec.transparent == true

      assert {:ok, %{unmet: unmet}} = Browser.open(spec)
      assert :transparent in unmet
      assert :always_on_top in unmet
      assert :frameless in unmet
    end

    test "reports click-through as unmet rather than claiming success" do
      Browser.open(Shell.spec(Windows.fetch("npc")))
      assert {:ok, %{unmet: [:click_through]}} = Browser.set_click_through("npc", true)
      assert :click_through in Browser.unmet("npc")
    end

    test "a main-window spec requests no native-only property, so nothing is unmet" do
      assert {:ok, %{unmet: []}} = Browser.open(Shell.spec(Windows.main()))
    end

    test "focus marks the window and close removes it" do
      Browser.open(Shell.spec(Windows.fetch("dps")))
      assert :ok = Browser.focus("dps")
      assert Browser.recorded("dps").focused
      assert :ok = Browser.close("dps")
      assert Browser.list() == []
    end
  end

  describe "task 3.4 — the conformance suite" do
    test "passes against the browser adapter" do
      assert :ok = Conformance.run(Browser)
    end

    test "fails against an incomplete stub and names the missing operation" do
      assert {:error, errors} = Conformance.run(IncompleteAdapter)
      assert Enum.any?(errors, &String.contains?(&1, "close/1"))
      assert Enum.any?(errors, &String.contains?(&1, "focus/1"))
      assert Enum.any?(errors, &String.contains?(&1, "IncompleteAdapter"))
    end
  end
end
