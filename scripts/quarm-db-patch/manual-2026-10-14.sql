-- Hand translations of MySQL-only statements (UPDATE ... JOIN, LEAST, @vars)
-- in the upstream migrations that apply_patches.py replays. Each block runs
-- immediately after the upstream file it names. Server-state tables
-- (respawn_times, character_loot_lockouts, ...) are skipped: quarm.db has no
-- live state.

-- @@ after: 2026_09_27_Plane_of_Torment_Key_Container_Respawn.sql
UPDATE spawn2 SET respawntime = 1800
WHERE id = 346828 AND zone = 'potorment' AND respawntime = 216000
  AND spawngroupID IN (SELECT id FROM spawngroup
                       WHERE name = 'potorment_An_Unimaginable_Horror75');

-- @@ after: 2026_09_30_Spawn_Timer_Seven_Day_Cap.sql
UPDATE spawn2
SET respawntime = CASE
        WHEN id = 345473 AND respawntime > 604800 THEN 237600
        ELSE MIN(respawntime, 604800)
    END,
    boot_respawntime = MIN(boot_respawntime, 604800)
WHERE respawntime > 604800 OR boot_respawntime > 604800;

-- @@ after: 2026_10_06_Lower_Plane_Respawn_Lockout_Alignment.sql
CREATE TEMP TABLE la (spawn_id INT, zone_name TEXT, npc_id INT PRIMARY KEY,
                      cyc INT, loot INT);
INSERT INTO la VALUES
    (344762,'podisease',205091,86400,0),(345473,'pojustice',201492,86400,0),
    (360642,'codecay',200007,86400,0),(360642,'codecay',200245,86400,0),
    (346764,'potorment',207015,86400,0),(369184,'ponightmare',204010,86400,0),
    (345905,'ponightmare',204034,86400,0),(345919,'ponightmare',204035,86400,0),
    (346876,'potorment',207002,86400,0),(346763,'potorment',207003,86400,0),
    (346875,'potorment',207004,86400,0),(346877,'potorment',207027,86400,0),
    (346955,'potorment',207028,86400,0),(364399,'povalor',208157,86400,0),
    (364399,'povalor',208202,86400,0),(364399,'povalor',208208,86400,0),
    (367163,'hohonorb',220012,237600,237600),
    (367227,'hohonorb',220021,237600,237600),
    (366604,'hohonorb',220022,237600,237600),
    (360643,'codecay',200226,237600,237600);
UPDATE npc_types
SET loot_lockout = (SELECT loot FROM la WHERE npc_id = npc_types.id),
    instance_spawn_timer_override = CASE
        WHEN id = 201492 OR instance_spawn_timer_override <> 0
            THEN (SELECT cyc FROM la WHERE npc_id = npc_types.id) * 1000
        ELSE 0
    END
WHERE id IN (SELECT npc_id FROM la);
UPDATE spawn2
SET respawntime = (SELECT cyc FROM la
                   WHERE la.spawn_id = spawn2.id AND la.zone_name = spawn2.zone
                   LIMIT 1)
WHERE EXISTS (SELECT 1 FROM la
              WHERE la.spawn_id = spawn2.id AND la.zone_name = spawn2.zone);
DROP TABLE la;

-- @@ after: 2026_10_06_Final_PoP_Respawn_Lockout_Sync.sql
CREATE TEMP TABLE fs (npc_id INT PRIMARY KEY, cyc INT, loot INT, ovr INT);
INSERT INTO fs VALUES
    (205134,86400,0,0),(204047,86400,0,0),(204028,86400,0,0),
    (207052,237600,237600,0),(214053,86400,0,0),(214054,86400,0,0),
    (214092,86400,0,0),(212014,237600,237600,237600000),
    (212023,237600,237600,237600000),(212026,237600,0,237600000),
    (212046,237600,0,237600000),(212055,237600,237600,237600000),
    (215060,NULL,0,NULL),(215005,NULL,0,NULL),(215051,NULL,0,NULL),
    (215053,NULL,0,NULL),(216040,64800,0,64800000),(216041,64800,0,64800000),
    (216043,64800,0,64800000),(217003,237600,237600,237600000),
    (217005,237600,237600,237600000),(217019,237600,237600,237600000),
    (217032,237600,237600,237600000),(217036,237600,237600,237600000),
    (217049,237600,237600,237600000),(217051,237600,237600,237600000),
    (217056,237600,237600,237600000),(217059,237600,237600,237600000),
    (217063,237600,237600,237600000),(204010,86400,0,0),(200020,86400,0,0),
    (206053,86400,0,0),(206054,86400,0,0),(206055,86400,0,0),
    (209016,10800,0,0),(209059,10800,0,0),(209060,10800,0,0),
    (209061,10800,0,0),(209070,10800,0,0),(209071,10800,0,0),
    (209072,10800,0,0),(209082,10800,0,0);
UPDATE spawn2
SET respawntime = (SELECT fs.cyc FROM fs
                   JOIN spawnentry se ON se.npcID = fs.npc_id
                   WHERE se.spawngroupID = spawn2.spawngroupID
                     AND fs.cyc IS NOT NULL
                   LIMIT 1)
WHERE spawngroupID IN (SELECT se.spawngroupID FROM fs
                       JOIN spawnentry se ON se.npcID = fs.npc_id
                       WHERE fs.cyc IS NOT NULL);
UPDATE npc_types
SET loot_lockout = (SELECT loot FROM fs WHERE npc_id = npc_types.id),
    instance_spawn_timer_override = COALESCE(
        (SELECT ovr FROM fs WHERE npc_id = npc_types.id),
        instance_spawn_timer_override)
WHERE id IN (SELECT npc_id FROM fs);
DROP TABLE fs;

-- @@ after: 2026_10_06_NightmareB_Instance_Book_Position.sql
UPDATE spawn2 SET x = 1605, y = 123, z = 220, heading = 6
WHERE id = 21450298 AND zone = 'ponightmare'
  AND spawngroupID IN (SELECT spawngroupID FROM spawnentry
                       WHERE npcID = 7151013);

-- @@ after: 2026_10_06_Torment_Key_Avatar_Spawn_Chance.sql
UPDATE spawnentry
SET chance = (SELECT w.new_chance FROM torment_key_spawn_weights w
              WHERE w.group_id = spawnentry.spawngroupID
                AND w.npc_id = spawnentry.npcID)
WHERE EXISTS (SELECT 1 FROM torment_key_spawn_weights w
              WHERE w.group_id = spawnentry.spawngroupID
                AND w.npc_id = spawnentry.npcID
                AND spawnentry.chance IN (w.old_chance, w.new_chance));
DROP TABLE torment_key_spawn_weights;

-- @@ after: 2026_10_07_Air_Earth_Event_Gate_Respawns.sql
UPDATE spawn2 SET respawntime = 237600
WHERE id IN (369495,369498,369497,369496,369482,369483,369484,369485);
UPDATE npc_types SET instance_spawn_timer_override = 237600000
WHERE id IN (215414,215415,215416,215417,218391,218390,218392,218412);
DROP TABLE pop_elemental_gate_sync;

-- @@ after: 2026_10_07_Fennin_Guardian_Respawn.sql
UPDATE spawn2 SET respawntime = 496800
WHERE id = 367088 AND zone = 'pofire'
  AND spawngroupID IN (SELECT spawngroupID FROM spawnentry
                       WHERE npcID = 217050);

-- @@ after: 2026_10_07_Fire_Mini_66_Hour_Cycles.sql
UPDATE spawn2 SET respawntime = 237600
WHERE id IN (SELECT spawn_id FROM fire_mini_restore)
  AND (SELECT COUNT(*) FROM fire_mini_restore t
       JOIN spawn2 s ON s.id = t.spawn_id AND s.zone = 'pofire'
                    AND s.spawngroupID = t.spawn_group_id
       JOIN spawnentry se ON se.spawngroupID = t.spawn_group_id
                         AND se.npcID = t.npc_id) = 10;
UPDATE npc_types
SET loot_lockout = 237600, instance_spawn_timer_override = 237600000
WHERE id IN (SELECT npc_id FROM fire_mini_restore);
DROP TABLE fire_mini_restore;

-- @@ after: 2026_10_07_Gintolaken_Prerequisite_Respawns.sql
UPDATE spawn2
SET respawntime = (SELECT t.respawn_seconds FROM gintolaken_prerequisite_sync t
                   WHERE t.spawn_id = spawn2.id)
WHERE zone = 'poearthb'
  AND id IN (SELECT spawn_id FROM gintolaken_prerequisite_sync);
UPDATE npc_types SET instance_spawn_timer_override = 0
WHERE id IN (222008,222009,222010);
DROP TABLE gintolaken_prerequisite_sync;

-- @@ after: 2026_10_07_HoH_Trial_Starter_Respawns.sql
UPDATE spawn2 SET respawntime = 64800
WHERE zone = 'hohonora' AND id IN (360859,360971,361021);
UPDATE npc_types SET instance_spawn_timer_override = 64800000
WHERE id IN (211051,211050,211060)
  AND instance_spawn_timer_override = 259200000;
DROP TABLE hoh_trial_starter_sync;

-- @@ after: 2026_10_07_Nitram_Open_World_Escort.sql
UPDATE spawn2 SET raid_target_spawnpoint = 0, enabled = 1
WHERE id = 345275 AND zone = 'poinnovation' AND spawngroupID = 206033;
