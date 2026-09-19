defmodule PQCompanion.ConfigTest do
  @moduledoc """
  Wave 1 task 5.1 — the settings reader/writer scoped to the sidebar keys.

  The load-bearing property here is **additivity**: sidebar preferences are added
  to the reference's file without renaming, dropping or re-typing anything the
  reference app reads, so the file stays valid for rollback.
  """

  use ExUnit.Case, async: true

  alias PQCompanion.Config

  # A fixture shaped like a real reference file: a Windows path, a nested
  # `preferences` map with keys we do not own, a map-of-floats, lists, and
  # booleans. `omitempty` means the reference omits empty sidebar lists.
  @reference_yaml """
  eq_path: 'C:\\EverQuest'
  character: Fippy
  character_class: 5
  server_addr: 127.0.0.1:9000
  onboarding_completed: true
  chat_retention_days: 30
  preferences:
    map_style: detailed
    map_show_group: true
    zoom_factor: 1.25
    overlay_zoom_factors:
      dps: 1.4
      threat: 0.9
    sidebar_hidden:
      - /loot
    sidebar_order:
      - /items
      - /spells
    sidebar_favorites:
      - /items
    pop_flags_enabled: false
    faction_tracker_enabled: false
    raids_enabled: true
  backup:
    enabled: true
    keep: 5
  combat:
    min_fight_seconds: 5
  """

  defp tmp_dir! do
    dir = Path.join(System.tmp_dir!(), "pq_config_test_#{System.unique_integer([:positive])}")
    File.mkdir_p!(dir)
    dir
  end

  defp write_fixture!(dir, body \\ @reference_yaml) do
    path = Path.join(dir, "config.yaml")
    File.write!(path, body)
    path
  end

  describe "load/1" do
    test "a missing file is an empty config, not an error" do
      assert Config.load(Path.join(tmp_dir!(), "config.yaml")) == {:ok, %{}}
    end

    test "an empty or null document is an empty config" do
      dir = tmp_dir!()
      assert Config.load(write_fixture!(dir, "")) == {:ok, %{}}
      assert Config.load(write_fixture!(dir, "---\n")) == {:ok, %{}}
    end

    test "invalid YAML is an error, not a crash" do
      dir = tmp_dir!()
      assert {:error, {:invalid_yaml, _}} = Config.load(write_fixture!(dir, "a: [1, 2\n"))
    end

    test "accessors default to empty when keys are absent" do
      empty = %{}
      assert Config.preferences(empty) == %{}
      assert Config.sidebar_hidden(empty) == []
      assert Config.sidebar_order(empty) == []
      assert Config.sidebar_favorites(empty) == []
      assert Config.sidebar_collapsed(empty) == %{}
      assert Config.flags(empty) == %{}
    end
  end

  describe "task 5.1 — sidebar fields are additive" do
    test "reading the reference fixture yields the existing sidebar preferences" do
      config = Config.load!(write_fixture!(tmp_dir!()))
      assert Config.sidebar_hidden(config) == ["/loot"]
      assert Config.sidebar_order(config) == ["/items", "/spells"]
      assert Config.sidebar_favorites(config) == ["/items"]
      assert Config.sidebar_collapsed(config) == %{}
      assert Config.flags(config)["raids_enabled"] == true
    end

    test "saving sidebar preferences preserves every pre-existing key and value" do
      dir = tmp_dir!()
      path = write_fixture!(dir)
      before = Config.load!(path)

      config =
        before
        |> Config.put_sidebar_hidden(["/loot", "/chat"])
        |> Config.put_sidebar_collapsed(%{"database" => true, "characters" => false})

      assert :ok = Config.save(config, path)
      after_save = Config.load!(path)

      # Every key present before the write is still present with the same value,
      # except the two sidebar keys we deliberately changed.
      expected =
        before
        |> put_in(["preferences", "sidebar_hidden"], ["/loot", "/chat"])
        |> put_in(["preferences", "sidebar_collapsed_sections"], %{
          "database" => true,
          "characters" => false
        })

      assert after_save == expected
    end

    test "no existing preference is renamed, dropped or re-typed" do
      dir = tmp_dir!()
      path = write_fixture!(dir)
      before = Config.load!(path) |> Config.put_sidebar_favorites(["/spells"])
      assert :ok = Config.save(before, path)

      prefs = Config.load!(path) |> Config.preferences()

      assert prefs["map_style"] == "detailed"
      assert prefs["map_show_group"] == true
      assert prefs["zoom_factor"] == 1.25
      assert prefs["overlay_zoom_factors"] == %{"dps" => 1.4, "threat" => 0.9}
      assert prefs["pop_flags_enabled"] == false
      assert prefs["raids_enabled"] == true
      assert prefs["sidebar_favorites"] == ["/spells"]
      # and the top-level keys we do not own survive too
      assert Config.load!(path)["eq_path"] == "C:\\EverQuest"
      assert Config.load!(path)["backup"] == %{"enabled" => true, "keep" => 5}
    end

    test "an unknown key written by a future version survives a round-trip" do
      dir = tmp_dir!()
      path = Path.join(dir, "config.yaml")
      File.write!(path, "preferences:\n  something_new: keep-me\n")

      config = path |> Config.load!() |> Config.put_sidebar_hidden(["/loot"])
      assert :ok = Config.save(config, path)
      assert Config.preferences(Config.load!(path))["something_new"] == "keep-me"
    end
  end

  describe "save/2 atomicity" do
    test "leaves no temporary file beside the settings file" do
      dir = tmp_dir!()
      path = write_fixture!(dir)
      assert :ok = Config.save(Config.load!(path), path)

      leftovers = dir |> File.ls!() |> Enum.filter(&String.ends_with?(&1, ".tmp"))
      assert leftovers == []
    end

    test "creates the directory when it does not exist" do
      path = Path.join([tmp_dir!(), "nested", "deeper", "config.yaml"])
      assert :ok = Config.save(%{"preferences" => %{}}, path)
      assert File.exists?(path)
    end

    test "a failed save leaves the previous file intact" do
      dir = tmp_dir!()
      path = write_fixture!(dir)
      original = File.read!(path)

      # Make the temp write impossible: point the save at a directory path that is
      # actually a regular file, so mkdir_p fails before anything is replaced.
      blocker = Path.join(dir, "blocker")
      File.write!(blocker, "not a directory")

      assert {:error, _} = Config.save(%{"preferences" => %{}}, Path.join(blocker, "config.yaml"))
      assert File.read!(path) == original
    end
  end

  describe "the written file is normal YAML the reference can parse" do
    test "the sidebar keys land under preferences" do
      dir = tmp_dir!()
      path = Path.join(dir, "config.yaml")
      File.write!(path, "eq_path: /eq\npreferences:\n  map_style: outline\n")

      config = path |> Config.load!() |> Config.put_sidebar_hidden(["/loot"])
      assert :ok = Config.save(config, path)

      assert {:ok, reparsed} = YamlElixir.read_from_file(path)
      assert reparsed["eq_path"] == "/eq"
      assert get_in(reparsed, ["preferences", "map_style"]) == "outline"
      assert get_in(reparsed, ["preferences", "sidebar_hidden"]) == ["/loot"]
    end
  end
end
