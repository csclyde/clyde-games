WITH global_stats AS (
    SELECT
        AVG(e.cursemark) AS global_avg_cursemark
    FROM event_items ei
    JOIN events e
        ON ei.event_id = e.id
    WHERE
        ei.slot IS NOT NULL
)

SELECT
    ei.slot,
    ei.item,
    COUNT(*) AS uses,
    SUM(e.cursemark) AS total_score,
    ROUND(AVG(e.cursemark), 2) AS avg_cursemark,
    ROUND(MAX(gs.global_avg_cursemark), 2) AS global_avg_cursemark,
    ROUND(AVG(e.cursemark) - MAX(gs.global_avg_cursemark), 2) AS above_global_avg,
    MAX(e.cursemark) AS max_cursemark
FROM event_items ei
JOIN events e
    ON ei.event_id = e.id
CROSS JOIN global_stats gs
WHERE
    ei.slot IS NOT NULL
GROUP BY
    ei.slot,
    ei.item
HAVING
    COUNT(*) >= 50
ORDER BY
    avg_cursemark DESC;