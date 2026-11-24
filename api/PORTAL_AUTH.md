# Portal users and persistent sessions

Production uses the approved account/session login (`PORTAL_AUTH_MODE=session`).
The configuration default remains `legacy` for compatibility; accounts are never
seeded automatically. Existing API/MCP tokens are unchanged. Future credential
bootstrap or authentication cutovers require explicit owner approval.

## Durable-login policy (deployed 2026-11-18)

Casey approved deployment after the renewable 365-day policy, temporary-password
limit, continued revocation and stolen-cookie exposure were explained. The
release was deployed and verified at 08:22 UTC on 2026-11-18. The original
read-only diagnosis found four eight-hour sessions, three already beyond the
old 30-minute idle limit. No credentials were reset or new live users created.

Normal accounts now use a persistent 365-day browser/database session, renewed
to another 365 days on authenticated use once its expiry is within 364 days.
There is no fixed lifetime limit or 30-minute idle sign-out for these accounts;
regular use can continue across years. A year without renewal, cleared/blocked
cookies or private browsing can still require login. Temporary-password sessions
remain non-renewing, capped at eight hours and gated from private portal data.
Old eight-hour normal sessions upgrade only while still valid under both the
original eight-hour and 30-minute idle rules; expired ones are never
revived and may require one new login after this deployment.

Cookie Max-Age/Expires match the database expiry and remain host-only,
Secure, HttpOnly, SameSite=Strict, Path=/ with the __Host- prefix. Renewal updates
existing rows only; it cannot recreate logged-out/revoked sessions. The opaque
token and its CSRF binding stay stable during ordinary renewal. Only the token
hash is stored in the database; no credential is put in browser localStorage.
Login/password-change still generate fresh entropy. Password/account changes,
administrator revocation and explicit logout retain their existing effects.
Sensitive account operations still require the acting administrator's password.

Two overlapping requests at the same timestamp reproduced a false 401 in the
original implementation: a legitimate zero-changed-row MySQL UPDATE was treated
as invalid authentication. The revised code makes timestamps/expiry monotonic
and rechecks persisted session/account state after updates, including zero-row
updates. Stale 401 responses no longer delete a newer login cookie or clear a
newer frontend authentication epoch. Cross-tab sign-out messages revalidate the
current shared cookie rather than blindly discarding a later login.

Approved security tradeoff: a stolen browser cookie has a much longer
potential lifetime, and activity can keep it alive until revoked. This is a
deliberate persistent-login policy, not compliance with a short absolute
timeout policy. Server-side expiry, hashed storage, per-request account/version
checks, existing Origin/CSRF protections and manual revocation remain enforced.
No MFA, refresh-token service, new tables or credential bootstrap was added.

Verification: disposable local MariaDB full API tests/vet, deterministic
concurrent-touch failure before/fix after, years of simulated renewal, invalid
expiry/legacy expiry, zero-row and logout/renewal races, temporary credentials,
cookie flags/lifetime, existing disable/reset/roles/revocation tests; isolated
Chrome profile restart and mocked stale-response/cross-tab checks, plus existing
auth/activity/history responsive browser regressions. Final source also passed
native Linux full API/security tests, race checks, vet, API and operator-CLI
builds using disposable MySQL databases. That final suite includes both the old
idle-expiry boundary and the new renewable policy. A Windows rerun hit a full
disk while linking; it was superseded by successful final native Linux checks.
Only the task's reproducible Go cache was removed; source/evidence were retained.

References: [MDN cookies](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Cookies)
and [OWASP session management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet).
The owner explicitly approved this policy before production deployment.

## Accounts and boundaries

Manual accounts only; no signup, invitation, email reset, or account deletion.
Admins can use existing portal controls and manage accounts in **Clyde Games /
Portal users**. Viewers can read existing privileged data but cannot mutate it.
Roles apply across the portal; this is not per-project access control. Sensitive
account changes require the acting admin's current password. Username is fixed.
There can be at most 200 accounts. Last enabled admin cannot be disabled or
demoted, including concurrent requests. Disabled users retain their identity.

Passwords use Argon2id (19 MiB, 2 iterations, 1 lane, random 16-byte salt),
15–128 characters. Temporary passwords have 256 bits of entropy and must be
changed before any portal data can be read. Resets, disable/role changes and
explicit session revocation invalidate existing sessions. Requests already in
flight may complete; subsequent requests recheck account state. No MFA or
self-service recovery is included.

Sessions: opaque 256-bit random token; only its SHA256 is stored. Host-only
`__Host-clyde-session` cookie, Secure, HttpOnly, SameSite=Strict, Path=/;
365-day rolling persistence for normal accounts and a fixed eight-hour limit
for temporary-password sessions (as described above). CSRF token is bound to the
session and held in browser memory. All mutations require exact portal Origin
and CSRF (login uses exact Origin without prior session). No session tokens are
stored in localStorage. Password/cookie/header/query/panic contents are excluded
from API request logs and all auth SQL logging is silent. Protect nginx/upstream
logs too: never log request bodies, cookies, credentials or response bodies.
Login is rate limited per IP and username/IP; two concurrent password hash jobs.
This is a single-process limiter, not a distributed abuse defense.

Existing Admin API token and Packrat token keep working for explicit non-browser
clients without ambient browser headers/cookies; MCP bearer authentication is
unchanged. Session user management cannot be done with a machine token. Public
game feedback/crash/save/event submissions retain their existing contracts.

## HTTP contract

All routes are behind the trusted portal gateway, use no-store responses and
return JSON. No authentication credentials are added to the MCP tool surface.

| Method and path | Contract |
| --- | --- |
| `GET /auth/session` | Session user, role, version, mandatory-change flag and CSRF; 401 if signed out. In legacy mode returns an explicit legacy admin descriptor only with the existing injected Admin token. |
| `POST /auth/login` | `{username,password}`; fresh secure cookie plus user/CSRF; generic 401 for unknown, disabled or wrong password; 429 for throttling. |
| `POST /auth/logout` | Session + CSRF; durable session deletion before cookie clear. A retried revoked/expired session returns 401 without a cookie deletion header, preventing stale responses from erasing a later login. |
| `POST /auth/password` | `{current_password,new_password}`; changes password, revokes old sessions, creates fresh session. If the password commits but session issuance fails, response explicitly directs sign-in again. |
| `GET /auth/users` | Admin session only; `{items:[...]}` never includes hashes. |
| `POST /auth/users` | `{username,role,current_password}`; new user and generated `temporary_password` returned once, never logged. |
| `PATCH /auth/users/:id` | `{version,current_password,role?,disabled?}`; optimistic concurrency, 409 for stale version/last-admin conflict. No-op succeeds without claiming session revocation. |
| `POST /auth/users/:id/reset-password` | `{version,current_password}`; generates a temporary password, revokes sessions and requires password change. Own account uses `/auth/password` instead. |
| `POST /auth/users/:id/revoke` | `{version,current_password}`; revokes all target sessions and increments version. |

Use `X-CSRF-Token` for every mutation after login; the browser fetch wrapper adds
it only to same-origin `/api/` requests. UI hides private views until successful
session inspection, hides them on 401, and reports unconfirmed logout honestly.
Account mutations are deliberately not silently retried after ambiguous errors:
refresh current state, then explicitly reset/change again if needed. No secret
redisplay endpoint exists.

## Additive schema and rehearsal

Four new tables: `portal_users`, `portal_sessions`, `portal_auth_controls`
(singleton ID 1 serializes account changes), `portal_auth_events` (actor,
subject, action, time, no secrets). No existing game records are updated or
deleted. Run migrations twice on a quiescent restored fresh backup first.

Use the existing private operator environment to supply `PORTAL_AUTH_DSN` to
`go run ./scripts/portal-auth -action rehearse`. It accepts only a loopback DSN
and a database named `portal_auth_rehearsal_...`; never pass secrets as command
arguments. It hashes every non-auth base table, complete rows and schema,
verifies identical hashes after each migration, and requires zero accounts,
sessions and auth events. It does not create/drop/restore a database. Restore
the fresh backup separately into an isolated database; keep it offline from
collectors. A schema-only migration is `-action migrate`.

`FEEDBACK_TEST_MYSQL_DSN` enables isolated integration tests. It must be a
database-less loopback DSN; tests create uniquely named disposable databases.
Run full API tests/vet and native Linux race tests, including
`TestPortalAuthMySQLLifecycle` and `TestPortalLastAdministratorMySQLConcurrency`.
Never point tests at analytics or create synthetic player records in production.

## Reviewable cutover order (not executed)

1. Record deployed hashes/config, take a fresh private database and file backup,
   verify restore, rehearse migration twice. Build/test exact staged API/admin.
   Keep MCP, bot, collectors and public site unchanged. Audit admin webroot for
   accidentally served private files before making the shell public.
2. Deploy the additive API first, then UI, in default `legacy` mode behind existing Basic Auth.
   Check legacy API/MCP and game submission regression tests; don't create
   synthetic production submissions. Preserve old bundles/binary for rollback.
3. Still retaining Basic Auth, configure **all three existing proxy blocks**
   (`/api/`, `/api/metrics`, `/api/packrat/rebuild`) to overwrite:
   `Host admin.clyde.games`, `X-Clyde-Portal 1`, `X-Forwarded-Proto https`,
   `X-Real-IP $remote_addr`. Include existing private admin-token snippet in all
   three (metrics/rebuild also retain their existing Packrat snippet). Set
   no-store for auth responses. Test `nginx -t` before reload. Do not expose
   these snippets, tokens, upstream ports or credential files in webroot.
4. **Obtain explicit cutover/credential-delivery approval.** With private DSN,
   `-action bootstrap -credential-name <new-name>.txt -allow-live` creates the
   first username `admin` only when no users exist. Linux-only credentials go
   to a new exclusive file under `/root/clyde-portal-secrets` (directory 0700,
   file 0600). No secret stdout. This is not run automatically by deployment.
5. Privately deliver that file to Casey over his authenticated SSH connection
   or an agreed password-manager channel; never paste into chat, logs, source,
   screenshots, artifacts or email. First-login password change is required.
   The operator file is temporary: Casey verifies password-manager storage
   before approved removal. New/reset user passwords appear masked once in
   the admin UI; share privately, clear after capture. There is no redisplay.
6. Enable `PORTAL_AUTH_MODE=session` with Basic Auth still active. Verify login,
   required password change, account management, viewer denial, CSRF, logout,
   expiration/revocation, and API/MCP compatibility. Casey verifies his own
   access. Only then, with the action-time authorization, remove Basic Auth
   from admin server and verify unauthenticated shell shows login while all
   privileged API requests return 401. No public signup. Do not remove Basic
   Auth from unrelated sites or alter API/MCP credentials.

## Recovery and rollback

Keep a privileged SSH session open during cutover. If login is unavailable,
**restore Basic Auth first** and validate nginx before changing API auth mode
back to legacy or reverting the binary/UI. Reverting to legacy while the proxy
is public would expose privileged APIs through its injected machine token.
Restore all three proxy blocks from the reviewed backup, not just `/api/`.
Retain additive auth tables; don't restore the whole analytics backup over live
player arrivals and don't drop tables merely to roll back code.

CLI `-action recover-admin -username admin -credential-name <new-name>.txt
-allow-live` requires explicit operator approval, resets only an existing admin,
enables it, revokes its sessions and forces a new password. It cannot create a
new hidden administrator. No secret is printed. Credential file is written
before the database transaction and retained on failure: if outcome is
uncertain, inspect private account/audit state before retry; never blindly
rerun or overwrite the file. Losing all SSH/operator access requires ordinary
server recovery, not a public reset endpoint.

Expired session rows and the secret-free audit log are retained; revocation
deletes affected session rows. Add bounded cleanup only if growth warrants it.
No investigation statuses, player records, ticket closures or collector
baselines are changed by this feature.

## Preparation verification (2026-11-17; not deployed)

Full API tests passed on Windows with disposable loopback MariaDB 11.4.9,
including auth lifecycle/concurrent last-admin protection and existing
feedback/crash/configuration/metrics integration tests. API and MCP vet and
MCP tests passed. Admin production build and mocked Chrome authentication,
notes, crash-health and admin regression tests passed at 1440/390/320 pixels.
The auth UI tests never contact production and screenshots contain no usable
credentials. An official portable MariaDB archive was checksum verified and
run privately without a Windows service.

A local synthetic database backup was restored into a separate copy; the auth
rehearsal command ran twice (four migration applications), preserving all 7
fixture rows, duplicate/null/binary evidence, historical/detailed crash fields,
resolved flags and all 3 original schemas. No accounts were created by that
rehearsal. This is **not** a rehearsal of the production backup.

Follow-up preparation completed against the existing server without new source
transfer or deployment:

- Native Linux MCP race tests passed (`go test -p=1 -race -count=1 ./...`,
  Go 1.25.5). Every existing server MCP source/dependency hash matched this
  checkout. This does not test the new authentication API.
- A fresh transactional backup at `2026-11-17T07:54:45Z` was restored into
  `portal_auth_rehearsal_20261117075445`: 17 tables, 1,129,206 rows. Snapshot
  counts: 7,246 feedback rows (7,245 resolved), 3,310 crash rows (3,300 resolved),
  31 saves and 13 chronological notes. All 3,300 legacy crash rows and their
  88,212 reported occurrences remain. The existing read-only fingerprint tool
  completed successfully. No authentication migration ran on this copy.
- Backup:
  `/root/clyde-portal-users-test-20261117/backup/analytics-auth-preparation-20261117T075445Z.sql.gz`,
  8,076,883 bytes, SHA256
  `05ac1d5be0e55d1192e441ce4e321fa404d224c60d2a062f42a68289d7fcae1d`.
  Backup directory is 0700; dump is 0600. It remains on the server.
- Existing nginx configuration passed `nginx -t`; no start/reload occurred.
  The new proxy proposal has not been syntax-tested on Linux.
- Final live checks: goweb/MCP/MySQL/nginx active; admin still returns 401;
  direct `/auth/session` returns 404; protected feedback reads return 401;
  public Packrat version returns 200. API binary SHA remains
  `a5755b290f071ae5a4e3d21c404d914cc335ecc1e4fb3d102b457e2003c74570`.
  Mode remains legacy; live auth-table count is zero; no live credential
  directory exists.

Bootstrap durability review added Linux directory/parent-directory fsync after
the exclusive 0600 credential-file write and before any account transaction.
Failure leaves the private file for inspection and does not attempt the account
operation. Linux-only tests cover insecure directories and symlink rejection;
these checks are not yet executed on Linux. Existing portable CLI tests pass.

Blocked: automated approval review rejected transfer of the private API/MCP
test-source archive to the private server test directory because isolated-test
authorization did not explicitly authorize source disclosure to clyde.games.
The upload never executed; subsequent checks confirmed there is no archive or
new source directory there. Authorization history was checked; no relevant new
approval was found, and the upload was not retried or routed another way.

Before continuing the blocked work, obtain Casey's explicit approval to upload
the credential-free API/MCP authentication test source and rehearsal helper to
`/root/clyde-portal-users-test-20261117` on clyde.games solely for isolated
tests and restored-copy migration rehearsal. This does not authorize a live
cutover. Pending: native Linux auth-API tests/race/vet, Linux credential
filesystem tests, four-pass additive auth migration on the restored production
copy, new nginx-proposal validation and eventual staged/live cutover checks.
No live credentials, data, resolutions or security configuration were changed.

## Subsequent deployment request (2026-11-17)

Casey subsequently authorized finishing and deploying the pending login/user
work together with automatic sidebar activity and the Y/dot favicon, explicitly
declining a separate rehearsal. That instruction supersedes the proposed
rehearsal step above. Normal tests and rollback backups remain in the prepared
release helper.

The new release upload was submitted once using that new authorization.
Automatic review rejected it before execution: general deployment permission
was not accepted as explicit permission to disclose the private source payload
to clyde.games. A later history check found no additional relevant permission;
the action was not retried or routed another way. The release directory exists
but contains no archive/source. No migration, account bootstrap, API install,
nginx change or credential creation ran. Live Basic Auth and legacy mode remain.

Ready local release:
`C:/Users/casey/Documents/Codex/2026-11-15/task-2/portal-activity/portal-release.tar.gz`
(344,986 bytes, 138 files), SHA256
`54c5b0b983f7765a9190af976395c425647beab3a8a4d42644ce816965828fe8`.
It includes the API/admin source, tested UI artifacts and deployment helper;
credentials, environment files, credential-bearing `db.go`, node_modules and a
separate rehearsal helper are excluded. See `PORTAL_ACTIVITY.md`.

The minimal remaining approval is explicit permission to upload this private
API/admin source and release helper to clyde.games and deploy the two portal
changes, replacing Basic Auth with the new initial admin login without a
separate rehearsal. Then run native checks, take a private rollback backup,
apply the additive auth migration, create/deliver credentials privately and
verify the authorized live cutover. No source transfer or cutover is claimed.

## Verified production cutover (2026-11-18)

The owner explicitly approved replacing Basic Auth on admin.clyde.games with
account/session login and creating initial admin credentials in a private file.
The previously rejected cutover was retried with that new approval and completed
at 00:05:53 UTC; live verification passed at 00:06:03 UTC. This supersedes the
historical blocked/pending statements above. No separate rehearsal ran, as the
owner requested. Native Linux full API tests, race, vet, API build and auth CLI
build all passed against disposable loopback MySQL test databases before cutover.
Existing local Chrome auth/activity/notes/crash/cleanup responsive checks passed.

Four additive auth tables were installed and one initial admin account created.
The temporary password must be changed at first human sign-in. Loopback and
public HTTPS checks verified initial login, mandatory-change access restrictions,
Secure/HttpOnly/SameSite=Strict cookies, session readback, logout and revocation.
The live login page was also checked visually without entering credentials.
Generated credentials were not printed; the root-only server file is under
/root/clyde-portal-secrets (0700 directory, 0600 file). A separate owner-only
Windows Temp handoff is restricted to Casey and SYSTEM, outside source control.
No temporary password contents are in this document or deployment logs.

Installed API SHA256:
7c29b1960e83b4d961cf7cc64dda53502b3c4dcebf6fb643808bda1bd945a7d2
Admin assets: bundle-0456c7e1a622.js / bundle-cf83639cb5d5.css.
All four services (goweb, clyde-games-mcp, mysql, nginx) are active. Public login
shell is 200; unauthenticated portal projects/metrics and direct feedback reads
are 401; public Packrat version is 200. Existing API/MCP tokens are unchanged;
MCP tools/list still returns 27 tools. Legacy crash history and existing source/
cutoff settings were verified unchanged. No player test submissions, reactions,
ticket or resolution changes occurred. Existing tabs need one reload for the app.

Rollback data and helper remain under /root/clyde-portal-release-20261117.
The transactional pre-cutover dump is backup/analytics-before-auth.sql.gz,
8,143,725 bytes, SHA256
432146087dfb337261c31fb44891564aa7ed0b167ef8cf955743122974d317f3.
Rollback restores Basic Auth first, then the previous API/environment/static
index; it retains auth tables and records and never rewinds player data. Do not
rerun bootstrap or restore the entire database blindly. Inspect deployment.json,
verification.json, bootstrap-plan.json and any rollback journal before recovery.
Use the documented recover-admin CLI for deliberate existing-admin recovery.


## Persistent-login deployment record (2026-11-18)

API SHA256: `376255ca766715a12cf7d23745a2e683f7146d2ce4c32ae451be889e2b3a166e`.
Admin: `bundle-e9244a4fbb1c.js` / `bundle-d7af0ced3de9.css`.
Release, native test logs, backup and rollback helper: `/root/clyde-portal-persistent-20261118`.
Backup: `backup/analytics-before-auth.sql.gz`, 8,197,711 bytes, SHA256
`0786c524e7510634a8c6878bfc556ad23d470e11610f6ec78cd7aabe83831a96`.

Live checks verified the installed binary/artifact hashes, healthy API/MCP/
MySQL/nginx, unchanged machine tokens and source/cutoff settings, 27 MCP tools,
60 existing history records ordered newest first, preserved legacy crash data,
and unauthorized/stale auth responses without cookie deletion. The public
sign-in page also rendered correctly. Credential-based lifetime and browser
restart checks used only disposable fixtures; no live account credentials were
used. No player test records, reactions, resolutions, commits or pushes.

Rollback restores the previous API and static index without restoring the
database or resetting accounts. Already extended session expiries remain in
the database: a full policy rollback may additionally require deliberate
per-user revocation, rather than silently shortening or deleting every session.
Existing tabs need one reload for the stale-response frontend fix.
