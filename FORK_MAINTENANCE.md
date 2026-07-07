# Fork Maintenance

This fork keeps `main` as a deployable branch made from latest `upstream/main`
plus a small set of fork-only commits. These changes are not intended to be
opened as upstream PRs unless explicitly decided later.

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
- `docker/docker-compose.yml`: ensure local build still starts the correct app
  command and persists runtime paths.

## Validation Checklist

Run before pushing rebased `main`:

```bash
go test ./...
cd frontend && npm run build
```

For deployment validation:

```bash
cd docker
docker compose build --no-cache app
docker compose up -d
docker compose logs -f app
```
