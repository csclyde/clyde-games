WITH outcomes AS (
	SELECT id, type FROM events WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND type IN ('player_died', 'complete_run')
  AND mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND (type = 'player_died' OR (type = 'complete_run' AND mode = 'journey')) AND mode = 'journey'
	AND EXISTS (SELECT 1 FROM event_items i WHERE i.event_id = events.id AND i.item <> '')
),
samples AS (
	SELECT
		SUM(CASE WHEN type = 'player_died' THEN 1 ELSE 0 END) AS death_loadouts,
		SUM(CASE WHEN type = 'complete_run' THEN 1 ELSE 0 END) AS clear_loadouts
	FROM outcomes
),
items AS (
	SELECT DISTINCT o.id, o.type, ei.slot, ei.item,
		CASE WHEN COALESCE(ei.meld_path, '') <> '' OR COALESCE(ei.parent, '') <> '' THEN 'Meld' ELSE 'Equipped' END AS placement
	FROM outcomes o JOIN event_items ei ON ei.event_id = o.id
	WHERE ei.item <> ''
)
SELECT i.slot, i.item, i.placement,
	SUM(CASE WHEN i.type = 'player_died' THEN 1 ELSE 0 END) AS death_loadouts_with_item,
	ROUND(100.0 * SUM(CASE WHEN i.type = 'player_died' THEN 1 ELSE 0 END) / NULLIF(MAX(s.death_loadouts), 0), 1) AS death_share_pct,
	MAX(s.death_loadouts) AS total_death_loadouts,
	SUM(CASE WHEN i.type = 'complete_run' THEN 1 ELSE 0 END) AS clear_loadouts_with_item,
	ROUND(100.0 * SUM(CASE WHEN i.type = 'complete_run' THEN 1 ELSE 0 END) / NULLIF(MAX(s.clear_loadouts), 0), 1) AS clear_share_pct,
	MAX(s.clear_loadouts) AS total_clear_loadouts
FROM items i CROSS JOIN samples s
GROUP BY i.slot, i.item, i.placement
ORDER BY clear_loadouts_with_item DESC, death_loadouts_with_item DESC, i.item, i.slot, i.placement;
