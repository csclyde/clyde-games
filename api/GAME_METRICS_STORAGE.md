# Game metrics storage

The Clyde Games API accepts both shipped clients at `POST /event`. The submitted project selects the game database. Project-less legacy clients route to Cursemark and retain the legacy build cutoff. Accounts, feedback, crashes, settings, and diagnostic saves use `analytics`. Other projects keep their existing collection in `analytics`. The active Cursemark and Necrovale events and loadouts use `cursemark_metrics` and `necrovale_metrics` on the same MySQL server. A shared sequence keeps event IDs unique across the stores.

The Necrovale API accepts optional Hopestone and gold balances and optional cumulative outcome counters. Older clients can omit the fields. The economy cards group balances by build, mode, and outcome and show sample counts. The lifetime card uses the latest report per installation and mode in its time window. Journey counters describe the reporting save slot. Roguelike deaths persist independently of the individual run save.

## Cutover

1. Verify the deployed API, portal, and MCP baseline and record their release paths and hashes. Confirm that the database uses InnoDB and a server version supporting atomic multi-table `RENAME TABLE`.
2. Create a restricted, transactional dump of `analytics.events` and `analytics.event_items`. Restore the dump into a disposable schema, compare event and item counts, and run both the split and rollback against three disposable schemas. Exercise the release API, the portal, and both MCP project scopes with distinct fixtures. Keep the backup and archive on the database host.
3. Put event ingestion and admin mutation traffic into maintenance and drain it. Stop every process that writes events, including the API. Keep writers stopped through backup, copy, and verification. Requests during this window may fail and older clients may have no retry. A concurrent old writer can bypass these tools' migration lock, so verify the drain before continuing.
4. Take a final transactional dump with writers stopped, verify it against the source, and preserve it. Run `./api metrics-storage split`. The tool stages all rows, checks the copy counts, and switches the tables and sequence marker in one rename. The printed archive database holds the original tables. A preparation failure leaves the active names in place.
5. Check total and per-project event counts, item counts, and the archive before starting the release API. Confirm that the shared tables hold only other projects and the API verifies both stores at startup. Start the release MCP and publish the portal together with the API. Verify both authenticated dashboards, MCP metrics, the shared feedback-block endpoint, and unauthenticated access controls. Read existing data for live checks.

Pass three explicit schema names after `split` or `rollback` for an isolated rehearsal. The normal API startup uses the production schema names.

## Rollback

Drain and stop every event writer. From the new API binary run `./api metrics-storage rollback`. The command combines the current stores, preserves the IDs and Necrovale fields, and keeps the split tables in a new archive. It includes writes accepted since cutover and preserves intervening deletions. Check counts and start the previous API and MCP binaries and portal files. Keep the archive and the dump under restricted access. Restore an older backup only when intentionally recovering to its point in time.

MCP tools `list_metrics`, `get_metric`, and `query_metric` require `project: "cursemark"` or `project: "necrovale"`. Use IDs from the selected project's catalog. Unavailable storage produces an error for that game.
