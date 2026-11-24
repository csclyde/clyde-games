SELECT COUNT(DISTINCT NULLIF(p_id, '')) AS reporting_installations
FROM events WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND type IN ('player_died', 'complete_run')
  AND mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND (type = 'player_died' OR (type = 'complete_run' AND mode = 'journey'));
