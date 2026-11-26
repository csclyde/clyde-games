# Cursemark Marketing

Open **Cursemark → Marketing** with a portal administrator account. Create a Beat with its brief and CTA. Attach targets, then save or edit placements in the Beat. The Steam tracking link is stable. Record publication details only after publishing elsewhere. Archiving a Beat, target, or placement preserves its links, research, and attributed results.

Research and draft writing take place in Codex. The portal and Marketing MCP store structured reports and edited drafts. An OpenAI key and server-side generation are unnecessary. Research is editable under **Targets → Edit research** and describes the exact public destination or creator channels. Review the saved rules before publishing. Private drafting alone does not trigger a Hold or insert disclosure text.

## Creator records

Use **New Target → Creator contact** to save a creator's name, private email, and primary platform. The creator address is available in the authenticated portal detail. Broad lists and Marketing MCP responses omit addresses. **Cursemark coverage and outreach** records the checked date, verified public examples, and categorized reply facts as a separate report. Updating general target research preserves this report. Save summarized facts only. Keep mail text, addresses, and private mail links out of evidence notes and external research.

Use **Import creators** to upload a private UTF-8 CSV with headers `name,email,platform,channel_urls,games_covered`. Platform values are `youtube`, `twitch` and `other`. Separate multiple channels or games with `|`. The upload accepts up to 200 rows and 512 KB. It skips duplicate addresses and returns counts.

## Steamworks imports

Upload the full English UTM Analytics breakdown for Cursemark, app 3219180, in **Steamworks Imports**. Include source, medium, campaign and content fields. Enter the exact inclusive dates. Imports match existing placement tracking links. A new file for the same dates replaces that window. Identical retries are idempotent. Partial overlaps are rejected. Missing values remain unavailable. Base-game UTM reports do not attribute demo downloads. [Steamworks UTM Analytics](https://partner.steamgames.com/doc/marketing/utm_analytics)

## Clyde Games MCP

Use the existing authenticated Clyde Games MCP. Pass `project: "cursemark"` on every call.

- `marketing_list_beats`, `marketing_get_beat`, `marketing_save_beat` list, read or save Beat briefs.
- `marketing_list_targets`, `marketing_get_target`, `marketing_save_target` manage destinations and creator records. Keep an existing creator email by omitting it from an edit.
- `marketing_set_target_attachment` attaches or detaches a target while keeping existing placements.
- `marketing_save_target_research` stores a sourced report from research done in Codex. `marketing_save_creator_evidence` stores the separate checked coverage or reply facts using its own version.
- `marketing_list_targets` includes platform and an impact summary. `marketing_get_target` adds `impact_evidence` and any matched `steam_attribution`. `marketing_save_target_impact` uses `project`, `target_id`, `version` from `impact_evidence.version`, and a replacement `observations` array (up to 24). Retain existing observations when changing a single entry. Each observation needs `metric`, `url`, `title`, `observed_on` (UTC `YYYY-MM-DD`), `value` (integer or `null`), and `scope` (`dedicated` or `shared`). Optional `comments` and `note` show context.
- `marketing_save_target_history` uses `project`, `target_id`, `version` from `history.version` (0 for the first saved record), and `history` with `outreach_status`, `activity_status`, `checked_on`, `sources` and an optional short `note`. Keep private correspondence and contact addresses out. For creators, use their separate coverage report and save `activity_status: "not_checked"` here. For communities, use authored prior posts or matching published placements and `outreach_status: "not_applicable"`.

When outreach was searched using a verified identity and yielded no result, save `none_found`. Save `identity_unverified` for name-only or ambiguous searches and `not_checked` for unreviewed history. The New badge marks a searched-for gap in coverage, prior Casey posting, or outreach. It is independent of impact and posting readiness. A creator can be a new contact who already covered Cursemark. Creator targets can be saved without a private email when a verified public channel is supplied. Check channel IDs and known handle aliases before adding a creator.

### Impact rubric

| Metric | Low (1) | OK (2) | Great (3) |
| --- | ---: | ---: | ---: |
| YouTube video or archive views (`youtube_views`) | 0–999 | 1,000–9,999 | 10,000+ |
| YouTube Shorts views (`youtube_short_views`) | 0–2,999 | 3,000–29,999 | 30,000+ |
| Reddit displayed net score (`reddit_score`) | Below 10 | 10–49 | 50+ |

These are editorial thresholds specific to each signal. The badge takes the best band from a single qualifying observation and uses the newest observation date to break a tie. Counts are lifetime snapshots on the reported UTC dates. Reposts, related platforms, comment counts and observations of shared audiences are never summed. The same video or Reddit post cannot be stored twice through URL aliases. Unknown counts stay null. A verified zero qualifies as Low. Mixed-game videos or streams are `shared` and their whole-item reach is shown for context without affecting the badge. Native Reddit scores can fluctuate.

For YouTube, first save confirmed Cursemark coverage on the creator record and match the exact video ID and format. Note sponsorship when disclosed. For Reddit, verify the exact permalink is an authored Cursemark post by `u/CLYDEgames` in the target subreddit. Use primary platform observations and check Cursemark's Steam app `3219180` or the verified demo when the game's identity needs disambiguation. An unrelated research example, follower count, self-reported conversion or a promotional thread's overall statistics do not qualify.

Steamworks attribution shows matched, non-overlapping report windows for published placements linked to the target by their stored UTM. It remains separate from the reach rating. Missing rows or fields stay unavailable. The report dates show a span and it can contain gaps.
- `marketing_save_placement`, `marketing_get_placement` manage manually authored drafts, add already-published details, and read tracking results.
- `marketing_set_archived` archives or restores a Beat, target, or placement.
- `marketing_list_steamworks_reports`, `marketing_import_steamworks_csv` manage report windows. Supply a base64 CSV up to 42 KiB through MCP and confirm the full export. The portal accepts larger files up to 4 MB.

List calls return up to 50 records. Continue with the last ID while `has_more` is true. Beat placement pages use the same ID cursor. Editing calls require the current version. Creator evidence starts at version 0. Publishing and sending remain manual.
