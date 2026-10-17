-- Live-server changes announced in the October 5-17, 2026 patch notes that
-- no upstream SQL file carries. The quests repo's 2026_10_06 alignment files
-- still give these four 66-hour loot lockouts; the patch notes (credited
-- [kyinen] [Edalyn]) say they respawn on 66 hours with no loot lockout, so a
-- later server-side change removed it. Runs after every upstream file.

-- Halls of Honor B: Ralthazor, Edium and Halon of Marr.
-- Plane of Torment: Sorrowsong.
UPDATE npc_types SET loot_lockout = 0
WHERE id IN (220012, 220021, 220022, 207052);
