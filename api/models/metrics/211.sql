WITH snapshots AS (
    SELECT mode, lifetime_clears, lifetime_deaths, clear_history_complete, death_history_complete,
           ROW_NUMBER() OVER (PARTITION BY p_id, mode ORDER BY created_at DESC, id DESC) AS recency
    FROM events
    WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
      AND created_at >= CURDATE() - INTERVAL 29 DAY AND created_at < CURDATE() + INTERVAL 1 DAY
      AND type IN ('player_died', 'complete_run') AND p_id <> ''
), counters AS (
    SELECT mode, 'Clears' AS counter, lifetime_clears AS total, clear_history_complete AS complete
    FROM snapshots WHERE recency = 1 AND mode = 'journey'
    UNION ALL
    SELECT mode, 'Deaths', lifetime_deaths, death_history_complete
    FROM snapshots WHERE recency = 1 AND mode IN ('journey', 'roguelike')
), reported AS (
    SELECT mode, counter, total,
           CASE WHEN total IS NULL THEN 'Unavailable'
                WHEN complete = 1 THEN 'Full history'
                ELSE 'Partial history' END AS coverage
    FROM counters
)
SELECT mode, counter, coverage, COUNT(*) AS installations, AVG(total) AS average, MAX(total) AS highest
FROM reported
GROUP BY mode, counter, coverage
ORDER BY mode, counter, coverage;
