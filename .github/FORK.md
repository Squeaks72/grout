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

## Changing it

Edit on a branch off `tim`, merge or push to `tim`, and the build runs. Do not open pull requests upstream for this
work unless asked.
