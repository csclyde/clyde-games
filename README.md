### CLYDE GAMES website

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
