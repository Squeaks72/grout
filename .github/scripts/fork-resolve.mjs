#!/usr/bin/env node
// Resolves the conflicts of `git merge --squash` that are only two sides adding strings to the same place
// in a locale file. Anything else is a real collision and fails, leaving the merge for the caller to abort.
//
// During the squash, "ours" is upstream and "theirs" is this fork's work. Both sides' lines are kept, once
// each, in key order. A key both sides set to different values keeps upstream's.

import { execFileSync } from 'node:child_process'
import { readFileSync, writeFileSync } from 'node:fs'

const conflicted = execFileSync('git', ['diff', '--name-only', '--diff-filter=U'], { encoding: 'utf8' })
  .split('\n').filter(Boolean)

const LOCALE = /^resources\/locales\/active\.[A-Za-z-]+\.toml$/
const ENTRY = /^([A-Za-z0-9_]+) = .*$/
const HUNK = /<<<<<<< [^\n]*\n([\s\S]*?)=======\n([\s\S]*?)>>>>>>> [^\n]*\n/g

let unresolvable = false

for (const file of conflicted) {
  if (!LOCALE.test(file)) {
    console.error(`cannot resolve ${file}: not a locale file`)
    unresolvable = true
    continue
  }
  let text = readFileSync(file, 'utf8')
  let ok = true
  text = text.replace(HUNK, (_, ours, theirs) => {
    const lines = [...ours.split('\n'), ...theirs.split('\n')].filter((l) => l !== '')
    if (!lines.every((l) => ENTRY.test(l))) {
      ok = false // a table or a multi-line value: not a plain addition
      return _
    }
    const byKey = new Map()
    for (const l of lines) {
      const key = ENTRY.exec(l)[1]
      if (!byKey.has(key)) byKey.set(key, l) // upstream's line comes first, so it wins
    }
    return [...byKey.keys()].sort().map((k) => byKey.get(k)).join('\n') + '\n'
  })
  if (!ok || /^(<<<<<<<|=======|>>>>>>>)/m.test(text)) {
    console.error(`cannot resolve ${file}: a conflict that is not plain additions`)
    unresolvable = true
    continue
  }
  writeFileSync(file, text)
  execFileSync('git', ['add', file])
  console.log(`resolved ${file}`)
}

process.exit(unresolvable ? 1 : 0)
