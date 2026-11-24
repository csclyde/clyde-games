WITH RECURSIVE
days AS (
	SELECT CURRENT_DATE - INTERVAL 29 DAY AS day
	UNION ALL
	SELECT day + INTERVAL 1 DAY FROM days WHERE day < CURRENT_DATE
),
daily AS (
	SELECT DATE(created_at) AS day,
		SUM(CASE WHEN type = 'player_died' AND mode = 'journey' THEN 1 ELSE 0 END) AS journey_deaths,
		SUM(CASE WHEN type = 'player_died' AND mode = 'roguelike' THEN 1 ELSE 0 END) AS roguelike_deaths,
		SUM(CASE WHEN type = 'complete_run' AND mode = 'journey' THEN 1 ELSE 0 END) AS journey_completions
	FROM events
	WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND type IN ('player_died', 'complete_run')
  AND mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND (type = 'player_died' OR (type = 'complete_run' AND mode = 'journey'))
	GROUP BY DATE(created_at)
)
SELECT DATE_FORMAT(days.day, '%Y-%m-%d') AS date,
	COALESCE(daily.journey_deaths, 0) AS journey_deaths,
	COALESCE(daily.roguelike_deaths, 0) AS roguelike_deaths,
	COALESCE(daily.journey_completions, 0) AS journey_completions
FROM days LEFT JOIN daily USING (day)
WHERE EXISTS (SELECT 1 FROM daily)
ORDER BY days.day;
