# Todo: Recipe feedback & fork

See `tasks/recipe-fork/plan.md` for full specs. Spec: `SPEC.md`.

## Phase 1: De-risk and foundation
- [ ] T1 (M): `Reviser` interface, prompt, parsing, Anthropic impl, `ANTHROPIC_REVISE_MODEL`
- [ ] T2 (S): `internal/diff` line diff
- [ ] **Checkpoint A**: quality probe, at least 7/10 revisions acceptable (human gate)
- [ ] T3 (M): Migration 010 + feedback capture end to end

## Phase 2: Fork and exclusion
- [ ] T4 (M): `CreateFork` in the store
- [ ] T5 (M): Superseded exclusion in list/page/search/random + `IncludeSuperseded` for export/import
- [ ] **Checkpoint B**: store behaviour, tests + lint
- [ ] T6 (M): "Suggest a fix" shows a diff
- [ ] T7 (M): Accept and save as fork; lineage UI
- [ ] **Checkpoint C**: end-to-end in browser (human review)

## Phase 3: Harden and ship
- [ ] T8 (S): Delete and edge behaviour
- [ ] T9 (S): Hardening (limits, ingress path, logging)
- [ ] T10 (XS): Release (bump `addon/config.yaml`, tag)
