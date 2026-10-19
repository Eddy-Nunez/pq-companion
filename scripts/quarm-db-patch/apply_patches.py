#!/usr/bin/env python3
"""Replay Project Quarm content migrations onto quarm.db (SQLite).

quarm.db is built from periodic MySQL dumps, but Project Quarm also ships
content changes as SQL migration files between dumps. This script replays the
migrations that landed after the 2026-09-27 dump, in upstream order, so the
app's respawn/lockout/loot data matches the live server until the next dump.

Two upstream sources (the quests repo's sql/ is applied manually on the server
and overrides EQMacEmu's values, e.g. PoP trash 750/930s -> 930/1230s):
  - SecretsOTheP/EQMacEmu  utils/sql/git/content/  @ b61ec17 (none newer
                                                     than 2026_10_02)
  - SecretsOTheP/quests    sql/                    @ d4625cd
Then the local LOCAL_FILES run last: live changes from the official patch
notes that no upstream SQL file carries.

MySQL-only statements are rewritten where mechanical (ON DUPLICATE KEY UPDATE,
FROM DUAL, LEAST) or hand-translated in manual-2026-10-14.sql. Statements on
live server state (respawn_times, character lockouts, data_buckets, launcher
tables) are skipped: quarm.db holds no live state.

Usage:
  python3 -I apply_patches.py --db quarm.db \
      --eqmacemu-dir <EQMacEmu>/utils/sql/git/content --quests-dir <quests>/sql

Drop this once a dump taken after 2026-10-17 has been converted.
"""

import argparse
import os
import re
import sqlite3
import sys

EQMACEMU_FILES = [
    "2026_09_23_Codecay_Planes_of_Power_Expansion.sql",
    "2026_09_25_PoP_Instance_Books_and_Graveyards.sql",
    "2026_09_26_PoEarthB_Book_Destination.sql",
    "2026_09_27_Light_of_Marr_Instant_Group_Heal.sql",
    "2026_09_27_Plane_of_Torment_Key_Container_Respawn.sql",
    "2026_09_28_Protect_Old_Froglok_from_Vicious_Norrathians.sql",
    "2026_09_28_Seventh_Hammer_Respawn_Correction.sql",
    "2026_09_29_History_of_Evils_Respawn.sql",
    "2026_09_29_Hope_Stone_Drop_Rate.sql",
    "2026_09_29_Make_PoP_Spell_Turnins_Tradeable.sql",
    "2026_09_29_Make_Spell_Scrolls_Tradeable.sql",
    "2026_09_29_Plane_of_Time_Instance_Book.sql",
    "2026_09_29_Restore_PoP_Instance_Books.sql",
    "2026_09_29_Restore_Timekeeper_of_Druzzil_Ro.sql",
    "2026_09_29_SolRoTower_Lava_Portal_Size.sql",
    "2026_09_30_PoJ_Trial_Mark_Minimums.sql",
    "2026_09_30_PoP_Trash_Respawn_Timers.sql",
    "2026_09_30_Pre_PoP_Respawn_Repair.sql",
    "2026_09_30_Spawn_Timer_Seven_Day_Cap.sql",
    "2026_09_30_T_Chardok_Instance_Respawn.sql",
    "2026_10_01_Nitram_Xanamech_Repeat.sql",
    "2026_10_01_Plane_of_Sky_Quest_NPC_Movement.sql",
    "2026_10_02_PoP_Instance_Corpse_Punts.sql",
]

# Order follows the quests repo's DEPLOYMENT.txt and the files' own "apply
# after" notes rather than filename order.
QUESTS_FILES = [
    "2026_09_26_PoP_Keyring_Keys.sql",
    "2026_10_03_Codecay_Throne_Lua_Access.sql",
    "2026_10_03_PoP_Trash_Respawn_Timers.sql",
    "2026_10_03_PoP_Silk_And_Ooze_Drops.sql",
    "2026_10_03_Lower_Planes_Named_Respawns.sql",
    "2026_10_03_Bertox_Event_Trash_Respawns.sql",
    "2026_10_06_Coirnav_Respawn_Lockout_Alignment.sql",
    "2026_10_06_Lower_Plane_Respawn_Lockout_Alignment.sql",
    "2026_10_06_Final_PoP_Respawn_Lockout_Sync.sql",
    "2026_10_06_Hope_Stone_Rarity_No_Drop.sql",
    "2026_10_06_NightmareB_Instance_Book_Position.sql",
    "2026_10_06_Rathe_Avatar_Timer_Alignment.sql",
    "2026_10_06_Torment_Key_Avatar_Spawn_Chance.sql",
    "2026_10_07_Air_Earth_Event_Gate_Respawns.sql",
    "2026_10_07_Dresolik_Guardian_Reset.sql",
    "2026_10_07_Fennin_Guardian_Respawn.sql",
    "2026_10_07_Fire_Mini_66_Hour_Cycles.sql",
    "2026_10_07_Gintolaken_Prerequisite_Respawns.sql",
    "2026_10_07_HoH_A_Remove_Loot_Lockouts.sql",
    "2026_10_07_HoH_Trial_Starter_Respawns.sql",
    "2026_10_07_Nitram_Open_World_Escort.sql",
    "2026_10_07_Rizlona_First_Form_Loot_Lockout.sql",
    "2026_10_09_Elemental_Instrument_Lootdrops.sql",
    "2026_10_09_Elemental_Plane_View_Distances.sql",
    "2026_10_11_PoK_Gork_Mork_Library_Teleports.sql",
    "2026_10_15_Torment_Key_Progression.sql",
    "2026_10_15_Time_B_Graveyard_Recovery.sql",
    "2026_10_17_Torment_Putrescent_Blood.sql",
]

# Not replayed: launcher/PvP/rule/timer-cleanup files touch server state only.

LOCAL_FILES = [
    "patch-notes-2026-10-17.sql",
]

STATE_TABLES = ("respawn_times", "character_loot_lockouts",
                "character_legacy_items", "data_buckets", "launcher_zones")

MANUAL_MARK = re.compile(r"^-- @@ after: (\S+)\s*$", re.M)


def strip_comments(src):
    src = re.sub(r"/\*.*?\*/", "", src, flags=re.S)
    return "\n".join(l for l in src.splitlines()
                     if not l.strip().startswith("--"))


def statements(src):
    buf = ""
    for line in strip_comments(src).splitlines(True):
        buf += line
        if sqlite3.complete_statement(buf):
            stmt = buf.strip()
            buf = ""
            if stmt:
                yield stmt


def rewrite(stmt):
    """Mechanical MySQL -> SQLite rewrites. Returns None to skip."""
    up = stmt.upper()
    if up.startswith(("START TRANSACTION", "COMMIT", "SET @", "SELECT",
                      "DROP TEMPORARY", "ALTER TABLE")):
        return None
    if any(re.search(r"\b%s\b" % t, stmt) for t in STATE_TABLES):
        return None
    if "ON DUPLICATE KEY UPDATE" in up:
        stmt = re.sub(r"\s+ON DUPLICATE KEY UPDATE.*;\s*$", ";", stmt,
                      flags=re.S | re.I)
        stmt = re.sub(r"^INSERT INTO", "INSERT OR IGNORE INTO", stmt, flags=re.I)
    stmt = re.sub(r"\bFROM DUAL\b", "", stmt, flags=re.I)
    stmt = re.sub(r"\bLEAST\(", "MIN(", stmt)
    return stmt


def load_manual(path):
    src = open(path).read()
    parts = MANUAL_MARK.split(src)
    return {parts[i]: parts[i + 1] for i in range(1, len(parts), 2)}


def run_file(db, path, manual, failures):
    name = os.path.basename(path)
    failed = 0
    for stmt in statements(open(path).read()):
        sql = rewrite(stmt)
        if sql is None:
            continue
        try:
            db.execute(sql)
        except sqlite3.Error as e:
            failed += 1
            if name not in manual:
                failures.append(f"{name}: {e}: {stmt[:80]!r}")
    if name in manual:
        for stmt in statements(manual.pop(name)):
            db.execute(stmt)  # hand translations must not fail
    print(f"applied {name}" + (f" ({failed} via manual)" if failed else ""))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--db", required=True)
    ap.add_argument("--eqmacemu-dir", required=True)
    ap.add_argument("--quests-dir", required=True)
    ap.add_argument("--manual", default=os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "manual-2026-10-14.sql"))
    args = ap.parse_args()

    manual = load_manual(args.manual)
    db = sqlite3.connect(args.db)
    failures = []
    for f in EQMACEMU_FILES:
        run_file(db, os.path.join(args.eqmacemu_dir, f), manual, failures)
    for f in QUESTS_FILES:
        run_file(db, os.path.join(args.quests_dir, f), manual, failures)
    here = os.path.dirname(os.path.abspath(__file__))
    for f in LOCAL_FILES:
        run_file(db, os.path.join(here, f), manual, failures)
    if manual:
        failures.append(f"unused manual blocks: {sorted(manual)}")
    if failures:
        db.rollback()
        print("\n".join(["FAILED:"] + failures), file=sys.stderr)
        sys.exit(1)
    db.commit()
    db.execute("ANALYZE")
    db.commit()
    db.close()


if __name__ == "__main__":
    main()
