package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func goodManifest() *ForkManifest {
	return &ForkManifest{
		Commit: strings.Repeat("b", 40),
		Tag:    "v5.4.1.0-tim.bbbbbbb",
		URL:    forkDownloadURL + "v5.4.1.0-tim.bbbbbbb/Grout-Onion-tim.zip",
		Size:   1024,
		SHA256: strings.Repeat("a", 64),
	}
}

func TestForkInfo_OffersADifferentBuild(t *testing.T) {
	info, err := forkInfo(goodManifest(), strings.Repeat("a", 40), "5.4.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !info.UpdateAvailable || info.DownloadURL == "" || info.AssetSHA256 == "" {
		t.Errorf("a different build was not offered: %+v", info)
	}
}

func TestForkInfo_SameBuildIsUpToDate(t *testing.T) {
	m := goodManifest()
	info, err := forkInfo(m, m.Commit, "5.4.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if info.UpdateAvailable {
		t.Error("the installed build was offered to itself")
	}
}

// A package that was not stamped, such as the first fork build copied over by
// hand, cannot be told apart from the current one, so it is offered it.
func TestForkInfo_UnstampedPackageIsOffered(t *testing.T) {
	info, err := forkInfo(goodManifest(), "", "5.4.1.0")
	if err != nil || !info.UpdateAvailable {
		t.Fatalf("unstamped package not offered: %+v, %v", info, err)
	}
}

func TestForkInfo_RefusesAManifestItCannotTrust(t *testing.T) {
	noSum := goodManifest()
	noSum.SHA256 = ""
	elsewhere := goodManifest()
	elsewhere.URL = "https://example.invalid/Grout-Onion-tim.zip"
	incomplete := goodManifest()
	incomplete.Commit = ""

	for name, m := range map[string]*ForkManifest{"no checksum": noSum, "other host": elsewhere, "incomplete": incomplete} {
		if _, err := forkInfo(m, "x", "5.4.1.0"); err == nil {
			t.Errorf("%s: manifest was accepted", name)
		}
	}
}

func TestReadForkBuild(t *testing.T) {
	dir := t.TempDir()
	if got := readForkBuild(dir); got != "" {
		t.Errorf("no stamp read as %q", got)
	}
	os.WriteFile(filepath.Join(dir, forkBuildFile), []byte("abc123\n"), 0644)
	if got := readForkBuild(dir); got != "abc123" {
		t.Errorf("stamp read as %q", got)
	}
}
