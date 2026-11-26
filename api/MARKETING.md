# Cursemark marketing

Open **Cursemark → Marketing → Beats** with an admin account. Create a Beat with a name, a brief that serves as the hook, and a Wishlist or Purchase call to action. Add a manual placement or use **Research and draft posts** to save a ranked batch of editable drafts. The editor labels uncertain rules **Hold**. Review the source, follow the community's rules for self-promotion and AI disclosure, and publish manually. After publishing, record the live URL, date and any details on the placement. The Steam link stays stable as the draft is edited. The editor can copy draft text with its `{steam_link}` placeholder filled in.

## OpenAI setup

Provision a key belonging to the Cursemark marketing OpenAI project. Set `CURSEMARK_OPENAI_API_KEY` in a root-owned, mode-0600 environment file loaded by the existing Clyde Games API service, then restart that service. Use the existing service secret management workflow. Production sets `CURSEMARK_OPENAI_MODEL` to `gpt-6.2-sol`. The API default is `gpt-6.1-terra` when the setting is absent. The API sends the key directly to OpenAI. The browser receives only a configuration flag. The model receives the Beat name, brief, and call to action. Enter only Beat text that can be shared with the provider. Research calls are initiated by admins and may incur OpenAI charges.

The Responses API request uses `web_search` and a structured schema with `store: false`. The API treats fetched content as evidence and has no posting or messaging tools. AI rule links must be returned in the search sources or URL citations. The corresponding date is when the application ran its research. A Candidate still needs human review. See the official [web search guide](https://developers.openai.com/api/docs/guides/tools-web-search), [structured outputs guide](https://developers.openai.com/api/docs/guides/structured-outputs), and [GPT-6.1 Terra features](https://developers.openai.com/api/docs/models/gpt-6.1-terra).

## Steamworks imports

In **Marketing → Steamworks Imports**, upload the full English-language UTM Analytics breakdown for **Cursemark (3219180)**, including the source, medium, campaign and content fields. The user importing must have the Steamworks permission to retrieve the file and portal admin access to upload it. Select the exact inclusive start and end dates used in Steamworks. If the export contains an App ID column, the API verifies it. Matching needs the full UTM identity from a Cursemark placement. Unmatched Steam rows are excluded. CSV files are processed in memory with a 4 MB limit and only matching aggregate counts and file hashes are persisted.

Use disjoint date ranges for additive results. To refresh a range, download a new export for the exact same dates. The new snapshot replaces that window, including clearing entries Steam no longer includes. Reimporting a file already seen for that range has no effect. A partial overlap is rejected without altering any results. Steam sometimes omits low-traffic rows. Blank and suppressed values are displayed as unavailable, while reported zeroes remain zero.

Results summarize all available Steamworks metrics in imported windows for manually recorded posts. The call to action informs the draft copy. Base-game Steam UTM reports do not attribute demo downloads. See the official [Steamworks UTM documentation](https://partner.steamgames.com/doc/marketing/utm_analytics).

## Release and rollback

The simplified editor adds a call-to-action column to `marketing_beats`. Existing Beat metadata, placements and imported reports remain stored. When a legacy Beat has no brief, the editor uses its old hook. If the old CTA wording gives a clear choice, the editor displays it. Otherwise an editor can choose one. Store a restorable backup before activation. Reverting the API binary and portal index leaves Beat and import history in place.
