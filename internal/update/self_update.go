package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gix/internal/buildinfo"
	"gix/internal/config"
	"gix/internal/install"
	"gix/internal/version"
)

type Manager struct {
	client    *Client
	installer *install.Manager
}

type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	Repo            string
	UpdateAvailable bool
}

type UpdateResult struct {
	CheckResult
	InstalledPath string
	Notes         []string
}

func NewManager(client *Client, installer *install.Manager) *Manager {
	return &Manager{
		client:    client,
		installer: installer,
	}
}

func ResolveRepo(cfg config.Config) string {
	if strings.TrimSpace(cfg.SelfUpdate.Repo) != "" {
		return strings.TrimSpace(cfg.SelfUpdate.Repo)
	}
	return strings.TrimSpace(buildinfo.DefaultUpdateRepo)
}

func ResolveBaseURL(cfg config.Config) string {
	if strings.TrimSpace(cfg.SelfUpdate.BaseURL) != "" {
		return strings.TrimSpace(cfg.SelfUpdate.BaseURL)
	}
	return "https://api.github.com"
}

func (m *Manager) Check(ctx context.Context, cfg config.Config, requestedVersion string) (CheckResult, Release, error) {
	repo := ResolveRepo(cfg)
	if repo == "" {
		return CheckResult{}, Release{}, fmt.Errorf("self-update repo is not configured")
	}
	var (
		release Release
		err     error
	)
	if strings.TrimSpace(requestedVersion) != "" {
		release, err = m.client.ReleaseByTag(ctx, repo, requestedVersion)
	} else {
		release, err = m.client.LatestRelease(ctx, repo)
	}
	if err != nil {
		return CheckResult{}, Release{}, err
	}
	current := buildinfo.Version
	updateAvailable := true
	if current != "" && current != "dev" {
		if cmp, err := version.CompareTags(current, release.TagName, "v"); err == nil {
			updateAvailable = cmp < 0
		} else {
			updateAvailable = normalizeVersion(current) != normalizeVersion(release.TagName)
		}
	}
	return CheckResult{
		CurrentVersion:  current,
		LatestVersion:   release.TagName,
		Repo:            repo,
		UpdateAvailable: updateAvailable,
	}, release, nil
}

func (m *Manager) Update(ctx context.Context, cfg config.Config, requestedVersion string) (UpdateResult, error) {
	check, release, err := m.Check(ctx, cfg, requestedVersion)
	if err != nil {
		return UpdateResult{}, err
	}
	if !check.UpdateAvailable {
		return UpdateResult{CheckResult: check}, nil
	}

	binaryAsset, err := FindAsset(release, BinaryAssetName(release.TagName))
	if err != nil {
		return UpdateResult{}, err
	}
	checksumAsset, err := FindAsset(release, ChecksumsAssetName(release.TagName))
	if err != nil {
		return UpdateResult{}, err
	}

	tempDir, err := os.MkdirTemp("", "gix-self-update-*")
	if err != nil {
		return UpdateResult{}, err
	}
	defer os.RemoveAll(tempDir)

	binaryPath := filepath.Join(tempDir, binaryAsset.Name)
	checksumPath := filepath.Join(tempDir, checksumAsset.Name)
	if err := m.client.DownloadFile(ctx, binaryAsset.DownloadURL, binaryPath); err != nil {
		return UpdateResult{}, err
	}
	if err := m.client.DownloadFile(ctx, checksumAsset.DownloadURL, checksumPath); err != nil {
		return UpdateResult{}, err
	}
	if err := verifyChecksum(binaryPath, checksumPath, binaryAsset.Name); err != nil {
		return UpdateResult{}, err
	}

	installResult, err := m.installer.InstallFromPath(ctx, binaryPath, true)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{
		CheckResult:   check,
		InstalledPath: installResult.TargetPath,
		Notes:         installResult.Notes,
	}, nil
}

func verifyChecksum(binaryPath string, checksumPath string, assetName string) error {
	expected, err := lookupChecksum(checksumPath, assetName)
	if err != nil {
		return err
	}
	file, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum verification failed for %s", assetName)
	}
	return nil
}

func lookupChecksum(path string, assetName string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", assetName)
}

func normalizeVersion(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "v")
}
