# Release Smoke Validation Results

## Environment

- Vault: `/Users/lancerogers/iCloud/ObsidianVault`
- Binary: `projects/camp-graph/bin/camp-graph`
- Schema: `graphdb/v2alpha1`

## Smoke Commands

```
cd /Users/lancerogers/iCloud/ObsidianVault
camp-graph build
camp-graph query "JobSearch" --scope "Work/JobSearch" --limit 10 --json
camp-graph related --path "Work/JobSearch/Action Plan.md" --limit 10 --json
camp-graph query "ShinySwap" --scope "Business/ShinySwap" --limit 10 --json
camp-graph related --path "Business/ShinySwap/DesignDocs/ActionPlan.md" --limit 10 --json
camp-graph status --json
```

## Pass Conditions (Contract)

- [x] `query JobSearch` returns at least one result under `Work/JobSearch`
  (observed: 10 results, all scoped to `Work/JobSearch`)
- [x] `related Action Plan.md` returns at least three items from
  `Work/JobSearch` (observed: 5 `same_scope` items from `Work/JobSearch`)
- [x] `query ShinySwap` returns at least one result under `Business/ShinySwap`
  (observed: 1 result under `Business/ShinySwap`)
- [x] `related DesignDocs/ActionPlan.md` returns at least three items from
  `Business/ShinySwap` (observed: 4+ items under
  `Business/ShinySwap/DesignDocs`)
- [x] `status --json` reports `search_available=true` (observed: `true`)

## Wall-Clock Timings

| Command | Wall Time |
| ------- | --------- |
| Initial `build` (13,143 nodes, 4,332 search docs, 18,894 indexed files, 19,758 edges) | 7.45 s |
| Warm `query "JobSearch" --scope "Work/JobSearch" --limit 10 --json` | 0.021 s |
| `related --path "Work/JobSearch/Action Plan.md" --limit 10 --json` | 0.030 s |
| `refresh --json` (no-op fast path when inventory diff is empty) | sub-100 ms (unit-tested) |

All timings are well within the responsiveness bar implied by the
release design. The build path now performs SHA-256 fingerprinting
on every worktree file so `indexed_files` is accurate immediately
after `build`; the extra ~3 s vs. the pre-fingerprint run is worth
the correctness gain.

## Notes

- `status --json` reports `indexed_files: 18894` immediately after
  `build`. Fingerprint rows are now written in the same transaction
  as nodes, edges, search_docs, and graph_meta.
- `refresh` takes a no-op fast path when the inventory diff reports
  zero added/changed/deleted files: it updates only `last_refresh_at`
  and `last_refresh_mode`, avoiding the SaveFullBuild rewrite.
- Ranking ordering matches the contract: `same_scope` items surface
  before cross-scope lexical hits in the related output.
