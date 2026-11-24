package endpoints

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var steamMarkdownEscape = strings.NewReplacer(
	"\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]",
	"<", "\\<", ">", "\\>", "#", "\\#", "~", "\\~", "|", "\\|",
)

func steamMarkdown(n *html.Node, origin ...string) string {
	if n == nil {
		return ""
	}
	base := &url.URL{Scheme: "https", Host: "steamcommunity.com", Path: "/"}
	if len(origin) > 0 {
		if u, err := url.Parse(origin[0]); err == nil && u.Scheme == "https" && u.Host == "steamcommunity.com" {
			base = u
		}
	}
	return strings.TrimSpace(steamMarkdownNode(n, base))
}

func steamMarkdownNode(n *html.Node, base *url.URL) string {
	if n.Type == html.TextNode {
		return steamMarkdownEscape.Replace(n.Data)
	}
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || hasClass(n, "forum_comment_permlink")) {
		return ""
	}
	var out strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(steamMarkdownNode(child, base))
	}
	body := out.String()
	trimmed := strings.TrimSpace(body)
	if n.Type != html.ElementNode {
		return body
	}
	if n.Data == "blockquote" || hasClass(n, "bb_blockquote") {
		if trimmed == "" {
			return ""
		}
		lines := strings.Split(trimmed, "\n")
		for i := range lines {
			if strings.TrimSpace(lines[i]) == "" {
				lines[i] = ">"
			} else {
				lines[i] = "> " + lines[i]
			}
		}
		return "\n\n" + strings.Join(lines, "\n") + "\n\n"
	}
	if hasClass(n, "bb_quoteauthor") {
		label := "**" + steamMarkdownEscape.Replace(strings.Join(strings.Fields(textWithBreaks(n)), " ")) + "**"
		if link := firstDescendant(n, "a", ""); link != nil {
			if href := steamMarkdownLink(attr(link, "href"), base); href != "" {
				label = "[" + label + "](<" + href + ">)"
			}
		}
		return label + "\n\n"
	}
	if hasClass(n, "bb_h1") || hasClass(n, "bb_h2") || hasClass(n, "bb_h3") {
		level := 1
		if hasClass(n, "bb_h2") {
			level = 2
		} else if hasClass(n, "bb_h3") {
			level = 3
		}
		return "\n\n" + strings.Repeat("#", level) + " " + trimmed + "\n\n"
	}
	if hasClass(n, "bb_code") || n.Data == "pre" {
		content := strings.Trim(textWithBreaks(n), "\r\n")
		fence := steamCodeFence(content, 3)
		return "\n\n" + fence + "\n" + content + "\n" + fence + "\n\n"
	}
	switch n.Data {
	case "br":
		return "\n"
	case "hr":
		return "\n\n---\n\n"
	case "b", "strong":
		return steamWrapInline(body, "**")
	case "i", "em":
		return steamWrapInline(body, "*")
	case "s", "del", "strike":
		return steamWrapInline(body, "~~")
	case "code":
		content := textContent(n)
		fence := steamCodeFence(content, 1)
		if strings.HasPrefix(content, "`") || strings.HasSuffix(content, "`") {
			content = " " + content + " "
		}
		return fence + content + fence
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return "\n\n" + strings.Repeat("#", int(n.Data[1]-'0')) + " " + trimmed + "\n\n"
	case "p", "div", "section":
		return "\n\n" + trimmed + "\n\n"
	case "img":
		label := strings.TrimSpace(attr(n, "alt"))
		if label == "" {
			label = "Steam image"
		}
		if link := steamMarkdownLink(attr(n, "src"), base); link != "" {
			return "[" + steamMarkdownEscape.Replace(label) + "](<" + link + ">)"
		}
		return steamMarkdownEscape.Replace(label)
	case "a":
		if link := steamMarkdownLink(attr(n, "href"), base); link != "" && trimmed != "" {
			return "[" + trimmed + "](<" + link + ">)"
		}
	case "ul", "ol":
		var rows []string
		index := 1
		if start, e := strconv.Atoi(attr(n, "start")); e == nil && start >= 0 && start < 1000000 {
			index = start
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode || c.Data != "li" {
				continue
			}
			mark := "- "
			if n.Data == "ol" {
				mark = strconv.Itoa(index) + ". "
				index++
			}
			text := strings.TrimSpace(steamMarkdownNode(c, base))
			rows = append(rows, mark+strings.ReplaceAll(text, "\n", "\n"+strings.Repeat(" ", len(mark))))
		}
		return "\n\n" + strings.Join(rows, "\n") + "\n\n"
	}
	return body
}

func steamWrapInline(body, marker string) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return body
	}
	first := strings.Index(body, trimmed)
	return body[:first] + marker + trimmed + marker + body[first+len(trimmed):]
}

func steamCodeFence(value string, minimum int) string {
	run, max := 0, minimum-1
	for _, c := range value {
		if c == '`' {
			run++
			if run > max {
				max = run
			}
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max+1)
}

func steamMarkdownLink(raw string, base *url.URL) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.User != nil {
		return ""
	}
	if !u.IsAbs() {
		u = base.ResolveReference(u)
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	return strings.NewReplacer("<", "%3C", ">", "%3E", " ", "%20").Replace(u.String())
}
