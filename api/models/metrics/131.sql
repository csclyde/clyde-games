SELECT
    COUNT(*) AS total_runs,
    ROUND(AVG(e.cursemark), 2) AS average_run_depth,
    MIN(e.cursemark) AS minimum_run_depth,
    MAX(e.cursemark) AS maximum_run_depth
FROM events e
WHERE
    e.type = 'player_died'
    AND e.p_id IS NOT NULL
    AND e.cursemark IS NOT NULL;