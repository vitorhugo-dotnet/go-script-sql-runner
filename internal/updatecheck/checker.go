package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	RepositoryID       int64 = 1326685411
	defaultAPIBaseURL        = "https://api.github.com"
	defaultAssetName         = "go-script-sql-runner.exe"
	githubAPIVersion         = "2026-03-10"
)

type Result struct {
	CurrentTag  string `json:"currentTag"`
	LatestTag   string `json:"latestTag"`
	Available   bool   `json:"available"`
	ReleaseURL  string `json:"releaseUrl"`
	DownloadURL string `json:"downloadUrl"`
}

type Checker struct {
	client       *http.Client
	apiBaseURL   string
	repositoryID int64
	currentTag   string
}

type repositoryResponse struct {
	FullName string `json:"full_name"`
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func New(currentTag string) *Checker {
	return &Checker{
		client:       &http.Client{Timeout: 5 * time.Second},
		apiBaseURL:   defaultAPIBaseURL,
		repositoryID: RepositoryID,
		currentTag:   strings.TrimSpace(currentTag),
	}
}

func (c *Checker) Check(ctx context.Context) (Result, error) {
	var repository repositoryResponse
	if err := c.getJSON(ctx, fmt.Sprintf("/repositories/%d", c.repositoryID), &repository); err != nil {
		return Result{}, fmt.Errorf("resolve update repository: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(repository.FullName), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Result{}, fmt.Errorf("resolve update repository: invalid full_name %q", repository.FullName)
	}

	releasePath := fmt.Sprintf(
		"/repos/%s/%s/releases/latest",
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
	)
	var release releaseResponse
	if err := c.getJSON(ctx, releasePath, &release); err != nil {
		return Result{}, fmt.Errorf("get latest release: %w", err)
	}

	result := Result{
		CurrentTag: c.currentTag,
		LatestTag:  release.TagName,
		Available:  isNewerBuild(c.currentTag, release.TagName),
		ReleaseURL: release.HTMLURL,
	}
	for _, asset := range release.Assets {
		if asset.Name == defaultAssetName {
			result.DownloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return result, nil
}

func (c *Checker) getJSON(ctx context.Context, path string, target any) error {
	if c.client == nil {
		return fmt.Errorf("HTTP client is not configured")
	}

	requestURL := strings.TrimRight(c.apiBaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	req.Header.Set("User-Agent", "go-script-sql-runner")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode GitHub API response: %w", err)
	}
	return nil
}

func isNewerBuild(currentTag, latestTag string) bool {
	currentBuild, currentOK := parseBuildNumber(currentTag)
	latestBuild, latestOK := parseBuildNumber(latestTag)
	return currentOK && latestOK && latestBuild > currentBuild
}

func parseBuildNumber(tag string) (int, bool) {
	value := strings.TrimPrefix(strings.TrimSpace(tag), "build-")
	if value == tag || value == "" {
		return 0, false
	}
	build, err := strconv.Atoi(value)
	if err != nil || build < 0 {
		return 0, false
	}
	return build, true
}
