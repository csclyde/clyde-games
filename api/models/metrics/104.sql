SELECT
    e.cursemark,

    SUM(CASE WHEN e.type = 'clear_level' THEN 1 ELSE 0 END) AS level_clears,
    SUM(CASE WHEN e.type = 'player_died' THEN 1 ELSE 0 END) AS player_deaths,

    COUNT(*) AS total_clear_or_death_events

FROM events e
WHERE
    e.type IN ('clear_level', 'player_died')
    AND e.level IS NOT NULL
    AND e.cursemark IS NOT NULL
    AND e.level NOT IN (
        'CoveAwakening',
        'DeadwoodGrove',
        'CurserotChapel',
		'GreywaterBight',
        'ForsakenRoad',
        'AbyssalGate'
    )
GROUP BY
    e.cursemark
ORDER BY
    e.cursemark;