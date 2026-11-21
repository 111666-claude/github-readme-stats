package fetchers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeGitHub starts a hermetic GraphQL endpoint and points the fetch layer at
// it. Queries are answered from a handler so each test can shape the payload.
func newFakeGitHub(t *testing.T, handler func(query string) (int, string)) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(body, &request)
		status, payload := handler(request.Query)
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)

	t.Setenv("GITHUB_API_URL", server.URL)
	t.Setenv("PAT_1", "test-token")
}

func TestFetchStats_ParsesFakeResponse(t *testing.T) {
	newFakeGitHub(t, func(query string) (int, string) {
		if !strings.Contains(query, "contributionsCollection") {
			return http.StatusBadRequest, `{"errors":[{"type":"BAD_QUERY","message":"unexpected"}]}`
		}
		return http.StatusOK, `{
  "data": {"user": {
    "name": "Octo Cat", "login": "octocat",
    "commits": {"totalCommitContributions": 123},
    "reviews": {"totalPullRequestReviewContributions": 4},
    "repositoriesContributedTo": {"totalCount": 5},
    "pullRequests": {"totalCount": 6},
    "openIssues": {"totalCount": 2},
    "closedIssues": {"totalCount": 3},
    "followers": {"totalCount": 10},
    "repositories": {
      "totalCount": 2,
      "nodes": [
        {"name": "alpha", "stargazers": {"totalCount": 700}},
        {"name": "beta", "stargazers": {"totalCount": 300}}
      ],
      "pageInfo": {"hasNextPage": false, "endCursor": ""}
    }
  }}
}`
	})

	stats, err := FetchStats("octocat", false, nil, false, false, false, nil)
	if err != nil {
		t.Fatalf("FetchStats: %v", err)
	}
	if stats.Name != "Octo Cat" {
		t.Errorf("Name = %q", stats.Name)
	}
	if stats.TotalStars != 1000 {
		t.Errorf("TotalStars = %d, want 1000", stats.TotalStars)
	}
	if stats.TotalIssues != 5 {
		t.Errorf("TotalIssues = %d, want 5", stats.TotalIssues)
	}
	if stats.Rank.Level == "" {
		t.Error("rank level not computed")
	}
}

func TestFetchStats_NotFoundIsReportedAsUserNotFound(t *testing.T) {
	newFakeGitHub(t, func(string) (int, string) {
		return http.StatusOK, `{"errors":[{"type":"NOT_FOUND","message":"Could not resolve to a User"}]}`
	})

	_, err := FetchStats("missing", false, nil, false, false, false, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	custom, ok := err.(interface{ Error() string })
	if !ok || !strings.Contains(custom.Error(), "Could not resolve") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchTopLanguages_ParsesFakeResponse(t *testing.T) {
	newFakeGitHub(t, func(string) (int, string) {
		return http.StatusOK, `{
  "data": {"user": {"repositories": {"nodes": [
    {"name": "alpha", "languages": {"edges": [
      {"size": 5000, "node": {"color": "#00ADD8", "name": "Go"}},
      {"size": 2000, "node": {"color": "#f1e05a", "name": "JavaScript"}}
    ]}},
    {"name": "beta", "languages": {"edges": [
      {"size": 1000, "node": {"color": "#563d7c", "name": "CSS"}}
    ]}}
  ]}}}
}`
	})

	langs, err := FetchTopLanguages("octocat", nil, 1, 0)
	if err != nil {
		t.Fatalf("FetchTopLanguages: %v", err)
	}
	if len(langs) != 3 {
		t.Fatalf("language count = %d, want 3", len(langs))
	}
	if langs["Go"].Size != 5000 {
		t.Errorf("Go size = %d", langs["Go"].Size)
	}
}

func TestFetchRepo_ParsesFakeResponseAndTruncationInput(t *testing.T) {
	longName := strings.Repeat("long-name-", 10)
	newFakeGitHub(t, func(query string) (int, string) {
		if !strings.Contains(query, "getRepo") {
			return http.StatusBadRequest, `{"errors":[{"type":"BAD_QUERY","message":"unexpected"}]}`
		}
		return http.StatusOK, `{
  "data": {
    "user": {"repository": {
      "name": "` + longName + `",
      "nameWithOwner": "octocat/` + longName + `",
      "stargazers": {"totalCount": 900},
      "description": "A repository with a very long name.",
      "primaryLanguage": {"color": "#00ADD8", "id": "go", "name": "Go"},
      "forkCount": 120
    }},
    "organization": {"repository": null}
  }
}`
	})

	repo, err := FetchRepo("octocat", longName)
	if err != nil {
		t.Fatalf("FetchRepo: %v", err)
	}
	if repo.Name != longName {
		t.Errorf("repo name not preserved by fetcher")
	}
	if repo.Stargazers.TotalCount != 900 {
		t.Errorf("stars = %d", repo.Stargazers.TotalCount)
	}
}
