// Command release-diff gates a LabMail tag: notes headings and green required CI.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	var (
		notesPath    = flag.String("notes", "", "release notes file to validate")
		notesOnly    = flag.Bool("notes-only", false, "validate --notes headings only")
		previousTag  = flag.Bool("previous-tag", false, "print the previous tag, or empty-tree")
		requireCI    = flag.Bool("require-ci", false, "require all required CI jobs success on HEAD")
		ciFixture    = flag.String("ci-fixture", "", "JSON file of check runs (tests / offline)")
		requiredJobs = flag.String("required-jobs", strings.Join(requiredCIJobs(), ","), "comma-separated required CI job names")
	)
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fatal(err)
	}

	if *previousTag {
		tag, err := previousTagName(root)
		if err != nil {
			fatal(err)
		}
		fmt.Println(tag)
		return
	}
	if *requireCI {
		tag := strings.TrimSpace(os.Getenv("RELEASE_TAG"))
		if tag == "" {
			fatal(fmt.Errorf("RELEASE_TAG is required for -require-ci"))
		}
		sha, err := gitOutput(root, "rev-parse", "HEAD")
		if err != nil {
			fatal(err)
		}
		jobs := splitCSV(*requiredJobs)
		runs, err := loadCheckRuns(*ciFixture)
		if err != nil {
			fatal(err)
		}
		if err := evaluateChecksForTag(jobs, runs, strings.TrimSpace(sha), tag); err != nil {
			fatal(err)
		}
		return
	}
	if *notesOnly {
		if *notesPath == "" {
			fatal(fmt.Errorf("-notes-only requires -notes"))
		}
		body, err := os.ReadFile(*notesPath)
		if err != nil {
			fatal(fmt.Errorf("read notes: %w", err))
		}
		if err := validateNotes(string(body)); err != nil {
			fatal(err)
		}
		return
	}

	fmt.Fprintf(os.Stderr, "usage: release-diff [-notes-only -notes PATH] [-require-ci] [-previous-tag]\n")
	os.Exit(2)
}

// requiredCIJobs are the GitHub Actions job IDs that must succeed on the tag
// commit. Matches CI.yml required jobs, including container-test after DEP-001.
func requiredCIJobs() []string {
	return []string{
		"format",
		"lint",
		"unit",
		"race",
		"fuzz-smoke",
		"generated-file",
		"documentation",
		"security-scan",
		"changelog",
		"parity",
		"config-compat",
		"container-test",
		"web",
	}
}

func requiredNoteHeadings() []string {
	return []string{
		"Highlights",
		"Added",
		"Changed",
		"Fixed",
		"Removed or deprecated",
		"Security",
		"SMTP behavior",
		"Inbox UI",
		"REST API",
		"MCP API and protocol compatibility",
		"Configuration and schema",
		"Deployment and operations",
		"Observability",
		"Compatibility and migration",
		"This tree versus stacked siblings",
		"Known limitations",
		"Complete functionality-difference review",
		"CI and release evidence",
	}
}

func validateNotes(notes string) error {
	var missing []string
	for _, h := range requiredNoteHeadings() {
		if !hasHeading(notes, h) {
			missing = append(missing, h)
		}
	}
	var placeholders []string
	for _, p := range []string{"TODO", "TBD", "FIXME", "{{"} {
		if strings.Contains(notes, p) {
			placeholders = append(placeholders, p)
		}
	}
	if strings.Contains(strings.ToLower(notes), "public mta") &&
		!strings.Contains(strings.ToLower(notes), "not a") {
		placeholders = append(placeholders, "public-MTA completeness claim")
	}
	if len(missing) == 0 && len(placeholders) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("incomplete release notes")
	if len(missing) > 0 {
		b.WriteString("\n  missing headings:\n    ")
		b.WriteString(strings.Join(missing, "\n    "))
	}
	if len(placeholders) > 0 {
		b.WriteString("\n  unfilled template placeholders:\n    ")
		b.WriteString(strings.Join(placeholders, "\n    "))
	}
	return fmt.Errorf("%s", b.String())
}

func hasHeading(notes, heading string) bool {
	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "## "+heading {
			return true
		}
	}
	return false
}

type checkSuite struct {
	Event string `json:"event"`
}

type checkRun struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion"`
	HeadSHA     string     `json:"headSha"`
	CompletedAt time.Time  `json:"completedAt,omitempty"`
	Event       string     `json:"event"`
	Ref         string     `json:"ref"`
	CheckSuite  checkSuite `json:"checkSuite"`
}

func (r checkRun) effectiveEvent() string {
	if r.Event != "" {
		return r.Event
	}
	return r.CheckSuite.Event
}

type fixtureFile struct {
	CheckRuns []checkRun `json:"checkRuns"`
}

func loadCheckRuns(fixturePath string) ([]checkRun, error) {
	if fixturePath != "" {
		raw, err := os.ReadFile(fixturePath)
		if err != nil {
			return nil, fmt.Errorf("read ci fixture: %w", err)
		}
		var doc fixtureFile
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parse ci fixture: %w", err)
		}
		return doc.CheckRuns, nil
	}
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	repo := os.Getenv("GITHUB_REPOSITORY")
	sha := os.Getenv("GITHUB_SHA")
	if sha == "" {
		return nil, fmt.Errorf("GITHUB_SHA is required for -require-ci (or pass -ci-fixture)")
	}
	if repo == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY is required for -require-ci (or pass -ci-fixture)")
	}
	if token == "" {
		return nil, fmt.Errorf("GH_TOKEN or GITHUB_TOKEN is required for -require-ci (or pass -ci-fixture)")
	}
	tag := strings.TrimSpace(os.Getenv("RELEASE_TAG"))
	if tag == "" {
		return nil, fmt.Errorf("RELEASE_TAG is required for -require-ci (or pass -ci-fixture)")
	}
	return pollTagCI(token, repo, sha, tag)
}

var (
	githubAPI      = "https://api.github.com"
	ciPollInterval = 15 * time.Second
	ciPollTimeout  = 40 * time.Minute
	errCIPending   = errors.New("ci run for the tag push is not complete")
)

type ghWorkflowRuns struct {
	WorkflowRuns []ghWorkflowRun `json:"workflow_runs"`
}

type ghWorkflowRun struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	Event      string `json:"event"`
	HeadSHA    string `json:"head_sha"`
	HeadBranch string `json:"head_branch"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
}

type ghJobs struct {
	Jobs []struct {
		Name        string `json:"name"`
		Status      string `json:"status"`
		Conclusion  string `json:"conclusion"`
		CompletedAt string `json:"completed_at"`
		HeadSHA     string `json:"head_sha"`
	} `json:"jobs"`
}

func pollTagCI(token, repo, sha, tag string) ([]checkRun, error) {
	deadline := time.Now().Add(ciPollTimeout)
	for {
		runs, err := fetchTagCIJobs(token, repo, sha, tag)
		if err == nil || !errors.Is(err, errCIPending) || !time.Now().Before(deadline) {
			return runs, err
		}
		wait := ciPollInterval
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait > 0 {
			time.Sleep(wait)
		}
	}
}

func fetchTagCIJobs(token, repo, sha, tag string) ([]checkRun, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	listURL := fmt.Sprintf("%s/repos/%s/actions/runs?head_sha=%s&event=push&per_page=100",
		strings.TrimRight(githubAPI, "/"), repo, url.QueryEscape(sha))
	var runs []ghWorkflowRun
	for listURL != "" {
		body, hdr, err := githubGet(client, token, listURL)
		if err != nil {
			return nil, err
		}
		var doc ghWorkflowRuns
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
		runs = append(runs, doc.WorkflowRuns...)
		listURL = nextLink(hdr.Get("Link"))
	}
	chosen, err := pickTagCIRun(runs, sha, tag)
	if err != nil {
		return nil, err
	}
	jobsURL := fmt.Sprintf("%s/repos/%s/actions/runs/%d/jobs?per_page=100",
		strings.TrimRight(githubAPI, "/"), repo, chosen.ID)
	var jobs []checkRun
	for jobsURL != "" {
		body, hdr, err := githubGet(client, token, jobsURL)
		if err != nil {
			return nil, err
		}
		var doc ghJobs
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
		for _, j := range doc.Jobs {
			item := checkRun{
				Name:       j.Name,
				Status:     j.Status,
				Conclusion: j.Conclusion,
				HeadSHA:    chosen.HeadSHA,
				Event:      "push",
				Ref:        "refs/tags/" + tag,
			}
			if j.CompletedAt != "" {
				if ts, err := time.Parse(time.RFC3339, j.CompletedAt); err == nil {
					item.CompletedAt = ts
				}
			}
			jobs = append(jobs, item)
		}
		jobsURL = nextLink(hdr.Get("Link"))
	}
	return jobs, nil
}

func pickTagCIRun(runs []ghWorkflowRun, sha, tag string) (*ghWorkflowRun, error) {
	var best *ghWorkflowRun
	var bestCreated time.Time
	for i := range runs {
		r := &runs[i]
		if r.Path != ".github/workflows/ci.yml" || r.Event != "push" || r.HeadSHA != sha {
			continue
		}
		if r.HeadBranch == "" || r.HeadBranch != tag {
			continue
		}
		created, _ := time.Parse(time.RFC3339, r.CreatedAt)
		if best == nil || r.ID > best.ID || (r.ID == best.ID && created.After(bestCreated)) {
			cp := *r
			best = &cp
			bestCreated = created
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no ci.yml push run for tag %s at %s: %w", tag, sha, errCIPending)
	}
	if !strings.EqualFold(best.Status, "completed") {
		return nil, fmt.Errorf("ci run %d for tag %s is %s: %w", best.ID, tag, best.Status, errCIPending)
	}
	return best, nil
}

func githubGet(client *http.Client, token, rawURL string) ([]byte, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("github actions: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, resp.Header, nil
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) && !strings.Contains(part, "rel=next") {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

func evaluateChecks(required []string, runs []checkRun, wantSHA string) error {
	return evaluateTagged(required, runs, wantSHA, "")
}

func evaluateChecksForTag(required []string, runs []checkRun, wantSHA, wantTag string) error {
	if strings.TrimSpace(wantTag) == "" {
		return fmt.Errorf("RELEASE_TAG is required")
	}
	return evaluateTagged(required, runs, wantSHA, wantTag)
}

func evaluateTagged(required []string, runs []checkRun, wantSHA, wantTag string) error {
	var problems []string
	for _, name := range required {
		hits, wrongSHA := matchingRuns(runs, name, wantSHA, wantTag)
		if len(hits) == 0 {
			if wrongSHA > 0 {
				problems = append(problems, fmt.Sprintf("%s: head SHA != tag commit %s", name, wantSHA))
			} else {
				problems = append(problems, name+": missing (required job did not run)")
			}
			continue
		}
		best := latestCompleted(hits)
		if best == nil {
			problems = append(problems, name+": not completed (status="+hits[len(hits)-1].Status+")")
			continue
		}
		if !strings.EqualFold(best.Status, "completed") || !strings.EqualFold(best.Conclusion, "success") {
			problems = append(problems, fmt.Sprintf("%s: %s/%s (required job must succeed)", name, best.Status, best.Conclusion))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("release cannot proceed after a failed or missing required job:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func matchingRuns(runs []checkRun, name, wantSHA, wantTag string) (hits []checkRun, wrongSHA int) {
	for _, r := range runs {
		if !checkNameMatches(r.Name, name) {
			continue
		}
		// Empty head_sha is not the tag commit. A blank fixture must not
		// satisfy -require-ci.
		if wantSHA != "" && r.HeadSHA != wantSHA {
			wrongSHA++
			continue
		}
		if !tagPushRun(r, wantTag) {
			continue
		}
		hits = append(hits, r)
	}
	return hits, wrongSHA
}

// tagPushRun reports whether r is a push of a tag. wantTag empty accepts any
// refs/tags/ name. A non-empty wantTag accepts only that tag.
func tagPushRun(r checkRun, wantTag string) bool {
	if r.effectiveEvent() != "push" {
		return false
	}
	const prefix = "refs/tags/"
	if !strings.HasPrefix(r.Ref, prefix) {
		return false
	}
	name := r.Ref[len(prefix):]
	if name == "" {
		return false
	}
	if wantTag == "" {
		return true
	}
	return name == wantTag
}

func latestCompleted(runs []checkRun) *checkRun {
	var best *checkRun
	for i := range runs {
		r := &runs[i]
		if !strings.EqualFold(r.Status, "completed") {
			continue
		}
		if best == nil || r.CompletedAt.After(best.CompletedAt) {
			best = r
		}
	}
	return best
}

func checkNameMatches(got, want string) bool {
	got = strings.TrimSpace(got)
	if got == want {
		return true
	}
	if i := strings.LastIndex(got, " / "); i >= 0 {
		if suffix := strings.TrimSpace(got[i+3:]); suffix == want {
			return true
		}
	}
	return false
}

func previousTagName(root string) (string, error) {
	out, err := gitOutput(root, "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "4b825dc642cb6eb9a060e54bf8d69288fbee4904", nil // empty tree
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "4b825dc642cb6eb9a060e54bf8d69288fbee4904", nil
	}
	return out, nil
}

func gitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "release-diff: %v\n", err)
	os.Exit(1)
}
