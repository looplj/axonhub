package fetch

import (
	"strings"

	"gopkg.in/yaml.v3"
)

func parseMarkdownFrontMatter(markdown string) (string, string, string) {
	markdown = strings.TrimPrefix(markdown, "\ufeff")

	startContentOffset, endMarkerLineStart, afterEndMarkerOffset, ok := findFrontMatterOffsets(markdown)
	if !ok {
		return "", "", markdown
	}

	metaSection := markdown[startContentOffset:endMarkerLineStart]
	title, description := parseFrontMatterMeta(metaSection)

	content := markdown[afterEndMarkerOffset:]
	content = trimLeadingNewlines(content, 2)

	return title, description, content
}

func findFrontMatterOffsets(markdown string) (startContentOffset int, endMarkerLineStart int, afterEndMarkerOffset int, ok bool) {
	i := 0
	foundStart := false
	firstNonEmptySeen := false

	for i <= len(markdown) {
		lineStart := i
		lineEnd := strings.IndexByte(markdown[i:], '\n')
		next := len(markdown)

		if lineEnd >= 0 {
			lineEnd = i + lineEnd
			next = lineEnd + 1
		} else {
			lineEnd = len(markdown)
		}

		line := markdown[lineStart:lineEnd]
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)

		if !foundStart {
			if trimmed == "" {
				i = next
				continue
			}

			firstNonEmptySeen = true

			if trimmed != "---" {
				return 0, 0, 0, false
			}

			foundStart = true
			startContentOffset = next
			i = next

			continue
		}

		if firstNonEmptySeen && (trimmed == "---" || trimmed == "...") {
			endMarkerLineStart = lineStart
			afterEndMarkerOffset = next

			return startContentOffset, endMarkerLineStart, afterEndMarkerOffset, true
		}

		i = next
	}

	return 0, 0, 0, false
}

func parseFrontMatterMeta(metaSection string) (string, string) {
	type meta struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
	}

	var parsed meta
	if err := yaml.Unmarshal([]byte(metaSection), &parsed); err == nil {
		title := cleanText(parsed.Title)

		description := strings.TrimSpace(parsed.Description)
		if description != "" {
			description = cleanText(description)
		}

		return title, description
	}

	lines := strings.Split(metaSection, "\n")

	var (
		title       string
		description string
	)

	var (
		blockKey   string
		blockStyle string
		blockLines []string
	)

	i := 0
	for i < len(lines) {
		line := strings.TrimSuffix(lines[i], "\r")

		if blockKey != "" {
			if line == "" || (!strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t")) {
				value := finalizeBlockValue(blockStyle, blockLines)

				switch blockKey {
				case "title":
					title = cleanText(value)
				case "description":
					if blockStyle == "|" {
						description = strings.TrimSpace(value)
					} else {
						description = cleanText(value)
					}
				}

				blockKey = ""
				blockStyle = ""
				blockLines = nil

				continue
			}

			blockLines = append(blockLines, strings.TrimLeft(line, " \t"))
			i++

			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			i++
			continue
		}

		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			i++
			continue
		}

		key := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])

		switch strings.ToLower(key) {
		case "title":
			if value == "|" || value == ">" {
				blockKey = "title"
				blockStyle = value
				blockLines = nil
				i++

				continue
			}

			title = cleanText(stripWrappingQuote(value))
		case "description":
			if value == "|" || value == ">" {
				blockKey = "description"
				blockStyle = value
				blockLines = nil
				i++

				continue
			}

			description = cleanText(stripWrappingQuote(value))
		}

		i++
	}

	if blockKey != "" {
		value := finalizeBlockValue(blockStyle, blockLines)

		switch blockKey {
		case "title":
			title = cleanText(value)
		case "description":
			if blockStyle == "|" {
				description = strings.TrimSpace(value)
			} else {
				description = cleanText(value)
			}
		}
	}

	return title, description
}

func finalizeBlockValue(style string, lines []string) string {
	if style == ">" {
		return strings.Join(lines, " ")
	}

	return strings.Join(lines, "\n")
}

func stripWrappingQuote(s string) string {
	if len(s) < 2 {
		return s
	}

	if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}

	return s
}

func trimLeadingNewlines(s string, max int) string {
	for range max {
		if after, ok := strings.CutPrefix(s, "\r\n"); ok {
			s = after
			continue
		}

		if after, ok := strings.CutPrefix(s, "\n"); ok {
			s = after
			continue
		}

		break
	}

	return s
}
