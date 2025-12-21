package db

import "strings"

func normalizeReactions(reactions map[string][]string) map[string][]string {
	if reactions == nil {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(reactions))
	for reaction, users := range reactions {
		reaction = strings.TrimSpace(reaction)
		if reaction == "" {
			continue
		}
		cleaned := dedupeNonEmpty(users)
		if len(cleaned) == 0 {
			continue
		}
		out[reaction] = cleaned
	}
	return out
}

func dedupeNonEmpty(values []string) []string {
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
	return out
}

func cloneReactions(reactions map[string][]string) map[string][]string {
	if reactions == nil {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(reactions))
	for reaction, users := range reactions {
		copied := make([]string, len(users))
		copy(copied, users)
		out[reaction] = copied
	}
	return out
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func removeString(values []string, value string) ([]string, bool) {
	if len(values) == 0 {
		return values, false
	}
	removed := false
	out := make([]string, 0, len(values))
	for _, item := range values {
		if item == value {
			removed = true
			continue
		}
		out = append(out, item)
	}
	return out, removed
}
