package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTagGateRejectsNonTagCheckRun asserts the release gate accepts only a
// successful check from the tag push itself. A pull_request (or main) run
// whose head SHA happens to equal the tag commit must not open the gate.
func TestTagGateRejectsNonTagCheckRun(t *testing.T) {
	const sha = "abc123def456"
	raw := []byte(`{
		"name": "unit",
		"status": "completed",
		"conclusion": "success",
		"headSha": "` + sha + `",
		"event": "pull_request",
		"checkSuite": {"event": "pull_request"}
	}`)
	var run checkRun
	if err := json.Unmarshal(raw, &run); err != nil {
		t.Fatal(err)
	}
	err := evaluateChecks([]string{"unit"}, []checkRun{run}, sha)
	if err == nil {
		t.Fatal("evaluateChecks accepted a pull_request check run matched only by head SHA")
	}
}

// TestTagGateRejectsMainPush: a push to refs/heads/main for the same SHA
// is not the tag push.
func TestTagGateRejectsMainPush(t *testing.T) {
	const sha = "abc123def456"
	raw := []byte(`{
		"name": "unit",
		"status": "completed",
		"conclusion": "success",
		"headSha": "` + sha + `",
		"event": "push",
		"ref": "refs/heads/main"
	}`)
	var run checkRun
	if err := json.Unmarshal(raw, &run); err != nil {
		t.Fatal(err)
	}
	if err := evaluateChecks([]string{"unit"}, []checkRun{run}, sha); err == nil {
		t.Fatal("evaluateChecks accepted a push to refs/heads/main")
	}
}

// TestTagGateAcceptsTagPush is the guard: event=push and a refs/tags/ ref
// with a successful job counts. Passes before and after the gate change
// once the fields are present; before the change any success for the SHA counts.
func TestTagGateAcceptsTagPush(t *testing.T) {
	const sha = "abc123def456"
	raw := []byte(`{
		"name": "unit",
		"status": "completed",
		"conclusion": "success",
		"headSha": "` + sha + `",
		"event": "push",
		"ref": "refs/tags/v1.2.3"
	}`)
	var run checkRun
	if err := json.Unmarshal(raw, &run); err != nil {
		t.Fatal(err)
	}
	if err := evaluateChecks([]string{"unit"}, []checkRun{run}, sha); err != nil {
		t.Fatal(err)
	}
}

// TestTagGateRejectsEmptyEventRef: a completed success with no workflow
// event or ref does not count.
func TestTagGateRejectsEmptyEventRef(t *testing.T) {
	const sha = "abc123def456"
	run := checkRun{Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: sha}
	if err := evaluateChecks([]string{"unit"}, []checkRun{run}, sha); err == nil {
		t.Fatal("evaluateChecks accepted a check run with empty event and ref")
	}
}

// TestReleaseWorkflowDoesNotInterpolateTagIntoShell asserts tag and
// workflow_dispatch ref values are not pasted into a shell script.
func TestReleaseWorkflowDoesNotInterpolateTagIntoShell(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), `ref="${{`) {
		t.Fatal("release.yml interpolates the tag/ref inside a run: shell script (ref=\"${{\")")
	}
}

func TestReleaseWorkflowPassesTagAsEnv(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "RELEASE_REF:") {
		t.Fatal("release.yml does not pass the tag as RELEASE_REF")
	}
	if !strings.Contains(text, "RELEASE_TAG:") {
		t.Fatal("release.yml does not set RELEASE_TAG from the sanitized tag")
	}
}

func TestCIRunsOnVersionTags(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "tags:") || !strings.Contains(text, "v*") {
		t.Fatal("ci.yml push trigger does not include tags matching v*")
	}
}

func TestTagGateRejectsDifferentTag(t *testing.T) {
	const sha = "abc123def456"
	run := checkRun{
		Name: "unit", Status: "completed", Conclusion: "success", HeadSHA: sha,
		Event: "push", Ref: "refs/tags/v9.9.9",
	}
	if err := evaluateChecksForTag([]string{"unit"}, []checkRun{run}, sha, "v1.2.3"); err == nil {
		t.Fatal("evaluateChecksForTag accepted a different tag on the same SHA")
	}
	if err := evaluateChecks([]string{"unit"}, []checkRun{run}, sha); err != nil {
		t.Fatalf("3-arg evaluateChecks rejected a refs/tags ref: %v", err)
	}
}

func TestFetchSelectsTagPushCIRun(t *testing.T) {
	const (
		sha   = "abc123def456"
		tag   = "v1.2.3"
		token = "test-token"
		repo  = "hilather/go-lab-maildev"
	)
	t.Setenv("GITHUB_REPOSITORY", repo)
	jobsBody := greenJobsJSON()
	var jobsFor []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertGitHubHeaders(t, r, token)
		switch r.URL.Path {
		case "/repos/hilather/go-lab-maildev/actions/runs":
			if r.URL.Query().Get("head_sha") != sha || r.URL.Query().Get("event") != "push" || r.URL.Query().Get("per_page") != "100" {
				t.Errorf("runs query %s", r.URL.RawQuery)
			}
			fmt.Fprintf(w, `{"workflow_runs":[
				{"id":1,"path":".github/workflows/ci.yml","event":"pull_request","head_sha":%q,"head_branch":"feature","status":"completed","conclusion":"success","created_at":"2026-10-01T00:00:00Z"},
				{"id":2,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":"main","status":"completed","conclusion":"success","created_at":"2026-10-02T00:00:00Z"},
				{"id":9,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":"","status":"completed","conclusion":"success","created_at":"2026-10-03T00:00:00Z"},
				{"id":4,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":%q,"status":"completed","conclusion":"success","created_at":"2026-10-04T00:00:00Z"}
			]}`, sha, sha, sha, sha, tag)
		case "/repos/hilather/go-lab-maildev/actions/runs/4/jobs":
			jobsFor = append(jobsFor, "4")
			_, _ = w.Write([]byte(jobsBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	withGitHubAPI(t, srv.URL)

	runs, err := fetchTagCIJobs(token, repo, sha, tag)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobsFor) != 1 || jobsFor[0] != "4" {
		t.Fatalf("jobs fetched for %v, want only the tag run", jobsFor)
	}
	if err := evaluateChecksForTag(requiredCIJobs(), runs, sha, tag); err != nil {
		t.Fatal(err)
	}
}

func TestFetchRejectsNonTagRuns(t *testing.T) {
	const (
		sha   = "abc123def456"
		token = "test-token"
		repo  = "hilather/go-lab-maildev"
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertGitHubHeaders(t, r, token)
		if r.URL.Path != "/repos/hilather/go-lab-maildev/actions/runs" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"workflow_runs":[
			{"id":1,"path":".github/workflows/ci.yml","event":"pull_request","head_sha":%q,"head_branch":"feature","status":"completed","conclusion":"success","created_at":"2026-10-01T00:00:00Z"},
			{"id":2,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":"main","status":"completed","conclusion":"success","created_at":"2026-10-02T00:00:00Z"}
		]}`, sha, sha)
	}))
	t.Cleanup(srv.Close)
	withGitHubAPI(t, srv.URL)
	_, err := fetchTagCIJobs(token, repo, sha, "v1.2.3")
	if !errors.Is(err, errCIPending) {
		t.Fatalf("err=%v want pending when only pull_request and main runs exist", err)
	}
}

func TestFetchPendingAndTerminal(t *testing.T) {
	const (
		sha   = "abc123def456"
		tag   = "v1.2.3"
		token = "test-token"
		repo  = "hilather/go-lab-maildev"
	)
	t.Run("in progress", func(t *testing.T) {
		var jobs int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/jobs") {
				jobs++
			}
			fmt.Fprintf(w, `{"workflow_runs":[{"id":7,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":%q,"status":"in_progress","conclusion":"","created_at":"2026-10-04T00:00:00Z"}]}`, sha, tag)
		}))
		t.Cleanup(srv.Close)
		withGitHubAPI(t, srv.URL)
		_, err := fetchTagCIJobs(token, repo, sha, tag)
		if !errors.Is(err, errCIPending) {
			t.Fatalf("err=%v want pending", err)
		}
		if jobs != 0 {
			t.Fatalf("fetched jobs for an incomplete run: %d", jobs)
		}
	})
	t.Run("newer failure ignores older success", func(t *testing.T) {
		var jobsFor []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertGitHubHeaders(t, r, token)
			switch r.URL.Path {
			case "/repos/hilather/go-lab-maildev/actions/runs":
				fmt.Fprintf(w, `{"workflow_runs":[
					{"id":10,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":%q,"status":"completed","conclusion":"success","created_at":"2026-10-01T00:00:00Z"},
					{"id":11,"path":".github/workflows/ci.yml","event":"push","head_sha":%q,"head_branch":%q,"status":"completed","conclusion":"failure","created_at":"2026-10-02T00:00:00Z"}
				]}`, sha, tag, sha, tag)
			case "/repos/hilather/go-lab-maildev/actions/runs/11/jobs":
				jobsFor = append(jobsFor, "11")
				fmt.Fprintf(w, `{"jobs":[{"name":"unit","status":"completed","conclusion":"failure","completed_at":"2026-10-02T00:05:00Z","head_sha":%q}]}`, sha)
			default:
				http.NotFound(w, r)
			}
		}))
		t.Cleanup(srv.Close)
		withGitHubAPI(t, srv.URL)
		runs, err := fetchTagCIJobs(token, repo, sha, tag)
		if err != nil {
			t.Fatal(err)
		}
		if len(jobsFor) != 1 || jobsFor[0] != "11" {
			t.Fatalf("jobs fetched for %v, want only the newest run", jobsFor)
		}
		err = evaluateChecksForTag([]string{"unit"}, runs, sha, tag)
		if err == nil || errors.Is(err, errCIPending) {
			t.Fatalf("completed failure must be terminal, err=%v", err)
		}
	})
	t.Run("http 404 is terminal", func(t *testing.T) {
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			http.Error(w, "missing", http.StatusNotFound)
		}))
		t.Cleanup(srv.Close)
		withGitHubAPI(t, srv.URL)
		prevInterval, prevTimeout := ciPollInterval, ciPollTimeout
		ciPollInterval = time.Hour
		ciPollTimeout = time.Hour
		t.Cleanup(func() {
			ciPollInterval = prevInterval
			ciPollTimeout = prevTimeout
		})
		_, err := pollTagCI(token, repo, sha, tag)
		if err == nil || errors.Is(err, errCIPending) {
			t.Fatalf("404 should be terminal, err=%v", err)
		}
		if hits != 1 {
			t.Fatalf("poll retried a terminal HTTP error %d times", hits)
		}
	})
}

func withGitHubAPI(t *testing.T, base string) {
	t.Helper()
	prev := githubAPI
	githubAPI = base
	t.Cleanup(func() { githubAPI = prev })
}

func assertGitHubHeaders(t *testing.T, r *http.Request, token string) {
	t.Helper()
	if r.Header.Get("Authorization") != "Bearer "+token {
		t.Errorf("Authorization=%q", r.Header.Get("Authorization"))
	}
	if r.Header.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("Accept=%q", r.Header.Get("Accept"))
	}
	if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version=%q", r.Header.Get("X-GitHub-Api-Version"))
	}
}

func greenJobsJSON() string {
	var b strings.Builder
	b.WriteString(`{"jobs":[`)
	for i, name := range requiredCIJobs() {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"name":%q,"status":"completed","conclusion":"success","completed_at":"2026-10-04T00:01:00Z","head_sha":"ignored"}`, name)
	}
	b.WriteString(`]}`)
	return b.String()
}
