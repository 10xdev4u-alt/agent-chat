package rooms

import (
	"regexp"
)

func stripCode(body string) string {
	body = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(body, " ")
	body = regexp.MustCompile("`[^`\n]*`").ReplaceAllString(body, " ")
	return body
}

var mentionTokenRe = regexp.MustCompile(`@([A-Za-z0-9_.-]+)`)

func ParseMentions(body string, knownAgents []string) []string {
	body = stripCode(body)

	known := make(map[string]bool, len(knownAgents))
	for _, a := range knownAgents {
		if a != "" {
			known[a] = true
		}
	}

	seen := map[string]bool{}
	var result []string
	for _, m := range mentionTokenRe.FindAllStringSubmatch(body, -1) {
		name := m[1]
		if name == "all" || name == "here" {
			continue
		}
		if !known[name] {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

func IsMentionAll(body string) bool {
	body = stripCode(body)
	for _, m := range mentionTokenRe.FindAllStringSubmatch(body, -1) {
		if m[1] == "all" {
			return true
		}
	}
	return false
}

func IsMentionHere(body string) bool {
	body = stripCode(body)
	for _, m := range mentionTokenRe.FindAllStringSubmatch(body, -1) {
		if m[1] == "here" {
			return true
		}
	}
	return false
}
