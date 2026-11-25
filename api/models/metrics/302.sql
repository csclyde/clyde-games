SELECT SUM(gold) AS actual_earnings
FROM events
WHERE deleted_at IS NULL AND LOWER(env) IN ('prod', 'production') AND type = 'season_completed'
  AND created_at >= CURDATE() - INTERVAL 29 DAY AND created_at < CURDATE() + INTERVAL 1 DAY;
