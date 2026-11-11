# This fork

A personal fork of [rommapp/grout](https://github.com/rommapp/grout) that carries Onion (Miyoo Mini) work the
upstream project chose not to take: opt-in save sync around launching a game, a per-game core setting, handling
for a save that is in two core folders, and an optional patch to Onion's `runtime.sh`. Upstream declined it in
rommapp/grout #318–#324 (the maintainer does not want Grout changing files a CFW owns), so it lives here.

| Branch | What it is |
|---|---|
| `main` | A pure mirror of upstream. Never committed to; fast-forwarded every sync. |
| `tim` | Upstream plus the work. The default branch, so the scheduled workflows run from it. |

## Getting a build

Every push to `tim` and every sync builds the Onion package and publishes it as a release on this fork:

- **`tim-latest`**: the newest build, replaced each time.
- **`v<version>-tim.<sha>`**: an immutable copy of every build, to roll back to. The newest ten are kept.

Download `Grout-Onion-tim.zip`, unzip it, and copy the `Grout` folder to `App/` on the SD card.

Once a stamped build is installed (every build since the update channel was added), Grout's own update button
updates to the newest `tim-latest` build. It never moves to upstream by itself: that is
**Settings > Advanced > Switch to Upstream Grout**. Each build records the commit it came from in
`Grout/grout/fork-build` and publishes `fork-update.json` (commit, address, size, SHA-256) beside the zip; the app
compares the commit, since the fork keeps upstream's version number. The first stamped build has to be copied over by
hand.

## How it stays current

`fork-sync.yml` runs twice a day: it mirrors upstream onto `main`, rebases `tim` onto it, pushes, and builds. A
rebase that conflicts is retried as one squashed commit (the old stack is kept as the tag `tim-pre-sync-YYYYMMDD`),
and locale strings that both sides added are merged by `.github/scripts/fork-resolve.mjs`. Anything else is a real
collision with upstream: the run fails and nothing is pushed. Issues are off on this fork, so the failed run's
email is the alert.

Run it by hand when the fork is behind:

```sh
gh workflow run fork-sync.yml --repo Squeaks72/grout --ref tim
```

The `SYNC_TOKEN` secret (a fine-grained PAT on this repo with Contents and Workflows read/write) is needed on the
days upstream changes a file under `.github/workflows/`, which `GITHUB_TOKEN` is not allowed to push. Upstream's own
workflows are kept disabled here.

## The plan to stop using this fork

This fork exists only because upstream will not take the work. The intent is to move back to upstream's release and
switch the automation off, but **only once upstream adequately handles every feature below**. One feature missing
means the fork stays, because moving back would lose it.

"Adequately" means a player gets the same outcome, not the same implementation: upstream may use a different hook,
a launcher, a service or a different screen. It does not have to match the fork's wording or layout.

### Feature checklist

The nightly caretaker keeps the Status column current (`Fork only` or `Upstream` plus the release/PR that covers it)
and records when a feature was dropped from the fork because upstream took over (upstream wins on overlap).

| # | Feature | What upstream has to do | Status |
|---|---------|-------------------------|--------|
| 1 | Sync before launch | On Onion, bring a game's save up to date from RomM as the game starts, with no step by the player, stopping quietly with no known Wi-Fi | Fork only |
| 2 | Upload after quit and at power-off | Send the save back after the game is quit, and when the device is powered off from the game | Fork only |
| 3 | Games on a core of their own | The launch and after-quit sync also covers games given a per-game core (not just games started through the emulator's `launch.sh`) | Fork only |
| 4 | Resume state | A downloaded save is not undone by the game's auto resume state | Fork only |
| 5 | Sync save in the game's Y menu | A per-game "sync this save now" entry reachable from the game list, with no need to open Grout first | Fork only |
| 6 | Honest sync results | A sync says what it did (took RomM's save, sent the device's, really in sync, or why nothing happened), never a bare "up to date" | Fork only |
| 7 | Game Core row | Set a downloaded game's core in Grout, with its save following that core's folder | Fork only |
| 8 | Saves in two core folders | Detect a game whose save sits in more than one core folder and let the player settle it, including choosing RomM's save | Fork only |
| 9 | Use RomM's Save / Upload Local Save | Per-game override of which side wins, with a confirmation, whatever is newer | Fork only |
| 10 | Clean Up Saves | Put away saves of games no longer on the device, only when RomM holds an identical copy, never deleting | Fork only |
| 11 | DNS vs network error wording and short startup retry | A resolver that is not answering yet is not reported as "could not resolve hostname" | Fork only |

Not a feature to cover: the **Switch to Upstream Grout** button and the fork's own build and sync workflows.

### How the caretaker decides

The nightly Claude caretaker task (see below) reads upstream every night and rates each feature above as covered or
not covered, and then rates upstream overall:

- **Full**: every feature above is covered by a **published, non-prerelease** upstream release at least 48 hours old,
  shown by both its docs or changelog and its code. Only then is the fork retired: the caretaker marks this file as
  retired, disables the `fork-*` workflows, turns itself off, and reports the checklist for the Miyoo. Nothing is
  deleted; re-enabling the workflows undoes it.
- **Partial**: at least one feature is not covered, covered only by a button, only inside Grout, only on some
  platforms, or only in an unreleased branch, pull request or pre-release. Reported as worth a look with the list of
  what is still missing; nothing changes.
- **None**: nothing relevant.

If upstream covers one feature before the rest, upstream wins on that feature: the fork drops its own version (Status
becomes `Upstream`) and keeps the rest. A fork whose every row reads `Upstream` is a Full.

Moving the Miyoo back is one button: **Settings > Advanced > Switch to Upstream Grout**. After a confirmation (press
X) it downloads upstream's current stable release (whatever its version number, checksum-verified), then removes the
fork's additions: the `launch.sh` lines, the `runtime.sh` lines, the Y-menu "Sync save (Grout)" script, and it turns
Sync on Launch and Cover Core Overrides off. Grout exits and the launcher swaps the new version in on the next start.
If the download fails nothing is changed.

What it leaves alone: per-game cores in Onion's own `Roms/<system>/.game_config/` keep working, and the `.backup`
folders in `Saves/` can be deleted once the saves are confirmed good. After the swap, `grout/config.json` is kept;
upstream ignores the fork's extra settings. If a hook line is ever left behind, search `Emu/*/launch.sh` for
`# grout-sync-on-launch`, and copy `.tmp_update/runtime.sh.grout-backup` over `runtime.sh`.

## Changing it

Edit on a branch off `tim`, merge or push to `tim`, and the build runs. Do not open pull requests upstream for this
work unless asked.
