WITH session_stats AS (
SELECT
    s_id,
    MIN(created_at) AS first_clear,
    MAX(created_at) AS last_clear,
    COUNT(*) AS clear_count,
	TIMESTAMPDIFF(SECOND, MIN(created_at), MAX(created_at)) AS total_seconds
  FROM events
  WHERE type = 'clear_level'
  GROUP BY s_id
  HAVING
    clear_count > 1
    AND total_seconds <= 6 * 60 * 60
)
SELECT
  ROUND(AVG(TIMESTAMPDIFF(SECOND, first_clear, last_clear) / (clear_count - 1)) / 60, 2)
    AS avg_minutes_per_clear
FROM session_stats;
