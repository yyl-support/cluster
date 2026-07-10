package converter

import (
	"regexp"
	"strings"
)

var (
	// Matches any character that is NOT a lowercase letter, digit, '-', or '.'
	illegalCharRegex = regexp.MustCompile(`[^a-z0-9\-\.]`)
	// Ensures name starts and ends with alphanumeric
	startEndAlnumRegex = regexp.MustCompile(`^[^a-z0-9]+|[^a-z0-9]+$`)
)

func makeArgoTemplateName(parts ...string) string {
	var cleaned []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Convert to lowercase
		lower := strings.ToLower(part)
		// Remove illegal chars
		clean := illegalCharRegex.ReplaceAllString(lower, "-")
		// Collapse multiple hyphens/dots into single hyphen
		clean = regexp.MustCompile(`[-\.]+`).ReplaceAllString(clean, "-")
		// Trim leading/trailing non-alphanumeric
		clean = startEndAlnumRegex.ReplaceAllString(clean, "")
		if clean != "" {
			cleaned = append(cleaned, clean)
		}
	}

	name := strings.Join(cleaned, "-")

	// Truncate to 63 chars, but avoid ending with non-alphanumeric
	if len(name) > 63 {
		name = name[:63]
		// Trim trailing illegal ending
		name = startEndAlnumRegex.ReplaceAllString(name, "")
		// If still too long after trim, hard truncate to 63 and ensure alnum end
		if len(name) > 63 {
			name = name[:63]
			for len(name) > 0 && !isAlnum(name[len(name)-1]) {
				name = name[:len(name)-1]
			}
		}
	}

	// Final safety: if empty, fallback (shouldn't happen in practice)
	if name == "" {
		name = "codearts-build"
	}

	return name
}

func isAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
