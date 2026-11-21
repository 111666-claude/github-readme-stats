package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/github-readme-stats/internal/cards"
	"github.com/soulteary/github-readme-stats/internal/common"
	"github.com/soulteary/github-readme-stats/internal/fetchers"
	"github.com/soulteary/github-readme-stats/internal/translations"
)

// Card kinds selectable through /api/composite.
const (
	CompositeKindStats    = "stats"
	CompositeKindPin      = "pin"
	CompositeKindTopLangs = "top-langs"
)

// CompositeHandler handles the /api/composite route.
func CompositeHandler(c *gin.Context) {
	username := c.Query("username")
	repo := c.Query("repo")
	cardsParam := c.Query("cards")
	layout := c.Query("layout")
	gap := c.Query("gap")
	padding := c.Query("padding")
	fontFamily := c.Query("font")
	theme := c.Query("theme")
	titleColor := c.Query("title_color")
	textColor := c.Query("text_color")
	iconColor := c.Query("icon_color")
	bgColor := c.Query("bg_color")
	borderColor := c.Query("border_color")
	locale := c.Query("locale")
	borderRadius := c.Query("border_radius")
	disableAnimations := c.Query("disable_animations")

	c.Header("Content-Type", "image/svg+xml")

	colors := map[string]string{
		"title_color":  titleColor,
		"text_color":   textColor,
		"bg_color":     bgColor,
		"border_color": borderColor,
		"theme":        theme,
	}

	accessResult := common.GuardAccess(c.Writer, username, "username", colors)
	if !accessResult.IsPassed {
		c.String(http.StatusForbidden, accessResult.Response)
		return
	}

	svg, err := BuildCompositeSVG(CompositeRequest{
		Username:          username,
		Repo:              repo,
		Cards:             cardsParam,
		Layout:            layout,
		Gap:               gap,
		Padding:           padding,
		FontFamily:        fontFamily,
		Theme:             theme,
		TitleColor:        titleColor,
		TextColor:         textColor,
		IconColor:         iconColor,
		BgColor:           bgColor,
		BorderColor:       borderColor,
		Locale:            locale,
		BorderRadius:      borderRadius,
		DisableAnimations: disableAnimations,
		CardWidth:         c.Query("card_width"),
		Hide:              c.Query("hide"),
		HideTitle:         c.Query("hide_title"),
		HideBorder:        c.Query("hide_border"),
		LangsCount:        c.Query("langs_count"),
		ShowIcons:         c.Query("show_icons"),
		HideRank:          c.Query("hide_rank"),
		IncludeAllCommits: c.Query("include_all_commits"),
	})
	if err != nil {
		common.SetErrorCacheHeaders(c.Writer)
		c.String(compositeErrorStatus(err), renderCompositeError(err, colors))
		return
	}

	cacheSeconds := common.ResolveCacheSeconds(
		0,
		common.StatsCardTTL.Default,
		common.StatsCardTTL.Min,
		common.StatsCardTTL.Max,
	)
	common.SetCacheHeaders(c.Writer, cacheSeconds)
	c.String(http.StatusOK, svg)
}

// CompositeRequest holds query values for a composite card request.
type CompositeRequest struct {
	Username          string
	Repo              string
	Cards             string
	Layout            string
	Gap               string
	Padding           string
	FontFamily        string
	Theme             string
	TitleColor        string
	TextColor         string
	IconColor         string
	BgColor           string
	BorderColor       string
	Locale            string
	BorderRadius      string
	DisableAnimations string
	CardWidth         string
	Hide              string
	HideTitle         string
	HideBorder        string
	LangsCount        string
	ShowIcons         string
	HideRank          string
	IncludeAllCommits string
}

// BuildCompositeSVG validates a composite request, renders each selected card
// and assembles them. A fetch failure for an individual card is reported as an
// error; a missing required parameter is reported before any card is built.
func BuildCompositeSVG(request CompositeRequest) (string, error) {
	kinds, err := parseCompositeCards(request.Cards)
	if err != nil {
		return "", err
	}

	fontFamily, ok := common.SanitizeFontFamily(request.FontFamily)
	if !ok {
		return "", &cards.CompositeError{
			Message:          "Invalid font",
			SecondaryMessage: "Use letters, digits, spaces, dots, hyphens or underscores (max 64 characters)",
		}
	}

	gap, err := parseCompositeNumber(request.Gap, cards.CompositeDefaultGap, cards.CompositeMaxGap, "gap")
	if err != nil {
		return "", err
	}
	padding, err := parseCompositeNumber(request.Padding, cards.CompositeDefaultPadding, cards.CompositeMaxPadding, "padding")
	if err != nil {
		return "", err
	}

	locale := strings.ToLower(strings.TrimSpace(request.Locale))
	if locale != "" && !translations.IsLocaleAvailable(locale) {
		return "", &cards.CompositeError{
			Message:          "Something went wrong",
			SecondaryMessage: "Language not found",
		}
	}

	borderRadiusFloat := 4.5
	if request.BorderRadius != "" {
		if val, err := strconv.ParseFloat(request.BorderRadius, 64); err == nil {
			borderRadiusFloat = val
		}
	}

	var needStats, needPin, needTopLangs bool
	for _, kind := range kinds {
		switch kind {
		case CompositeKindStats:
			needStats = true
		case CompositeKindPin:
			needPin = true
		case CompositeKindTopLangs:
			needTopLangs = true
		}
	}

	if request.Username == "" {
		return "", &cards.CompositeError{
			Message:          `Missing params "username" make sure you pass the parameters in URL`,
			SecondaryMessage: "/api/composite?username=USERNAME&repo=REPO&cards=stats,top-langs",
		}
	}
	if needPin && request.Repo == "" {
		return "", &cards.CompositeError{
			Message:          `Missing params "repo" make sure you pass the parameters in URL`,
			SecondaryMessage: "/api/composite?username=USERNAME&repo=REPO&cards=stats,top-langs",
		}
	}

	var stats *fetchers.StatsData
	var repoData *fetchers.RepoData
	var langs map[string]*fetchers.LanguageStats

	if needStats {
		stats, err = fetchers.FetchStats(
			request.Username,
			common.ParseBoolean(request.IncludeAllCommits) != nil && *common.ParseBoolean(request.IncludeAllCommits),
			nil,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			return "", compositeFetchError("stats", err)
		}
	}
	if needPin {
		repoData, err = fetchers.FetchRepo(request.Username, request.Repo)
		if err != nil {
			return "", compositeFetchError("pin", err)
		}
	}
	if needTopLangs {
		langs, err = fetchers.FetchTopLanguages(request.Username, nil, 1, 0)
		if err != nil {
			return "", compositeFetchError("top-langs", err)
		}
	}

	compositeCards := make([]cards.CompositeCard, 0, len(kinds))
	for _, kind := range kinds {
		var svg string
		switch kind {
		case CompositeKindStats:
			svg = cards.RenderStatsCard(stats, cards.StatsCardOptions{
				Hide:              common.ParseArray(request.Hide),
				ShowIcons:         flagValue(request.ShowIcons),
				HideTitle:         flagValue(request.HideTitle),
				HideBorder:        flagValue(request.HideBorder),
				HideRank:          flagValue(request.HideRank),
				IncludeAllCommits: flagValue(request.IncludeAllCommits),
				TitleColor:        request.TitleColor,
				IconColor:         request.IconColor,
				TextColor:         request.TextColor,
				BgColor:           request.BgColor,
				BorderColor:       request.BorderColor,
				Theme:             request.Theme,
				BorderRadius:      borderRadiusFloat,
				Locale:            locale,
				DisableAnimations: flagValue(request.DisableAnimations),
				CardWidth:         intValue(request.CardWidth),
				TruncateTitle:     true,
			})
		case CompositeKindPin:
			svg = cards.RenderRepoCard(repoData, cards.RepoCardOptions{
				HideBorder:    flagValue(request.HideBorder),
				TitleColor:    request.TitleColor,
				IconColor:     request.IconColor,
				TextColor:     request.TextColor,
				BgColor:       request.BgColor,
				BorderColor:   request.BorderColor,
				Theme:         request.Theme,
				BorderRadius:  borderRadiusFloat,
				Locale:        locale,
				TruncateTitle: true,
			})
		case CompositeKindTopLangs:
			svg = cards.RenderTopLanguages(langs, cards.TopLanguagesCardOptions{
				Hide:              common.ParseArray(request.Hide),
				HideTitle:         flagValue(request.HideTitle),
				HideBorder:        flagValue(request.HideBorder),
				CardWidth:         intValue(request.CardWidth),
				TitleColor:        request.TitleColor,
				TextColor:         request.TextColor,
				BgColor:           request.BgColor,
				Theme:             request.Theme,
				BorderColor:       request.BorderColor,
				LangsCount:        intValue(request.LangsCount),
				Locale:            locale,
				BorderRadius:      borderRadiusFloat,
				DisableAnimations: flagValue(request.DisableAnimations),
				TruncateTitle:     true,
			})
		}
		compositeCards = append(compositeCards, cards.CompositeCard{Kind: kind, SVG: svg})
	}

	result, err := cards.RenderComposite(compositeCards, cards.CompositeOptions{
		Layout:     request.Layout,
		Gap:        gap,
		Padding:    padding,
		FontFamily: fontFamily,
	})
	if err != nil {
		return "", err
	}
	return result.SVG, nil
}

func compositeFetchError(kind string, err error) error {
	secondary := common.RetrieveSecondaryMessage(err)
	if secondary == "" || secondary == err.Error() {
		secondary = err.Error()
	}
	return &cards.CompositeError{
		Message:          "Card data unavailable: " + kind,
		SecondaryMessage: secondary,
		Kind:             cards.CompositeErrorData,
	}
}

// parseCompositeCards parses the cards query parameter (comma separated) and
// rejects duplicates, unknown kinds and invalid counts.
func parseCompositeCards(value string) ([]string, error) {
	raw := strings.Split(strings.TrimSpace(value), ",")
	var kinds []string
	seen := map[string]bool{}
	for _, item := range raw {
		kind := strings.ToLower(strings.TrimSpace(item))
		if kind == "" {
			continue
		}
		switch kind {
		case CompositeKindStats, CompositeKindPin, CompositeKindTopLangs:
		default:
			return nil, &cards.CompositeError{
				Message:          "Invalid cards selection",
				SecondaryMessage: fmt.Sprintf("Unknown card %q; supported: stats, pin, top-langs", kind),
			}
		}
		if seen[kind] {
			return nil, &cards.CompositeError{
				Message:          "Invalid cards selection",
				SecondaryMessage: fmt.Sprintf("Card %q was selected more than once", kind),
			}
		}
		seen[kind] = true
		kinds = append(kinds, kind)
	}

	if len(kinds) < cards.CompositeMinCards || len(kinds) > cards.CompositeMaxCards {
		return nil, &cards.CompositeError{
			Message:          "Invalid cards selection",
			SecondaryMessage: fmt.Sprintf("Select %d to %d distinct cards", cards.CompositeMinCards, cards.CompositeMaxCards),
		}
	}
	return kinds, nil
}

func parseCompositeNumber(value string, defaultValue, maxValue float64, name string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 || parsed > maxValue {
		return 0, &cards.CompositeError{
			Message:          fmt.Sprintf("Invalid %s", name),
			SecondaryMessage: fmt.Sprintf("%s must be a number between 0 and %d", name, int(maxValue)),
		}
	}
	return parsed, nil
}

// compositeErrorStatus maps a request validation failure to 400 and a data
// fetch failure or other server-side issue to 200, matching the existing card
// error endpoints which return an SVG with a 200 status.
func compositeErrorStatus(err error) int {
	compositeErr, isComposite := err.(*cards.CompositeError)
	if !isComposite || compositeErr.Kind == cards.CompositeErrorData {
		return http.StatusOK
	}
	return http.StatusBadRequest
}

func renderCompositeError(err error, colors map[string]string) string {
	secondary := common.RetrieveSecondaryMessage(err)
	if compositeErr, ok := err.(*cards.CompositeError); ok {
		secondary = compositeErr.SecondaryMessage
	}
	return common.RenderError(common.ErrorOptions{
		Message:          err.Error(),
		SecondaryMessage: secondary,
		RenderOptions:    colors,
		ShowRepoLink:     false,
	})
}

func flagValue(value string) bool {
	parsed := common.ParseBoolean(value)
	return parsed != nil && *parsed
}

func intValue(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}
