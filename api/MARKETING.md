# Cursemark marketing

Open **Cursemark → Marketing → Targets** with an admin account. Add or edit public destinations. Use **Update Research** on a target to save dated official rules and recent, linked game promotion examples. Engagement is shown only when available. A **Hold** marks unclear or restricted posting. Updating the target URL retains the old research and requires a refresh before drafting for the new destination.

In **Beats**, create a Beat with a name, a brief that serves as the hook, and a Wishlist or Purchase call to action. Attach any catalog targets you want to use and choose **Generate Post** per target. Drafting uses the saved research, and research older than 90 days puts the draft on Hold. Removing a target from a Beat preserves its posts and results. Placements are editable and publication is manual. After publishing, record the live URL, date and details on the placement. The Steam link stays stable as the draft is edited. The editor can copy draft text with its `{steam_link}` placeholder filled in.

## OpenAI setup

Provision a key belonging to the Cursemark marketing OpenAI project. Set `CURSEMARK_OPENAI_API_KEY` in a root-owned, mode-0600 environment file loaded by the existing Clyde Games API service, then restart that service. Use the existing service secret management workflow. Production sets `CURSEMARK_OPENAI_MODEL` to `gpt-6.2-sol`. The API default is `gpt-6.1-terra` when the setting is absent. The API sends the key directly to OpenAI. The browser receives only a configuration flag. The model receives the Beat name, brief, and call to action. Enter only Beat text that can be shared with the provider. Research calls are initiated by admins and may incur OpenAI charges.

Target research uses the Responses API with `web_search`, a structured schema, and `store: false`. Only an explicit **Update Research** runs web search. Only an explicit **Generate Post** drafts for an attached target using the Beat and saved research. Sources must appear in the search evidence and belong to the relevant destination. The checked date records when the service ran the research. A Candidate still needs human review. See the official [web search guide](https://developers.openai.com/api/docs/guides/tools-web-search) and [structured outputs guide](https://developers.openai.com/api/docs/guides/structured-outputs).

## Steamworks imports

In **Marketing → Steamworks Imports**, upload the full English-language UTM Analytics breakdown for **Cursemark (3219180)**, including the source, medium, campaign and content fields. The user importing must have the Steamworks permission to retrieve the file and portal admin access to upload it. Select the exact inclusive start and end dates used in Steamworks. If the export contains an App ID column, the API verifies it. Matching needs the full UTM identity from a Cursemark placement. Unmatched Steam rows are excluded. CSV files are processed in memory with a 4 MB limit and only matching aggregate counts and file hashes are persisted.

Use disjoint date ranges for additive results. To refresh a range, download a new export for the exact same dates. The new snapshot replaces that window, including clearing entries Steam no longer includes. Reimporting a file already seen for that range has no effect. A partial overlap is rejected without altering any results. Steam sometimes omits low-traffic rows. Blank and suppressed values are displayed as unavailable, while reported zeroes remain zero.

Results summarize all available Steamworks metrics in imported windows for manually recorded posts. The call to action informs the draft copy. Base-game Steam UTM reports do not attribute demo downloads. See the official [Steamworks UTM documentation](https://partner.steamgames.com/doc/marketing/utm_analytics).

## Release and rollback

The simplified editor adds a call-to-action column to `marketing_beats`. Target support adds two catalog and attachment tables plus an optional placement target ID. Existing Beat metadata, placements and imported reports remain stored. When a legacy Beat has no brief, the editor uses its old hook. If the old CTA wording gives a clear choice, the editor displays it. Otherwise an editor can choose one. Store a restorable backup before activation. Reverting the API binary and portal index leaves Beat, target, placement and import history in place.
