package ui

import (
	"errors"
	"fmt"

	"grout/cfw"
	"grout/settings"
	"grout/textmatch"
	"grout/update"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"go.uber.org/atomic"
)

// switchToUpstream moves this install off the fork and onto the upstream
// project's current release.
//
// The fork's own update check compares version numbers, which says nothing
// about whose build is installed, so this installs upstream's release whether
// or not it looks newer. The download is staged first: if it fails nothing has
// changed. Only then are the fork's additions to the frontend taken out, since
// upstream's Grout does not know about them and would leave them behind.
func (s *AdvancedSettingsScreen) switchToUpstream(input AdvancedSettingsInput, output AdvancedSettingsOutput) (AdvancedSettingsOutput, error) {
	output.Action = AdvancedSettingsActionBack
	config := input.Config
	activeCFW := cfw.GetCFW()

	info, err := gaba.ProcessMessage(
		localize("update_checking", "Checking for updates..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (*update.Info, error) { return update.CheckUpstreamRelease(activeCFW) },
	)
	if err != nil {
		gaba.GetLogger().Error("Failed to look up upstream release", "error", err)
		tellAdvanced(localizeWith("update_check_error", "{{.Error}}", map[string]any{"Error": err.Error()}))
		return output, nil
	}

	confirmed, err := confirmSwitchToUpstream(info)
	if err != nil {
		return output, err
	}
	if !confirmed {
		return output, nil
	}

	progress := &atomic.Float64{}
	_, err = gaba.ProcessMessage(
		localize("update_downloading", "Downloading update..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true, ShowProgressBar: true, Progress: progress},
		func() (any, error) {
			return nil, update.PerformUpdate(activeCFW, info.DownloadURL, info.AssetSize, info.AssetSHA256, progress)
		},
	)
	if err != nil {
		gaba.GetLogger().Error("Failed to install upstream release", "error", err)
		tellAdvanced(localizeWith("update_failed", "Update failed: {{.Error}}", map[string]any{"Error": err.Error()}))
		return output, nil
	}

	cleaned := removeForkAdditions(config)

	message := localize("switch_upstream_done", "Switched to upstream Grout.\nGrout will now exit.")
	if !cleaned {
		message = localize("switch_upstream_done_unclean",
			"Switched to upstream Grout, but some of the fork's\nadditions could not be removed. Check the logs.\nGrout will now exit.")
	}
	gaba.ConfirmationMessage(
		message,
		[]gaba.FooterHelpItem{{ButtonName: "A", HelpText: localize("button_exit", "Exit")}},
		gaba.MessageOptions{},
	)

	output.Action = AdvancedSettingsActionSwitchToUpstream
	return output, nil
}

// confirmSwitchToUpstream needs a deliberate X: A is the usual
// confirm button everywhere else, so it must not be enough here.
func confirmSwitchToUpstream(info *update.Info) (bool, error) {
	message := fmt.Sprintf("%s\n%s\n%s",
		localize("switch_upstream_warning", "Switch to the upstream Grout project?\nThe fork's save sync on launch, Game Core and\nresolve saves features will no longer be available."),
		localizeWith("switch_upstream_version", "Installs {{.Version}} ({{.Size}})",
			map[string]any{"Version": info.LatestVersion, "Size": textmatch.FormatBytes(info.AssetSize)}),
		localize("switch_upstream_hooks", "Its changes to your emulators will be removed."),
	)

	_, err := gaba.ConfirmationMessage(
		message,
		[]gaba.FooterHelpItem{
			FooterCancel(),
			{ButtonName: "X", HelpText: localize("switch_upstream_confirm", "Switch")},
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

// removeForkAdditions takes out everything the fork added to the frontend and
// turns its settings off, and reports whether all of it went. Each step is
// tried regardless of the others.
func removeForkAdditions(config *settings.Config) bool {
	logger := gaba.GetLogger()
	firmware := cfw.ActiveFirmware()
	clean := true

	if _, err := firmware.SetSyncOnLaunchHook(false, false); err != nil {
		logger.Error("Failed to remove launch hook", "error", err)
		clean = false
	}
	if err := firmware.SetRuntimeHook(false); err != nil {
		logger.Error("Failed to remove runtime hook", "error", err)
		clean = false
	}
	if err := firmware.RemoveSyncMenuEntry(); err != nil {
		logger.Error("Failed to remove Sync save menu entry", "error", err)
		clean = false
	}

	config.SyncOnLaunch = settings.LaunchSyncOff
	config.CoverCoreOverrides = false
	if err := settings.SaveConfig(config); err != nil {
		logger.Error("Failed to save config after switching to upstream", "error", err)
		clean = false
	}
	return clean
}

func tellAdvanced(message string) {
	gaba.ConfirmationMessage(message, []gaba.FooterHelpItem{FooterBack()}, gaba.MessageOptions{})
}
