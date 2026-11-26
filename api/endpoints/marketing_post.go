package endpoints

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const marketingPostInstructions = `Write one private, editable Cursemark draft for the single attached target. For a creator, write a pitch draft about their public coverage without claiming prior correspondence or consent to contact. Use the Beat brief as the hook and its Wishlist or Purchase call to action only when permitted by the saved research. Use saved research as evidence for format and timing. Treat dated examples as observations, never as permission. The editor personally rewrites and finalizes the public submission. Do not add an AI disclosure to this private draft. Leave any applicable disclosure or AI restriction in the separate saved rule evidence for the editor to apply when publishing. Follow the saved self-promotion, routing, flair, cadence, approval and link rules. If research says Hold or permission is unclear, produce a conservative draft for review and omit the CTA and links. If the rules clearly permit a Steam link and the CTA, use the literal {steam_link} placeholder. Include a title, copy, suggested timing and optional format notes in asset_notes. Do not search, contact, disclose an email address, claim permission or impersonate a community member.`

var errMarketingAttachmentChanged = errors.New("Beat target attachment changed")

func generateMarketingPost(c *gin.Context) {
	beatID, ok := marketingID(c, "beat")
	if !ok {
		return
	}
	targetID, ok := marketingID(c, "target")
	if !ok {
		return
	}
	var in struct {
		Version uint `json:"version"`
	}
	if !marketingJSON(c, &in) {
		return
	}
	db := marketingDB(c)
	beat, err := readMarketingBeat(db, beatID)
	if err != nil {
		marketingDBError(c, err)
		return
	}
	if beat.Version != in.Version {
		marketingError(c, 409, "The Beat changed. Reload before drafting.")
		return
	}
	draft := marketingBeatDetails(beat).marketingBeatDraft
	if !draft.validCTA() || strings.TrimSpace(draft.Brief) == "" {
		marketingError(c, 400, "Add a brief and choose Wishlist or Purchase before drafting.")
		return
	}
	target, err := readTarget(db, targetID)
	if err != nil {
		marketingTargetError(c, err)
		return
	}
	if err := db.First(&models.MarketingBeatTarget{}, "beat_id = ? AND target_id = ?", beatID, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			marketingError(c, 409, "Attach this target to the Beat first.")
			return
		}
		marketingDBError(c, err)
		return
	}
	research := targetDetails(target).Research
	researchURL := target.URL
	platform := ""
	destination := map[string]any{"name": withoutContact(target.Name), "url": target.URL, "kind": "destination"}
	if target.IsCreator {
		person, err := readMarketingCreator(db, targetID)
		if err != nil {
			marketingDBError(c, err)
			return
		}
		fields := creatorFields(person)
		publicLinks := []string{}
		for _, link := range fields.ChannelURLs {
			if clean, ok := creatorChannel(link); ok {
				publicLinks = append(publicLinks, clean)
			}
		}
		researchURL = creatorResearchKey(person)
		platform = fields.Platform
		destination = map[string]any{"kind": "creator", "platform": platform, "public_channel_urls": publicLinks}
	}
	if research == nil || target.CheckedAt == nil || target.ResearchURL != researchURL {
		marketingError(c, 409, "Run Update Research for this destination before drafting.")
		return
	}
	if time.Since(*target.CheckedAt) > 90*24*time.Hour {
		research.Hold = true
		research.HoldReason = "Saved research is over 90 days old. Update research and confirm the rules before posting."
	}
	key, ok := startMarketingAI(c)
	if !ok {
		return
	}
	defer marketingGeneration.Unlock()
	cleanResearchText(research)
	draft.Name = withoutContact(draft.Name)
	draft.Brief = withoutContact(draft.Brief)
	input := map[string]any{"beat": draft, "target": destination, "saved_research": research,
		"research_date": target.CheckedAt.UTC().Format(marketingDay)}
	data, _, err := marketingAsk(c.Request.Context(), key, "cursemark_target_post", marketingPostInstructions, input,
		marketingObject(marketingTextFields("title", "copy", "timing", "asset_notes")), false)
	if err != nil {
		marketingError(c, 502, "OpenAI did not return a usable draft. Existing placements were kept.")
		return
	}
	var post models.MarketingPlacementDraft
	if json.Unmarshal(data, &post) != nil || strings.TrimSpace(post.Copy) == "" {
		marketingError(c, 502, "OpenAI returned an incomplete draft. Existing placements were kept.")
		return
	}
	if !target.IsCreator {
		u, _ := url.Parse(target.URL)
		platform = strings.TrimPrefix(u.Hostname(), "www.")
	}
	post.Platform = platform
	post.Community = target.Name
	post.AudienceFit = research.Takeaway
	post.RuleEvidence = placementRules(research)
	if target.IsCreator && post.RuleEvidence == "" {
		post.RuleEvidence = "Based on publicly sourced creator coverage. Contact and sending are manual."
	}
	post.Decision = "hold"
	if len(research.Sources) > 0 {
		post.RulesURL = research.Sources[0]
		post.RulesChecked = target.CheckedAt.UTC().Format(marketingDay)
		if !research.Hold {
			post.Decision = "candidate"
		}
	}
	if !checkMarketingDraft(&post) {
		marketingError(c, 502, "OpenAI returned an invalid draft. Existing placements were kept.")
		return
	}
	placement := models.MarketingPlacement{Project: marketingProject, BeatID: beatID, TargetID: &targetID,
		Version: 1, UTM: marketingToken(), AI: true, SourceCited: post.RulesURL != "", MarketingPlacementDraft: post}
	err = db.Transaction(func(tx *gorm.DB) error {
		current, err := readMarketingBeat(tx.Clauses(clause.Locking{Strength: "UPDATE"}), beatID)
		if err != nil {
			return err
		}
		fresh, err := readTarget(tx.Clauses(clause.Locking{Strength: "UPDATE"}), targetID)
		if err != nil {
			return err
		}
		link := tx.First(&models.MarketingBeatTarget{}, "beat_id = ? AND target_id = ?", beatID, targetID)
		if errors.Is(link.Error, gorm.ErrRecordNotFound) || current.Version != beat.Version || fresh.Version != target.Version {
			return errMarketingAttachmentChanged
		}
		if link.Error != nil {
			return link.Error
		}
		return tx.Create(&placement).Error
	})
	if errors.Is(err, errMarketingAttachmentChanged) {
		marketingError(c, 409, "The Beat, target or attachment changed while drafting. No post was saved.")
		return
	}
	if err != nil {
		marketingDBError(c, err)
		return
	}
	c.JSON(201, marketingPost{MarketingPlacement: placement, SteamURL: marketingUTM(placement)})
}

func placementRules(research *marketingResearch) string {
	rules := []string{research.HoldReason}
	for _, rule := range []struct{ label, value string }{
		{"Links", research.Rules.Links}, {"Self-promotion", research.Rules.SelfPromotion}, {"AI", research.Rules.AI},
		{"Flair", research.Rules.Flair}, {"Cadence", research.Rules.Cadence},
	} {
		if rule.value != "" {
			rules = append(rules, rule.label+" – "+rule.value)
		}
	}
	evidence := []rune(strings.TrimSpace(strings.Join(rules, "\n")))
	if len(evidence) > 2500 {
		evidence = append(evidence[:2499], '…')
	}
	return string(evidence)
}
