package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var plankaWriteMu sync.Mutex
var plankaIDPattern = regexp.MustCompile(`^[0-9]+$`)
var plankaKeyPattern = regexp.MustCompile(`^[A-Za-z0-9:_-]{1,160}$`)

const plankaLimit = 2 << 20

type mcpPlankaCard struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	BoardID     string `json:"boardId"`
	ListID      string `json:"listId"`
}
type mcpPlankaSession struct {
	base, token string
	client      *http.Client
}

func (s *mcpPlankaSession) call(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Planka unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Planka returned HTTP %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, plankaLimit+1))
	if err != nil || len(raw) > plankaLimit {
		return fmt.Errorf("Planka response exceeded limit or could not be read; no partial result")
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid Planka JSON")
	}
	return nil
}
func newMCPPlanka(ctx context.Context) (*mcpPlankaSession, error) {
	base, user, pass, err := plankaConfig()
	if err != nil {
		return nil, err
	}
	s := &mcpPlankaSession{base: base, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	var login struct {
		Item string `json:"item"`
	}
	if err = s.call(ctx, "POST", "/api/access-tokens", gin.H{"emailOrUsername": user, "password": pass}, &login); err != nil {
		return nil, err
	}
	if login.Item == "" {
		return nil, fmt.Errorf("Planka returned no token")
	}
	s.token = login.Item
	return s, nil
}

type mcpPlankaBoardResponse struct {
	Item     plankaBoard `json:"item"`
	Included struct {
		Lists []plankaItem    `json:"lists"`
		Cards []mcpPlankaCard `json:"cards"`
	} `json:"included"`
}

func (s *mcpPlankaSession) inventory(ctx context.Context) ([]plankaProject, []mcpPlankaCard, error) {
	var projects struct {
		Items    []plankaProject `json:"items"`
		Included struct {
			Boards []plankaBoard `json:"boards"`
		} `json:"included"`
	}
	if err := s.call(ctx, "GET", "/api/projects", nil, &projects); err != nil {
		return nil, nil, err
	}
	if projects.Items == nil || projects.Included.Boards == nil {
		return nil, nil, fmt.Errorf("incomplete Planka project inventory")
	}
	if len(projects.Included.Boards) > 100 {
		return nil, nil, fmt.Errorf("Planka inventory exceeds 100 boards; no partial result")
	}
	cards := []mcpPlankaCard{}
	for i := range projects.Items {
		projects.Items[i].Boards = []plankaBoard{}
	}
	for _, b := range projects.Included.Boards {
		if !plankaIDPattern.MatchString(b.ID) {
			return nil, nil, fmt.Errorf("invalid upstream board ID")
		}
		var br mcpPlankaBoardResponse
		if err := s.call(ctx, "GET", "/api/boards/"+b.ID, nil, &br); err != nil {
			return nil, nil, err
		}
		if br.Included.Lists == nil || br.Included.Cards == nil {
			return nil, nil, fmt.Errorf("incomplete Planka board inventory")
		}
		b.Lists = br.Included.Lists
		for i := range projects.Items {
			if projects.Items[i].ID == b.ProjectID {
				projects.Items[i].Boards = append(projects.Items[i].Boards, b)
			}
		}
		cards = append(cards, br.Included.Cards...)
		if len(cards) > 10000 {
			return nil, nil, fmt.Errorf("Planka inventory exceeds 10000 tickets; no partial result")
		}
	}
	return projects.Items, cards, nil
}
func validPlankaDestination(projects []plankaProject, board, list string) bool {
	for _, p := range projects {
		for _, b := range p.Boards {
			if b.ID == board {
				for _, l := range b.Lists {
					if l.ID == list {
						return true
					}
				}
			}
		}
	}
	return false
}
func plankaOutput(c *gin.Context, v any) {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) > plankaLimit {
		c.JSON(502, gin.H{"error": "Planka inventory exceeds response safety limit; no partial result"})
		return
	}
	c.Data(200, "application/json", raw)
}
func plankaFailure(c *gin.Context, err error) { c.JSON(502, gin.H{"error": err.Error()}) }
func registerMCPPlanka(g *gin.RouterGroup) {
	g.GET("/planka/boards", func(c *gin.Context) {
		s, e := newMCPPlanka(c.Request.Context())
		if e != nil {
			plankaFailure(c, e)
			return
		}
		p, _, e := s.inventory(c.Request.Context())
		if e != nil {
			plankaFailure(c, e)
			return
		}
		plankaOutput(c, p)
	})
	g.GET("/planka/tickets", func(c *gin.Context) {
		s, e := newMCPPlanka(c.Request.Context())
		if e != nil {
			plankaFailure(c, e)
			return
		}
		_, cards, e := s.inventory(c.Request.Context())
		if e != nil {
			plankaFailure(c, e)
			return
		}
		plankaOutput(c, gin.H{"items": cards, "complete": true})
	})
	g.POST("/planka/tickets", mcpCreatePlanka)
	g.POST("/planka/note", mcpPlankaNote)
	g.POST("/planka/move", mcpMovePlanka)
}
func mcpCreatePlanka(c *gin.Context) {
	var a struct {
		BoardID     string `json:"board_id"`
		ListID      string `json:"list_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		SourceKey   string `json:"source_key"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if c.ShouldBindJSON(&a) != nil || !plankaIDPattern.MatchString(a.BoardID) || !plankaIDPattern.MatchString(a.ListID) || !plankaKeyPattern.MatchString(a.SourceKey) || strings.TrimSpace(a.Name) == "" || len(a.Name) > 200 || len(a.Description) > 20000 {
		c.JSON(400, gin.H{"error": "valid board_id, list_id, source_key, name (1-200 bytes) and description (at most 20000 bytes) required"})
		return
	}
	plankaWriteMu.Lock()
	defer plankaWriteMu.Unlock()
	s, e := newMCPPlanka(c.Request.Context())
	if e != nil {
		plankaFailure(c, e)
		return
	}
	p, cards, e := s.inventory(c.Request.Context())
	if e != nil {
		plankaFailure(c, e)
		return
	}
	marker := "[clyde-source:" + a.SourceKey + "]"
	for _, card := range cards {
		if strings.Contains(card.Description, marker) {
			plankaOutput(c, gin.H{"created": false, "existing": true, "item": card})
			return
		}
	}
	if !validPlankaDestination(p, a.BoardID, a.ListID) {
		c.JSON(400, gin.H{"error": "destination list not found in board"})
		return
	}
	var result struct {
		Item mcpPlankaCard `json:"item"`
	}
	e = s.call(c.Request.Context(), "POST", "/api/lists/"+a.ListID+"/cards", gin.H{"name": a.Name, "description": a.Description + "\n\n" + marker, "position": 0}, &result)
	if e != nil {
		plankaFailure(c, e)
		return
	}
	if result.Item.ID == "" {
		plankaFailure(c, fmt.Errorf("create outcome uncertain: inspect inventory before retrying"))
		return
	}
	plankaOutput(c, gin.H{"created": true, "item": result.Item, "source_resolved": false})
}
func mcpPlankaNote(c *gin.Context) {
	var a struct {
		ID      string `json:"id"`
		Note    string `json:"note"`
		NoteKey string `json:"note_key"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	if c.ShouldBindJSON(&a) != nil || !plankaIDPattern.MatchString(a.ID) || !plankaKeyPattern.MatchString(a.NoteKey) || strings.TrimSpace(a.Note) == "" || len(a.Note) > 20000 {
		c.JSON(400, gin.H{"error": "valid id, note_key and note (1-20000 bytes) required"})
		return
	}
	plankaWriteMu.Lock()
	defer plankaWriteMu.Unlock()
	s, e := newMCPPlanka(c.Request.Context())
	if e != nil {
		plankaFailure(c, e)
		return
	}
	marker := "[clyde-note:" + a.NoteKey + "]"
	before := ""
	for page := 0; page < 100; page++ {
		path := "/api/cards/" + a.ID + "/actions?withDetails=true"
		if before != "" {
			path += "&beforeId=" + before
		}
		var actions struct {
			Items []struct {
				ID   string `json:"id"`
				Data struct {
					Text string `json:"text"`
				} `json:"data"`
			} `json:"items"`
		}
		if e = s.call(c.Request.Context(), "GET", path, nil, &actions); e != nil {
			plankaFailure(c, e)
			return
		}
		if actions.Items == nil {
			plankaFailure(c, fmt.Errorf("incomplete Planka action history; no note added"))
			return
		}
		for _, action := range actions.Items {
			if strings.Contains(action.Data.Text, marker) {
				plankaOutput(c, gin.H{"updated": false, "ticket_id": a.ID, "note_id": action.ID})
				return
			}
		}
		if len(actions.Items) == 0 {
			var result struct {
				Item struct {
					ID string `json:"id"`
				} `json:"item"`
			}
			if e = s.call(c.Request.Context(), "POST", "/api/cards/"+a.ID+"/comment-actions", gin.H{"text": a.Note + "\n" + marker}, &result); e != nil {
				plankaFailure(c, e)
				return
			}
			if result.Item.ID == "" {
				plankaFailure(c, fmt.Errorf("note outcome uncertain: inspect ticket before retrying"))
				return
			}
			plankaOutput(c, gin.H{"updated": true, "ticket_id": a.ID, "note_id": result.Item.ID})
			return
		}
		next := actions.Items[len(actions.Items)-1].ID
		if !plankaIDPattern.MatchString(next) || next == before {
			plankaFailure(c, fmt.Errorf("cannot verify complete action history; no note added"))
			return
		}
		before = next
	}
	plankaFailure(c, fmt.Errorf("action history exceeds safety limit; no note added"))
}
func mcpMovePlanka(c *gin.Context) {
	var a struct {
		ID             string `json:"id"`
		BoardID        string `json:"board_id"`
		ListID         string `json:"list_id"`
		OwnerConfirmed bool   `json:"owner_confirmed"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	if c.ShouldBindJSON(&a) != nil || !plankaIDPattern.MatchString(a.ID) || !plankaIDPattern.MatchString(a.BoardID) || !plankaIDPattern.MatchString(a.ListID) {
		c.JSON(400, gin.H{"error": "valid ticket and destination IDs required"})
		return
	}
	if !a.OwnerConfirmed {
		c.JSON(403, gin.H{"error": "explicit owner instruction required; no automated ticket closure"})
		return
	}
	plankaWriteMu.Lock()
	defer plankaWriteMu.Unlock()
	s, e := newMCPPlanka(c.Request.Context())
	if e != nil {
		plankaFailure(c, e)
		return
	}
	p, _, e := s.inventory(c.Request.Context())
	if e != nil {
		plankaFailure(c, e)
		return
	}
	if !validPlankaDestination(p, a.BoardID, a.ListID) {
		c.JSON(400, gin.H{"error": "destination list not found in board"})
		return
	}
	var result struct {
		Item mcpPlankaCard `json:"item"`
	}
	if e = s.call(c.Request.Context(), "PATCH", "/api/cards/"+a.ID, gin.H{"boardId": a.BoardID, "listId": a.ListID, "position": 0}, &result); e != nil {
		plankaFailure(c, e)
		return
	}
	// The installed Planka API returns the updated card. A successful HTTP status
	// alone does not confirm the requested move; never claim success for an
	// incomplete or mismatched response, and do not retry a possibly-applied write.
	if result.Item.ID != a.ID || result.Item.BoardID != a.BoardID || result.Item.ListID != a.ListID {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Planka move outcome uncertain: returned card does not match requested ticket and destination; inspect the ticket before retrying", "outcome_uncertain": true})
		return
	}
	plankaOutput(c, gin.H{"moved": true, "item": result.Item, "source_resolved": false})
}
