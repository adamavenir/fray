package core

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxReactionRunes   = 20
	reactionPreviewLen = 40
)

// NormalizeReactionText trims and validates reaction text.
func NormalizeReactionText(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	if utf8.RuneCountInString(trimmed) > maxReactionRunes {
		return "", false
	}
	return trimmed, true
}

// FormatReactionEvent builds a grouped reaction event string.
func FormatReactionEvent(reactors []string, reaction, messageID, messageBody string) string {
	names := uniqueSorted(reactors)
	reactedBy := strings.Join(names, ", ")
	target := formatReactionTarget(messageID, messageBody)
	return fmt.Sprintf("%s reacted %s to %s", reactedBy, strconv.Quote(reaction), strconv.Quote(target))
}

func formatReactionTarget(messageID, messageBody string) string {
	preview := truncateReactionPreview(messageBody, reactionPreviewLen)
	if preview == "" {
		return fmt.Sprintf("#%s", messageID)
	}
	return fmt.Sprintf("#%s %s", messageID, preview)
}

func truncateReactionPreview(body string, maxLen int) string {
	compact := strings.Join(strings.Fields(body), " ")
	if len(compact) <= maxLen {
		return compact
	}
	return compact[:maxLen-3] + "..."
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
