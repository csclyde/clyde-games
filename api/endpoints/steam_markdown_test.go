package endpoints

import (
	"strings"
	"testing"
)

func TestSteamMarkdownKeepsQuoteBoundaries(t *testing.T) {
	doc := parseTestHTML(t, `<div class="content">New <b>words</b><blockquote class="bb_blockquote with_author"><div class="bb_quoteauthor">Originally posted by <b>Alice</b>:</div>Quoted <i>words</i><br>Second line <blockquote class="bb_blockquote">Earlier quote</blockquote></blockquote>Actual reply<br><a href="https://example.com/a?q=1">Source</a><script>discard</script></div>`)
	got := steamMarkdown(firstDescendant(doc, "div", "content"))
	for _, want := range []string{"New **words**", "> **Originally posted by Alice:**", "> Quoted *words*", "> Second line", "> > Earlier quote", "\n\nActual reply", "[Source](<https://example.com/a?q=1>)"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
	if strings.Contains(got, "discard") {
		t.Fatal(got)
	}
}
func TestSteamMarkdownPreservesListCodeAndSafeLinks(t *testing.T) {
	doc := parseTestHTML(t, `<div class="content"><h2>Things</h2><ol start="3"><li>First</li><li><code>&lt;tag&gt;</code></li></ol><a href="javascript:alert(1)">safe label</a><pre>  indentation&#10;&lt;example&gt;</pre></div>`)
	got := steamMarkdown(firstDescendant(doc, "div", "content"))
	for _, want := range []string{"## Things", "3. First\n4. `<tag>`", "safe label", "```\n  indentation\n<example>\n```"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
	if strings.Contains(got, "javascript:") {
		t.Fatal(got)
	}
}
func TestSteamMarkdownRelativeQuoteAttribution(t *testing.T) {
	doc := parseTestHTML(t, `<div class="content"><blockquote class="bb_blockquote"><div class="bb_quoteauthor">Originally posted by <b><a href="#c42">A person</a></b>:</div>One line</blockquote>My reply</div>`)
	u := "https://steamcommunity.com/app/3219180/discussions/0/1234/"
	got := steamMarkdown(firstDescendant(doc, "div", "content"), u)
	if !strings.Contains(got, "> [**Originally posted by A person:**](<"+u+"#c42>)") || !strings.HasSuffix(got, "\n\nMy reply") {
		t.Fatal(got)
	}
}
