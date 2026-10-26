WITH player_weapons AS (
    SELECT DISTINCT
        e.p_id,
        ei.item
    FROM event_items ei
    JOIN events e
        ON ei.event_id = e.id
    WHERE
        e.p_id IS NOT NULL
        AND ei.item IS NOT NULL
        AND ei.slot = 'Spell'
        AND e.type IN ('clear_level', 'level_cleared')
),

total_players AS (
    SELECT
        COUNT(DISTINCT p_id) AS total_players
    FROM player_weapons
)

SELECT
    pw.item,
    COUNT(*) AS players,
    ROUND(
        COUNT(*) * 1.0 / tp.total_players,
        3
    ) AS player_usage_rate
FROM player_weapons pw
CROSS JOIN total_players tp
GROUP BY
    pw.item,
    tp.total_players
ORDER BY
    players DESC;