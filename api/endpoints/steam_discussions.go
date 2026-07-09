package endpoints

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"api.clyde.games/models"
	"golang.org/x/net/html"
)

const steamCommunityBaseURL = "https://steamcommunity.com"

type steamDiscussionForum struct {
	Name string
	URL  string
}

type steamDiscussionTopic struct {
	ID        string
	Title     string
	Author    string
	URL       string
	LastPost  time.Time
	ForumName string
}

type steamDiscussionPost struct {
	ID        string
	Author    string
	Body      string
	URL       string
	CreatedAt time.Time
	IsTopic   bool
}

var steamDiscussionForums = []steamDiscussionForum{
	{
		Name: "Steam Discussion",
		URL:  steamCommunityBaseURL + "/app/" + cursemarkSteamAppID + "/discussions/0/",
	},
	{
		Name: "Steam Announcement Discussion",
		URL:  steamCommunityBaseURL + "/app/" + cursemarkSteamAppID + "/eventcomments/",
	},
}

func ImportSteamDiscussionsSince(cutoff time.Time, now time.Time) (int, int, error) {
	client := http.Client{Timeout: 12 * time.Second}
	imported := 0
	skipped := 0

	for _, forum := range steamDiscussionForums {
		topics, err := fetchSteamDiscussionTopicsSince(client, forum, cutoff)
		if err != nil {
			return imported, skipped, err
		}

		for _, topic := range topics {
			postImported, postSkipped, err := importSteamDiscussionTopic(client, topic, cutoff)
			if err != nil {
				return imported + postImported, skipped + postSkipped, err
			}

			imported += postImported
			skipped += postSkipped
		}
	}

	return imported, skipped, nil
}

func fetchSteamDiscussionTopicsSince(client http.Client, forum steamDiscussionForum, cutoff time.Time) ([]steamDiscussionTopic, error) {
	var topics []steamDiscussionTopic

	for page := 1; page <= 20; page++ {
		doc, err := fetchSteamHTML(client, pagedSteamForumURL(forum.URL, page))
		if err != nil {
			return topics, err
		}

		pageTopics := parseSteamDiscussionTopics(doc, forum)
		if len(pageTopics) == 0 {
			return topics, nil
		}

		recentOnPage := false
		for _, topic := range pageTopics {
			if topic.LastPost.IsZero() || topic.LastPost.Before(cutoff) {
				continue
			}

			recentOnPage = true
			topics = append(topics, topic)
		}

		if !recentOnPage {
			return topics, nil
		}
	}

	return topics, nil
}

func importSteamDiscussionTopic(client http.Client, topic steamDiscussionTopic, cutoff time.Time) (int, int, error) {
	imported := 0
	skipped := 0
	totalPages := 1

	for page := 1; page <= totalPages && page <= 20; page++ {
		doc, err := fetchSteamHTML(client, pagedSteamTopicURL(topic.URL, page))
		if err != nil {
			return imported, skipped, err
		}

		if page == 1 {
			totalPages = steamTopicCommentPages(doc)
		}

		posts := parseSteamDiscussionPosts(doc, topic)
		for _, post := range posts {
			if post.CreatedAt.IsZero() || post.CreatedAt.Before(cutoff) {
				continue
			}

			if strings.TrimSpace(post.Body) == "" {
				continue
			}

			exists, err := models.FeedbackMessageContains(post.URL)
			if err != nil {
				return imported, skipped, err
			}

			if exists {
				skipped += 1
				continue
			}

			feedback := models.Feedback{
				PID:      post.Author,
				Project:  "Cursemark",
				Message:  steamDiscussionFeedbackMessage(topic, post),
				Rating:   3,
				Env:      "production",
				Category: topic.ForumName,
				Platform: "Steam",
			}
			feedback.CreatedAt = post.CreatedAt

			_, err = models.AddFeedback(feedback)
			if err != nil {
				return imported, skipped, err
			}

			imported += 1
		}
	}

	return imported, skipped, nil
}

func fetchSteamHTML(client http.Client, requestURL string) (*html.Node, error) {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "clyde-games-feedback-sync/1.0")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("steam discussions returned %s for %s", res.Status, requestURL)
	}

	return html.Parse(res.Body)
}

func parseSteamDiscussionTopics(doc *html.Node, forum steamDiscussionForum) []steamDiscussionTopic {
	var topics []steamDiscussionTopic

	for _, node := range findAll(doc, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "forum_topic") && attr(node, "data-gidforumtopic") != ""
	}) {
		topicURL := absoluteSteamURL(attr(firstDescendant(node, "a", "forum_topic_overlay"), "href"))
		if topicURL == "" {
			continue
		}

		lastPostNode := firstDescendant(node, "div", "forum_topic_lastpost")
		lastPost := unixTime(attr(lastPostNode, "data-timestamp"))

		topics = append(topics, steamDiscussionTopic{
			ID:        attr(node, "data-gidforumtopic"),
			Title:     cleanText(textContent(firstDescendant(node, "div", "forum_topic_name"))),
			Author:    cleanText(textContent(firstDescendant(node, "div", "forum_topic_op"))),
			URL:       topicURL,
			LastPost:  lastPost,
			ForumName: forum.Name,
		})
	}

	return topics
}

func parseSteamDiscussionPosts(doc *html.Node, topic steamDiscussionTopic) []steamDiscussionPost {
	var posts []steamDiscussionPost

	if op := firstDescendant(doc, "div", "forum_op"); op != nil {
		post := steamDiscussionPost{
			ID:        topic.ID,
			Author:    cleanText(textContent(firstDescendant(op, "a", "forum_op_author"))),
			Body:      cleanText(textWithBreaks(firstDescendant(op, "div", "content"))),
			URL:       topic.URL,
			CreatedAt: unixTime(attr(firstTimestamp(op, "commentthread_comment_timestamp"), "data-timestamp")),
			IsTopic:   true,
		}
		posts = append(posts, post)
	}

	for _, comment := range findAll(doc, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "div" && hasClass(node, "commentthread_comment") && strings.HasPrefix(attr(node, "id"), "comment_")
	}) {
		commentID := strings.TrimPrefix(attr(comment, "id"), "comment_")
		posts = append(posts, steamDiscussionPost{
			ID:        commentID,
			Author:    cleanText(textContent(firstDescendant(comment, "a", "commentthread_author_link"))),
			Body:      cleanText(textWithBreaks(firstDescendant(comment, "div", "commentthread_comment_text"))),
			URL:       strings.TrimRight(topic.URL, "/") + "/#c" + commentID,
			CreatedAt: unixTime(attr(firstTimestamp(comment, "commentthread_comment_timestamp"), "data-timestamp")),
		})
	}

	return posts
}

func steamTopicCommentPages(doc *html.Node) int {
	total := 0
	for _, node := range findAll(doc, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "span" && strings.HasSuffix(attr(node, "id"), "pagetotal")
	}) {
		n, err := strconv.Atoi(strings.TrimSpace(textContent(node)))
		if err == nil && n > total {
			total = n
		}
	}

	if total == 0 {
		return 1
	}

	pages := (total + 14) / 15
	if pages < 1 {
		return 1
	}

	return pages
}

func steamDiscussionFeedbackMessage(topic steamDiscussionTopic, post steamDiscussionPost) string {
	prefix := "Steam discussion: " + topic.Title
	if !post.IsTopic {
		prefix += " (comment)"
	}

	return prefix + "\n\n" + post.Body + "\n" + post.URL
}

func pagedSteamForumURL(baseURL string, page int) string {
	if page <= 1 {
		return baseURL
	}

	return strings.TrimRight(baseURL, "/") + "/?fp=" + strconv.Itoa(page)
}

func pagedSteamTopicURL(baseURL string, page int) string {
	if page <= 1 {
		return baseURL
	}

	return strings.TrimRight(baseURL, "/") + "/?ctp=" + strconv.Itoa(page)
}

func absoluteSteamURL(value string) string {
	if value == "" {
		return ""
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}

	if parsed.IsAbs() {
		return value
	}

	return steamCommunityBaseURL + "/" + strings.TrimLeft(value, "/")
}

func unixTime(value string) time.Time {
	timestamp, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || timestamp <= 0 {
		return time.Time{}
	}

	return time.Unix(timestamp, 0)
}

func firstDescendant(node *html.Node, tag string, class string) *html.Node {
	if node == nil {
		return nil
	}

	matches := findAll(node, func(current *html.Node) bool {
		if current.Type != html.ElementNode || current.Data != tag {
			return false
		}

		return class == "" || hasClass(current, class)
	})

	if len(matches) == 0 {
		return nil
	}

	return matches[0]
}

func firstTimestamp(node *html.Node, class string) *html.Node {
	if node == nil {
		return nil
	}

	matches := findAll(node, func(current *html.Node) bool {
		return current.Type == html.ElementNode && current.Data == "div" && hasClass(current, class) && attr(current, "data-timestamp") != ""
	})

	if len(matches) == 0 {
		return nil
	}

	return matches[0]
}

func findAll(node *html.Node, matches func(*html.Node) bool) []*html.Node {
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current == nil {
			return
		}

		if matches(current) {
			nodes = append(nodes, current)
		}

		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(node)
	return nodes
}

func hasClass(node *html.Node, class string) bool {
	for _, item := range strings.Fields(attr(node, "class")) {
		if item == class {
			return true
		}
	}

	return false
}

func attr(node *html.Node, name string) string {
	if node == nil {
		return ""
	}

	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}

	return ""
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}

	if node.Type == html.TextNode {
		return node.Data
	}

	var parts []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parts = append(parts, textContent(child))
	}

	return strings.Join(parts, " ")
}

func textWithBreaks(node *html.Node) string {
	if node == nil {
		return ""
	}

	if node.Type == html.TextNode {
		return node.Data
	}

	if node.Type == html.ElementNode && node.Data == "br" {
		return "\n"
	}

	if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style" || hasClass(node, "forum_comment_permlink")) {
		return ""
	}

	var parts []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parts = append(parts, textWithBreaks(child))
	}

	text := strings.Join(parts, "")
	if node.Type == html.ElementNode && (node.Data == "div" || node.Data == "p" || node.Data == "blockquote") {
		text += "\n"
	}

	return text
}

func cleanText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}
