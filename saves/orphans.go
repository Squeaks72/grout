package saves

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"

	"grout/cfw"
	"grout/romm"
	"grout/settings"
)

// ErrNoGamesOnDevice is returned when no game is found on the device at all.
// That is far more likely a card that is not mounted than a device with no
// games, and every save would look like it had lost its game.
var ErrNoGamesOnDevice = errors.New("no games found on the device")

// OrphanedSave is a save on the device whose game is no longer on it.
type OrphanedSave struct {
	Local LocalSave
	// Size is in bytes, summed over a save kept as folders.
	Size int64
}

// Orphans are the saves left behind by games that are no longer on the
// device, split by whether RomM holds an identical copy.
type Orphans struct {
	// OnRomM have a byte-identical copy on RomM, so putting them away loses
	// nothing.
	OnRomM []OrphanedSave
	// NotOnRomM have no identical copy on RomM, or one that could not be
	// checked. They may be the only copy there is.
	NotOnRomM []OrphanedSave
}

// FindOrphanedSaves looks for saves whose game is no longer on the device.
func FindOrphanedSaves(client *romm.Client, config *settings.Config, deviceID string) (Orphans, error) {
	onDevice := ResolveLocalRoms(cfw.ScanRoms(*config))
	if len(onDevice) == 0 {
		return Orphans{}, ErrNoGamesOnDevice
	}

	orphaned := withoutGames(ScanSaves(config), onDevice)
	if len(orphaned) == 0 {
		return Orphans{}, nil
	}

	wanted := make(map[int]cfw.LocalRomFile, len(orphaned))
	for _, ls := range orphaned {
		wanted[ls.RomID] = cfw.LocalRomFile{RomID: ls.RomID}
	}
	return classifyOrphans(orphaned, fetchSavesForRoms(client, deviceID, wanted)), nil
}

// withoutGames keeps the saves whose game is not among the games on the device.
func withoutGames(local []LocalSave, onDevice map[int]cfw.LocalRomFile) []LocalSave {
	var orphaned []LocalSave
	for _, ls := range local {
		if _, present := onDevice[ls.RomID]; !present {
			orphaned = append(orphaned, ls)
		}
	}
	return orphaned
}

// classifyOrphans sorts saves by whether RomM already holds the same bytes. A
// save is only counted as safe when RomM's content hash is known and equal; a
// hash that is missing says nothing.
func classifyOrphans(orphaned []LocalSave, remote map[int][]romm.Save) Orphans {
	var out Orphans
	for _, ls := range orphaned {
		entry := OrphanedSave{Local: ls, Size: saveSize(ls)}

		hash, err := saveContentHash(ls)
		if err == nil && remoteHolds(remote[ls.RomID], hash) {
			out.OnRomM = append(out.OnRomM, entry)
		} else {
			out.NotOnRomM = append(out.NotOnRomM, entry)
		}
	}

	byName := func(list []OrphanedSave) {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Local.RomName < list[j].Local.RomName })
	}
	byName(out.OnRomM)
	byName(out.NotOnRomM)
	return out
}

func remoteHolds(saves []romm.Save, hash string) bool {
	if hash == "" {
		return false
	}
	for _, save := range saves {
		if save.ContentHash != nil && *save.ContentHash == hash {
			return true
		}
	}
	return false
}

// PutAwayOrphans moves saves into the .backup folder beside them, which loses
// nothing, and returns how many it moved. A save that cannot be moved is left
// where it is and the first reason is returned alongside the count.
func PutAwayOrphans(orphaned []OrphanedSave) (int, error) {
	var first error
	moved := 0
	for _, entry := range orphaned {
		if err := putAwaySave(entry.Local); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		moved++
	}
	return moved, first
}

// putAwaySave moves every path a save is made of, undoing the ones already
// moved if a later one fails so a save is never left half away.
func putAwaySave(ls LocalSave) error {
	paths := ls.RelatedDirs
	if !ls.IsDirectorySave || len(paths) == 0 {
		paths = []string{ls.FilePath}
	}

	type move struct{ from, to string }
	var done []move
	for _, path := range paths {
		to, err := putAway(path, filepath.Base(path))
		if err != nil {
			for i := len(done) - 1; i >= 0; i-- {
				undoPutAway(done[i].from, done[i].to)
			}
			return err
		}
		done = append(done, move{path, to})
	}
	return nil
}

func saveSize(ls LocalSave) int64 {
	paths := ls.RelatedDirs
	if !ls.IsDirectorySave || len(paths) == 0 {
		paths = []string{ls.FilePath}
	}
	var total int64
	for _, path := range paths {
		filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
			return nil
		})
	}
	return total
}
