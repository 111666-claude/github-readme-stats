package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeGitHubResponse selects a canned GraphQL payload based on the markers that
// appear in the outgoing query. It lets the real fetchers exercise their parsing
// and error handling against a hermetic server.
type fakeGitHubResponse struct {
	status int
	body   string
}

func setupFakeGitHub(t *testing.T, respond func(query string) fakeGitHubResponse) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(body, &request)

		result := respond(request.Query)
		w.Header().Set("Content-Type", "application/json")
		if result.status != 0 {
			w.WriteHeader(result.status)
		}
		_, _ = w.Write([]byte(result.body))
	}))
	t.Cleanup(server.Close)

	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("PAT_1", "test-token")
}

const fakeStatsData = `{
  "data": {
    "user": {
      "name": "Octo Cat",
      "login": "octocat",
      "commits": {"totalCommitContributions": 123},
      "reviews": {"totalPullRequestReviewContributions": 4},
      "repositoriesContributedTo": {"totalCount": 5},
      "pullRequests": {"totalCount": 6},
      "mergedPullRequests": {"totalCount": 0},
      "openIssues": {"totalCount": 2},
      "closedIssues": {"totalCount": 3},
      "followers": {"totalCount": 10},
      "repositoryDiscussions": {"totalCount": 0},
      "repositoryDiscussionComments": {"totalCount": 0},
      "repositories": {
        "totalCount": 2,
        "nodes": [
          {"name": "alpha", "stargazers": {"totalCount": 700}},
          {"name": "beta", "stargazers": {"totalCount": 300}}
        ],
        "pageInfo": {"hasNextPage": false, "endCursor": ""}
      }
    }
  }
}`

const fakeRepoData = `{
  "data": {
    "user": {
      "repository": {
        "name": "github-readme-stats",
        "nameWithOwner": "octocat/github-readme-stats",
        "isArchived": false,
        "isTemplate": false,
        "stargazers": {"totalCount": 900},
        "description": "Dynamically generated stats for your READMEs.",
        "primaryLanguage": {"color": "#00ADD8", "id": "go", "name": "Go"},
        "forkCount": 120
      }
    },
    "organization": {"repository": null}
  }
}`

const fakeTopLangsData = `{
  "data": {
    "user": {
      "repositories": {
        "nodes": [
          {"name": "alpha", "languages": {"edges": [
            {"size": 5000, "node": {"color": "#00ADD8", "name": "Go"}},
            {"size": 2000, "node": {"color": "#f1e05a", "name": "JavaScript"}}
          ]}},
          {"name": "beta", "languages": {"edges": [
            {"size": 1000, "node": {"color": "#563d7c", "name": "CSS"}}
          ]}}
        ]
      }
    }
  }
}`

func defaultFakeGitHub(query string) fakeGitHubResponse {
	switch {
	case strings.Contains(query, "getRepo"):
		return fakeGitHubResponse{body: fakeRepoData}
	case strings.Contains(query, "languages("):
		return fakeGitHubResponse{body: fakeTopLangsData}
	case strings.Contains(query, "contributionsCollection"):
		return fakeGitHubResponse{body: fakeStatsData}
	default:
		return fakeGitHubResponse{status: http.StatusBadRequest, body: `{"errors":[{"type":"BAD_QUERY","message":"unexpected query"}]}`}
	}
}

func newCompositeTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api", StatsHandler)
	router.GET("/api/pin", PinHandler)
	router.GET("/api/top-langs", TopLangsHandler)
	router.GET("/api/composite", CompositeHandler)
	return router
}

func performCompositeRequest(t *testing.T, router *gin.Engine, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/composite?"+query, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestCompositeHandler_HorizontalThreeCards(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&repo=github-readme-stats&cards=stats,pin,top-langs&layout=horizontal&gap=24&padding=10&theme=dark")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q", got)
	}
	svg := response.Body.String()

	for _, want := range []string{
		"Octo Cat's GitHub Stats",
		"github-readme-stats",
		"Most Used Languages",
		`data-card="stats"`,
		`data-card="pin"`,
		`data-card="top-langs"`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("composite SVG missing %q", want)
		}
	}

	// Horizontal placement: first card at padding, the second offset by the
	// first card's width plus the gap, and the third after that.
	statsW := extractCardAttribute(t, svg, "stats", "data-width")
	pinW := extractCardAttribute(t, svg, "pin", "data-width")
	if got := extractCardAttribute(t, svg, "stats", "data-x"); got != 10 {
		t.Errorf("stats x = %v, want 10", got)
	}
	if got := extractCardAttribute(t, svg, "pin", "data-x"); got != statsW+24+10 {
		t.Errorf("pin x = %v, want %v", got, statsW+24+10)
	}
	if got := extractCardAttribute(t, svg, "top-langs", "data-x"); got != statsW+24+pinW+24+10 {
		t.Errorf("top-langs x = %v, want %v", got, statsW+24+pinW+24+10)
	}

	// 3 cards, two 24px gaps and 10px padding on both sides.
	wantWidth := formatFloat(statsW + pinW + extractCardAttribute(t, svg, "top-langs", "data-width") + 48 + 20)
	if !strings.Contains(svg, `width="`+wantWidth+`"`) {
		t.Errorf("composite width missing want %s", wantWidth)
	}
}

func TestCompositeHandler_VerticalLayout(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&cards=stats,top-langs&layout=vertical&gap=12")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	svg := response.Body.String()

	if !strings.Contains(svg, `data-card="stats"`) || !strings.Contains(svg, `data-card="top-langs"`) {
		t.Fatal("expected both cards")
	}
	if extractCardAttribute(t, svg, "top-langs", "data-y") == extractCardAttribute(t, svg, "stats", "data-y") {
		t.Error("vertical layout should stack cards on different y offsets")
	}
}

func TestCompositeHandler_OriginalEndpointsUnchanged(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	for path, want := range map[string]string{
		"/api?username=octocat":                              "Octo Cat's GitHub Stats",
		"/api/top-langs?username=octocat":                    "Most Used Languages",
		"/api/pin?username=octocat&repo=github-readme-stats": "github-readme-stats",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("%s status = %d", path, recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("%s missing %q", path, want)
		}
	}
}

func TestCompositeHandler_InvalidLayout(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&cards=stats,top-langs&layout=diagonal")
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Invalid layout") {
		t.Errorf("body missing layout error: %s", body)
	}
	if !strings.Contains(body, "Supported layouts: horizontal, vertical") {
		t.Errorf("body missing layout guidance: %s", body)
	}
}

func TestCompositeHandler_InvalidCardSelection(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	for name, query := range map[string]string{
		"single card":     "username=octocat&cards=stats",
		"too many cards":  "username=octocat&cards=stats,pin,top-langs,stats",
		"unknown card":    "username=octocat&cards=stats,gist",
		"duplicate card":  "username=octocat&cards=stats,stats",
		"empty selection": "username=octocat&cards=",
	} {
		response := performCompositeRequest(t, router, query)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, response.Code)
		}
		if !strings.Contains(response.Body.String(), "Invalid cards selection") {
			t.Errorf("%s: missing selection error: %s", name, response.Body.String())
		}
	}
}

func TestCompositeHandler_InvalidGapPaddingFont(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	for name, query := range map[string]string{
		"negative gap":   "username=octocat&cards=stats,top-langs&gap=-1",
		"huge gap":       "username=octocat&cards=stats,top-langs&gap=1000",
		"non-number gap": "username=octocat&cards=stats,top-langs&gap=wide",
		"negative pad":   "username=octocat&cards=stats,top-langs&padding=-2",
		"css injection":  "username=octocat&cards=stats,top-langs&font=Arial%27%3Bfill%3Ared%3Bx%3A%27",
		"overlong font":  "username=octocat&cards=stats,top-langs&font=" + strings.Repeat("A", 65),
	} {
		response := performCompositeRequest(t, router, query)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, response.Code)
		}
	}
}

func TestCompositeHandler_OversizedCanvasRejected(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	// card_width is applied to the stats and top-langs cards; two 2500px cards
	// side by side exceed the 4096px canvas limit, and a single card wider than
	// the limit does too.
	for name, query := range map[string]string{
		"wide horizontal":  "username=octocat&repo=repo&cards=stats,top-langs&layout=horizontal&card_width=2500",
		"wide single card": "username=octocat&cards=stats,top-langs&layout=vertical&card_width=4200",
	} {
		response := performCompositeRequest(t, router, query)
		if response.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, response.Code)
		}
		if !strings.Contains(response.Body.String(), "Composite canvas too large") {
			t.Errorf("%s: body = %s", name, response.Body.String())
		}
	}
}

func TestCompositeHandler_InvalidLocale(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&cards=stats,top-langs&locale=not-a-locale")
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Language not found") {
		t.Errorf("body = %s", response.Body.String())
	}
}

func TestCompositeHandler_MissingRequiredParams(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	for name, query := range map[string]string{
		"missing username": "cards=stats,top-langs",
		"missing repo":     "username=octocat&cards=stats,pin",
	} {
		response := performCompositeRequest(t, router, query)
		if !strings.Contains(response.Body.String(), "Missing params") {
			t.Errorf("%s: body = %s", name, response.Body.String())
		}
	}
}

func TestCompositeHandler_CardDataUnavailable(t *testing.T) {
	setupFakeGitHub(t, func(query string) fakeGitHubResponse {
		if strings.Contains(query, "languages(") {
			return fakeGitHubResponse{body: `{"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a User"}]}`}
		}
		return defaultFakeGitHub(query)
	})
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&cards=stats,top-langs&layout=horizontal")
	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 with an error card", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, "Card data unavailable: top-langs") {
		t.Errorf("expected per-card failure message, got: %s", body)
	}
	if !strings.Contains(body, "Make sure the provided username is not an organization") {
		t.Errorf("expected upstream secondary message, got: %s", body)
	}
}

func TestCompositeHandler_LongRepositoryNameTruncates(t *testing.T) {
	longName := strings.Repeat("very-long-repository-name-", 6)
	setupFakeGitHub(t, func(query string) fakeGitHubResponse {
		if strings.Contains(query, "getRepo") {
			body := strings.Replace(fakeRepoData, "github-readme-stats", longName, 2)
			return fakeGitHubResponse{body: body}
		}
		return defaultFakeGitHub(query)
	})
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&repo=repo&cards=stats,pin")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	svg := response.Body.String()
	if !strings.Contains(svg, "...") {
		t.Error("expected long repository name to be truncated with an ellipsis")
	}
}

func TestCompositeHandler_FontFamilyApplied(t *testing.T) {
	setupFakeGitHub(t, defaultFakeGitHub)
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router,
		"username=octocat&cards=stats,top-langs&font=Fira+Code")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "font-family: 'Fira Code'") {
		t.Error("valid font family was not applied")
	}
}

func TestCompositeHandler_NoTokens(t *testing.T) {
	// No PAT means the fetch layer fails; the composite should degrade to an
	// explicit data-unavailable error rather than panicking.
	os.Unsetenv("PAT_1")
	t.Setenv("GITHUB_API_URL", "http://127.0.0.1:1")
	router := newCompositeTestRouter()

	response := performCompositeRequest(t, router, "username=octocat&cards=stats,top-langs")
	if response.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 error card", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Card data unavailable") {
		t.Errorf("expected data unavailable message, got %s", response.Body.String())
	}
}

func TestSingleCardEndpoint_DoesNotTruncateLongTitle(t *testing.T) {
	longName := strings.Repeat("very-long-repository-name-", 6)
	setupFakeGitHub(t, func(query string) fakeGitHubResponse {
		if strings.Contains(query, "getRepo") {
			return fakeGitHubResponse{body: strings.Replace(fakeRepoData, "github-readme-stats", longName, 2)}
		}
		return defaultFakeGitHub(query)
	})
	router := newCompositeTestRouter()

	request := httptest.NewRequest(http.MethodGet, "/api/pin?username=octocat&repo=repo", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "...") {
		t.Error("original single-card endpoint must not truncate the title")
	}
}

func extractCardAttribute(t *testing.T, svg, kind, attribute string) float64 {
	t.Helper()
	marker := `data-card="` + kind + `"`
	index := strings.Index(svg, marker)
	if index < 0 {
		t.Fatalf("card %q not found", kind)
	}
	// Walk back to the start of the containing group.
	start := strings.LastIndex(svg[:index], "<g")
	end := strings.Index(svg[index:], ">") + index
	group := svg[start:end]
	attributeMarker := attribute + `="`
	attrIndex := strings.Index(group, attributeMarker)
	if attrIndex < 0 {
		t.Fatalf("attribute %q not found on card %q", attribute, kind)
	}
	valueStart := attrIndex + len(attributeMarker)
	valueEnd := strings.Index(group[valueStart:], `"`) + valueStart
	value := group[valueStart:valueEnd]
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		t.Fatalf("could not parse %q as float: %v", value, err)
	}
	return parsed
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
