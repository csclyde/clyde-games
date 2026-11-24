SELECT run_step, mode, COUNT(*) AS deaths
FROM events WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND type IN ('player_died', 'complete_run')
  AND mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND type = 'player_died' AND run_step >= 0
GROUP BY run_step, mode ORDER BY run_step, mode;
