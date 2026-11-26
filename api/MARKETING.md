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
- `marketing_save_placement`, `marketing_get_placement` manage manually authored drafts, add already-published details, and read tracking results.
- `marketing_set_archived` archives or restores a Beat, target, or placement.
- `marketing_list_steamworks_reports`, `marketing_import_steamworks_csv` manage report windows. Supply a base64 CSV up to 42 KiB through MCP and confirm the full export. The portal accepts larger files up to 4 MB.

List calls return up to 50 records. Continue with the last ID while `has_more` is true. Beat placement pages use the same ID cursor. Editing calls require the current version. Creator evidence starts at version 0. Publishing and sending remain manual.
