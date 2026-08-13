# Fork Maintenance

This fork keeps `main` as a deployable branch made from latest `upstream/main`
plus a small set of fork-only commits. These changes are not intended to be
opened as upstream PRs unless explicitly decided later.

## Upstream Sync Status

- Last reviewed and rebased: 2026-08-13.
- Fork `main` is based on `upstream/main` commit `2e05458`.
- That sync includes upstream WebSocket recovery, media compression/viewer,
  outgoing document-filename persistence, frontend correctness fixes, and
  dependency updates through that commit.

## Branch Policy

- `upstream/main`: read-only reference to upstream.
- `main`: deployable fork branch, rebased onto `upstream/main`.
- `backup-main-before-upstream-YYYY-MM-DD`: safety branch before each rebase.
- `dockerbuildlocal`: legacy production branch until environments are moved.

Avoid merge commits from upstream into `main`. Use rebase so history stays:

```text
upstream/main
fork-only commit
fork-only commit
...
```

## Rebase Workflow

```bash
git fetch upstream
git switch main
git branch backup-main-before-upstream-YYYY-MM-DD main
git rebase upstream/main
# resolve conflicts
go test ./...
cd frontend && npm run build
git push --force-with-lease origin main
```

If upstream has implemented an equivalent feature, prefer dropping the fork
change instead of carrying duplicate behavior.

## Fork-Only Changes To Preserve

### Local Docker Deployment

Reason: deployments must build this fork's code, not use upstream's published
image.

Keep:
- `docker/docker-compose.yml` uses local `build`.
- Runtime mounts for `uploads`, `audio`, and `config.toml`.
- `docker/.env` stays untracked.
- Docker startup must run `whatomate server -migrate -config ...` either through
  image `CMD` or a full compose command; do not override it with just `server`.

### Docker Environment Override Contract

Reason: the config parser introduced upstream in `28ef7ce` uses a double
underscore between a config section and field. Single-underscore deployment
variables are silently loaded under the wrong key and do not override
`config.toml`.

Keep:
- Application overrides use `WHATOMATE_<SECTION>__<KEY>`; underscores inside
  section or field names remain single.
- `docker/docker-compose.yml` maps `POSTGRES_USER`, `POSTGRES_PASSWORD`, and
  `POSTGRES_DB` to `WHATOMATE_DATABASE__USER`,
  `WHATOMATE_DATABASE__PASSWORD`, and `WHATOMATE_DATABASE__NAME`.
- The Compose Redis password is passed to the application as
  `WHATOMATE_REDIS__PASSWORD`.
- Root and Docker environment examples, plus configuration/quickstart docs,
  stay aligned with the Compose contract.

Upstream watch: on every upstream sync, check whether upstream has corrected
its environment-variable documentation and wired Compose database credentials
into the application with double-underscore names. Incorporate an equivalent
upstream fix and drop any duplicate fork-only changes; do not change upstream
from this fork.

Existing-host check: the standard `POSTGRES_*` and `REDIS_PASSWORD` names do
not change. Before the first deployment of this fix, confirm each host defines
the expected values and that they match the credentials currently used in
`config.toml`/the initialized services. Rename any direct legacy
`WHATOMATE_<SECTION>_<KEY>` overrides to use the double separator. This matters
because the corrected application variables now take precedence over
`config.toml` as originally intended.

### Local Secret And Runtime Ignore Rules

Reason: production migration bundles, runtime media, generated frontend output,
and local secrets should not be accidentally copied into Docker contexts or
committed during fork maintenance.

Keep:
- Root `.dockerignore`.
- Ignore rules for `.env`, `config.toml`, `uploads/`, `audio/`,
  `docker/uploads/`, `docker/audio/`, scratch directories, logs,
  `frontend/node_modules/`, `frontend/dist/`, Playwright reports/results, and
  embedded frontend build output.

### Fork-Safe GitHub Workflows

Reason: this fork deploys from its own `main` branch, but should not
automatically run upstream publish/release workflows or upstream image pushes.

Keep:
- `.github/workflows/deploy-docs.yml` is manual-only via `workflow_dispatch`.
- `.github/workflows/develop-image.yml` is manual-only and guarded with
  `github.repository == 'shridarpatil/whatomate'`.
- `.github/workflows/e2e-tests.yml` is manual-only.
- `.github/workflows/release.yml` is manual-only and guarded with
  `github.repository == 'shridarpatil/whatomate'`.
- `.github/workflows/test.yml` is manual-only.

### Public Registration Disabled By Default

Reason: public deployments should not allow arbitrary organization/user signup.

Keep:
- `[auth].public_registration_enabled = false`
- `Register` rejects public registration unless explicitly enabled.

### Internal Webhook URL Safety

Reason: public deployments should not allow webhook/custom-action SSRF to
private networks unless intentionally used for trusted internal services like
n8n.

Keep:
- `[app].allow_internal_webhook_urls = false` by default.
- Production configs may set it true only when internal webhook targets are
trusted and required.

### Two-Factor Authentication

Reason: production admin/user protection.

Keep:
- TOTP fields on `User`.
- `/api/me/2fa/*` and `/api/auth/2fa/*` handlers.
- Organization `require_2fa` setting.
- Profile/login/settings UI for 2FA.

### Durable Outbound Webhook Delivery

Reason: integrations should not lose webhook events on temporary failures.

Keep:
- `WebhookDelivery` model.
- `WebhookDeliveryProcessor`.
- Outbound webhook `delivery_id`.
- Retry management endpoints and UI.

Integration note: receivers should treat `delivery_id` as the idempotency key.

### Media Retention And Media IDs

Reason: local media should be manageable over time, and WhatsApp media IDs should
remain visible/persisted for integrations and diagnostics.

Keep:
- `Message.MediaID`.
- Media ID in message responses where available.
- `MediaCleanupProcessor`.
- Organization media-retention settings.

### Super-Admin Organization Deletion

Reason: self-hosted admins need a controlled way to remove organizations.

Keep:
- `DELETE /api/organizations/{id}`.
- Guardrails preventing deletion of current/default/last organization.
- Settings UI delete action for super admins only.

### IVR Count Fix

Reason: dashboard/settings counts must include IVR flows correctly.

Keep the fork's IVR counting fix unless upstream has an equivalent fix.

### Contact Message Payload Compatibility

Reason: upstream docs showed top-level `text`, while existing code required
`content.body`.

Keep support for both request shapes:

```json
{ "type": "text", "text": "Hello" }
```

```json
{ "type": "text", "content": { "body": "Hello" } }
```

### Chatbot Auto Reply For Supported Inbound Types

Reason: acknowledgement-style chatbot replies should fire for any supported
incoming WhatsApp message, including media-only messages, not just text.

Keep:
- `chatbotInputForMessage` maps supported empty-content message types to a
  stable chatbot input marker.
- Unknown/unsupported Meta message types are skipped instead of auto-replied.
- Stored message content remains the real incoming text/caption; synthetic
  markers are only for chatbot routing/session history.

### Super-Admin Selected Organization For Browser Media And WebSocket

Reason: Axios requests can send `X-Organization-ID`, but native browser media
requests (`img`, `video`, `audio`, download links) cannot attach that custom
header. WebSocket connections also authenticate after the upgrade with a
short-lived token, so the token endpoint must honor the selected organization.

Keep:
- `frontend/src/views/chat/ChatView.vue` appends
  `organization_id=<selected_organization_id>` to `/api/media/{message_id}`.
- `internal/handlers/app.go` accepts `organization_id` as a GET-only fallback
  in `getOrgID` when `X-Organization-ID` is absent.
- The same existing org-switch checks still apply: super admins can access any
  existing org, non-super-admin users only orgs where they have membership.
- `internal/handlers/auth.go` uses `getOrgID` when generating
  `/api/auth/ws-token` tokens, so real-time events bind to the selected org.
- Regression tests in `internal/handlers/media_test.go` and
  `internal/handlers/auth_gaps_test.go`.

### Template Sync Persistence Integrity

Reason: Reservations treats a successful Whatomate template sync followed by a
template list as authoritative reconciliation evidence. A partial local sync
must not be reported as successful, or downstream recovery can repeat a Meta
template mutation.

Keep:
- `SyncTemplates` returns an internal-server error when its local template
  lookup fails unexpectedly.
- Template creates and updates check both the database error and the affected
  row count before incrementing the synced count.
- `TestApp_SyncTemplates_FailsWhenLocalPersistenceFails` verifies the endpoint
  fails closed instead of returning a partial success.

### Template Display Name Ownership

Reason: Meta owns the technical template `name`, while `display_name` is a
Whatomate-only friendly label used by operators. Meta template responses do not
contain that local label, so synchronization must not overwrite it, and editing
it must not trigger a provider review.

Keep:
- `SyncTemplates` initializes `display_name` from `name` for a new Meta import
  but preserves the existing local value during later syncs.
- `UpdateTemplate` changes an `APPROVED` or `REJECTED` template to `DRAFT` only
  when a provider-owned field changes. A display-name-only or no-op save keeps
  the current provider status.
- `TestApp_SyncTemplates_PreservesExistingDisplayName` covers both the existing
  label and new-import fallback behavior.
- `TestApp_UpdateTemplate_DisplayNameOnlyPreservesStatus` is paired with the
  existing approved/rejected content-edit tests so review-triggering changes
  remain protected.

Upstream watch: on every upstream sync, check whether upstream preserves local
display names during template synchronization and excludes display-name-only
updates from provider review transitions. If equivalent behavior and tests are
present, incorporate the upstream implementation and drop this duplicate fork
patch.

## Rebase Conflict Checks

When conflicts happen, check these areas carefully:

- `internal/handlers/organization.go`: merge upstream Meta embedded-signup
  settings with fork 2FA/media-retention/org-delete settings.
- `frontend/src/views/settings/SettingsView.vue`: preserve both upstream Meta
  app credentials UI and fork settings UI.
- `internal/models/models.go`: preserve upstream account fields such as SMB/PIN
  and fork fields such as TOTP, media ID, webhook delivery, and business calling.
- `frontend/src/services/api.ts`: preserve upstream interactive flow support and
  fork voice-call/message compatibility typing.
- `frontend/src/views/chat/ChatView.vue`: preserve selected-org media URLs for
  browser-loaded media.
- `internal/handlers/app.go`: preserve GET-only `organization_id` fallback in
  `getOrgID` unless upstream has another safe browser-media org override.
- `internal/handlers/auth.go`: preserve selected-org WebSocket token generation.
- `internal/handlers/templates.go`: preserve both fail-closed template sync and
  the separation between Meta-owned template content and local display names.
- `docker/docker-compose.yml`: ensure local build still starts the correct app
  command, persists runtime paths, and passes database/Redis credentials with
  the `WHATOMATE_<SECTION>__<KEY>` naming contract.
- `.github/workflows/*.yml`: keep fork publish/release/test automation manual or
  upstream-repository guarded unless this fork intentionally enables CI.
- `.dockerignore`: keep local secrets, runtime uploads/audio, scratch files, and
  generated build/test artifacts out of Docker contexts.

## Validation Checklist

Run before pushing rebased `main`:

```bash
go test ./...
cd frontend && npm run build
```

For the selected-org media/WebSocket patch specifically:

```bash
go test ./internal/handlers -run "ServeMedia|GetWSToken"
```

For template-sync reconciliation integrity specifically:

```bash
go test ./internal/handlers -run "TestApp_(SyncTemplates_(Success|FailsWhenLocalPersistenceFails|PreservesExistingDisplayName)|UpdateTemplate_(DisplayNameOnlyPreservesStatus|ApprovedToDraft|RejectedToDraft))"
```

For deployment validation:

```bash
docker compose --env-file docker/.env.example -f docker/docker-compose.yml config --no-env-resolution -q
cd docker
docker compose build --no-cache app
docker compose up -d
docker compose logs -f app
```
