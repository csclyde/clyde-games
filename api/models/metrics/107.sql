WITH level_attempts AS (
    SELECT
        e.level,
        e.cursemark,
        SUM(CASE WHEN e.type = 'clear_level' THEN 1 ELSE 0 END) AS clears,
        COUNT(*) AS attempts
    FROM events e
    WHERE
        e.type IN ('clear_level', 'player_died')
        AND e.level IS NOT NULL
        AND e.cursemark IS NOT NULL
        AND e.level NOT IN (
            'CoveAwakening',
            'DeadwoodGrove',
            'CurserotChapel',
            'ForsakenRoad',
            'AbyssalGate',
            'ShrineValoria',
			'Swordfork'
        )
    GROUP BY
        e.level,
        e.cursemark
),

cursemark_totals AS (
    SELECT
        cursemark,
        SUM(clears) AS total_clears,
        SUM(attempts) AS total_attempts
    FROM level_attempts
    GROUP BY cursemark
),

level_scores AS (
    SELECT
        la.level,
        la.attempts,

        (
            la.clears / NULLIF(la.attempts, 0)
            -
            (ct.total_clears - la.clears)
            / NULLIF(ct.total_attempts - la.attempts, 0)
        ) AS clear_rate_delta

    FROM level_attempts la
    JOIN cursemark_totals ct
        ON ct.cursemark = la.cursemark

    WHERE
        ct.total_attempts - la.attempts > 0
)

SELECT
    level,

    ROUND(
        -100 * SUM(clear_rate_delta * attempts)
        / NULLIF(SUM(attempts), 0),
        1
    ) AS difficulty_score

FROM level_scores

GROUP BY level

HAVING SUM(attempts) >= 50

ORDER BY difficulty_score DESC;