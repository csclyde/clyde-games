# Feedback save attachments from blocked installations

The desktop client already creates feedback first, extracts the returned ID,
then uploads an immutable current-session save with `feedback_id`, `pid` and
`project`. No production game-client change is required for this exception.
Structurally invalid, missing, inactive-slot and over-1-MiB snapshots remain
skipped by the client. Feedback creation failure never triggers a save upload.

The current server detects excessive event resources (Essence over 100,000 or
Tears over 10,000) and blocks the installation ID; administrators can also block
it. This is a resource heuristic, not proof that a file was manually edited.
The metrics block and removal of ordinary telemetry remain unchanged.

For `POST /savegame`, an attachment may bypass the blocked-player check only
when its nonzero feedback ID identifies a non-deleted, unresolved in-game
feedback row with the same exact non-anonymous PID and normalized project.
Missing/unknown identities, wrong projects/players, community-source rows,
missing/deleted/resolved reports and database errors cannot grant the exception.
The feedback row is locked in the insert transaction against manual resolution.
A `reason=feedback` label alone grants nothing. Both `feedback_id` and
`feedbackId` remain supported; malformed, zero, overflowing or conflicting
values return 400 rather than silently becoming unlinked saves.

The existing public client protocol identifies installations using caller-supplied
PIDs; this change does not claim cryptographic player authentication or add a
new upload credential. Admin-only downloads and API/MCP/portal authentication
are unchanged. The 1-MiB file cap, bounded request body, SHA-256 calculation,
project-specific save/metrics oldest-build floor and current save retention
policy still apply. An old-build feedback message can arrive without its save
if that save fails the configured floor, just as before.

Blocking or reblocking an installation removes only unlinked ordinary saves (including legacy NULL feedback IDs),
while retaining feedback-linked diagnostics. Existing manual feedback resolution
still performs its usual attachment cleanup. The existing one-month retention
event is unchanged. Previously deleted/rejected attachments are not recovered.

Verification uses only synthetic local data: raw/multipart size boundaries and
ID aliases; actual excessive-resource blocking, feedback-first HTTP intake,
byte/hash/association readback, ordinary upload and metric rejection, invalid
association/no-row checks, oldest-build rejection, repeated and concurrent
blocking/attachment intake; locked-association/database-error unit cases; full
API tests and vet against disposable loopback MariaDB; mocked HashLink client
telemetry regressions with normal and excessive-resource save snapshots.

No schema migration or production game-client change is needed. Activation is
backend-only; the accompanying user-history ordering change also updates admin
assets. See the verified deployment record below.


## Verified deployment: 2026-11-18

API SHA256: `0add5f853f048cf0f93ffd4488af250b2ca764f14e36adbf8308eb27de6407a5`.
Admin: `bundle-a64a4a3f15cb.js` / `bundle-14cb36136bb7.css`.
Native Linux full API tests, race, vet and builds passed, including the
legacy-NULL block regression. Existing browser responsive/history tests and the
120-assertion mocked HashLink client telemetry suite passed. Live read-only
history verification checked 60 existing records for newest-first order. The deployed
API matches the tested build; no production feedback/save test records or
reactions were created. Existing authentication, 27 MCP tools, source/build
settings and legacy crash history remain intact. No commits or pushes.

Release/journals and immediate rollback: `/root/clyde-feedback-save-history-20261118-compat`.
The original pre-change rollback remains under `/root/clyde-feedback-save-history-20261118`.
Both retain private database/file backups. Rollback restores API/static files
without restoring the database or changing session authentication. The immediate
rollback retains the two main features but reverts the legacy-NULL correction;
the earlier rollback restores the API/admin version preceding both features.
