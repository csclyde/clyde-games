# Game Editor feedback integration

Live project metadata: ID `game-editor`, display name `Game Editor`. Existing projects are unchanged. No Steam/Reddit/Discord sources, crash hooks or crash configuration were added. No test feedback was posted. This is a shared editor-feedback bucket reusable across game projects; put useful game/editor context in the message without including private machine paths or credentials.

Submit to `POST https://api.clyde.games/feedback`, `Content-Type: application/json`:

```json
{"Project":"game-editor","Message":"Describe the editor problem and reproduction steps","Category":"bug","Rating":3,"Env":"development","Platform":"windows","Build":"2026-11-16 12:00:00"}
```

The routing field is **`Project`, not `Game`**. The current API ignores unknown `Game` fields. The integration must require a nonempty message and this exact project ID, even though the legacy ingestion handler does not strictly reject missing values. HTTP201 returns the stored record, including numeric `ID` and `Project`. Handle non-2xx and uncertain network outcomes explicitly; this legacy manual-submission endpoint has no client request-key dedup contract, so do not silently auto-retry an uncertain submission.

No token, cookie, API key or admin credential is required by this existing public client-ingestion route. Never embed administrative or MCP secrets. Browser calls should omit credentials; existing wildcard CORS supports uncredentialed POST/OPTIONS. Project ID is a routing claim, not an authentication boundary.

Other fields are optional. `Category` is free text (no server enum), stored in a255-byte TINYTEXT column; `bug`/`feedback` are useful client choices. `Rating` is an unsigned byte0–255;0–5 matches the existing portal presentation. `Build`, `Commit`, `Platform`, `Env` and optional existing opaque `PID` are TINYTEXT (255bytes each). `Message` uses MySQL TEXT (65535bytes); the handler has no separate explicit message validation limit. A2000-character client limit is a conservative recommendation, not a new API rule. Send actual build/context values or omit them; do not fabricate identifiers. Do not send `Resolved`, `ID`, source identity, triage or save fields. Source identity/triage supplied by clients is cleared by the backend.

No crash integration is included. The generic backend does not introduce project capability flags here; the editor should call only the feedback route and should not register crash submission hooks as part of this change.
