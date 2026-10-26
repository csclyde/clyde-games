### CLYDE GAMES website

The admin **Metrics** tab queries MySQL directly and uses Chart.js for rendering.
It includes the 19 active Clyde Games Metabase questions imported on 2026-10-26;
archived questions and Metabase's sample e-commerce content are excluded. Levels
and Skills retain the dashboard grouping. The three standalone questions remain
in the API catalog but are hidden from the portal. SQL and visualization settings live in `api/models/metrics/`
and are embedded in the API binary. No Metabase service is needed by this page.

Metrics requests use the authenticated admin Nginx `/api/metrics` proxy, which
injects the existing private token from `packrat-deploy-token.conf`. Deploy the
updated Nginx configuration with the frontend and API. Results are cached for
five minutes, with at most two concurrent queries and a 45 second timeout.
Tables and CSV downloads are limited to 1,000 rows, explicitly indicated when
truncated. All projects preserves the original SQL; selecting a project scopes
every event reference, including baselines, to that project. The saved crash
question additionally retains its original Cursemark/unresolved filters.

Run `npm run build:admin` and `cd api && go test ./...` for verification.
`METRICS_LIVE_TEST=1 go test ./models -run TestMetricsLiveParity -v` also checks
the local MySQL results using SELECTs only. Its all-projects/Cursemark comparison
assumes Cursemark is the sole project with event data, as at migration time.

Packrat documentation deployment is available under **Admin → Tools → Rebuild Docs**.
The admin calls `/api/packrat/rebuild`; `admin/nginx.conf` proxies it to the Go
API on port 9990, behind the admin's HTTP basic authentication. Install that
configuration as the admin Nginx site configuration. Its private include,
`/etc/nginx/snippets/packrat-deploy-token.conf`, must contain
`proxy_set_header X-Packrat-Deploy-Token "<token>";`, using the same token as
`PACKRAT_DEPLOY_TOKEN` in the deployed API's `.env`. Keep the token out of Git.
Validate Nginx configuration before reloading it.

The API runs `/root/Packrat/scripts/deploy-docs.sh` with a 15 minute timeout.
The server needs Git access to the Packrat remote, Node.js 22.12 or newer, npm,
and write access to `/var/www/packrat`. Deployment pulls the checked out branch,
builds VitePress, and publishes only after the build succeeds. The admin shows
the command output on both success and failure.
