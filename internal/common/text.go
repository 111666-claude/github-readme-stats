package common

import (
	"html"
	"strings"
	"unicode"
)

const truncationEllipsis = "..."

// TruncateTextToWidth shortens plain (unencoded) text until it fits within
// maxWidth at the given font size, appending an ellipsis when characters were
// removed. It is rune aware so CJK names are never cut in the middle of a
// character. HTML entities are decoded for measurement and the plain-text
// result is returned; callers encode it when rendering. A non-positive maxWidth
// returns the original text (no constraint).
func TruncateTextToWidth(text string, maxWidth, fontSize float64) string {
	if maxWidth <= 0 || text == "" {
		return text
	}
	// Encoded entities (e.g. "&#39;") are not character widths. Measure the
	// decoded form so a safe margin is what we budget for.
	text = html.UnescapeString(text)
	if fontSize <= 0 {
		fontSize = 10
	}
	if MeasureText(text, fontSize) <= maxWidth {
		return text
	}

	if MeasureText(truncationEllipsis, fontSize) > maxWidth {
		return truncationEllipsis
	}

	runes := []rune(text)
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high + 1) / 2
		candidate := strings.TrimRight(string(runes[:mid]), " ") + truncationEllipsis
		if MeasureText(candidate, fontSize) <= maxWidth {
			low = mid
		} else {
			high = mid - 1
		}
	}

	if low <= 0 {
		return truncationEllipsis
	}
	return EncodeHTML(strings.TrimRight(string(runes[:low]), " ") + truncationEllipsis)
}

// SanitizeFontFamily validates a user supplied font family for use inside SVG
// CSS. It accepts Unicode letters/digits plus spaces, dots, hyphens and
// underscores, and caps the name at 64 characters. Invalid input is rejected so
// it can never break out of the style block.
func SanitizeFontFamily(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", true
	}
	if len([]rune(name)) > 64 {
		return "", false
	}

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case ' ', '.', '-', '_':
			continue
		}
		return "", false
	}

	return name, true
}
