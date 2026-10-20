package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GitHubClient reads releases from a GitHub repository.
type GitHubClient struct {
	// Repo is the "owner/name" pair.
	Repo string
	// BaseURL is the API root. It is configurable so the panel can be tested
	// against a stub server.
	BaseURL string
	// HTTP is the client used for the requests.
	HTTP *http.Client
	// Token authenticates the requests. It raises the rate limit.
	Token string
}

// NewGitHubClient returns a client for a repository.
func NewGitHubClient(repo string) *GitHubClient {
	return &GitHubClient{
		Repo:    repo,
		BaseURL: "https://api.github.com",
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Asset is a downloadable file attached to a release.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// Release is a published release.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Version returns the release version.
func (r *Release) Version() Version {
	version, err := Parse(r.TagName)
	if err != nil {
		return Version{}
	}
	return version
}

// Asset returns the asset with the given name.
func (r *Release) Asset(name string) (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// Releases returns the published releases, newest first.
func (c *GitHubClient) Releases(ctx context.Context) ([]Release, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=20", strings.TrimSuffix(base, "/"), c.Repo)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ryxoui-panel")
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("update: fetch releases: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: releases request failed with status %d", response.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("update: decode releases: %w", err)
	}
	return releases, nil
}

// Latest returns the newest published release, skipping drafts.
func (c *GitHubClient) Latest(ctx context.Context) (Release, error) {
	releases, err := c.Releases(ctx)
	if err != nil {
		return Release{}, err
	}
	for _, release := range releases {
		if release.Draft {
			continue
		}
		return release, nil
	}
	return Release{}, fmt.Errorf("update: no published release found")
}
