# Investigation status and canonical notes (MCP0.5.0)

> Deployed November 19: crash investigation now uses one canonical group record
> with worker claims. Individual crash notes remain labeled legacy history;
> feedback remains per item. The crash per-entry write workflow below is
> superseded by [group investigation contracts](CRASH_GROUP_INVESTIGATIONS.md).
> The external 5:30 analysis task remains paused pending its group-aware prompt.



Deployed and verified November 16 at 23:30 UTC after explicit source-upload approval, 15 native checks and twice-restored preservation rehearsal. The existing notes and recoverable archive data are unchanged. Game Editor is live as game-editor. See [release and rollback evidence](INVESTIGATION_STATUS_DEPLOYMENT_20261116.md).

Concise chronological notes attached to each individual database feedback/crash entry are the canonical investigation record. No Markdown, linked evidence or server files are required by future runs. Corrections append; original notes/history are not rewritten. The actual report `resolved` flag remains authoritative for closure. **Only the owner closes reports manually.** A tested local fix is not a released fix or permission to close anything.

## Visible states

* `unreviewed`: no explicit investigation state yet.
* `investigating`: investigation is underway.
* `blocked`: cannot proceed; concise note explains why.
* `needs_confirmation`: reproduction, player combination or validation remains uncertain.
* `ready_for_review`: fix complete and tested, awaiting the owner's review; report stays open.

Existing `fixed` notes read as `ready_for_review` without rewriting history. Effective state is the newest nonblank state in that record's log. Newest nonblank release state is separately reported (`local`, `deployed`, `released`); context-only notes change neither. `local` explicitly means unreleased. State timestamp, last-note timestamp and note count are derived from the immutable log. No new schema columns or historical data changes are needed.

Feedback cards show their own status without expansion. Crash queue cards show a common state only when every stored report agrees; mixed states are shown with counts and never imply the whole group is fixed. Each individual collapsed crash-report summary also shows its own status, including within AV presentation families. Status unavailable/loading is distinguished from unreviewed. Saving a note refreshes the visible status safely. The separate bottom-page recorded-investigations section is removed.

## Authenticated API and MCP

Existing portal AdminAuth and dedicated MCP authentication still protect reads/appends. Game-client ingestion contracts are unchanged.

* `GET /report-investigations?kind=feedback|crash&project=...&ids=1,2`:1–200 distinct positive individual IDs, all from one explicit canonical project. Missing/deleted/wrong-project IDs fail the entire request; there is no silently partial result. Returns `{kind,project,complete:true,items:[{id,state,state_at?,release_state?,last_note_at?,note_count}],resolution_policy}`. Internal MCP route is `/internal/mcp/report-investigations`.
* `GET /report-notes/:kind/:id?project=...&after_id=0&limit=20`: returns `{report:{id,project,resolved,message:""},investigation,kind,items,has_more,next_after_id,frontier_id,limit,resolution_policy}`. Follow `has_more`/cursor; max50. Notes are chronological by ID. Effective status is computed within the read's captured note frontier. Notes contain text, authenticated source and server timestamp; archived evidence metadata/links are no longer returned.
* `POST /report-notes/:kind/:id?project=...`: `{request_key,note,state?,release_state?}` returns `{note,replayed,resolution_changed:false}`. Concise text1–2000characters, stable case-sensitive request key8–80 ASCII letters/digits/`-_.:`. Exact same payload/key safely replays; differing content conflicts409. Row locking and unique source/request identity preserve concurrency. Blank state is context only. Legacy `fixed` remains accepted for exact retry compatibility, while new work uses `ready_for_review`.
* `GET /report-notes?kind=...&project=...`: bounded database annotated-record index, including closed entries; useful for MCP discovery. It has no separate bottom-page UI.

MCP0.5.0 keeps27 tools: `get_report_investigation_status({kind,project,ids})` replaces `get_report_evidence`. `get_report_notes({kind,id,project,after_id?,limit?})`, `append_report_note({kind,id,project,request_key,note,state?,release_state?})` and `list_report_investigations` remain. Append no longer offers an evidence/file payload in its schema. Refresh/reconnect clients for discovery.

Before work, read the selected source report, its effective investigation status and its concise note log. Record useful attempt/outcome/tests/blocker and actual release availability. Use ready-for-review after a tested completed fix, and leave the source open. For shared crashes, retain notes/statuses on each actual database record; presentation families never merge identities. Do not resolve/close/move anything automatically. Unknown items remain available for review.

## Preservation and rollback

The 13 existing notes already preserve useful findings for individual feedback7884/7885/7886 and crashes3826/3907–3913: formulas/loadout uncertainty, tested local StarOrbiter/Tortite fixes, unreproduced underlying physics fault, diagnostic-only fix and existing Planka reference. Their original text, states, timestamps, IDs and resolution flags are preserved. The private archived originals remain recoverable backup data; no archive links/download API or MCP evidence tool remain in the normal workflow, and the notes do not depend on them. No irreplaceable original is permanently deleted.

Deployment must first verify a fresh restored-copy replay preserves every original note/evidence field and schema, original report/save/project fields, events, source configuration and cutoffs. Rollback restores previous binaries/unit/admin index while retaining database notes and all new arrivals. Never restore an old database over later production data. Historical deployment documents describe older surfaces; this contract describes the current cleanup.


## Crash group log visibility (deployed 2026-11-18)

Crash cards now put an expandable **Investigation Notes** log immediately below compact runtime/incident details. Its inline status summarizes only currently open reports on that card; resolved history cannot make the open incident look fixed. The log reads notes from all retained members of the same real group, including resolved members. Access violations remain a presentation family: their combined log identifies each source report, and individual real group identities remain available. Unknown legacy identities are not inferred or combined. Notes and changes of investigation/release state remain canonical on each individual report; adding from the group log requires an explicit report selection and uses the existing idempotent append route. No group-owned notes, resolution action, schema migration or data rewrite is introduced.

Portal-only read route: `GET /crash-investigation/:id?project=...&family=access_violations&after_id=0&limit=20`. `family` is optional and is accepted only for an AV anchor. Explicit canonical project is required (explicit empty scope retains untagged legacy compatibility); wrong-project/deleted anchors return404. Existing AdminAuth/session/viewer permissions apply. Responses include `{project,anchor_id,group_key?,family?,report_count,open_report_count,note_count,items,has_more,next_after_id,frontier_id,limit,includes_resolved_reports:true}`. Each chronological item has `{id,report_id,created_at,actor,note,state?,release_state?,report_resolved,report_build?,report_group_key?}`. `report_count` counts stored rows rather than aggregate occurrences. Reads capture a maximum note ID per request, page by ascending ID, and cap pages at50; new notes appear on subsequent reads. No raw client identifiers, stacks or archived evidence are added to this projection. Existing individual API/MCP read/append contracts stay unchanged; MCP callers continue to read/write notes on each source record.

The **Returned** badge means at least one currently open detailed report was received after that incident's recorded resolution. It does not establish that a released fix failed. AV family badges apply if any underlying incident returned. Detailed first/last receipts cover retained detailed history, while recent reports and reporting IDs use the selected analysis window; IDs are not verified people. Unknown legacy timing remains unknown. Runtime/incident metadata is inline; the separate More incident data disclosure is removed. The stack starts with six frames and its own Expand/Collapse stack trace control. Incidental guard diagnostics are hidden only in the UI; original API/database evidence, grouping identities and diagnostics remain intact. Manual resolution and partial-retry behavior are unchanged.


Deployed at 2026-11-18T23:01:04.104563+00:00 after full native Linux tests, race checks, vet and builds. Read-only production checks verified 12 notes across 11 retained members (10 open) of the PhysicsWorld group, including resolved history. All pre-existing crash/feedback/note/evidence fingerprints matched. No schema migration, auth-policy change or player-record mutation. See [deployment and rollback](CRASH_INVESTIGATION_DEPLOYMENT_20261118.md).
