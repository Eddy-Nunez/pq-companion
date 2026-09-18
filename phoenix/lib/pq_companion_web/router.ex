defmodule PQCompanionWeb.Router do
  use PQCompanionWeb, :router

  pipeline :browser do
    plug :accepts, ["html"]
    plug :fetch_session
    plug :fetch_live_flash
    plug :put_root_layout, html: {PQCompanionWeb.Layouts, :root}
    plug :protect_from_forgery
    plug :put_secure_browser_headers
  end

  pipeline :api do
    plug :accepts, ["json"]
  end

  # Main window: opaque shell + titlebar/sidebar/content inner layout.
  scope "/", PQCompanionWeb do
    pipe_through :browser

    live_session :main, root_layout: {PQCompanionWeb.Layouts, :root} do
      live "/", WindowLive, :home
    end
  end

  # Overlay windows: transparent shell + no chrome. Mounted per window so the
  # shell knows which window it is; the registry is the single source of truth,
  # so a window cannot exist without a route (task 2.3).
  scope "/w", PQCompanionWeb do
    pipe_through :browser

    live_session :overlays, root_layout: {PQCompanionWeb.Layouts, :overlay} do
      # One route serves all 16 windows; OverlayLive validates the id against
      # PQCompanion.Windows and redirects an unknown one. The guard against
      # losing a window is the test that enumerates the registry, not a route
      # count.
      live "/:slug", OverlayLive, :overlay
    end
  end

  if Application.compile_env(:pq_companion, :dev_routes) do
    import Phoenix.LiveDashboard.Router

    scope "/dev" do
      pipe_through :browser

      live_dashboard "/dashboard", metrics: PQCompanionWeb.Telemetry
    end
  end
end
