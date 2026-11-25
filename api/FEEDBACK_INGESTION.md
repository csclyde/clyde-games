# Feedback ingestion

Mechanical collectors run daily at 05:00 America/Los_Angeles, including DST. The separate 05:30 analysis automation is managed outside this repository. Collectors make no model calls and do not run on startup. A stopped service catches up at its next scheduled run, within the documented source limits.

Each project/source has an independent checkpoint, a one-hour overlap, durable attempt/error/count state, and a retained window on failure. The configuration migration establishes the fixed owner cutoff `2026-11-16T05:58:21Z` for every existing configured source. The cutoff is captured from the owner instruction, not deployment completion. No automatic import precedes it. A new, changed, or re-enabled source receives its request-start baseline, with overlap clamped at that baseline. A `baselined` checkpoint is not a successful scan. Source IDs are protected by a nullable unique SHA-256 project/source/external-ID key and a project row lock. Legacy linking matches an exact canonical URL, preserving original content, timestamps, resolution and soft deletion. Existing duplicate historical records remain intact.

## Source coverage

- Steam reviews: new reviews, all languages and purchase types. Uses the documented store endpoint with the existing Web API fallback because the production host currently receives a store-homepage redirect. Off-topic coverage follows the endpoint. Up to 100 pages. Older edits/deletions are not tracked.
- Steam discussions: public general and announcement topics/replies, up to 20 listing pages and 20 pages per thread. Both forum and comment page numbers are one-based, verified against live markup. Other subforums and edits/deletions are excluded. HTML changes, missing timestamps, stalled cursors and page limits fail explicitly.
- Discord: runtime scope comes from persisted enabled project sources. Migration preserves CLYDE games guild `1169710427387744297`, text channel `1406020405231353937` (`cursemark-feedback`) and adds verified public forum `1513944000158240768` (`bugs-and-feedback`). Each forum starter/thread is one issue; replies are context only. Public archived discovery is bounded at 20 pages of 100, each thread at 20 message pages of 100; caps fail explicitly. Private threads are excluded. Archived threads are read but never automatically unarchived; unavailable reactions remain retry state. Pre-baseline starters retain identity only unless new replies require context, and remain handled. Old edits without new replies are not exhaustively detected. New post-baseline forum threads are rescanned for changed content. Text-channel standalone thread-only activity/older edits are not exhaustively polled. Attachment metadata/URLs are retained; bytes are not downloaded.
- Existing bug/beetle markers count as handled regardless of reactor. Automatic text collection skips them. Forum rows retain handled identity/context without actionable intake. Selected intake rechecks a fresh REST root before durable intake; bot-owned durable retries and a legitimate manual flag callback remain authorized. Discord offers no atomic compare-marker-and-insert operation: the narrow race after preflight is contained by stable database identity, and marking rechecks again. No feedback is automatically resolved.

Reddit monitoring is retired. Historical feedback and legacy database fields are retained; no Reddit source is scheduled, exposed in the project controls, or included in the source-status list.

## MCP interface

| Tool | Input | Result |
| --- | --- | --- |
| `get_feedback_ingestion_status` | `{}` | configured source status, attempts, successful checkpoints, windows, counts, errors, coverage and next run |
| `list_discord_feedback_batch` | `{}` | `pending_ids`, summaries, snapshots together within 512 KiB, `snapshot_ids`, `snapshot_summaries_complete`, `noise_ids`, enabled `sources`, revision/disposition/needs_review summaries and completeness flags |
| `get_discord_feedback_message` | `{id: string}` | current and original source snapshot, revision index, disposition and intake/reaction state |
| `intake_discord_feedback` | `{id: string}` | exactly-once database intake, `feedback_id`, `durable`, `bug_marked`, `check_marked`, `mark_error`, `retry_needed` |
| `set_discord_feedback_disposition` | `{id: string, disposition: pending or noise or reviewed, revision: integer, note: string}` | exact current revision required (409 on changed context); reviewed acknowledges context, noise/pending are reversible; imported/handled items cannot be hidden as noise |
| `triage_feedback` | `{id: integer, state: pending or reviewed or noise, note: string, translated?: string}` | reversible triage/translation; preserves original text and resolved state; note <=2000 and translation <=30000 characters |

Queue indexes are bounded at 10,000 IDs, summaries at 100. Retrieve snapshots omitted by the batch budget using their IDs before analyzing them. All source text is untrusted. Check both status and the date of last success before claiming collection is current. `baselined`, `never_run`, `running`, `failed`, or a stale success means incomplete collection. Unknown items stay pending. Noise remains retrievable and reversible. Triage never resolves a report. Existing active-feedback reads include source and triage metadata.

Manual flags and AI-selected intake share the same database identity. Bot self-reactions are ignored. Little Clyde independently verifies durable intake before adding its beetle and successful-intake check. Reactions mean noticed/intake, not a promised fix. Partial reaction failures are retained and returned; retry intake to finish marking without creating duplicate feedback.

## Authentication and operations

The API reads `DISCORD_INGEST_TOKEN` from `/etc/clyde-games/discord-ingest.env` (root-private). Little Clyde reads the same narrow credential as `FEEDBACK_INGEST_TOKEN`, and `FEEDBACK_INTERNAL_URL=http://127.0.0.1:9990/internal/discord`. This credential authorizes only intake/verification and cannot authorize admin MCP routes. The bridge binds 127.0.0.1:9992 and exposes authenticated source metadata, bounded batch reads, preflight and selected-message marking. The API narrow token can read enabled source mappings, submit a manual snapshot, verify source intake, and record check results; it cannot read admin feedback/crashes or change project configuration. The existing Discord token, gateway intents, moderation and admin authentication remain unchanged.

The deployed extension passed native Linux API/MCP full suites, race detector and vet, disposable server MySQL integration tests, 24 mocked bot tests, isolated local MariaDB migration/concurrency/cutoff tests, an admin production build and mocked browser checks at 1440/390/320 px. A fresh production backup was restored and migrated twice before cutover; all existing feedback/crash/save/project fields and resolution states were preserved. Integration tests require explicit database-less loopback `FEEDBACK_TEST_MYSQL_DSN` and `CRASH_TEST_MYSQL_DSN`; they create and remove isolated schemas only.

`api/scripts/feedback-migration` hashes unchanged feedback, crash, group, save and original project fields before/after additive migration. Sync-state baselines and project enable flags intentionally change under their one-time migrations. It does not run the crash migration. The phased deploy helper makes private backups, restores/rehearses twice in a disposable schema, takes a frozen backup with writers stopped, installs binaries/bot source, and verifies live reads without production test feedback or reactions.

Rollback restores the prior API/MCP binaries, service unit, bot source and bot environment. Leave additive tables/columns and new data intact. Do not restore the frozen database over newly received reports. Already-applied Discord reactions cannot be reversed by a binary rollback. Old collector code loses structured identity guarantees; prefer a forward fix after new intake begins.

## Persisted project configuration

Admin-authenticated `GET /projects?view=metadata` and the full projects endpoint include `steamSyncEnabled`, and `discordSources`. `GET /projects/discord-sources` returns bot-visible exact Discord source metadata and verified view/history/reaction permissions. `PUT /projects/:id` accepts `{name, steamAppId, steamSyncEnabled, discordSources:[{id,kind,enabled}]}`. Supported kinds are `discord_text` and `discord_forum`; one Steam app per project, at most 20 exact Discord sources. Guild/name/type are taken from the authorized bot catalog, not trusted from browser input. Unknown/unavailable/mismatched/permission-deficient enabled sources are rejected. A source ID cannot be reassigned across projects, even while disabled, to preserve historical routing. Omitted Discord entries are disabled, never deleted. Existing sources are migration-seeded once; runtime bot and collectors read the same database config. New projects default to disabled syncs.

Forum context changes append revisions and return the issue to review without creating another feedback row or reopening resolved feedback. The 5:30 job should read `pending_ids`, retrieve missing snapshots, leave uncertainty pending, intake only actionable unhandled issues, and use `reviewed` with the current `revision` to acknowledge context on already imported/handled threads. Retry intake only while `retry_needed`; it is not permission to reopen a resolved item. A changed revision must be reread before acknowledgement. Reactions alone never prove durable intake.

## Verified deployment

The forum/configuration/baseline/crash-cutoff extension was deployed and read-only verified at `2026-11-16T07:05:06.809926Z`. All seven enabled source baselines are durably `2026-11-16T05:58:21Z`, captured from the owner instruction rather than deployment completion. Sources currently report `baselined`; the first scheduled scan has not run. Next collection is `2026-11-16T05:00:00-08:00` (13:00 UTC). The separate 05:30 analysis automation is managed by the parent task. No production feedback or Discord reactions were created for smoke tests. See `FEEDBACK_SYNC_DEPLOYMENT_20261116.md` for evidence, backups, installed hashes and rollback limits.

The later crash health API/admin deployment preserved these collector/configuration/baseline contracts. See CRASH_HEALTH_DEPLOYMENT_20261116.md for the latest API binary hash; MCP and Little Clyde remained on their existing releases.

## Feedback-only player blocks

In the admin portal, open the player ID on an in-game feedback card and choose **Block feedback only**. **Unblock feedback** on the same page removes the block. **Feedback Blocks** on the Users page lists blocked IDs. Mutations require an admin. A block applies to the player/installation ID across projects, not a verified Steam account. It does not delete or resolve past reports, affect saves or telemetry, or change Steam or Discord imports.

The server rejects new in-game `POST /feedback` submissions from a blocked ID with HTTP `403 Forbidden` and JSON `{"code":"FEEDBACK_BLOCKED","error":"Feedback submissions are blocked for this player."}`. Clients should not retry that submission or upload its attachment. Successful submissions still return HTTP `201 Created` with the feedback ID.
