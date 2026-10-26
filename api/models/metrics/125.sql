WITH item_by_cursemark AS (
    SELECT
        e.cursemark,
        ei.item,
        COUNT(DISTINCT e.id) AS uses
    FROM event_items ei
    JOIN events e
        ON ei.event_id = e.id
    WHERE
        ei.slot = 'Weapon'
        AND ei.item IS NOT NULL
        AND e.cursemark IS NOT NULL
        AND e.type IN ('clear_level', 'level_cleared')
    GROUP BY
        e.cursemark,
        ei.item
),

baseline AS (
    SELECT
        item,
        uses AS starting_uses
    FROM item_by_cursemark
    WHERE cursemark = 1
)

SELECT
    ibc.cursemark,
    ibc.item,

    ibc.uses,

    b.starting_uses,

    ROUND(
        ibc.uses * 100.0 / b.starting_uses,
        2
    ) AS retention_percent

FROM item_by_cursemark ibc
JOIN baseline b
    ON ibc.item = b.item

WHERE
    ibc.cursemark >= 1
    AND b.starting_uses >= 50

ORDER BY
    ibc.cursemark,
    ibc.item;