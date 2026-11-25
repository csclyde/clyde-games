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

MCP tools `list_metrics`, `get_metric`, and `query_metric` require an explicit game project. Use IDs from the selected project's catalog. Unavailable storage produces an error for that game.

## Farm Builder

Farm Builder appears under Games with project ID `farm-builder`. Public
`POST /feedback` and `POST /crash` accept the shared JSON fields and store the
reports in `analytics`. Recovered crashes can use the optional header documented
in [Crash occurrence storage](CRASH_REPORTS.md).

`POST /event` accepts the following season snapshot and returns HTTP 201.
The same `PID`, run `SID`, and positive `RunStep` return the same stored event.
Send a fresh 32-character lowercase hexadecimal `SID` per run and reuse the
same snapshot on retries. The endpoint needs no event ID header. Existing
clients may send `X-Clyde-Event-ID`, which collection ignores.

```json
{
  "Project": "farm-builder",
  "Type": "season_completed",
  "PID": "7db70ccabf3a4732986ac14ce017abf5",
  "SID": "915fa90c426a41fcbac1698d63e239c6",
  "RunStep": 1,
  "Level": "spring",
  "Gold": 11,
  "Items": [
    {"Item": "Garden-Square", "Slot": "piece", "Level": 2},
    {"Item": "Crop+L", "Slot": "feature", "Level": 1}
  ],
  "Env": "production",
  "Build": "0.1.0",
  "Platform": "windows"
}
```

`Gold` is the amount credited for finishing this season. `Level` is `spring`,
`summer`, `fall`, or `winter`. Each item records its exact case-sensitive
catalog ID and a positive quantity in `Level`. Combine repeated
`(Slot, Item)` entries in the game snapshot. Season snapshots remain in
`farm_builder_metrics`. The dashboard reports the last 30 days for
`production` and `prod` events. MCP metrics accept `project: "farm-builder"`
and IDs 301–303.

To add the database to an existing split, stop legacy event writers and run
`./api metrics-storage add-farm-builder` before starting the new API.
Provisioning can be rerun and leaves the existing database tables in place.
It stops if it finds Farm Builder history in the shared events table. Preserve
and migrate any such history before activation. Keep Farm Builder event
producers paused if reverting to a binary that predates its dedicated store.
