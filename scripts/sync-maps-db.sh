#!/usr/bin/env bash
# sync-maps-db.sh — make the data-latest release's maps.db current before a
# release. Run by /newrelease (step 2b).
#
#   scripts/sync-maps-db.sh            # check only; exit 1 if out of date
#   scripts/sync-maps-db.sh --upload   # refresh POIs and upload if they differ
#
# Why: CI builds the installer with whatever maps.db is on data-latest. That
# file used to be checked only for presence, so a stale copy — POIs generated
# against an older quarm.db, or before a mapgen fix — shipped silently. In
# Oct 2026 the published copy was from Aug 4: no PoP raid targets, no Plane of
# Earth traps, while quarm.db had moved on twice.
#
# Steps:
#   1. Local quarm.db must be the published one, or the POIs would be built
#      from data the release does not ship.
#   2. Regenerate the POIs in the local maps.db (mapgen -pois-only; geometry
#      is untouched and needs no EQ client).
#   3. Compare content, not file bytes, with the published maps.db: refreshing
#      the same data twice gives different bytes (row ids, free pages), so a
#      file hash would report stale forever.
set -euo pipefail

upload=false
[[ "${1:-}" == "--upload" ]] && upload=true

root="$(cd "$(dirname "$0")/.." && pwd)"
data="$root/backend/data"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

[[ -f "$data/maps.db" ]] || {
  echo "ERROR: backend/data/maps.db missing. Geometry needs a full build on a"
  echo "machine with the EQ client: cd backend && go run ./cmd/mapgen -client <EQ dir>"
  exit 1
}

# 1. quarm.db must match what data-latest publishes.
remote_quarm="$(gh release view data-latest --json assets \
  --jq '.assets[] | select(.name=="quarm.db") | .digest' | sed 's/^sha256://')"
local_quarm="$(shasum -a 256 "$data/quarm.db" | cut -d' ' -f1)"
if [[ "$remote_quarm" != "$local_quarm" ]]; then
  echo "ERROR: local quarm.db differs from data-latest's."
  echo "  local  $local_quarm"
  echo "  remote $remote_quarm"
  echo "Fetch the published one first:"
  echo "  gh release download data-latest --pattern quarm.db --dir backend/data --clobber"
  exit 1
fi

# 2. Refresh POIs from that quarm.db.
(cd "$root/backend" && go run ./cmd/mapgen -pois-only >/dev/null)

# 3. Compare content with the published maps.db.
content() {
  sqlite3 "$1" "
    SELECT zone,x,y,z,category,label,source,ref_id FROM map_poi
      ORDER BY zone,category,label,x,y,z;
    SELECT zone,layer,nlines,hex(lines) FROM map_layer ORDER BY zone,layer;
    SELECT * FROM map_zone ORDER BY zone;" | shasum -a 256 | cut -d' ' -f1
}
if gh release download data-latest --pattern maps.db --dir "$tmp" >/dev/null 2>&1; then
  remote_maps="$(content "$tmp/maps.db")"
  remote_pois="$(sqlite3 "$tmp/maps.db" 'SELECT count(*) FROM map_poi')"
else
  remote_maps="missing"
  remote_pois=0
fi
local_maps="$(content "$data/maps.db")"
local_pois="$(sqlite3 "$data/maps.db" 'SELECT count(*) FROM map_poi')"

if [[ "$remote_maps" == "$local_maps" ]]; then
  echo "maps.db on data-latest is current ($local_pois POIs)."
  exit 0
fi

echo "maps.db on data-latest is OUT OF DATE (published $remote_pois POIs, current $local_pois)."
if ! $upload; then
  echo "Run: scripts/sync-maps-db.sh --upload"
  exit 1
fi

gh release upload data-latest "$data/maps.db" --clobber
gh release download data-latest --pattern maps.db --dir "$tmp/verify" >/dev/null
if [[ "$(content "$tmp/verify/maps.db")" != "$local_maps" ]]; then
  echo "ERROR: uploaded maps.db does not match local after upload."
  exit 1
fi
echo "Uploaded and verified maps.db on data-latest ($local_pois POIs)."
