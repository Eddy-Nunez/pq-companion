defmodule PQCompanion.WindowsTest do
  use ExUnit.Case, async: true

  alias PQCompanion.Windows

  test "the registry holds the main window plus exactly 16 overlays" do
    assert length(Windows.overlays()) == 16
    assert length(Windows.all()) == 17
    assert Windows.main().kind == :main
  end

  # The reference's 16 OverlayPage routes in frontend/src/App.tsx. Enumerated so
  # a window cannot be silently dropped during the port.
  @reference_ids ~w(
    dps hps buffTimer detrimTimer customTimer trigger npc discordVoice
    threat rollTracker respawnTimer zoneLockouts raidReadiness liveMap
    chChain chMetronome
  )

  test "the overlay ids match the reference's overlayKeys exactly" do
    assert Enum.map(Windows.overlays(), & &1.id) == @reference_ids
  end

  test "every window has a unique id and a unique route" do
    ids = Enum.map(Windows.all(), & &1.id)
    routes = Enum.map(Windows.all(), & &1.route)
    assert ids == Enum.uniq(ids)
    assert routes == Enum.uniq(routes)
  end

  test "overlays are transparent, frameless and always-on-top; the main window is not" do
    for w <- Windows.overlays() do
      assert w.transparent and w.always_on_top and w.frameless
      refute w.resizable
      assert Windows.overlay?(w)
    end

    main = Windows.main()
    refute main.transparent
    refute main.frameless
    assert main.resizable
    refute Windows.overlay?(main)
  end

  test "the shell spec carries every property a shell needs" do
    alias PQCompanion.Shell

    for w <- Windows.all() do
      spec = Shell.spec(w)
      assert spec.id == w.id
      assert spec.route == w.route
      assert spec.title == w.label
      assert is_binary(spec.token) and spec.token != ""

      for key <-
            ~w(transparent always_on_top click_through frameless resizable display_only zoom)a do
        assert Map.has_key?(spec, key), "spec for #{w.id} is missing #{key}"
      end

      assert is_boolean(spec.transparent)
      assert is_float(spec.zoom)
      assert {:ok, :verified} = Shell.verify_token(spec.token, w.id)

      json = Jason.decode!(Shell.to_json(spec))
      assert json["id"] == w.id
      assert json["token"] == spec.token
      assert json["kind"] == to_string(w.kind)
    end
  end

  test "id and slug are distinct and both resolve (the bug that broke /w/buff-timer)" do
    # Routes are kebab-case, ids are the reference's camelCase overlayKeys.
    # Mounting by the URL segment must resolve the camelCase id.
    buff = Windows.fetch_by_slug("buff-timer")
    assert buff.id == "buffTimer"
    assert Windows.fetch("buffTimer") == buff
    refute Windows.fetch("buff-timer"), "a slug must not resolve via fetch/1"
    assert Windows.fetch_by_slug("nope") == nil

    for w <- Windows.overlays() do
      assert w.route == "/w/" <> w.slug
      assert Windows.fetch_by_slug(w.slug) == w
      assert Windows.fetch_by_slug(w.slug).id == w.id
    end
  end

  test "fetch/1 and fetch_by_route/1 agree with the registry" do
    assert Windows.fetch("dps").route == "/w/dps"
    assert Windows.fetch_by_route("/w/dps").id == "dps"
    assert Windows.fetch("nope") == nil
    assert Windows.fetch_by_route("/nope") == nil
  end
end
