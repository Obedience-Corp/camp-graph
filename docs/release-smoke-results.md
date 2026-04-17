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
| Initial `build` (13,143 nodes, 4,332 search docs, 19,758 edges) | 4.28 s |
| Warm `query "JobSearch" --scope "Work/JobSearch" --limit 10 --json` | 0.021 s |
| `related --path "Work/JobSearch/Action Plan.md" --limit 10 --json` | 0.030 s |

All timings are well within the responsiveness bar implied by the
release design.

## Notes

- `status --json` reports `indexed_files: 0` after an initial build
  because the full-build path does not populate the
  `indexed_files` table yet; `refresh` populates it on the second run.
  This is documented as follow-up work; the build still produces a
  coherent graph DB with `graph_meta`, `search_docs`, and
  `search_docs_fts` populated.
- Ranking ordering matches the contract: `same_scope` items surface
  before cross-scope lexical hits in the related output.
