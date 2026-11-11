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

Grout's in-app updater knows nothing about this fork and will offer upstream's release. Taking it replaces this
build with upstream's, which does not understand `--sync-rom`: turn **Sync on Launch** off first.

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

This fork exists only because upstream will not take the work. The goal it serves is: **on Onion (Miyoo Mini), a
game's save is brought up to date from RomM automatically as the game starts, and sent back after it is quit,
without opening Grout or pressing anything first.** When upstream does that, the fork is no longer needed, and the
intent is to move back to upstream's release and switch the automation off altogether.

The nightly Claude caretaker task (see below) watches for this. It classifies upstream as:

- **Full**: a published upstream release (not a pull request, branch or pre-release) that does the above on Onion
  automatically. The fork is retired: the caretaker disables `fork-sync.yml` and `fork-build.yml`, marks this file as
  retired, turns itself off, and reports the checklist for the Miyoo. Nothing is deleted; re-enabling the workflows
  undoes it.
- **Partial**: upstream does some of it, but needs a manual step (a sync button, launching games from inside Grout, a
  subset of the features). Reported as worth a look; nothing changes.
- **None**: nothing to do.

Moving the Miyoo back is done by hand, on the device:

1. In this build's Grout: Settings > Save Sync, set **Cover Core Overrides** to False and restart the device, then set
   **Sync on Launch** to **Off**. Turning it off removes the lines Grout added to each emulator's `launch.sh`.
2. Delete `App/romscripts/Grout - Sync save.sh`, and check that no `Emu/*/launch.sh` still contains
   `# grout-sync-on-launch`. If `.tmp_update/runtime.sh` still has such lines, copy `runtime.sh.grout-backup` over it.
3. Install upstream's current release over `App/Grout`, keeping `grout/config.json`.
4. Per-game cores live in Onion's own `Roms/<system>/.game_config/` and keep working. The `.backup` folders in
   `Saves/` can be deleted once the saves are confirmed good.

## Changing it

Edit on a branch off `tim`, merge or push to `tim`, and the build runs. Do not open pull requests upstream for this
work unless asked.
