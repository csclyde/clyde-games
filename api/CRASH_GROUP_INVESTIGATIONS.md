# Crash group investigations - deployed November 19, 2026

Crash investigation is one durable unit per canonical project plus real GroupKey.
The Access Violations card is intentionally one investigation per project across
its presentation family; original real incident keys and reports remain intact.
A shared AV investigation does not imply a shared root cause or a fix across
all constituent incidents; unresolved causes must remain explicit in its state/log.
Missing legacy identities remain separate report-anchored units rather than
inferred matches. Normal ingestion/migration already assigns real identities;
a later manual identity repair requires deliberate reassociation, never a silent
merge. The stable investigation key includes scope/type, so project, family,
real-key and unknown-legacy namespaces cannot collide.

## Canonical record and legacy history

Two additive tables hold units and durable worker claims:
`crash_investigation_units`, `crash_investigation_claims`. A unique unit key and
row lock serialize aliases of the same group; claims have a unique binary
(unit_id,request_key) identity and a restricted unit foreign key. New immutable
notes use the existing report_notes table with kind `crashgrp` and the unit ID.
Existing kind `crash` notes stay byte-for-byte attached to their original reports;
the group read merges them chronologically as labeled legacy history, including
resolved members. Existing archived originals are retained and not linked in UI.
No old notes, crash rows, grouping identities, counts or resolution flags change.
The migration can be replayed; units materialize lazily on the first deliberate
claim or group note. GETs never create units or claim work.

The latest nonblank group state/release state is canonical. Before any group state
exists, retained legacy notes supply the latest state; conflicting latest legacy
member states become ONE needs_confirmation investigation pending review. The
legacy acknowledgement frontier covers only actually annotated report IDs; it
never jumps to the current maximum. Feedback investigation remains per feedback.
The owner alone closes reports; ready_for_review and local are not closure/release.

## Work lifecycle and duplicate prevention

Normal MCP list_active_crashes now returns GROUP work, not individual occurrence
tasks. It includes complete active_groups [{key,project,anchor_id}], compatibility
active_ids containing unique group evidence anchors, active_count (groups),
unit_type=crash_group, index_complete, up to 200 summary items, limit/truncated.
The read fails explicitly above 100000 retained rows or 10000 active groups; it
never silently conceals an incomplete work index. Each group read uses a consistent read-only transaction and is bounded to
50 notes/page. Individual get_crash remains a read-only evidence operation.

Before code investigation, get the group and acquire its claim. Only acquired=true
authorizes the normal worker path to proceed. Leases last 30minutes and can renew
with the same request_key plus returned token before expiry. A different worker
is refused during an active lease. Exact acquisition retry returns the same grant;
expired/completed grants cannot restart. A fresh run key can resume abandoned
work after expiry. Tokens are private coordination capabilities, not report text.

MCP group notes require the active token. complete=true requires an outcome state,
atomically saves the note and releases the lease, acknowledging only report IDs
captured at acquisition. Identical note retries replay across ANY member anchor,
including after completion/expiry. Different payload under the same key conflicts.
Transactions use READ COMMITTED with the unique unit row lock so a waiter sees a
previous worker's committed outcome. New evidence during the run stays pending.

New receipts and genuinely returned crashes revisit the SAME unit. Returned is
computed against each real incident's retained resolution timestamp, even within
an AV family; a later resolution in another incident cannot hide a genuine return.
The revision fingerprints real resolution boundaries; an old-cycle result cannot
overwrite a newer lifecycle. Unknown legacy receipt times are never invented.
A ready-for-review investigation with unreviewed evidence displays needs_confirmation.
A blocked/confirmation outcome without new evidence is not automatically repeated.
The owner can explicitly request another look by adding a group note with state
investigating. State-bearing portal notes acknowledge the current evidence; any
concurrent owner note fences stale worker writes. None of these operations resolves
reports, posts messages or moves tickets.

New per-report crash notes are blocked at persistence (409 crash_group_required),
including context-only writes, except exact replay of a previously saved legacy
request. MCP append_report_note rejects crash initiation before upstream dispatch.
Old report-note/index reads remain available as explicitly labeled legacy evidence,
not separate work. The legacy note response also carries the canonical group state,
rather than advertising independent per-report investigation state.
get_report_investigation_status(kind=crash,ids) returns deduplicated group states.
There is no per-member investigation control in the portal.

## Authenticated contracts

Existing portal session/admin/viewer and dedicated MCP authentication remain.
Portal viewers may read but cannot append; MCP actor is server-derived, never
client-supplied. Project is an exact explicit canonical query parameter; explicit
empty scope retains untagged legacy compatibility. A foreign/deleted anchor is404.
Unknown fields, oversized bodies, invalid states/cursors/keys fail closed.

* GET /crash-investigations?project=all|<canonical> — bounded group queue above.
* GET /crash-investigation/:anchor?project=...&after_id=0&limit=20 — canonical
  investigation state/key/revision/coverage/needs_review/returned, a complete
  report_ids evidence index (including resolved members), report_index_complete, plus chronological
  items, has_more,next_after_id,frontier_id. Legacy items scope=report_history retain
  source report IDs/build/resolution; new items scope=group belong to the unit.
  Optional family=access_violations is validated; family selection is otherwise
  inferred from authoritative evidence, never user-selected to split work.
* POST /crash-investigation/:anchor/notes?project=... — portal admin body
  {request_key,note,state?,release_state?}; appends ONE group log, no report target.
* Internal MCP equivalents use /internal/mcp. MCP notes additionally require
  claim_token and optional complete. Portal accounts cannot submit claim tokens.
* MCP-only POST /internal/mcp/crash-investigation/:anchor/claim?project=... —
  {request_key,claim_token?}; result {acquired,reason?,claim_token?,expires_at?,investigation}.
  Skip acquired=false. Lost/expired claim or lifecycle/manual-note conflicts fail409.

MCP 0.6.0 exposes 30 tools, adding:
get_crash_investigation({id,project,after_id?,limit?}),
claim_crash_investigation({id,project,request_key,claim_token?}),
append_crash_investigation_note({id,project,request_key,note,state?,release_state?,claim_token,complete?}).
All member IDs resolve to the same key; IDs in these calls are evidence anchors,
not new investigation identities. The SDK instructions and list_active_crashes
contract require group read/claim before investigation. Feedback tools are unchanged.

## External worker and rollout limitation

Repository inspection found no crash AI scheduler, worker queue or autonomous
investigation executor. Steam/Reddit/Discord collectors remain mechanical. The
separate 5:30 analysis automation is hosted outside this checkout. Its owner
thread is `01a1a1a9-ce77-7175-bb26-af812d3e00d4`; task
`6af783fc452c8191a9e86fe4a325862a` (Player feedback watch) runs daily at
05:30 America/Los_Angeles. It was explicitly paused and rechecked disabled before
cutover. Its existing prompt still describes per-report work: leave it paused
until the owner adopts active_groups -> read/claim -> evidence -> group outcome.
A prepared compatible section is retained with the release evidence. The 5am
mechanical collectors and task schedule were unchanged. Server leases and the
per-report write rejection enforce the supported workflow; an independent external
process cannot be prevented from doing unclaimed analysis outside this API.

Deployed and verified at 2026-11-19T00:42:58Z after native Linux tests/race/vet,
fresh backup and twice-replayed migration on a restored copy. MCP was stopped
and drained before migration. Live group readback retained 12 legacy notes across
11 reports, including resolved history. No player records, notes, claims or
resolution states were fabricated for production verification. See
[deployment and preservation evidence](CRASH_GROUP_DEPLOYMENT_20261119.md).
Reconnect MCP clients to discover the three new tools. The game-client API,
authentication tokens and session policy are unchanged.

Rollback preserves both new tables and all new/old notes; never restore an old
DB over new arrivals or drop the canonical record. An older binary/UI cannot
show new crashgrp notes and can re-enable per-report work, so pause external crash
analysis on rollback until group-aware API/MCP is restored. Existing feedback,
crash closure and Planka status remain manual and untouched.
