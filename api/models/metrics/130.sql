WITH clear_counts AS (
    SELECT
        e.cursemark,
        COUNT(*) AS level_clears
    FROM events e
    WHERE
        e.type = 'clear_level'
        AND e.level IS NOT NULL
        AND e.cursemark IS NOT NULL
        AND e.level NOT IN (
            'CoveAwakening',
            'GreywaterBight',
            'DeadwoodGrove',
            'CurserotChapel',
            'ForsakenRoad',
            'AbyssalGate'
        )
    GROUP BY e.cursemark
),
baseline AS (
    SELECT level_clears AS cursemark_1_clears
    FROM clear_counts
    WHERE cursemark = 1
)
SELECT
    MIN(c.cursemark) AS cursemark_where_90_percent_of_runs_have_ended
FROM clear_counts c
CROSS JOIN baseline b
WHERE c.level_clears <= b.cursemark_1_clears * 0.01;