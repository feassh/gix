package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gix/internal/buildinfo"
	"gix/internal/config"
	"gix/internal/install"
)

func TestCheckLatestRelease(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/gix/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v9.9.9",
			"name":     "v9.9.9",
			"assets":   []any{},
		})
	}))
	defer server.Close()

	cfg := config.DefaultValues()
	_ = cfg.Set("self_update.repo", "acme/gix")
	_ = cfg.Set("self_update.base_url", server.URL)
	resolved, err := cfg.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}

	manager := NewManager(NewClient(server.URL), install.NewManager())
	result, _, err := manager.Check(context.Background(), resolved, "")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.UpdateAvailable {
		t.Fatalf("expected update to be available")
	}
	if result.LatestVersion != "v9.9.9" {
		t.Fatalf("unexpected latest version: %s", result.LatestVersion)
	}
}

func TestUpdateDownloadsAndInstallsBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix installation assertions")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")

	tag := "v9.9.9"
	assetName := BinaryAssetName(tag)
	binaryPayload := []byte("updated-binary")
	sum := sha256.Sum256(binaryPayload)
	checksumPayload := hex.EncodeToString(sum[:]) + "  " + assetName + "\n"

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/gix/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": tag,
				"name":     tag,
				"assets": []map[string]string{
					{"name": assetName, "browser_download_url": server.URL + "/downloads/" + assetName},
					{"name": ChecksumsAssetName(tag), "browser_download_url": server.URL + "/downloads/" + ChecksumsAssetName(tag)},
				},
			})
		case "/downloads/" + assetName:
			_, _ = w.Write(binaryPayload)
		case "/downloads/" + ChecksumsAssetName(tag):
			_, _ = w.Write([]byte(checksumPayload))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalVersion := buildinfo.Version
	originalRepo := buildinfo.DefaultUpdateRepo
	buildinfo.Version = "v0.1.0"
	buildinfo.DefaultUpdateRepo = "acme/gix"
	defer func() {
		buildinfo.Version = originalVersion
		buildinfo.DefaultUpdateRepo = originalRepo
	}()

	cfg := config.DefaultValues()
	_ = cfg.Set("self_update.base_url", server.URL)
	resolved, err := cfg.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}

	manager := NewManager(NewClient(server.URL), install.NewManager())
	result, err := manager.Update(context.Background(), resolved, "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	target := filepath.Join(home, ".local", "bin", "gix")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(data) != string(binaryPayload) {
		t.Fatalf("unexpected installed binary content")
	}
	if result.LatestVersion != tag {
		t.Fatalf("unexpected latest version: %s", result.LatestVersion)
	}
}

func TestLookupChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checksums.txt")
	content := "abc123  gix_v1.2.3_linux_amd64\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
	value, err := lookupChecksum(path, "gix_v1.2.3_linux_amd64")
	if err != nil {
		t.Fatalf("lookupChecksum: %v", err)
	}
	if strings.TrimSpace(value) != "abc123" {
		t.Fatalf("unexpected checksum: %s", value)
	}
}
