# Template Display Name Preservation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve local template display names across Meta synchronization and display-only edits.

**Architecture:** Compare the persisted and updated template's Meta-owned fields before changing approval status. During Meta upsert, omit `display_name` from updates to existing rows while retaining the current name fallback for new imports.

**Tech Stack:** Go, GORM, testify, PostgreSQL handler tests

## Global Constraints

- Keep `name` as the Meta identifier and `display_name` as local presentation metadata.
- Do not change API payload shapes, database schema, or Meta submission behavior.
- Keep the patch on `fix/template-sync-integrity`; do not modify upstream.

---

### Task 1: Protect the display-name contract

**Files:**
- Modify: `internal/handlers/templates_test.go`
- Modify: `internal/handlers/templates.go`
- Modify: `FORK_MAINTENANCE.md`

**Interfaces:**
- Consumes: `TemplateRequest` and Meta template-list responses
- Produces: unchanged `TemplateResponse`, with stable local `display_name`

- [x] **Step 1: Add a failing display-only update test**

  Create approved and rejected templates, send only a new `display_name`, and
  assert both the label and original status are preserved. Existing content
  edit tests continue asserting a transition to `DRAFT`.

- [x] **Step 2: Run the focused update tests and confirm the new assertion fails with `DRAFT`**

  Run `go test ./internal/handlers -run 'TestApp_UpdateTemplate_(DisplayNameOnlyPreservesStatus|ApprovedToDraft|RejectedToDraft)$'` with isolated PostgreSQL and Redis test services.

- [x] **Step 3: Add a failing existing-template synchronization test**

  Seed `synced_template_one` with display name `Event approval (EN)`, run
  `SyncTemplates`, and assert the local label remains while Meta-owned fields
  refresh. Also assert newly imported templates use their technical name as
  the fallback display name.

- [x] **Step 4: Run the focused sync tests and confirm the label is overwritten before the fix**

  Run `go test ./internal/handlers -run 'TestApp_SyncTemplates_(PreservesExistingDisplayName|Success)$'` with isolated PostgreSQL and Redis test services.

- [x] **Step 5: Implement the minimal handler changes**

  Move the `DRAFT` transition until after updates and gate it on differences
  in Meta-owned fields. Remove `display_name` from the existing-row sync update
  map; leave new-template initialization unchanged.

- [x] **Step 6: Record fork maintenance and upstream-watch guidance**

  Document the local/provider ownership boundary, regression tests, rebase
  conflict location, and instruction to drop the fork patch if upstream later
  implements equivalent behavior.

- [x] **Step 7: Run focused handler tests and inspect the final diff**

  Run the update/sync test group, `git diff --check`, and review only the
  expected handler, tests, maintenance, design, and plan files.

- [x] **Step 8: Commit the verified patch on the current branch**

  Commit with `fix: preserve template display names`.
