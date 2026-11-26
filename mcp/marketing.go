package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type marketingProjectArgs struct {
	Project string `json:"project" jsonschema:"Explicit project, exactly cursemark"`
}
type marketingListArgs struct {
	marketingProjectArgs
	AfterID         int  `json:"after_id,omitempty" jsonschema:"Optional positive cursor from the last included ID"`
	IncludeArchived bool `json:"include_archived,omitempty"`
}
type marketingBeatArgs struct {
	marketingProjectArgs
	BeatID int `json:"beat_id"`
}
type marketingBeatPageArgs struct {
	marketingBeatArgs
	AfterID int `json:"after_id,omitempty" jsonschema:"Placement ID cursor, Beat result totals cover all records"`
}
type marketingTargetArgs struct {
	marketingProjectArgs
	TargetID int `json:"target_id"`
}
type marketingPlacementArgs struct {
	marketingBeatArgs
	PlacementID int `json:"placement_id"`
}
type marketingBeatWrite struct {
	marketingProjectArgs
	BeatID       int    `json:"beat_id,omitempty" jsonschema:"Omit to create; supply to change an existing Beat"`
	Version      uint   `json:"version,omitempty" jsonschema:"Required current version on update"`
	Name         string `json:"name"`
	Brief        string `json:"brief"`
	CallToAction string `json:"call_to_action" jsonschema:"wishlist or purchase"`
}
type marketingTargetWrite struct {
	marketingProjectArgs
	TargetID     int      `json:"target_id,omitempty" jsonschema:"Omit to create; supply to update"`
	Version      uint     `json:"version,omitempty"`
	Kind         string   `json:"kind" jsonschema:"destination or creator; existing kind cannot change"`
	Name         string   `json:"name"`
	URL          string   `json:"url,omitempty" jsonschema:"Public destination URL for a destination"`
	Email        string   `json:"email,omitempty" jsonschema:"Optional private address. Create with verified public channel URLs when no address is known. Omit on update to preserve. Never returned by this MCP."`
	Platform     string   `json:"platform,omitempty" jsonschema:"youtube, twitch or other for creators"`
	ChannelURLs  []string `json:"channel_urls,omitempty"`
	GamesCovered []string `json:"games_covered,omitempty"`
}
type marketingArchive struct {
	marketingProjectArgs
	Kind     string `json:"kind" jsonschema:"beat, target or placement"`
	ID       int    `json:"id"`
	BeatID   int    `json:"beat_id,omitempty" jsonschema:"Required for placement"`
	Version  uint   `json:"version"`
	Archived *bool  `json:"archived" jsonschema:"true archives; false restores"`
}
type marketingLinkArgs struct {
	marketingBeatArgs
	TargetID int   `json:"target_id"`
	Attached *bool `json:"attached"`
}
type marketingDraftWrite struct {
	marketingBeatArgs
	PlacementID   int     `json:"placement_id,omitempty" jsonschema:"Omit to add a draft or record"`
	TargetID      int     `json:"target_id,omitempty" jsonschema:"Optional attached target for a new draft; retained on edit"`
	Version       uint    `json:"version,omitempty"`
	Platform      string  `json:"platform"`
	Community     string  `json:"community"`
	AudienceFit   string  `json:"audience_fit,omitempty"`
	Timing        string  `json:"timing,omitempty"`
	Title         string  `json:"title,omitempty"`
	Copy          string  `json:"copy,omitempty"`
	AssetNotes    string  `json:"asset_notes,omitempty"`
	RuleEvidence  string  `json:"rule_evidence,omitempty"`
	RulesURL      string  `json:"rules_url,omitempty"`
	RulesChecked  string  `json:"rules_checked,omitempty"`
	Decision      string  `json:"decision" jsonschema:"hold or candidate"`
	PublishedAt   *string `json:"published_at,omitempty" jsonschema:"RFC3339 use only for a post already published externally"`
	PublishedURL  string  `json:"published_url,omitempty"`
	PublishedNote string  `json:"published_note,omitempty"`
}
type marketingExampleInput struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	Date       string `json:"date" jsonschema:"YYYY-MM-DD for this verified example"`
	Engagement string `json:"engagement,omitempty"`
	WhatWorked string `json:"what_worked,omitempty"`
}
type marketingResearchInput struct {
	Rules struct {
		Links         string `json:"links,omitempty"`
		SelfPromotion string `json:"self_promotion,omitempty"`
		AI            string `json:"ai,omitempty"`
		Flair         string `json:"flair,omitempty"`
		Cadence       string `json:"cadence,omitempty"`
	} `json:"rules"`
	Sources       []string                `json:"sources" jsonschema:"Verified public HTTPS URLs for the exact target, at most six"`
	Examples      []marketingExampleInput `json:"examples" jsonschema:"Recent dated public examples for this target, at most four"`
	Takeaway      string                  `json:"takeaway,omitempty"`
	Hold          bool                    `json:"hold"`
	HoldReason    string                  `json:"hold_reason,omitempty"`
	AIRestriction string                  `json:"ai_restriction,omitempty" jsonschema:"unspecified, final_content or assisted_submissions"`
	Games         []string                `json:"games_covered,omitempty"`
}
type marketingResearchWrite struct {
	marketingTargetArgs
	Version  uint                   `json:"version"`
	Research marketingResearchInput `json:"research"`
}
type marketingCoverageWrite struct {
	Status    string `json:"status" jsonschema:"covered, self_reported, no_result, inaccessible, not_checked"`
	CheckedAt string `json:"checked_at,omitempty"`
	Note      string `json:"note,omitempty" jsonschema:"Factual summary, no contact addresses or private links"`
	Items     []struct {
		Title  string `json:"title"`
		URL    string `json:"url"`
		Date   string `json:"date,omitempty"`
		Format string `json:"format" jsonschema:"video, short, stream, post, article or other"`
	} `json:"items"`
}
type marketingReplyWrite struct {
	Status    string `json:"status" jsonschema:"replied_to_outreach, inbound_first, bounced, no_reply_found, not_checked"`
	CheckedAt string `json:"checked_at,omitempty"`
	Date      string `json:"date,omitempty"`
	Note      string `json:"note,omitempty" jsonschema:"Useful facts only. Never include correspondence, addresses or private links"`
}
type marketingEvidenceWrite struct {
	marketingTargetArgs
	Version  uint                    `json:"version" jsonschema:"Read evidence version first; 0 creates the report"`
	Coverage *marketingCoverageWrite `json:"coverage,omitempty"`
	Reply    *marketingReplyWrite    `json:"reply,omitempty"`
}

type marketingImpactWrite struct {
	marketingTargetArgs
	Version      *uint `json:"version" jsonschema:"Read impact_evidence.version first; 0 creates the first snapshot"`
	Observations *[]struct {
		Metric     string `json:"metric" jsonschema:"youtube_views, youtube_short_views or reddit_score"`
		URL        string `json:"url" jsonschema:"Exact public content permalink. YouTube must match confirmed Cursemark coverage; Reddit must be a verified Casey-authored Cursemark post in this target subreddit"`
		Title      string `json:"title"`
		ObservedOn string `json:"observed_on" jsonschema:"UTC observation date YYYY-MM-DD"`
		Value      *int64 `json:"value" jsonschema:"Visible count or null for unavailable; only Reddit net score can be negative"`
		Comments   *int64 `json:"comments,omitempty" jsonschema:"Optional visible comment count"`
		Scope      string `json:"scope" jsonschema:"dedicated for Cursemark-only measurement; shared for mixed-game metrics shown for context only"`
		Note       string `json:"note,omitempty" jsonschema:"Optional sponsorship, snapshot or source qualification. No email addresses or private links"`
	} `json:"observations" jsonschema:"Replacement set (up to 24); keep unchanged observations when editing individual entries"`
}

type marketingHistoryWrite struct {
	marketingTargetArgs
	Version *uint `json:"version" jsonschema:"Read history.version first; 0 if a target has no history yet"`
	History *struct {
		OutreachStatus string   `json:"outreach_status" jsonschema:"contacted, inbound_first, none_found (searched using verified identity), identity_unverified, not_checked, or not_applicable (Reddit)"`
		ActivityStatus string   `json:"activity_status" jsonschema:"post_found, coverage_found, none_found or not_checked. Use not_checked for creators and store actual coverage with marketing_save_creator_evidence."`
		CheckedOn      string   `json:"checked_on" jsonschema:"UTC date YYYY-MM-DD"`
		Sources        []string `json:"sources" jsonschema:"Up to six public links. Never use mailbox links or private identifiers."`
		Note           string   `json:"note,omitempty" jsonschema:"Factual status summary without contact details, correspondence, or links"`
	} `json:"history"`
}
type marketingReportImport struct {
	marketingProjectArgs
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	ConfirmFull bool   `json:"confirm_full" jsonschema:"True only for a complete unfiltered Cursemark breakdown for the given inclusive dates"`
	CSVBase64   string `json:"csv_base64" jsonschema:"Base64 encoded Steamworks CSV up to 42 KiB. Use the admin portal for larger exports."`
}

func checkedMarketingPaths(project string, path string, ids ...int) (string, error) {
	if project != "cursemark" {
		return "", fmt.Errorf("choose project cursemark explicitly")
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		if id < 1 || uint64(id) > 4294967295 {
			return "", fmt.Errorf("invalid marketing record ID")
		}
		args[i] = id
	}
	return "/marketing/cursemark" + fmt.Sprintf(path, args...), nil
}
func marketingQuery(after int, archived bool) (string, error) {
	if after < 0 || uint64(after) > 4294967295 {
		return "", fmt.Errorf("invalid marketing cursor")
	}
	q := url.Values{}
	if after > 0 {
		q.Set("after_id", strconv.Itoa(after))
	}
	if archived {
		q.Set("include_archived", "1")
	}
	if len(q) == 0 {
		return "", nil
	}
	return "?" + q.Encode(), nil
}

func (b *backend) callMarketing(ctx context.Context, method, path, content string, body io.Reader) (*mcp.CallToolResult, any, error) {
	req, err := http.NewRequestWithContext(ctx, method, b.base+"/internal/mcp"+path, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("Content-Type", content)
	res, err := b.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marketing API unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode == 204 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: `{"ok":true}`}}}, nil, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("marketing API returned HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil || len(raw) > maxResponse || !json.Valid(raw) {
		return nil, nil, fmt.Errorf("marketing API response unavailable or over its limit")
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}

type marketingOperation struct {
	Method, Path string
	Body         any
}

func marketingTool[T any](s *mcp.Server, b *backend, name, description string, read bool, operation func(T) (marketingOperation, error)) {
	nondestructive := false
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: description, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: read, DestructiveHint: &nondestructive}}, func(ctx context.Context, _ *mcp.CallToolRequest, args T) (*mcp.CallToolResult, any, error) {
		op, err := operation(args)
		if err != nil {
			return nil, nil, err
		}
		var input io.Reader
		if op.Body != nil {
			raw, err := json.Marshal(op.Body)
			if err != nil {
				return nil, nil, err
			}
			input = bytes.NewReader(raw)
		}
		return b.callMarketing(ctx, op.Method, op.Path, "application/json", input)
	})
}

func addMarketingTools(s *mcp.Server, b *backend) {
	for _, list := range []struct{ name, path, description string }{
		{"marketing_list_beats", "/beats", "List Cursemark Beats in ID order, at most 50. Follow has_more using the last ID. Optionally include archived records."},
		{"marketing_list_targets", "/targets", "List Cursemark destinations and creator summaries, at most 50. Does not return addresses or report evidence. Follow has_more using the last ID."},
		{"marketing_list_steamworks_reports", "/reports", "List saved Cursemark Steamworks CSV report windows, at most 50. Read a Beat or placement to see its attributed results."},
	} {
		marketingTool(s, b, list.name, list.description, true, func(a marketingListArgs) (marketingOperation, error) {
			p, err := checkedMarketingPaths(a.Project, list.path)
			if err != nil {
				return marketingOperation{}, err
			}
			q, err := marketingQuery(a.AfterID, a.IncludeArchived)
			return marketingOperation{Method: "GET", Path: p + q}, err
		})
	}
	marketingTool(s, b, "marketing_get_beat", "Read a Cursemark Beat, attached target IDs and a page of placements with Steam UTM links and attributed results. Aggregates include historical placements.", true, func(a marketingBeatPageArgs) (marketingOperation, error) {
		p, err := checkedMarketingPaths(a.Project, "/beats/%d", a.BeatID)
		if err != nil {
			return marketingOperation{}, err
		}
		q, err := marketingQuery(a.AfterID, false)
		return marketingOperation{Method: "GET", Path: p + q}, err
	})
	marketingTool(s, b, "marketing_get_target", "Read one Cursemark target including saved research and creator coverage and outreach facts. Contact addresses are restricted to the portal.", true, func(a marketingTargetArgs) (marketingOperation, error) {
		p, e := checkedMarketingPaths(a.Project, "/targets/%d", a.TargetID)
		return marketingOperation{Method: "GET", Path: p}, e
	})
	marketingTool(s, b, "marketing_get_placement", "Read one existing draft or externally published placement and its stable Steam link and attribution.", true, func(a marketingPlacementArgs) (marketingOperation, error) {
		p, e := checkedMarketingPaths(a.Project, "/beats/%d/placements/%d", a.BeatID, a.PlacementID)
		return marketingOperation{Method: "GET", Path: p}, e
	})
	marketingTool(s, b, "marketing_save_beat", "Create a Cursemark Beat or update it with the current version. Supply name, brief and wishlist or purchase CTA.", false, func(a marketingBeatWrite) (marketingOperation, error) {
		if a.CallToAction != "wishlist" && a.CallToAction != "purchase" {
			return marketingOperation{}, fmt.Errorf("choose wishlist or purchase")
		}
		if a.BeatID == 0 {
			p, e := checkedMarketingPaths(a.Project, "/beats")
			return marketingOperation{Method: "POST", Path: p, Body: a}, e
		}
		if a.Version == 0 {
			return marketingOperation{}, fmt.Errorf("supply the current Beat version")
		}
		p, e := checkedMarketingPaths(a.Project, "/beats/%d", a.BeatID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_save_target", "Create or update an explicit Cursemark destination or creator. Use only owner-authorized contact data. Omit a creator's email on update to retain the current address. Saving does not run research or send messages.", false, func(a marketingTargetWrite) (marketingOperation, error) {
		if a.Kind != "destination" && a.Kind != "creator" {
			return marketingOperation{}, fmt.Errorf("choose destination or creator")
		}
		if a.TargetID == 0 {
			p, e := checkedMarketingPaths(a.Project, "/targets")
			return marketingOperation{Method: "POST", Path: p, Body: a}, e
		}
		if a.Version == 0 {
			return marketingOperation{}, fmt.Errorf("supply the current target version")
		}
		p, e := checkedMarketingPaths(a.Project, "/targets/%d", a.TargetID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_set_archived", "Archive or restore a Beat, target or placement with its current version. Preserves history, attachments, URLs and attribution.", false, func(a marketingArchive) (marketingOperation, error) {
		if a.Archived == nil || a.Version == 0 {
			return marketingOperation{}, fmt.Errorf("supply archived and the current version")
		}
		var p string
		var e error
		switch a.Kind {
		case "beat":
			p, e = checkedMarketingPaths(a.Project, "/beats/%d/archive", a.ID)
		case "target":
			p, e = checkedMarketingPaths(a.Project, "/targets/%d/archive", a.ID)
		case "placement":
			p, e = checkedMarketingPaths(a.Project, "/beats/%d/placements/%d/archive", a.BeatID, a.ID)
		default:
			return marketingOperation{}, fmt.Errorf("choose beat, target or placement")
		}
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_set_target_attachment", "Attach or detach a target from a Cursemark Beat. Detaching preserves all placements and attribution.", false, func(a marketingLinkArgs) (marketingOperation, error) {
		if a.Attached == nil {
			return marketingOperation{}, fmt.Errorf("choose attached true or false")
		}
		p, e := checkedMarketingPaths(a.Project, "/beats/%d/targets/%d", a.BeatID, a.TargetID)
		method := "DELETE"
		if *a.Attached {
			method = "PUT"
		}
		return marketingOperation{Method: method, Path: p}, e
	})
	marketingTool(s, b, "marketing_save_placement", "Save a manually authored Cursemark draft for an optional attached target, edit by current version, or record details of a post published externally. Does not generate, publish or send anything. On edit, pass the full draft and publication fields to retain them.", false, func(a marketingDraftWrite) (marketingOperation, error) {
		if a.Decision != "hold" && a.Decision != "candidate" {
			return marketingOperation{}, fmt.Errorf("choose hold or candidate")
		}
		if a.PlacementID == 0 {
			p, e := checkedMarketingPaths(a.Project, "/beats/%d/placements", a.BeatID)
			return marketingOperation{Method: "POST", Path: p, Body: a}, e
		}
		if a.Version == 0 {
			return marketingOperation{}, fmt.Errorf("supply the current placement version")
		}
		p, e := checkedMarketingPaths(a.Project, "/beats/%d/placements/%d", a.BeatID, a.PlacementID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_save_target_research", "Save a curated, sourced target report from research performed externally by Codex. Requires current target version, public links and dates for examples. Does not call a model, search or update outreach evidence.", false, func(a marketingResearchWrite) (marketingOperation, error) {
		if a.Version == 0 {
			return marketingOperation{}, fmt.Errorf("supply the current target version")
		}
		p, e := checkedMarketingPaths(a.Project, "/targets/%d/research", a.TargetID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_save_creator_evidence", "Save a creator's separate Cursemark coverage or summarized prior reply facts. Use verified public coverage links, statuses and dates. Keep addresses, correspondence and private links out. Omit sections to preserve them.", false, func(a marketingEvidenceWrite) (marketingOperation, error) {
		if a.Coverage == nil && a.Reply == nil {
			return marketingOperation{}, fmt.Errorf("provide coverage or reply facts")
		}
		p, e := checkedMarketingPaths(a.Project, "/targets/%d/creator-evidence", a.TargetID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_save_target_impact", "Save dated measured Cursemark observations for a target. Start with marketing_get_target for the current impact_evidence version and observations. Use one row per source permalink. Keep unknown counts null and mixed-game views shared. This never sends anything or updates creator replies, coverage or Steamworks attribution.", false, func(a marketingImpactWrite) (marketingOperation, error) {
		if a.Version == nil || a.Observations == nil || len(*a.Observations) > 24 {
			return marketingOperation{}, fmt.Errorf("supply the current impact version and up to 24 observations")
		}
		p, e := checkedMarketingPaths(a.Project, "/targets/%d/impact", a.TargetID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	marketingTool(s, b, "marketing_save_target_history", "Save independent prior Cursemark outreach and public activity statuses for a chosen target. A verified identity search with no results differs from an unverified identity. For Reddit use prior Casey posts only, not Gmail. Keep inbox links, messages and addresses out. This operation never contacts the target.", false, func(a marketingHistoryWrite) (marketingOperation, error) {
		if a.Version == nil || a.History == nil {
			return marketingOperation{}, fmt.Errorf("supply the current history version and dated facts")
		}
		p, e := checkedMarketingPaths(a.Project, "/targets/%d/history", a.TargetID)
		return marketingOperation{Method: "PUT", Path: p, Body: a}, e
	})
	nondestructive := false
	mcp.AddTool(s, &mcp.Tool{Name: "marketing_import_steamworks_csv", Description: "Import the complete authenticated Cursemark UTM breakdown for exact dates. Repeated files are idempotent and partial window overlaps are rejected. Requires confirmation and base64 CSV up to 42 KiB; use the portal for larger files.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &nondestructive, IdempotentHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a marketingReportImport) (*mcp.CallToolResult, any, error) {
		p, e := checkedMarketingPaths(a.Project, "/reports")
		if e != nil {
			return nil, nil, e
		}
		if !a.ConfirmFull || a.StartDate == "" || a.EndDate == "" || len(a.CSVBase64) > 57344 {
			return nil, nil, fmt.Errorf("confirm the full export and supply valid dates and a CSV up to 42 KiB")
		}
		payload, err := base64.StdEncoding.DecodeString(a.CSVBase64)
		if err != nil || len(payload) == 0 || len(payload) > 42<<10 {
			return nil, nil, fmt.Errorf("invalid base64 CSV or above the 42 KiB MCP limit")
		}
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		for key, value := range map[string]string{"start_date": a.StartDate, "end_date": a.EndDate, "full_breakdown": "yes"} {
			if err := w.WriteField(key, value); err != nil {
				return nil, nil, err
			}
		}
		f, err := w.CreateFormFile("csv", "steamworks.csv")
		if err != nil {
			return nil, nil, err
		}
		if _, err = f.Write(payload); err != nil {
			return nil, nil, err
		}
		if err = w.Close(); err != nil {
			return nil, nil, err
		}
		return b.callMarketing(ctx, "POST", p, w.FormDataContentType(), &body)
	})
}
