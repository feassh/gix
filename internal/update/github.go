package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	client  *http.Client
}

type Release struct {
	TagName string
	Name    string
	Assets  []Asset
}

type Asset struct {
	Name        string
	DownloadURL string
}

func NewClient(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) LatestRelease(ctx context.Context, repo string) (Release, error) {
	return c.release(ctx, fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, repo))
}

func (c *Client) ReleaseByTag(ctx context.Context, repo string, tag string) (Release, error) {
	return c.release(ctx, fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.baseURL, repo, tag))
}

func (c *Client) release(ctx context.Context, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "gix-self-update")
	resp, err := c.client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Release{}, fmt.Errorf("github releases request failed: %s", strings.TrimSpace(string(body)))
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, err
	}
	release := Release{TagName: payload.TagName, Name: payload.Name}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, Asset{
			Name:        asset.Name,
			DownloadURL: asset.BrowserDownloadURL,
		})
	}
	return release, nil
}

func FindAsset(release Release, name string) (Asset, error) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, nil
		}
	}
	return Asset{}, fmt.Errorf("release asset %q not found", name)
}

func BinaryAssetName(tag string) string {
	base := fmt.Sprintf("gix_%s_%s_%s", tag, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func ChecksumsAssetName(tag string) string {
	return fmt.Sprintf("gix_%s_checksums.txt", tag)
}

func (c *Client) DownloadFile(ctx context.Context, url string, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "gix-self-update")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download failed: %s", strings.TrimSpace(string(body)))
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}
