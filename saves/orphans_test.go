package saves

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"grout/cfw"
	"grout/romm"
)

func writeSave(t *testing.T, dir, name, content string) LocalSave {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return LocalSave{RomID: 1, RomName: name, FileName: name, FilePath: path}
}

func md5Of(content string) *string {
	sum := md5.Sum([]byte(content))
	s := hex.EncodeToString(sum[:])
	return &s
}

func TestWithoutGames_KeepsOnlySavesWhoseGameIsGone(t *testing.T) {
	local := []LocalSave{{RomID: 1}, {RomID: 2}, {RomID: 3}}
	onDevice := map[int]cfw.LocalRomFile{2: {RomID: 2}}

	got := withoutGames(local, onDevice)

	if len(got) != 2 || got[0].RomID != 1 || got[1].RomID != 3 {
		t.Fatalf("withoutGames = %+v, want ROMs 1 and 3", got)
	}
}

func TestClassifyOrphans_OnlyAnIdenticalCopyOnRomMIsSafe(t *testing.T) {
	dir := t.TempDir()
	same := writeSave(t, dir, "same.srm", "alpha")
	same.RomID = 1
	differs := writeSave(t, dir, "differs.srm", "bravo")
	differs.RomID = 2
	unknown := writeSave(t, dir, "unknown.srm", "charlie")
	unknown.RomID = 3
	nohash := writeSave(t, dir, "nohash.srm", "delta")
	nohash.RomID = 4

	remote := map[int][]romm.Save{
		1: {{RomID: 1, ContentHash: md5Of("alpha")}},
		2: {{RomID: 2, ContentHash: md5Of("something else")}},
		4: {{RomID: 4}},
	}

	got := classifyOrphans([]LocalSave{same, differs, unknown, nohash}, remote)

	if len(got.OnRomM) != 1 || got.OnRomM[0].Local.RomID != 1 {
		t.Errorf("OnRomM = %+v, want only ROM 1", got.OnRomM)
	}
	if len(got.NotOnRomM) != 3 {
		t.Errorf("NotOnRomM = %d saves, want 3 (differs, unknown, no hash)", len(got.NotOnRomM))
	}
	if got.OnRomM[0].Size != int64(len("alpha")) {
		t.Errorf("Size = %d, want %d", got.OnRomM[0].Size, len("alpha"))
	}
}

func TestPutAwayOrphans_MovesIntoBackupAndKeepsTheBytes(t *testing.T) {
	dir := t.TempDir()
	ls := writeSave(t, dir, "game.srm", "precious")

	moved, err := PutAwayOrphans([]OrphanedSave{{Local: ls}})
	if err != nil || moved != 1 {
		t.Fatalf("PutAwayOrphans = %d, %v", moved, err)
	}
	if _, err := os.Stat(ls.FilePath); !os.IsNotExist(err) {
		t.Error("the save is still where it was")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".backup", "game*.srm"))
	if len(matches) != 1 {
		t.Fatalf("backup copies = %v, want exactly one", matches)
	}
	if data, _ := os.ReadFile(matches[0]); string(data) != "precious" {
		t.Errorf("backup holds %q", data)
	}
}

func TestPutAwayOrphans_MovesEveryFolderOfASaveKeptAsFolders(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "UCUS98751DATA00"), filepath.Join(dir, "UCUS98751DATA01")
	for _, d := range []string{a, b} {
		os.MkdirAll(d, 0755)
		os.WriteFile(filepath.Join(d, "PARAM.SFO"), []byte("x"), 0644)
	}
	ls := LocalSave{RomID: 1, FilePath: a, IsDirectorySave: true, RelatedDirs: []string{a, b}}

	moved, err := PutAwayOrphans([]OrphanedSave{{Local: ls}})
	if err != nil || moved != 1 {
		t.Fatalf("PutAwayOrphans = %d, %v", moved, err)
	}
	for _, d := range []string{a, b} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("%s is still in place", d)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, ".backup")); len(entries) != 2 {
		t.Errorf("backup holds %d entries, want 2", len(entries))
	}
}
