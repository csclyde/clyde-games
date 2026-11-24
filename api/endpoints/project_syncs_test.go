package endpoints

import (
	"api.clyde.games/models"
	"testing"
)

func TestDiscordCatalogRejectsUnavailableAndWrongTypes(t *testing.T) {
	source := models.DiscordSyncSource{ID: "1513944000158240768", Kind: "discord_forum", Enabled: true}
	good := discordCatalogItem{ID: source.ID, GuildID: "1169710427387744297", Kind: source.Kind, Name: "verified forum", CanRead: true, CanReact: true}
	for _, c := range [][]discordCatalogItem{nil, {{ID: good.ID, Kind: "discord_text", CanRead: true, CanReact: true}}, {{ID: good.ID, Kind: good.Kind, CanRead: true}}} {
		if _, e := validateDiscordCatalogItems([]models.DiscordSyncSource{source}, c); e == nil {
			t.Fatal("unauthorized source accepted")
		}
	}
	out, e := validateDiscordCatalogItems([]models.DiscordSyncSource{source}, []discordCatalogItem{good})
	if e != nil || len(out) != 1 || out[0].GuildID != good.GuildID || out[0].Name != good.Name {
		t.Fatal(out, e)
	}
	if _, e := validateDiscordCatalogItems([]models.DiscordSyncSource{source, source}, []discordCatalogItem{good}); e == nil {
		t.Fatal("duplicate accepted")
	}
}
