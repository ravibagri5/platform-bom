// Package upstream fetches release information for components from their
// upstream projects.
package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ravibagri5/platform-bom/internal/api"
	"github.com/ravibagri5/platform-bom/internal/version"
)

const (
	maxNotes     = 16 << 10
	pagesToFetch = 2
)

var slugRE = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// Release is an upstream release of a component.
type Release struct {
	Version     string    `json:"version"`
	Tag         string    `json:"tag"`
	Name        string    `json:"name,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`
	URL         string    `json:"url"`
	Notes       string    `json:"notes,omitempty"`
	Prerelease  bool      `json:"prerelease,omitempty"`
}

// Client fetches and caches GitHub releases.
type Client struct {
	HTTP     *http.Client
	BaseURL  string
	Token    string
	CacheDir string
	TTL      time.Duration

	mu  sync.Mutex
	mem map[string]cacheEntry
}

type cacheEntry struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Releases  []Release `json:"releases"`
}

// NewClient returns a client using GITHUB_TOKEN when set and caching on disk.
func NewClient() *Client {
	c := &Client{
		HTTP:    &http.Client{Timeout: 20 * time.Second},
		BaseURL: "https://api.github.com",
		Token:   os.Getenv("GITHUB_TOKEN"),
		TTL:     6 * time.Hour,
		mem:     map[string]cacheEntry{},
	}
	if dir, err := os.UserCacheDir(); err == nil {
		c.CacheDir = filepath.Join(dir, "pbom", "upstream")
	}
	return c
}

// Releases returns stable releases for up, newest first. Stale cache is
// returned if the upstream cannot be reached.
func (c *Client) Releases(ctx context.Context, up *api.Upstream) ([]Release, error) {
	if up == nil || up.GitHub == "" {
		return nil, errors.New("no upstream configured")
	}
	if !slugRE.MatchString(up.GitHub) {
		return nil, fmt.Errorf("invalid github slug %q", up.GitHub)
	}
	all, err := c.cached(ctx, up.GitHub)
	if err != nil {
		return nil, err
	}
	return filter(all, up), nil
}

func filter(all []Release, up *api.Upstream) []Release {
	out := make([]Release, 0, len(all))
	for _, r := range all {
		tag := r.Tag
		if up.TagPrefix != "" {
			if !strings.HasPrefix(tag, up.TagPrefix) {
				continue
			}
			tag = strings.TrimPrefix(tag, up.TagPrefix)
		}
		v, ok := version.Parse(tag)
		if !ok || v.Parts < 2 {
			continue
		}
		if (r.Prerelease || v.Prerelease()) && !up.IncludePrereleases {
			continue
		}
		r.Version = v.String()
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return version.CompareStrings(out[i].Version, out[j].Version) > 0 })
	return out
}

func (c *Client) cached(ctx context.Context, slug string) ([]Release, error) {
	c.mu.Lock()
	if e, ok := c.mem[slug]; ok && time.Since(e.FetchedAt) < c.TTL {
		c.mu.Unlock()
		return e.Releases, nil
	}
	c.mu.Unlock()

	disk, diskErr := c.readDisk(slug)
	if diskErr == nil && time.Since(disk.FetchedAt) < c.TTL {
		c.store(slug, disk, false)
		return disk.Releases, nil
	}
	releases, err := c.fetch(ctx, slug)
	if err != nil {
		if diskErr == nil {
			return disk.Releases, nil
		}
		return nil, err
	}
	c.store(slug, cacheEntry{FetchedAt: time.Now(), Releases: releases}, true)
	return releases, nil
}

func (c *Client) store(slug string, e cacheEntry, persist bool) {
	c.mu.Lock()
	c.mem[slug] = e
	c.mu.Unlock()
	if !persist || c.CacheDir == "" {
		return
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return
	}
	if data, err := json.Marshal(e); err == nil {
		_ = os.WriteFile(c.cacheFile(slug), data, 0o644)
	}
}

func (c *Client) readDisk(slug string) (cacheEntry, error) {
	var e cacheEntry
	if c.CacheDir == "" {
		return e, errors.New("no cache dir")
	}
	data, err := os.ReadFile(c.cacheFile(slug))
	if err != nil {
		return e, err
	}
	return e, json.Unmarshal(data, &e)
}

func (c *Client) cacheFile(slug string) string {
	return filepath.Join(c.CacheDir, strings.ReplaceAll(slug, "/", "__")+".json")
}

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

func (c *Client) fetch(ctx context.Context, slug string) ([]Release, error) {
	owner, repo, _ := strings.Cut(slug, "/")
	var out []Release
	for page := 1; page <= pagesToFetch; page++ {
		u := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100&page=%d",
			c.BaseURL, url.PathEscape(owner), url.PathEscape(repo), page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		var batch []ghRelease
		err = decodeResponse(resp, &batch)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", slug, err)
		}
		for _, r := range batch {
			if r.Draft {
				continue
			}
			notes := r.Body
			if len(notes) > maxNotes {
				notes = strings.ToValidUTF8(notes[:maxNotes], "") + "\n\n…"
			}
			out = append(out, Release{
				Tag: r.TagName, Name: r.Name, URL: r.HTMLURL, Notes: notes,
				Prerelease: r.Prerelease, PublishedAt: r.PublishedAt,
			})
		}
		if len(batch) < 100 {
			break
		}
	}
	return out, nil
}

func decodeResponse(resp *http.Response, v any) error {
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github returned %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(v)
}
