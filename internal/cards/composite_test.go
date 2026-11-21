package cards

import (
	"fmt"
	"strings"
	"testing"

	"github.com/soulteary/github-readme-stats/internal/fetchers"
)

func testStatsCard(t *testing.T, theme, bgColor string) string {
	t.Helper()
	stats := &fetchers.StatsData{
		Name:          "Octo Cat",
		TotalStars:    1234,
		TotalCommits:  987,
		TotalPRs:      42,
		TotalIssues:   7,
		ContributedTo: 11,
		Rank:          fetchers.RankData{Level: "A", Percentile: 12.5},
	}
	return RenderStatsCard(stats, StatsCardOptions{
		ShowIcons:    true,
		Theme:        theme,
		BgColor:      bgColor,
		NumberFormat: "short",
	})
}

func testTopLangsCard(t *testing.T, theme string) string {
	t.Helper()
	langs := map[string]*fetchers.LanguageStats{
		"Go":         {Name: "Go", Size: 5000, Color: "#00ADD8"},
		"JavaScript": {Name: "JavaScript", Size: 3000, Color: "#f1e05a"},
		"CSS":        {Name: "CSS", Size: 1000, Color: "#563d7c"},
	}
	return RenderTopLanguages(langs, TopLanguagesCardOptions{Theme: theme})
}

func testRepoCard(t *testing.T, theme string) string {
	t.Helper()
	repo := &fetchers.RepoData{
		Name:          "github-readme-stats",
		NameWithOwner: "octocat/github-readme-stats",
		Description:   "Dynamically generated stats for your READMEs.",
		PrimaryLanguage: &fetchers.LanguageData{
			Name:  "Go",
			Color: "#00ADD8",
		},
		Stargazers: fetchers.StargazersData{TotalCount: 100},
		Forks:      fetchers.ForksData{TotalCount: 20},
	}
	return RenderRepoCard(repo, RepoCardOptions{Theme: theme, Locale: "en"})
}

func TestRenderComposite_HorizontalDimensionsUseMaxHeight(t *testing.T) {
	statsSVG := testStatsCard(t, "dark", "")
	langsSVG := testTopLangsCard(t, "dark")

	statsW, statsH, err := parseSVGDimensions(statsSVG)
	if err != nil {
		t.Fatalf("stats dimensions: %v", err)
	}
	langsW, langsH, err := parseSVGDimensions(langsSVG)
	if err != nil {
		t.Fatalf("languages dimensions: %v", err)
	}

	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: statsSVG},
		{Kind: "top-langs", SVG: langsSVG},
	}, CompositeOptions{Layout: CompositeLayoutHorizontal, Gap: 20})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}

	wantWidth := statsW + langsW + 20
	wantHeight := statsH
	if langsH > wantHeight {
		wantHeight = langsH
	}
	if result.Width != wantWidth {
		t.Errorf("width = %v, want %v", result.Width, wantWidth)
	}
	if result.Height != wantHeight {
		t.Errorf("height = %v, want %v", result.Height, wantHeight)
	}
	if !strings.Contains(result.SVG, fmt.Sprintf(`width="%.0f"`, wantWidth)) {
		t.Errorf("composite SVG root width missing want %v", wantWidth)
	}
}

func TestRenderComposite_VerticalDimensionsUseMaxWidth(t *testing.T) {
	statsSVG := testStatsCard(t, "default", "")
	repoSVG := testRepoCard(t, "default")

	statsW, statsH, _ := parseSVGDimensions(statsSVG)
	repoW, repoH, _ := parseSVGDimensions(repoSVG)

	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: statsSVG},
		{Kind: "pin", SVG: repoSVG},
	}, CompositeOptions{Layout: CompositeLayoutVertical, Gap: 12, Padding: 8})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}

	wantWidth := statsW
	if repoW > wantWidth {
		wantWidth = repoW
	}
	wantWidth += 16
	wantHeight := statsH + repoH + 12 + 16
	if result.Width != wantWidth {
		t.Errorf("width = %v, want %v", result.Width, wantWidth)
	}
	if result.Height != wantHeight {
		t.Errorf("height = %v, want %v", result.Height, wantHeight)
	}
}

func TestRenderComposite_PreservesTitlesAndDataAcrossCards(t *testing.T) {
	statsSVG := testStatsCard(t, "dark", "")
	langsSVG := testTopLangsCard(t, "dark")

	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: statsSVG},
		{Kind: "top-langs", SVG: langsSVG},
	}, CompositeOptions{Layout: CompositeLayoutHorizontal})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}

	// Sub-card titles and per-card data must survive composition verbatim.
	for _, want := range []string{
		"Octo Cat's GitHub Stats",
		"Most Used Languages",
		`data-testid="stars"`,
		`data-testid="lang-name"`,
	} {
		if !strings.Contains(result.SVG, want) {
			t.Errorf("composite output missing %q", want)
		}
	}
	if strings.Count(result.SVG, "data-testid=\"card-title\"") != 2 {
		t.Errorf("expected one title per sub-card, got %d", strings.Count(result.SVG, `data-testid="card-title"`))
	}
	if strings.Count(result.SVG, `data-testid="card-bg"`) != 2 {
		t.Errorf("expected one card background per sub-card")
	}
}

func TestRenderComposite_NamespacesGradientIDs(t *testing.T) {
	// Both cards use a gradient background, which gives both id="gradient".
	statsSVG := testStatsCard(t, "dark", "20,000000,111111")
	langsSVG := testTopLangsCard(t, "dark")

	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: statsSVG},
		{Kind: "top-langs", SVG: langsSVG},
	}, CompositeOptions{Layout: CompositeLayoutHorizontal})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}

	if strings.Count(result.SVG, `id="gradient"`) != 0 {
		t.Errorf("un-namespaced gradient id leaked into composite output")
	}
	for _, want := range []string{`id="cc0-gradient"`, `url(#cc0-gradient)`} {
		if !strings.Contains(result.SVG, want) {
			t.Errorf("composite output missing %q", want)
		}
	}
	// The second card has no gradient, but every unique id should be namespaced.
	for _, badID := range []string{`id="titleId"`, `id="descId"`} {
		if strings.Count(result.SVG, badID) > 2 {
			t.Errorf("duplicate un-namespaced %s found", badID)
		}
	}
}

func TestRenderComposite_NamespacingDoesNotTouchText(t *testing.T) {
	statsSVG := testStatsCard(t, "dark", "")
	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: statsSVG},
		{Kind: "top-langs", SVG: testTopLangsCard(t, "dark")},
	}, CompositeOptions{Layout: CompositeLayoutVertical})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}
	if !strings.Contains(result.SVG, ">Total Stars:</text>") {
		t.Errorf("visible label text was rewritten during namespacing")
	}
	if !strings.Contains(result.SVG, ">Go</text>") {
		t.Errorf("language name text was rewritten during namespacing")
	}
}

func TestRenderComposite_InvalidLayout(t *testing.T) {
	card := CompositeCard{Kind: "stats", SVG: testStatsCard(t, "dark", "")}
	_, err := RenderComposite([]CompositeCard{card, card}, CompositeOptions{Layout: "diagonal"})
	if err == nil {
		t.Fatal("expected invalid layout error")
	}
	var compositeErr *CompositeError
	if !asCompositeError(err, &compositeErr) || compositeErr.Message != "Invalid layout" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRenderComposite_InvalidGapAndPadding(t *testing.T) {
	card := CompositeCard{Kind: "stats", SVG: testStatsCard(t, "dark", "")}
	for _, options := range []CompositeOptions{
		{Layout: CompositeLayoutHorizontal, Gap: -1},
		{Layout: CompositeLayoutHorizontal, Gap: CompositeMaxGap + 1},
		{Layout: CompositeLayoutHorizontal, Padding: -5},
		{Layout: CompositeLayoutHorizontal, Padding: CompositeMaxPadding + 1},
	} {
		if _, err := RenderComposite([]CompositeCard{card, card}, options); err == nil {
			t.Errorf("expected error for options %+v", options)
		}
	}
}

func TestRenderComposite_RejectsTooFewAndTooManyCards(t *testing.T) {
	card := CompositeCard{Kind: "stats", SVG: testStatsCard(t, "dark", "")}
	if _, err := RenderComposite([]CompositeCard{card}, CompositeOptions{}); err == nil {
		t.Error("expected error for one card")
	}
	if _, err := RenderComposite([]CompositeCard{card, card, card, card}, CompositeOptions{}); err == nil {
		t.Error("expected error for four cards")
	}
}

func TestRenderComposite_RejectsOversizedCanvas(t *testing.T) {
	// Each card is 400px wide; 3 cards can never exceed 4096, so force the
	// limit with a manually widened card.
	wide := `<svg width="3000" height="200" viewBox="0 0 3000 200" xmlns="http://www.w3.org/2000/svg"><rect width="3000" height="200"/></svg>`
	card := CompositeCard{Kind: "stats", SVG: wide}
	_, err := RenderComposite([]CompositeCard{card, card}, CompositeOptions{
		Layout: CompositeLayoutHorizontal,
		Gap:    50,
	})
	if err == nil {
		t.Fatal("expected oversized canvas error")
	}
	var compositeErr *CompositeError
	if !asCompositeError(err, &compositeErr) || compositeErr.Message != "Composite canvas too large" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRenderComposite_FontOverrideIsScoped(t *testing.T) {
	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: testStatsCard(t, "dark", "")},
		{Kind: "top-langs", SVG: testTopLangsCard(t, "dark")},
	}, CompositeOptions{Layout: CompositeLayoutHorizontal, FontFamily: "Fira Code"})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}
	if !strings.Contains(result.SVG, "font-family: 'Fira Code'") {
		t.Error("font override missing from composite styles")
	}
	if !strings.Contains(result.SVG, ".composite-card text") {
		t.Error("font override is not scoped to composite cards")
	}
}

func TestRenderComposite_NamespacesClipPathAndMaskIDs(t *testing.T) {
	card := func(label string) string {
		return `<svg width="200" height="100" viewBox="0 0 200 100" xmlns="http://www.w3.org/2000/svg">` +
			`<defs><clipPath id="clip"><rect width="10" height="10"/></clipPath>` +
			`<mask id="rect-mask"><rect width="10" height="10" fill="white"/></mask></defs>` +
			`<style>.header{fill:red}.rank-percentile-header{fill:blue}</style>` +
			`<text class="header" clip-path="url(#clip)">` + label + `</text>` +
			`<rect mask="url(#rect-mask)"/></svg>`
	}
	result, err := RenderComposite([]CompositeCard{
		{Kind: "stats", SVG: card("First")},
		{Kind: "pin", SVG: card("Second")},
	}, CompositeOptions{Layout: CompositeLayoutHorizontal})
	if err != nil {
		t.Fatalf("RenderComposite: %v", err)
	}

	for _, want := range []string{`clip-path="url(#cc0-clip)"`, `mask="url(#cc1-rect-mask)"`} {
		if !strings.Contains(result.SVG, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{`url(#clip)`, `url(#rect-mask)`} {
		if strings.Contains(result.SVG, bad) {
			t.Errorf("un-namespaced reference %q leaked", bad)
		}
	}
	if strings.Count(result.SVG, `id="cc0-clip"`) != 1 || strings.Count(result.SVG, `id="cc1-clip"`) != 1 {
		t.Error("clip ids not namespaced once per card")
	}
	// A longer class name that merely contains "header" must not be rewritten.
	if strings.Contains(result.SVG, ".cc0-rank-percentile-header") {
		t.Error("nested class selector was partially rewritten")
	}
	if !strings.Contains(result.SVG, ".rank-percentile-header") {
		t.Error("unrelated class selector was dropped")
	}
}

func asCompositeError(err error, target **CompositeError) bool {
	compositeErr, ok := err.(*CompositeError)
	if ok {
		*target = compositeErr
	}
	return ok
}
