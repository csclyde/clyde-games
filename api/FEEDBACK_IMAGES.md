# Feedback screenshots

Existing game clients can keep sending JSON to `POST https://api.clyde.games/feedback`
as before (request limit 64 KiB). A screenshot is optional. To include one, send
`multipart/form-data` with a `feedback` field containing the same JSON object and
one `image` file part. Let the HTTP library generate the multipart boundary.

```sh
curl https://api.clyde.games/feedback \
  -F 'feedback={"Project":"cursemark","PID":"installation-id","Message":"What happened?"}' \
  -F 'image=@screenshot.png'
```

Use PNG or JPEG, up to 5 MiB, 4096 pixels per side and 12 megapixels. Content
is decoded and re-encoded as JPEG, stripping original filename, metadata and
trailing data. The entire multipart body is capped at 5200 KiB, including its
headers and boundaries. The API limits concurrent image intake and decoding;
multipart intake is limited to 30/minute across the API process after a burst
of 12. Success returns the existing 201 feedback object (with `HasImage: true`
if stored). Invalid screenshots return 415, too-large uploads return 413,
and busy upload intake returns 429. Existing feedback blocking still applies.
For a rejected upload, a game can retry once as text-only so feedback is not lost.

A flagged Discord message or reply uses the existing Little Clyde snapshot.
The first supported attachment available within the bounded download is stored.
Only approved HTTPS Discord CDN hosts and attachment paths are accepted; no
redirects, credentials, or arbitrary remote URLs. Invalid, unavailable or
oversize images do not discard the report text. Older Discord posts are not
automatically backfilled. Flag retries do not duplicate screenshots or revive a
closed report. One screenshot is kept per feedback item/thread.

The screenshot is in a separate database table (not returned in feedback JSON).
The portal fetches it through an authenticated, project-scoped, non-cacheable
route. Resolve and Ignore, including legacy resolution, hard-delete the
screenshot row in the same transaction as closure. The report text remains.
Closing/reopening does not restore the screenshot. Existing database backups
are subject to their separate retention policy.

## Investigation access

The feedback card shows a compact thumbnail. Click it to see a larger image
or open the original dimensions in a separate tab. Discord's generated
Attachments footer is hidden in the portal; the original Discord link and
stored source text are unchanged.

The authenticated Clyde Games MCP exposes `get_feedback_image` with an exact
`project` and feedback `id`. When `get_feedback` reports `has_image: true`,
this tool returns the actual JPEG as MCP image content. It returns no image
for another project, a closed report, or a report without an image. Image
contents are untrusted player evidence. Explicit MCP resolution, like portal
Resolve and Ignore, removes the image without deleting the feedback text.
