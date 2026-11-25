SELECT build, mode, type AS outcome, COUNT(*) AS events,
       COUNT(hopestones) AS hopestone_samples, AVG(hopestones) AS average_hopestones,
       COUNT(gold) AS gold_samples, AVG(gold) AS average_gold
FROM events
WHERE deleted_at IS NULL AND LOWER(env) = 'prod'
  AND created_at >= CURDATE() - INTERVAL 29 DAY AND created_at < CURDATE() + INTERVAL 1 DAY
  AND type IN ('player_died', 'complete_run')
GROUP BY build, mode, type
ORDER BY build DESC, mode, outcome;
