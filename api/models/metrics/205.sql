SELECT NULLIF(build, '') AS build, NULLIF(`commit`, '') AS build_commit, mode, difficulty,
	SUM(CASE WHEN type = 'player_died' THEN 1 ELSE 0 END) AS deaths,
	CASE WHEN mode = 'journey' THEN SUM(CASE WHEN type = 'complete_run' AND mode = 'journey' THEN 1 ELSE 0 END) END AS completions,
	COUNT(DISTINCT NULLIF(p_id, '')) AS reporting_installations
FROM events WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND type IN ('player_died', 'complete_run')
  AND mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND (type = 'player_died' OR (type = 'complete_run' AND mode = 'journey'))
GROUP BY build, `commit`, mode, difficulty
ORDER BY build DESC, mode, difficulty, build_commit;
