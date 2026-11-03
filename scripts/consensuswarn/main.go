// Command consensuswarn reports whether a GitHub PR touches a function or method that is potentially called by
// one or more root functions or methods. The PR is loaded through the GitHub API, roots are specified by
// the `-root` flag. Methods use the form
//
//	example.com/pkg/path.Type.Method
//
// functions use
//
//	example.com/pkg/path.Function
//
// If the PR touches one or more callstacks, they're posted in a comment on the PR (once).
//
// Vendored from github.com/orijtech/consensuswarn@84343282daee (Apache-2.0, see LICENSE), modified to:
//   - fetch the PR diff through the GitHub API instead of the github.com diff URL, which returns 404 for
//     private repositories;
//   - honor the -apiurl flag, which upstream parsed but never used;
//   - post review comments on the hunk's base-side lines (side LEFT), which the analysis covers, instead of
//     one line past them on the head side, which GitHub rejects with 422 "line could not be resolved".
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var (
	dir        = flag.String("dir", ".", "base directory for the patch")
	ghtoken    = flag.String("ghtoken", "", "the GitHub API token")
	apiurl     = flag.String("apiurl", "https://api.github.com", "GitHub API URL")
	repository = flag.String("repository", "", "the GitHub owner/repository")
	prnum      = flag.Int("pr", 0, "the GitHub pull request number")
	rootNames  = stringSlice{}
)

const commentTitle = "Change potentially affects state."

func init() {
	flag.Var(&rootNames, "roots", "comma-separated list of root functions")
}

func main() {
	flag.Parse()
	if *prnum <= 0 {
		fmt.Fprintf(os.Stderr, "consensuswarn: invalid PR number: %d\n", *prnum)
		os.Exit(1)
	}
	*dir, _ = filepath.Abs(*dir)

	ctx := context.Background()
	var ts oauth2.TokenSource
	if *ghtoken != "" {
		ts = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *ghtoken},
		)
	}
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)
	baseURL, err := url.Parse(strings.TrimSuffix(*apiurl, "/") + "/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "consensuswarn: invalid API URL %q: %v\n", *apiurl, err)
		os.Exit(1)
	}
	gh.BaseURL = baseURL
	split := strings.SplitN(*repository, "/", 2)
	owner, repo := split[0], split[1]
	pr, patch, err := getDiff(ctx, gh, owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "consensuswarn: %v\n", err)
		os.Exit(2)
	}
	notified, err := hasComment(ctx, gh, owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "consensuswarn: %v\n", err)
		os.Exit(2)
	}
	if notified {
		fmt.Fprint(os.Stderr, "consensuswarn: ignoring PR because it was already commented\n")
		os.Exit(0)
	}
	if !pr.GetMergeable() {
		fmt.Fprint(os.Stderr, "consensuswarn: ignoring non-mergeable PR\n")
		os.Exit(0)
	}

	fset := new(token.FileSet)
	hunks, err := runCheck(fset, *dir, patch, rootNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "consensuswarn: %v\n", err)
		os.Exit(2)
	}
	comments, err := getReviewComments(ctx, gh, owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "consensuswarn: %v\n", err)
		os.Exit(2)
	}
	for _, hunk := range hunks {
		path := hunk.relFile
		// The analysis runs on the base checkout, so anchor the comment to the
		// hunk's lines in the base file. A pure addition has none; use the new ones.
		side := "LEFT"
		startLine := int(hunk.hunk.OrigStartLine)
		line := int(hunk.hunk.OrigStartLine+hunk.hunk.OrigLines) - 1
		if hunk.hunk.OrigLines == 0 {
			side = "RIGHT"
			startLine = int(hunk.hunk.NewStartLine)
			line = int(hunk.hunk.NewStartLine+hunk.hunk.NewLines) - 1
		}
		if comments[commentKey{path, line}] {
			continue
		}
		comment := new(bytes.Buffer)
		fmt.Fprintf(comment, "%s\n\nCall sequence:\n", commentTitle)
		fmt.Fprintf(comment, "```\n")
		for i := len(hunk.stack) - 1; i >= 0; i-- {
			e := hunk.stack[i]
			pos := fset.Position(e.pos)
			fmt.Fprintf(comment, "%s (%s:%d)\n", e.fun.FullName(), hunk.relFile, pos.Line)
		}
		fmt.Fprintf(comment, "```\n")
		rc := &reviewComment{
			CommitID: *pr.Head.SHA,
			Line:     line,
			Side:     side,
			Path:     path,
			Body:     comment.String(),
		}
		// GitHub requires start_line to precede line.
		if startLine < line {
			rc.StartLine = startLine
			rc.StartSide = side
		}
		err := postReviewComment(ctx, gh, owner, repo, rc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "consensuswarn: %v\n", err)
			os.Exit(2)
		}
	}
}

type reviewComment struct {
	CommitID  string `json:"commit_id"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
	Line      int    `json:"line"`
	Side      string `json:"side,omitempty"`
	Path      string `json:"path"`
	Body      string `json:"body"`
}

type commentKey struct {
	Path string
	Line int
}

func postReviewComment(ctx context.Context, gh *github.Client, owner, repo string, comment *reviewComment) error {
	url := fmt.Sprintf("%srepos/%s/%s/pulls/%d/comments", gh.BaseURL, owner, repo, *prnum)
	body, err := json.Marshal(comment)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	_, err = gh.Do(ctx, req, nil)
	return err
}

func hasComment(ctx context.Context, gh *github.Client, owner, repo string) (bool, error) {
	page := 0
	for {
		opt := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{Page: page}}
		comments, resp, err := gh.Issues.ListComments(ctx, owner, repo, *prnum, opt)
		if err != nil {
			return false, err
		}
		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), commentTitle) {
				return true, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return false, nil
}

func getReviewComments(ctx context.Context, gh *github.Client, owner, repo string) (map[commentKey]bool, error) {
	commentMap := make(map[commentKey]bool)
	page := 0
	for {
		url := fmt.Sprintf("%srepos/%s/%s/pulls/%d/comments?page=%d", gh.BaseURL, owner, repo, *prnum, page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		var comments []reviewComment
		resp, err := gh.Do(ctx, req, &comments)
		if err != nil {
			return nil, err
		}
		for _, comment := range comments {
			if strings.Contains(comment.Body, commentTitle) {
				commentMap[commentKey{comment.Path, comment.Line}] = true
			}
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return commentMap, nil
}

func getDiff(ctx context.Context, gh *github.Client, owner, repo string) (*github.PullRequest, *bytes.Buffer, error) {
	dur := time.Second
	retries := 0
	for {
		pr, _, err := gh.PullRequests.Get(ctx, owner, repo, *prnum)
		if err != nil {
			return nil, nil, err
		}
		if pr.Mergeable == nil {
			if retries > 5 {
				return nil, nil, fmt.Errorf("gave up waiting for mergeable PR; tried %d times", retries)
			}
			retries++
			time.Sleep(dur)
			dur *= 2
			continue
		}
		// pr.GetDiffURL() points at github.com, which 404s for private repositories even with a token.
		diff, _, err := gh.PullRequests.GetRaw(ctx, owner, repo, *prnum, github.RawOptions{Type: github.Diff})
		if err != nil {
			return nil, nil, err
		}
		return pr, bytes.NewBufferString(diff), nil
	}
}
