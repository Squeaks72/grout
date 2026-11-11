package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"grout/cfw"
	"grout/version"
)

// This build comes from a fork, so its updates come from the fork's releases,
// not upstream's versions file. The fork's build publishes a small manifest
// beside each package; a build is told apart from the one installed by the
// commit it was made from, because the fork keeps upstream's version number.
const (
	forkManifestURL = "https://github.com/Squeaks72/grout/releases/download/tim-latest/fork-update.json"
	forkDownloadURL = "https://github.com/Squeaks72/grout/releases/download/"

	// forkBuildFile sits beside the executable and holds the commit the
	// package was built from. A package without it was not built by the fork's
	// workflow and is always offered the current build.
	forkBuildFile = "fork-build"
)

// ForkManifest describes the fork's current build.
type ForkManifest struct {
	Commit  string `json:"commit"`
	Tag     string `json:"tag"`
	Version string `json:"version"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
}

// CheckForkUpdate asks whether the fork has a build other than the installed one.
func CheckForkUpdate(c cfw.CFW) (*Info, error) {
	if c != cfw.Onion {
		return nil, fmt.Errorf("the fork only builds for Onion")
	}
	if version.Get().Version == "dev" {
		return &Info{CurrentVersion: "dev"}, nil
	}

	manifest, err := fetchForkManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	return forkInfo(manifest, installedForkBuild(), version.Get().Version)
}

// forkInfo works out whether the manifest's build is worth offering.
func forkInfo(manifest *ForkManifest, installed, current string) (*Info, error) {
	if manifest.Commit == "" || manifest.Tag == "" {
		return nil, fmt.Errorf("the fork's update manifest is incomplete")
	}
	// The checksum pins the download, and the manifest says where to download
	// from; an entry without one is refused, as it is for upstream's.
	if manifest.SHA256 == "" {
		return nil, fmt.Errorf("no checksum published for %s", manifest.Tag)
	}
	// The manifest is fetched over the network, so it is not trusted to point
	// anywhere but the fork's own releases.
	if !strings.HasPrefix(manifest.URL, forkDownloadURL) {
		return nil, fmt.Errorf("the fork's update manifest points outside its releases")
	}

	info := &Info{
		CurrentVersion: describeForkBuild(current, installed),
		LatestVersion:  manifest.Tag,
	}
	if installed != "" && installed == manifest.Commit {
		return info, nil
	}

	info.UpdateAvailable = true
	info.DownloadURL = manifest.URL
	info.AssetSize = manifest.Size
	info.AssetSHA256 = manifest.SHA256
	return info, nil
}

func describeForkBuild(current, installed string) string {
	if installed == "" {
		return current
	}
	short := installed
	if len(short) > 7 {
		short = short[:7]
	}
	return current + "-tim." + short
}

// installedForkBuild is the commit the running package was built from, or "".
func installedForkBuild() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return readForkBuild(filepath.Dir(exe))
}

func readForkBuild(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, forkBuildFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func fetchForkManifest() (*ForkManifest, error) {
	client := &http.Client{Timeout: defaultTimeout}

	req, err := http.NewRequest(http.MethodGet, forkManifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Grout-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch the fork's manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch the fork's manifest: status %d", resp.StatusCode)
	}

	var manifest ForkManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode the fork's manifest: %w", err)
	}
	return &manifest, nil
}
