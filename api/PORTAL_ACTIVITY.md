# Portal feedback activity

The authenticated sidebar checks `/projects?view=navigation` every 15 seconds
while visible and every 60 seconds in the background. Focus, visibility, online
and page-restoration events catch up promptly. Requests do not overlap, time out
after 10 seconds, back off on errors, and stop with the authenticated view.
Prior project names/counts remain visible during transient failures.

Browsers may delay background timers or suspend tabs while asleep. This uses
lightweight polling, not operating-system push notifications; returning to the
tab catches up. No public stream, community posts, collectors or AI calls were
added. Existing open-feedback/crash totals remain; the new notification itself
is only a dot without a badge count.

Navigation JSON adds `openFeedbackIds` (decimal strings) and
`feedbackIdsComplete` to each existing project. These are unresolved,
non-deleted feedback database IDs, scoped by project, without message bodies.
The snapshot is bounded to 5,000 IDs globally with an extra sentinel query row.
An incomplete/count-mismatched snapshot never claims all feedback is seen.
Existing response fields and authentication are unchanged. No schema change is
needed for activity tracking.

Exact ID membership detects arrivals despite simultaneous resolutions or a
lower ID committing after a higher ID. The first complete snapshot establishes
a quiet baseline for existing feedback. Future arrivals get a dot beside the
project and on the main-site Clyde Games Y favicon, including background tabs.
Viewing at least half a card's visible viewport area in a focused, visible tab
acknowledges that ID. Filtered-out cards and other projects remain unseen.
Acknowledgement never closes, resolves or edits any database item.

Only seen IDs are stored locally, scoped to the portal account (legacy Basic
Auth uses the browser's legacy scope). Up to 5,000 recently seen IDs per project
are retained. Storage events merge acknowledgements across tabs. No message
text, author data, password, token or session is stored. If storage is disabled,
tracking still works for the current tab. Clearing browser storage starts a new
quiet baseline. Edits to an already-seen stable ID do not create a new-arrival
notification.

Feedback quietly refreshes when its selected project's activity changes and on
focus/reconnect. Keyed cards retain their component state. Unsaved notes,
selected investigation/fix fields, translations in progress and ticket dialogs
defer background replacement. A second check after the network response protects
a draft started while that request was in flight. A small pending-update action
allows refresh after the edit is finished. Searches and navigation are retained.

Account changes remount the private view by account identity; ordinary session
checks for the same account preserve drafts. Logout/401 stops polling and resets
the favicon. Browser-local notification state is separate from the canonical
database investigation notes/statuses and manual resolution.

Verification: exact-ID, late-commit, net-zero-count, incomplete-coverage,
cross-tab/persistence, prototype-key and poller lifecycle unit tests; SQL tests
for project scope, deletion/resolution filters, integer precision and bounds;
mocked Chrome arrival, visible-card acknowledgement, draft/request races,
search, project isolation, background/focus, reconnect, expiry and mobile checks.
No synthetic production feedback or reactions are used.

Casey authorized deployment of activity and the pending account/session login
replacement on 2026-11-17, explicitly declining a separate migration rehearsal.
Normal checks, a private rollback backup and post-deployment verification remain
part of the release. See `PORTAL_AUTH.md` for authentication behavior.

Production verification (2026-11-18 00:06 UTC): deployed together with the
account/session login. The authenticated navigation ID/completeness contract
and both public favicon asset hashes were verified on the live server. The
public login page renders correctly; live player arrivals were not fabricated.
Arrival/dot lifecycle, draft preservation, hidden/focus/reconnect, expiry and
responsive behavior were verified with mocked browser/API data before release.
Native full API tests/race/vet passed. See PORTAL_AUTH.md for deployment,
backup and rollback evidence. Browser suspension can delay background polling;
resume/focus catches up. New activity is browser-local and never resolves items.
