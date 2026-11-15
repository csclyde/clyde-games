# Coordinated admin API authentication deployment

This source change closes the direct API bypass of the portal's Basic Auth.
Deploy the API, portal bundle and `nginx.conf` together. **Deploying just the API
will intentionally make the old portal fail with unauthorized responses.**

1. Generate a dedicated random `ADMIN_API_TOKEN` of at least 32 characters. It
   must differ from MCP and existing deployment credentials. Store it in the API
   service's protected environment; do not commit it or embed it in JavaScript.
2. Create a root-owned, restricted `/etc/nginx/snippets/admin-api-token.conf`:

   ```nginx
   proxy_set_header X-Admin-API-Token "REPLACE_WITH_DEDICATED_SECRET";
   ```

3. Install the updated portal Nginx configuration, keeping its existing Basic
   Auth and TLS setup. The `/api/` prefix proxies to loopback and injects the
   admin token **after** browser authentication. Metrics/rebuild retain their
   existing `packrat-deploy-token.conf` authentication paths. Cross-origin
   requests are rejected before proxying.
4. Build and deploy the API and portal bundle in a coordinated maintenance
   window; validate the Nginx configuration before reloading. Review the actual
   server configuration first; this repository cannot prove what is live.
5. Verify unauthenticated direct admin API reads/writes fail; verify portal
   feedback/crashes, save downloads, project settings and metrics still work.
   The two legacy resolve endpoints now accept **POST only**, not mutating GET.
6. Verify game feedback/crash/event/save submission and version lookup still
   work without these admin credentials. Existing website words endpoints remain
   public. This does not add ingestion throttling or otherwise change them.

No live server changes, credentials or Nginx reload are performed by this patch.
External scripts previously calling admin endpoints need explicit server-side
credentials, or should use the narrower MCP interface instead. The admin token
is broad and must not be handed to the MCP service or game clients.

## Secret persistence and rollback

The API makefile now treats the deployed `.env` as authoritative and never
replaces an existing file from the checkout. `api/scripts/prepare-env.sh` only
initializes a missing file (private mode 0600, without printing contents), before
stopping the service. This preserves **all** deployed secrets, not just the old
deployment token. Future configuration edits must be explicit in the protected
service environment or deployed configuration; changing checkout `.env` is no
longer a deployment mechanism. Service environment values take precedence over
`.env`. Configure and verify `ADMIN_API_TOKEN` / `MCP_UPSTREAM_TOKEN` before the
coordinated rollout. Missing tokens fail closed, but will interrupt portal/MCP
access rather than authorize requests.

Do not roll back to the old unauthenticated API while its admin routes are
publicly reachable. Keep edge restrictions on private admin routes and the new
authenticated proxy, or stop external API access during rollback. Prefer a
forward fix retaining API authentication. Roll back the portal/API/proxy as a
coordinated set only after confirming private routes remain inaccessible to
unauthenticated requests and public game ingestion remains available. Preserve
the deployed secrets throughout rollback.
