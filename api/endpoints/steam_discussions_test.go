package endpoints

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestParseSteamDiscussionTopics(t *testing.T) {
	doc := parseTestHTML(t, `
		<div class="forum_topic" data-gidforumtopic="123">
			<a class="forum_topic_overlay" href="https://steamcommunity.com/app/3219180/discussions/0/123/"></a>
			<div class="forum_topic_lastpost" data-timestamp="1783331379">Jul 6 @ 2:49am</div>
			<div class="forum_topic_name">Game stuck at 60fps.</div>
			<div class="forum_topic_op">Player One</div>
		</div>
	`)

	topics := parseSteamDiscussionTopics(doc, steamDiscussionForum{Name: "Steam Discussion"})
	if len(topics) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(topics))
	}

	if topics[0].ID != "123" {
		t.Fatalf("expected topic id 123, got %q", topics[0].ID)
	}

	if topics[0].Title != "Game stuck at 60fps." {
		t.Fatalf("unexpected title %q", topics[0].Title)
	}

	if topics[0].Author != "Player One" {
		t.Fatalf("unexpected author %q", topics[0].Author)
	}

	if topics[0].LastPost.Unix() != 1783331379 {
		t.Fatalf("unexpected last post timestamp %d", topics[0].LastPost.Unix())
	}
}

func TestParseSteamDiscussionPosts(t *testing.T) {
	doc := parseTestHTML(t, `
		<div class="forum_op" id="forum_op_123">
			<a class="forum_op_author">Original Poster</a>
			<div class="commentthread_comment_timestamp" data-timestamp="1783331000"></div>
			<div class="content" id="forum_op_content_123">Opening<br>post</div>
		</div>
		<div class="commentthread_comment" id="comment_456">
			<a class="commentthread_author_link">Reply Person</a>
			<div class="commentthread_comment_timestamp"></div>
			<div class="commentthread_comment_timestamp" data-timestamp="1783331200"></div>
			<div class="commentthread_comment_text" id="comment_content_456">Reply<br>text<div class="forum_comment_permlink">#1</div></div>
		</div>
	`)

	posts := parseSteamDiscussionPosts(doc, steamDiscussionTopic{
		ID:    "123",
		Title: "Topic title",
		URL:   "https://steamcommunity.com/app/3219180/discussions/0/123/",
	})
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}

	if posts[0].Author != "Original Poster" || !posts[0].IsTopic {
		t.Fatalf("unexpected opening post: %#v", posts[0])
	}

	if posts[0].Body != "Opening\npost" {
		t.Fatalf("unexpected opening body %q", posts[0].Body)
	}

	if posts[1].Author != "Reply Person" {
		t.Fatalf("unexpected comment author %q", posts[1].Author)
	}

	if posts[1].Body != "Reply\ntext" {
		t.Fatalf("unexpected comment body %q", posts[1].Body)
	}

	if posts[1].URL != "https://steamcommunity.com/app/3219180/discussions/0/123/#c456" {
		t.Fatalf("unexpected comment url %q", posts[1].URL)
	}

	if !posts[1].CreatedAt.Equal(time.Unix(1783331200, 0)) {
		t.Fatalf("unexpected comment timestamp %s", posts[1].CreatedAt)
	}
}

func parseTestHTML(t *testing.T, markup string) *html.Node {
	t.Helper()

	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}

	return doc
}
