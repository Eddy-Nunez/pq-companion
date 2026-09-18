defmodule PQCompanionWeb.PageController do
  use PQCompanionWeb, :controller

  def home(conn, _params) do
    render(conn, :home)
  end
end
