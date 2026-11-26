# Crash occurrence storage and grouping

An accepted `POST /crash` creates a row with the submitted PID, project,
message, full raw stack (including GPU/runtime details), environment, category,
platform, build and commit. Client IDs, timestamps, counts, grouping and resolution
fields are ignored. Farm Builder may send `X-Clyde-Crash-Report-ID` with a
32-character lowercase hexadecimal value and a nonempty `PID`. The server
scopes the ID to the project and PID and returns HTTP 201 with the stored
occurrence on retry. An empty, malformed, or repeated header returns HTTP 400
with `INVALID_CRASH_REPORT_ID`. Requests without the header create separate
occurrences. Other projects use their existing occurrence behavior.

New rows have `OccurrenceVersion=1`, `Count=1`, server `IngestedAt` and matching
`CreatedAt`/`UpdatedAt`. Resolution changes only `Resolved` and `ResolvedAt`.
It never changes an occurrence time or overwrites metadata. `IngestedAt` records
receipt, not an inferred client failure time.

`GroupKey` is `v1:` plus SHA-256 of an ordered canonical JSON identity. Project,
category, platform, error and the full ordered stack must match. The v1 rule
removes known trailing runtime diagnostic sections, whitespace differences,
Haxe FilePos/ordinary caller source line numbers, absolute native addresses and
caller instruction offsets. The first failure source line/column or relative
instruction offset is retained, keeping different failure sites in one function
separate. Access-violation hex addresses in messages are normalized only when a
recognized source/symbol location is present; raw unsymbolized stack addresses
remain significant. Error numbers,
functions, source file paths, caller ordering and unknown stack text remain
significant. There is no edit-distance, shared-top-frame or transitive matching.
This intentionally leaves uncertain equivalents separate, including failures
whose actual failure location shifts between builds. `GroupIdentity`
exposes the exact canonical comparison input for inspection. Treat this rule as
versioned: changing it requires an explicit reclassification plan.

The admin groups complete report rows for presentation and exposes each member's
metadata/raw stack. Access violations also appear in the open group queue and
can be resolved there. Timeline/player statistics use individually recorded
reports only; a legacy aggregate cannot be placed accurately on that timeline.
Creating a Planka card retains the crash reports until explicit resolution.
No crash operation closes or moves Planka cards.

`POST /resolvecrash?id=<report id>` and MCP `resolve_crash(id)` resolve every
currently stored, non-deleted member with that group key, including legacy rows.
The portal's `POST /crash/ignore?id=<report id>` closes the same open groups
with an `ignored` disposition. The reports and their investigation history stay
available. A later occurrence can appear in the open queue.
MCP lists index individual active reports; detail returns `group_report_ids`
including resolved members so each can be fetched with `get_crash`.
The complete membership index fails explicitly above 100,000 members.
Legacy hash resolution remains supported when it identifies one group; optional
`project` scopes it. Ambiguous hashes fail and require an ID.

Ingestion and per-group resolution acquire the same unique `crash_groups` row
lock in a transaction. Resolution includes all members inserted before it gets
the lock. A later insert stays open and does not reopen old resolved reports.
Repeated resolution leaves existing resolution timestamps intact. Bulk legacy
`all=true` resolution is one SQL update of currently visible open rows; later
inserts stay open. The UI's Close All targets the displayed project groups and
checks every response; partial failures remain visible after refresh.

## Migration and deployment

The normal API startup migration adds the occurrence/group columns, group-key
index and identity-only `crash_groups` table, then backfills identities in batches
of 250. It is resumable, conditional on an empty key, and includes soft-deleted
history. Existing IDs, hashes, Count, raw metadata, resolution and both original
timestamps are preserved byte-for-byte. Existing rows remain version 0, even
when Count is 1. Their ingestion/resolution times remain null: old UpdatedAt may
have been changed by resolution. Historical metadata lost by aggregation cannot
be reconstructed, and old cross-project aggregation cannot be disentangled.
No synthetic report rows or guessed players/builds/times are created.

Before deployment, back up the database and rehearse against a restored copy.
Allow startup time and MySQL DDL capacity for additive columns/index creation
and backfill. Stop old API writers before migration; do not run mixed old/new
ingestion binaries, because an old binary would aggregate new occurrence rows.
Deploy API and admin together, plus MCP tool-description updates. Start the API
only after migration succeeds; startup already fails on migration errors.
Database users need the existing AutoMigrate DDL permissions and access to the
new locking table. No new credentials or ingestion authorization are needed.

Schema rollback should retain the additive data. Rolling back to the old
ingestion binary is unsafe for new occurrences; prefer a forward fix or pause
ingestion while preparing an explicitly reviewed rollback. Do not merge or
delete new occurrence rows to recreate old aggregation semantics.

## Local checks

Run `cd api && go test ./...`, `cd mcp && go test ./...`,
`npm run build:admin`, `node scripts/crash-groups.test.mjs` and
`node scripts/crash-smoke.cjs`. The browser test mocks every API request and
requires Playwright plus installed Chrome; `PLAYWRIGHT_MODULE` can point to an
external package. The existing API system-stat implementation requires Linux;
on Windows a local test-only overlay can replace its unsupported Statfs call.
Cross-build the unmodified Linux source separately in that case.

`CRASH_TEST_MYSQL_DSN` enables the model integration test against a disposable
loopback MySQL/MariaDB instance. The test creates and later drops its own randomly
named database; it refuses non-loopback DSNs and DSNs naming an existing database.
Never point tests at production. SQL-mock and browser tests need no database.

## Per-project accepted build cutoff

Crash settings are independent from metric/save retention. Admin-authenticated `/crash/settings?project=<exact ID>` GET/PUT and `/crash/builds?project=<ID>` scope the selector and enforcement. `all` cannot change a cutoff. The migration snapshots the current global `oldest_crash_build` value into each existing project's hashed crash setting key once; reruns preserve subsequent project edits. New projects accept all builds until deliberately configured. Untagged legacy clients keep the original global crash key and are never guessed into a game. Existing build parsing/empty-build policy is retained. A build below its project's cutoff is rejected within the transaction before a crash/group row is stored. Equal-boundary builds are accepted. Changing the threshold never deletes or resolves stored crash history. This extension is deployed; the four existing projects inherited `2026-10-16 13:32:18` independently. The untagged legacy global value remains unchanged. See FEEDBACK_SYNC_DEPLOYMENT_20261116.md.

## Crash health dashboard

The read-only crash health dashboard is deployed. See CRASH_HEALTH.md for receipt/build/reporting-ID/returned-group semantics and CRASH_HEALTH_DEPLOYMENT_20261116.md for installed artifacts, tests, preserved data and rollback. The occurrence storage, identity and resolution rules above remain unchanged.
