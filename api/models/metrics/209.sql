SELECT e.mode, CASE WHEN e.type = 'player_died' THEN 'Death' ELSE 'Completion' END AS outcome,
	ei.slot, ei.parent AS parent_item, ei.item AS melded_item, COUNT(DISTINCT e.id) AS outcome_loadouts
FROM events e JOIN event_items ei ON ei.event_id = e.id
WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND e.type IN ('player_died', 'complete_run')
  AND e.mode IN ('journey', 'roguelike')
  AND created_at >= CURRENT_DATE - INTERVAL 29 DAY AND (e.type = 'player_died' OR (e.type = 'complete_run' AND e.mode = 'journey'))
	AND ei.item <> '' AND ei.parent <> ''
GROUP BY e.mode, e.type, ei.slot, ei.parent, ei.item
ORDER BY outcome_loadouts DESC, e.mode, outcome, ei.slot, parent_item, melded_item;
