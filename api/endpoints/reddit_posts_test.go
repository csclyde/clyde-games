package endpoints

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchRedditFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/r/cursemark/new.rss" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		if r.UserAgent() != redditDefaultUserAgent {
			t.Fatalf("unexpected user agent: %q", r.UserAgent())
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
			<entry><id>t3_abc</id><title>Controller feedback</title><published>2026-07-11T10:00:00Z</published>
			<author><name>/u/player</name></author><content type="html">&lt;p&gt;Please add rebinding.&lt;/p&gt;</content>
			<link rel="alternate" href="https://www.reddit.com/r/cursemark/comments/abc/controller_feedback/" /></entry>
		</feed>`))
	}))
	defer server.Close()

	feed, err := fetchRedditFeed(http.Client{}, server.URL+"/r/cursemark/new.rss")
	if err != nil {
		t.Fatal(err)
	}
	if len(feed.Entries) != 1 || feed.Entries[0].ID != "t3_abc" {
		t.Fatalf("unexpected feed: %#v", feed)
	}
}

func TestFetchRedditFeedUsesShortCache(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><id>t3_cached</id></entry></feed>`))
	}))
	defer server.Close()

	feedURL := server.URL + "/new.rss"
	if _, err := fetchRedditFeed(http.Client{}, feedURL); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchRedditFeed(http.Client{}, feedURL); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("feed was requested %d times, want 1", requests)
	}
}

func TestRedditFeedbackMessageIncludesCanonicalPostLink(t *testing.T) {
	entry := redditFeedEntry{
		Title:   "Controller feedback",
		Content: "<p>Please add rebinding.</p>",
		Links: []redditFeedLink{{
			Rel:  "alternate",
			Href: "https://www.reddit.com/r/cursemark/comments/abc/controller_feedback/",
		}},
	}

	message := redditFeedbackMessage(entry)
	if !strings.Contains(message, "Controller feedback\n\nPlease add rebinding.") {
		t.Fatalf("message does not contain title and body: %q", message)
	}
	if !strings.Contains(message, "https://www.reddit.com/r/cursemark/comments/abc/controller_feedback/") {
		t.Fatalf("message does not contain Reddit post link: %q", message)
	}
}

func TestRedditEntryUsesPublishedTimeAndAlternateLink(t *testing.T) {
	entry := redditFeedEntry{
		Published: "2026-07-11T10:00:00Z",
		Updated:   "2026-07-11T11:00:00Z",
		Links: []redditFeedLink{
			{Rel: "self", Href: "https://www.reddit.com/feed-entry"},
			{Rel: "alternate", Href: "https://www.reddit.com/r/cursemark/comments/abc/post/"},
		},
	}

	createdAt, err := redditEntryTime(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !createdAt.Equal(time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected entry time: %s", createdAt)
	}
	if got := redditEntryURL(entry); got != "https://www.reddit.com/r/cursemark/comments/abc/post/" {
		t.Fatalf("unexpected entry URL: %q", got)
	}
}
