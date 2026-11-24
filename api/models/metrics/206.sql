SELECT mode, COALESCE(NULLIF(area, ''), 'Unknown') AS area,
	COALESCE(NULLIF(level, ''), 'Unknown') AS level, difficulty,
	COUNT(*) AS deaths, COUNT(DISTINCT NULLIF(p_id, '')) AS reporting_installations
FROM events WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND type IN ('player_died', 'complete_run')
  AND mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND type = 'player_died'
GROUP BY mode, COALESCE(NULLIF(area, ''), 'Unknown'), COALESCE(NULLIF(level, ''), 'Unknown'), difficulty
ORDER BY deaths DESC, mode, area, level, difficulty;
