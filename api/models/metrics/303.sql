SELECT e.build, e.level AS season, i.slot AS kind, i.item,
       SUM(i.level) AS total_quantity, COUNT(*) AS season_boards
FROM events e JOIN event_items i ON i.event_id = e.id
WHERE e.deleted_at IS NULL AND LOWER(e.env) IN ('prod', 'production') AND e.type = 'season_completed'
  AND e.created_at >= CURDATE() - INTERVAL 29 DAY AND e.created_at < CURDATE() + INTERVAL 1 DAY
GROUP BY e.build, e.level, i.slot, i.item
ORDER BY e.build DESC, e.level, i.slot, total_quantity DESC, i.item;
