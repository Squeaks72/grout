package ui

import (
	"errors"
	"fmt"
	"strings"

	"grout/romm"
	"grout/saves"
	"grout/textmatch"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

// cleanUpSaves puts away the saves of games that are no longer on the device.
//
// Only saves RomM holds an identical copy of are offered, and they go to the
// .backup folder beside them rather than being deleted. A save RomM does not
// have may be the only copy there is, so those are counted and left alone.
func (s *SaveSyncSettingsScreen) cleanUpSaves(input SaveSyncSettingsInput, output SaveSyncSettingsOutput) (SaveSyncSettingsOutput, error) {
	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout.Duration())

	var found saves.Orphans
	var findErr error
	gaba.ProcessMessage(
		localize("clean_up_saves_looking", "Looking for leftover saves..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			found, findErr = saves.FindOrphanedSaves(client, input.Config, input.Host.DeviceID)
			return nil, nil
		},
	)
	if findErr != nil {
		gaba.GetLogger().Error("Failed to look for leftover saves", "error", findErr)
		message := localizeWith("clean_up_saves_failed", "Could not look for leftover saves:\n{{.Error}}", map[string]any{"Error": findErr.Error()})
		if errors.Is(findErr, saves.ErrNoGamesOnDevice) {
			message = localize("clean_up_saves_no_games", "No games were found on this device,\nso nothing was touched.\nIs the card with your games in?")
		}
		tellAdvanced(message)
		return output, nil
	}

	if len(found.OnRomM) == 0 {
		message := localize("clean_up_saves_none", "No leftover saves found.")
		if len(found.NotOnRomM) > 0 {
			message = localizeWith("clean_up_saves_only_unsafe",
				"{{.Count}} saves belong to games no longer here, but RomM\ndoes not have an identical copy, so they were left alone.",
				map[string]any{"Count": len(found.NotOnRomM)})
		}
		tellAdvanced(message)
		return output, nil
	}

	confirmed, err := confirmCleanUp(found)
	if err != nil {
		return output, err
	}
	if !confirmed {
		return output, nil
	}

	moved, moveErr := saves.PutAwayOrphans(found.OnRomM)
	if moveErr != nil {
		gaba.GetLogger().Error("Could not put away every leftover save", "moved", moved, "error", moveErr)
		tellAdvanced(localizeWith("clean_up_saves_partial",
			"Put away {{.Moved}} of {{.Total}} saves.\nSome could not be moved. Check the logs.",
			map[string]any{"Moved": moved, "Total": len(found.OnRomM)}))
		return output, nil
	}

	tellAdvanced(localizeWith("clean_up_saves_done",
		"Put {{.Count}} saves in their .backup folders.",
		map[string]any{"Count": moved}))
	return output, nil
}

// confirmCleanUp lists what would go and asks for a deliberate X, since A is the
// usual confirm button everywhere else.
func confirmCleanUp(found saves.Orphans) (bool, error) {
	var size int64
	for _, entry := range found.OnRomM {
		size += entry.Size
	}

	const shown = 5
	lines := make([]string, 0, shown+1)
	for i, entry := range found.OnRomM {
		if i == shown {
			lines = append(lines, localizeWith("save_sync_and_more", "and {{.Count}} more", map[string]any{"Count": len(found.OnRomM) - shown}))
			break
		}
		lines = append(lines, entry.Local.RomName)
	}

	message := fmt.Sprintf("%s\n%s\n%s",
		localizeWith("clean_up_saves_confirm",
			"Put {{.Count}} saves ({{.Size}}) in .backup?\nThe games are gone and RomM has identical copies.",
			map[string]any{"Count": len(found.OnRomM), "Size": textmatch.FormatBytes(size)}),
		strings.Join(lines, "\n"),
		unsafeNote(len(found.NotOnRomM)),
	)

	_, err := gaba.ConfirmationMessage(
		strings.TrimRight(message, "\n"),
		[]gaba.FooterHelpItem{
			FooterCancel(),
			{ButtonName: "X", HelpText: localize("clean_up_saves_action", "Put Away")},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func unsafeNote(count int) string {
	if count == 0 {
		return ""
	}
	return localizeWith("clean_up_saves_unsafe",
		"{{.Count}} more are not on RomM and stay.",
		map[string]any{"Count": count})
}
