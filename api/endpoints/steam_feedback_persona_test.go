package endpoints

import (
	"api.clyde.games/models"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type steamPersonaTestTransport func(*http.Request) (*http.Response, error)

func (f steamPersonaTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestSteamPersonaCacheAndPreservesIDs(t *testing.T) {
	id := "76561197960435530"
	calls := 0
	client := &http.Client{Transport: steamPersonaTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "steamcommunity.com" || r.URL.Path != "/profiles/"+id+"/" || r.URL.Query().Get("xml") != "1" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		body := "<profile><steamID64>" + id + "</steamID64><steamID><![CDATA[Test & Player]]></steamID></profile>"
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	resolver := steamPersonaResolver{client: client, cache: map[string]steamPersonaEntry{}}
	rows := []models.Feedback{{PID: id, Source: "steam_reviews"}, {PID: id, Category: "Steam Discussion"}}
	resolver.attach(context.Background(), rows)
	resolver.attach(context.Background(), rows)
	if calls != 1 || rows[0].DisplayAuthor != "Test & Player" || rows[0].PID != id || rows[1].DisplayAuthor != "" {
		t.Fatal(calls, rows)
	}
}
func TestSteamPersonaRejectsMismatchedProfile(t *testing.T) {
	c := &http.Client{Transport: steamPersonaTestTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("<profile><steamID64>76561197960435531</steamID64><steamID>Wrong</steamID></profile>"))}, nil
	})}
	if name := loadSteamPersona(context.Background(), c, "76561197960435530"); name != "" {
		t.Fatal(name)
	}
}
