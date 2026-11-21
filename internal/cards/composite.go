package cards

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/soulteary/github-readme-stats/internal/common"
)

const (
	// CompositeLayoutHorizontal places cards left to right.
	CompositeLayoutHorizontal = "horizontal"
	// CompositeLayoutVertical stacks cards top to bottom.
	CompositeLayoutVertical = "vertical"

	CompositeMinCards = 2
	CompositeMaxCards = 3

	CompositeDefaultGap     = 16
	CompositeMaxGap         = 80
	CompositeDefaultPadding = 0
	CompositeMaxPadding     = 80

	// Canvas bounds keep a single request from producing an unusable image.
	CompositeMaxCanvasWidth  = 4096
	CompositeMaxCanvasHeight = 4096
)

// CompositeCard is one rendered sub-card to place on the canvas.
type CompositeCard struct {
	Kind   string
	SVG    string
	Width  float64
	Height float64
}

// CompositeOptions controls how rendered sub-cards are assembled.
type CompositeOptions struct {
	Layout     string
	Gap        float64
	Padding    float64
	FontFamily string
}

// CompositeResult is the composed SVG and its outer dimensions.
type CompositeResult struct {
	SVG    string
	Width  float64
	Height float64
}

// CompositeErrorKind classifies a composition failure for status mapping.
type CompositeErrorKind int

const (
	// CompositeErrorInvalid means the request itself was malformed.
	CompositeErrorInvalid CompositeErrorKind = iota
	// CompositeErrorData means a card's data could not be fetched.
	CompositeErrorData
)

// CompositeError describes a request-level composition failure.
type CompositeError struct {
	Message          string
	SecondaryMessage string
	Kind             CompositeErrorKind
}

func (e *CompositeError) Error() string { return e.Message }

// RenderComposite assembles already-rendered cards into a single SVG.
// Sub-card SVGs keep their titles and data; ids, classes and keyframe names are
// namespaced per card so gradients, masks and clips cannot collide.
func RenderComposite(cards []CompositeCard, options CompositeOptions) (*CompositeResult, error) {
	layout := options.Layout
	if layout == "" {
		layout = CompositeLayoutHorizontal
	}
	if layout != CompositeLayoutHorizontal && layout != CompositeLayoutVertical {
		return nil, &CompositeError{
			Message:          "Invalid layout",
			SecondaryMessage: "Supported layouts: horizontal, vertical",
		}
	}

	gap := options.Gap
	if gap < 0 || math.IsNaN(gap) || math.IsInf(gap, 0) || gap > CompositeMaxGap {
		return nil, &CompositeError{
			Message:          "Invalid gap",
			SecondaryMessage: fmt.Sprintf("Gap must be between 0 and %d", CompositeMaxGap),
		}
	}

	padding := options.Padding
	if padding < 0 || math.IsNaN(padding) || math.IsInf(padding, 0) || padding > CompositeMaxPadding {
		return nil, &CompositeError{
			Message:          "Invalid padding",
			SecondaryMessage: fmt.Sprintf("Padding must be between 0 and %d", CompositeMaxPadding),
		}
	}

	if len(cards) < CompositeMinCards {
		return nil, &CompositeError{
			Message:          "Invalid cards selection",
			SecondaryMessage: fmt.Sprintf("Select %d to %d cards", CompositeMinCards, CompositeMaxCards),
		}
	}
	if len(cards) > CompositeMaxCards {
		return nil, &CompositeError{
			Message:          "Invalid cards selection",
			SecondaryMessage: fmt.Sprintf("Select %d to %d cards", CompositeMinCards, CompositeMaxCards),
		}
	}

	// Parse dimensions before placement so an oversized canvas is rejected
	// without producing partial output. Copy the slice first so measured
	// dimensions never leak back into the caller's input.
	cards = append([]CompositeCard(nil), cards...)
	for i := range cards {
		width, height, err := parseSVGDimensions(cards[i].SVG)
		if err != nil {
			return nil, &CompositeError{
				Message:          "Invalid card output",
				SecondaryMessage: fmt.Sprintf("Card %q could not be measured", cards[i].Kind),
			}
		}
		cards[i].Width = width
		cards[i].Height = height
	}

	var width, height float64
	switch layout {
	case CompositeLayoutHorizontal:
		height = 0
		for _, card := range cards {
			width += card.Width
			height = math.Max(height, card.Height)
		}
		width += gap * float64(len(cards)-1)
	case CompositeLayoutVertical:
		width = 0
		for _, card := range cards {
			height += card.Height
			width = math.Max(width, card.Width)
		}
		height += gap * float64(len(cards)-1)
	}
	width += padding * 2
	height += padding * 2

	if width > CompositeMaxCanvasWidth || height > CompositeMaxCanvasHeight {
		return nil, &CompositeError{
			Message: "Composite canvas too large",
			SecondaryMessage: fmt.Sprintf(
				"Computed %dx%d px, maximum is %dx%d px",
				int(math.Ceil(width)), int(math.Ceil(height)),
				CompositeMaxCanvasWidth, CompositeMaxCanvasHeight,
			),
		}
	}

	var body strings.Builder
	cursor := padding
	for i, card := range cards {
		x, y := padding, padding
		switch layout {
		case CompositeLayoutHorizontal:
			x = cursor
			cursor += card.Width + gap
		case CompositeLayoutVertical:
			y = cursor
			cursor += card.Height + gap
		}

		embedded := namespaceSubSVG(card.SVG, fmt.Sprintf("cc%d", i))
		fmt.Fprintf(&body, `
  <g
    data-testid="composite-card"
    data-card="%s"
    data-x="%.0f"
    data-y="%.0f"
    data-width="%.0f"
    data-height="%.0f"
    class="composite-card"
    transform="translate(%.0f, %.0f)"
  >%s</g>`, common.EncodeHTML(card.Kind), x, y, card.Width, card.Height, x, y, embedded)
	}

	fontCSS := ""
	if options.FontFamily != "" {
		fontCSS = fmt.Sprintf(`
          .composite-card text {
            font-family: '%s', 'Segoe UI', Ubuntu, 'Helvetica Neue', Sans-Serif !important;
          }`, strings.ReplaceAll(options.FontFamily, "'", ""))
	}

	titleParts := make([]string, 0, len(cards))
	for _, card := range cards {
		titleParts = append(titleParts, card.Kind)
	}
	a11yTitle := common.EncodeHTML("Composite card: " + strings.Join(titleParts, ", "))

	svg := fmt.Sprintf(`<svg
  width="%.0f"
  height="%.0f"
  viewBox="0 0 %.0f %.0f"
  fill="none"
  xmlns="http://www.w3.org/2000/svg"
  role="img"
  aria-labelledby="compositeTitleId compositeDescId"
  data-testid="composite"
>
  <title id="compositeTitleId">%s</title>
  <desc id="compositeDescId">Combined GitHub statistics card generated by GitHub Readme Stats</desc>
  <style>%s</style>
  %s
</svg>`, width, height, width, height, a11yTitle, fontCSS, body.String())

	return &CompositeResult{Width: width, Height: height, SVG: svg}, nil
}

var (
	compositeSVGRootRe = regexp.MustCompile(`(?s)^\s*<svg\b[^>]*>`)
	compositeWidthRe   = regexp.MustCompile(`\bwidth="([0-9]+(?:\.[0-9]+)?)"`)
	compositeHeightRe  = regexp.MustCompile(`\bheight="([0-9]+(?:\.[0-9]+)?)"`)
)

func parseSVGDimensions(svg string) (float64, float64, error) {
	root := compositeSVGRootRe.FindString(svg)
	if root == "" {
		return 0, 0, fmt.Errorf("missing svg root")
	}
	widthMatch := compositeWidthRe.FindStringSubmatch(root)
	heightMatch := compositeHeightRe.FindStringSubmatch(root)
	if widthMatch == nil || heightMatch == nil {
		return 0, 0, fmt.Errorf("missing svg dimensions")
	}
	var width, height float64
	if _, err := fmt.Sscanf(widthMatch[1], "%f", &width); err != nil {
		return 0, 0, err
	}
	if _, err := fmt.Sscanf(heightMatch[1], "%f", &height); err != nil {
		return 0, 0, err
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("non-positive svg dimensions")
	}
	return width, height, nil
}

var (
	subStyleRe    = regexp.MustCompile(`(?s)<style>(.*?)</style>`)
	subIDDeclRe   = regexp.MustCompile(`\bid="([^"]+)"`)
	subClassRe    = regexp.MustCompile(`class=(["'])([^"']*)["']`)
	subKeyframeRe = regexp.MustCompile(`@keyframes\s+([A-Za-z_][A-Za-z0-9_-]*)`)
	subAriaRe     = regexp.MustCompile(`aria-labelledby="([^"]*)"`)
)

// namespaceSubSVG rewrites ids, class names and keyframe names inside one
// sub-card. This keeps gradients (id="gradient"), masks (id="rect-mask"),
// clip paths and CSS animations independent when several cards share a
// document. Text content is never rewritten.
func namespaceSubSVG(svg, prefix string) string {
	idSet := map[string]bool{}
	for _, match := range subIDDeclRe.FindAllStringSubmatch(svg, -1) {
		idSet[match[1]] = true
	}

	classSet := map[string]bool{}
	for _, match := range subClassRe.FindAllStringSubmatch(svg, -1) {
		for _, name := range strings.Fields(match[2]) {
			classSet[name] = true
		}
	}

	keyframeSet := map[string]bool{}
	for _, block := range subStyleRe.FindAllStringSubmatch(svg, -1) {
		for _, match := range subKeyframeRe.FindAllStringSubmatch(block[1], -1) {
			keyframeSet[match[1]] = true
		}
	}

	// Build one selector replacer per name group so each selector is rewritten
	// in a single pass. Longer names come first so a nested class such as
	// "rank-percentile-header" is not partially matched by "header".
	idReplacer := newSelectorReplacer("#", idSet)
	classReplacer := newSelectorReplacer(`\.`, classSet)
	// Keyframe names only appear in "@keyframes NAME" and in an animation /
	// animation-name declaration. Restricting the rewrite to that context keeps
	// an unrelated class (e.g. "…-fadeInAnimation") from double-prefixing.
	keyframeReplacer := newKeyframeReplacer(keyframeSet)

	// Style blocks: selectors and animation names must follow the renamed ids,
	// classes and keyframes.
	svg = subStyleRe.ReplaceAllStringFunc(svg, func(block string) string {
		inner := block[len("<style>") : len(block)-len("</style>")]
		if keyframeReplacer != nil {
			inner = keyframeReplacer.ReplaceAllStringFunc(inner, func(match string) string {
				sub := keyframeReplacer.FindStringSubmatch(match)
				return sub[1] + prefix + "-" + sub[2]
			})
		}
		if idReplacer != nil {
			inner = idReplacer.ReplaceAllStringFunc(inner, func(match string) string {
				return "#" + prefix + "-" + strings.TrimPrefix(match, "#")
			})
		}
		if classReplacer != nil {
			inner = classReplacer.ReplaceAllStringFunc(inner, func(match string) string {
				return "." + prefix + "-" + strings.TrimPrefix(match, ".")
			})
		}
		return "<style>" + inner + "</style>"
	})

	// Declarations and url()/aria references.
	for id := range idSet {
		newID := prefix + "-" + id
		svg = strings.ReplaceAll(svg, `id="`+id+`"`, `id="`+newID+`"`)
		urlRef := regexp.MustCompile(`url\(#` + regexp.QuoteMeta(id) + `\)`)
		svg = urlRef.ReplaceAllString(svg, "url(#"+newID+")")
	}
	svg = subAriaRe.ReplaceAllStringFunc(svg, func(match string) string {
		sub := subAriaRe.FindStringSubmatch(match)
		tokens := strings.Fields(sub[1])
		for i, token := range tokens {
			if idSet[token] {
				tokens[i] = prefix + "-" + token
			}
		}
		return `aria-labelledby="` + strings.Join(tokens, " ") + `"`
	})

	// Markup class attributes.
	svg = subClassRe.ReplaceAllStringFunc(svg, func(match string) string {
		sub := subClassRe.FindStringSubmatch(match)
		quote := sub[1]
		names := strings.Fields(sub[2])
		for i, name := range names {
			names[i] = prefix + "-" + name
		}
		return "class=" + quote + strings.Join(names, " ") + quote
	})

	return svg
}

// newSelectorReplacer builds a regexp matching "#name" or ".name" selectors,
// longest name first so nested class names are not partially rewritten.
func newSelectorReplacer(lead string, names map[string]bool) *regexp.Regexp {
	if len(names) == 0 {
		return nil
	}
	sorted := sortedKeys(names)
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}
	escaped := make([]string, len(sorted))
	for i, name := range sorted {
		escaped[i] = regexp.QuoteMeta(name)
	}
	return regexp.MustCompile(lead + `(` + strings.Join(escaped, "|") + `)\b`)
}

// newKeyframeReplacer builds a regexp that captures the keyframe reference
// context in group 1 and the keyframe name in group 2.
func newKeyframeReplacer(names map[string]bool) *regexp.Regexp {
	if len(names) == 0 {
		return nil
	}
	sorted := sortedKeys(names)
	escaped := make([]string, len(sorted))
	for i, name := range sorted {
		escaped[i] = regexp.QuoteMeta(name)
	}
	return regexp.MustCompile(`(@keyframes\s+|animation(?:-name)?\s*:\s*)` +
		`(` + strings.Join(escaped, "|") + `)\b`)
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
