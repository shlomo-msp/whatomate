# Environment Override Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct fork deployment and documentation to use the parser's double-underscore environment override contract.

**Architecture:** Preserve the host-facing Compose variables and translate them into correctly named Whatomate application overrides. Keep examples, operator documentation, and fork-maintenance guidance synchronized with that contract.

**Tech Stack:** Docker Compose, Koanf environment configuration, Astro/Starlight MDX documentation

## Global Constraints

- Do not modify or push to `upstream`.
- Do not add ambiguous compatibility parsing for legacy single-underscore names.
- Keep the changes on `fix/template-sync-integrity` until testing is complete.

---

### Task 1: Correct and document the deployment contract

**Files:**
- Modify: `docker/docker-compose.yml`
- Modify: `.env.example`
- Modify: `docker/.env.example`
- Modify: `docs/src/content/docs/getting-started/configuration.mdx`
- Modify: `docs/src/content/docs/getting-started/quickstart.mdx`
- Modify: `FORK_MAINTENANCE.md`

**Interfaces:**
- Consumes: `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, and `REDIS_PASSWORD` from the host environment file
- Produces: `WHATOMATE_DATABASE__USER`, `WHATOMATE_DATABASE__PASSWORD`, `WHATOMATE_DATABASE__NAME`, and `WHATOMATE_REDIS__PASSWORD` for the application container

- [x] **Step 1: Run a source-contract check and confirm it fails on the legacy Compose names**
- [x] **Step 2: Replace legacy application override names and align both environment samples**
- [x] **Step 3: Correct the configuration and quickstart documentation**
- [x] **Step 4: Record the fork rationale and upstream recheck instruction**
- [x] **Step 5: Render and validate the Compose configuration**
- [x] **Step 6: Run focused config tests and repository diff checks**
- [x] **Step 7: Commit the verified files on the current branch**
