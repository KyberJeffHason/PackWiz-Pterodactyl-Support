# AI AGENT BUILD PROMPT — Packwiz Manager for Pterodactyl v0.2

You are the lead engineer responsible for building **Packwiz Manager for Pterodactyl v0.2** as a production-quality open-source project.

Do not produce a mockup, design-only prototype, or a pile of disconnected snippets. Build a working repository, tests, packaging, release automation, installer, upgrade path, and documentation.

## 0. Product definition

Packwiz Manager is a Pterodactyl extension that lets server owners manage and publish Packwiz modpacks from inside Pterodactyl.

The user experience should add a **Packwiz** server route/tab to Pterodactyl. From it, authorized users can:

- create or import a Packwiz project;
- associate one Packwiz project with one or more Pterodactyl servers;
- search and add published mods from Modrinth;
- search and add published mods from CurseForge when a CurseForge API key is configured;
- upload unpublished/custom `.jar` files;
- add arbitrary Packwiz-managed files such as configs, KubeJS files, datapacks and resource packs;
- set each managed item to `client`, `server`, or `both` where applicable;
- edit the pack metadata (Minecraft version, loader, loader version, pack name/version);
- validate the pack;
- publish immutable revisions;
- roll back to an earlier revision;
- expose a stable HTTP Packwiz URL such as:
  `https://pack.example.com/<project-slug>/pack.toml`;
- connect a Pterodactyl Minecraft server to the published Packwiz project and install/update using `packwiz-installer-bootstrap.jar -g -s server`.

The system must support custom/unpublished JARs without requiring GitHub, Modrinth or CurseForge publication.

## 1. Non-negotiable architectural constraints

### 1.1 Do not modify Pterodactyl core directly

Pterodactyl does not provide a stable first-party plugin API. Build the Pterodactyl UI/API integration as a **Blueprint extension**.

Use current Blueprint documentation and APIs rather than assumptions. In particular:

- use `dashboard.components` and `Components.yml` for the server-side React route;
- use Blueprint client routes for authenticated client APIs;
- use Blueprint admin page/controller bindings for administrator settings;
- package the extension as `packwizmanager.blueprint`;
- do not patch Pterodactyl core unless a capability is impossible through Blueprint, and if a patch is truly required, isolate it in Blueprint extension install/update/remove scripts and document why.

The project MUST support **Blueprint Docker** as a first-class deployment target.

If the installer detects stock Pterodactyl Docker without Blueprint, it must not silently modify the container or replace the image. Install the service if requested, then stop the extension-install step with a clear message explaining that Blueprint is required.

### 1.2 Separate the panel extension from the Packwiz service

Build three layers:

1. **Blueprint extension**
   - React/TypeScript UI inside Pterodactyl.
   - Laravel/PHP authenticated proxy/controllers.
   - Pterodactyl permission checks.
   - Admin configuration page.
   - Never exposes service secrets to the browser.

2. **Packwiz Manager service**
   - Implement in **Go**.
   - Single deployable Linux binary.
   - Bind to loopback by default: `127.0.0.1:8090`.
   - JSON API consumed by the Blueprint backend.
   - Uses a strong local bearer token shared only with the Blueprint backend.
   - Owns Packwiz project state, uploads, release snapshots, provider integration and publishing.
   - Invoke the real Packwiz CLI for Packwiz operations rather than reimplementing its file/index semantics unless a specific operation has no CLI equivalent.
   - Ship a pinned Packwiz binary alongside the service release.

3. **Static Packwiz publishing**
   - The Go service may serve public immutable pack artifacts under a separate public route.
   - The public endpoint must expose only published pack files, never the management API, secrets, DB, drafts, logs or arbitrary host paths.
   - Recommended layout:
     - `/public/<slug>/pack.toml` → current published revision
     - `/public/<slug>/index.toml`
     - `/public/<slug>/releases/<revision>/...`
   - Make the stable `current` switch atomic.

### 1.3 Keep the Packwiz source independent from game server volumes

Default persistent paths:

```text
/etc/packwiz-manager/
  config.yaml
  service.token

/srv/packwiz-manager/
  db/
  projects/
  blobs/
  releases/
  tmp/
```

Do not store the canonical Packwiz project in `/var/lib/pterodactyl/volumes`.

Deleting or reinstalling a Minecraft server must not delete the canonical Packwiz project.

## 2. Repository layout

Build the repository using this structure unless there is a strong technical reason to adjust it:

```text
packwiz-manager/
├── AGENTS.md
├── README.md
├── LICENSE
├── Makefile
├── VERSION
├── service/
│   ├── cmd/packwiz-manager/
│   ├── internal/
│   │   ├── api/
│   │   ├── auth/
│   │   ├── config/
│   │   ├── db/
│   │   ├── files/
│   │   ├── packwiz/
│   │   ├── providers/
│   │   │   ├── modrinth/
│   │   │   └── curseforge/
│   │   ├── projects/
│   │   ├── publishing/
│   │   ├── pterodactyl/
│   │   ├── revisions/
│   │   └── security/
│   ├── migrations/
│   ├── go.mod
│   └── go.sum
├── extension/
│   ├── conf.yml
│   ├── admin/
│   ├── api/
│   ├── dashboard/
│   │   ├── Components.yml
│   │   ├── PackwizRoute.tsx
│   │   └── components/
│   ├── data/
│   └── tests/
├── installer/
│   ├── install.sh
│   └── uninstall.sh
├── scripts/
│   ├── build-blueprint.sh
│   ├── package-release.sh
│   └── smoke-test.sh
└── .github/
    └── workflows/
        ├── ci.yml
        ├── release.yml
        └── codeql.yml
```

## 3. Data model

Use SQLite for v0.2 unless there is a compelling reason to use PostgreSQL. SQLite is appropriate because the service is a single-node management process, makes installation simple, and the canonical pack files remain on disk.

Use migrations and foreign keys.

Minimum entities:

### `projects`
- UUID primary key
- slug, unique
- display name
- Minecraft version
- loader (`neoforge`, `forge`, `fabric`, `quilt`, etc.)
- loader version
- Packwiz pack version
- working directory
- current revision ID nullable
- created/updated timestamps

### `items`
Represents a managed mod or managed file.
- UUID
- project UUID
- kind (`mod`, `resourcepack`, `datapack`, `config`, `kubejs`, `file`)
- provider (`modrinth`, `curseforge`, `custom`, `url`, `local`)
- provider project ID nullable
- provider version/file ID nullable
- display name
- target path
- filename
- side (`client`, `server`, `both`)
- expected SHA-256
- source URL nullable
- blob ID nullable
- metadata JSON
- enabled
- created/updated timestamps

### `blobs`
- SHA-256 primary key
- byte size
- MIME type
- storage path
- original filename
- created timestamp

### `revisions`
- UUID or sortable revision ID
- project UUID
- monotonically increasing revision number
- pack version
- content digest
- immutable release directory
- changelog
- created by Pterodactyl user UUID/ID when available
- created timestamp

### `server_links`
- project UUID
- Pterodactyl server UUID
- server identifier
- update-on-start boolean
- side fixed to `server`
- bootstrap state/version
- startup integration state
- last sync status

### `audit_events`
- actor
- project
- operation
- request ID
- metadata excluding secrets
- timestamp

## 4. Core Packwiz engine behavior

Use a project working directory that is a valid Packwiz pack.

Every mutation that changes Packwiz-managed files must leave the working tree valid or roll back.

Implement operations such as:

- initialize pack;
- import existing Packwiz pack;
- run `packwiz refresh`;
- run `packwiz modrinth add/install`;
- run the appropriate Packwiz CurseForge command;
- run `packwiz url add` when appropriate;
- validate `pack.toml`;
- list entries;
- update an entry's side;
- remove entries safely;
- refresh hashes;
- export/snapshot a revision.

Do not assume Packwiz has stable GitHub releases. Pin Packwiz by commit in a repository file such as `PACKWIZ_REF`; CI should build that pinned commit from source with the required Go version.

Record:
- the Packwiz commit;
- Packwiz binary SHA-256;
- Packwiz Manager version;
in release metadata.

### Custom JAR workflow

For uploaded custom JARs:

1. Accept multipart upload.
2. Enforce configured maximum size.
3. Sanitize the original filename.
4. Verify it is a regular ZIP/JAR archive; do not trust MIME type alone.
5. Do not execute the JAR.
6. Compute SHA-256 while streaming.
7. Store the binary content-addressed.
8. Require user selection of:
   - display name;
   - destination category/path;
   - side: client/server/both.
9. Generate Packwiz metadata pointing at the public content-addressed blob URL or release file URL.
10. Run `packwiz refresh`.
11. Show the item in the UI as `Custom`.
12. Never use user-supplied filenames directly as filesystem paths.

Custom JARs are executable code when later launched by Minecraft. Display a warning in the UI and restrict upload/publish permissions.

## 5. Provider integrations

### 5.1 Modrinth

Implement server-side Modrinth API integration.

Required behavior:
- search projects;
- filter by Minecraft version;
- filter by loader;
- inspect compatible versions;
- show title, icon, author, version, loaders and MC versions;
- choose a version;
- add through Packwiz;
- dependency information should be shown and Packwiz dependency behavior respected;
- update checking.

Do not call Modrinth directly from browser React code. Browser → Blueprint client API → local service → Modrinth.

Use an identifying User-Agent and implement timeout, retry with jitter for safe idempotent GETs, and rate-limit handling.

### 5.2 CurseForge

CurseForge support is optional until an administrator sets an API key.

- Secret is stored server-side only.
- Never return it to the browser.
- Search compatible mods/files.
- Filter by MC version and loader.
- Add/update through Packwiz where possible.
- If a download is prohibited by provider rules, do not proxy or circumvent it. Display an actionable error.

The administrator settings page must clearly show:
- configured/not configured;
- never reveal the current key after it is saved;
- test-connection button.

## 6. Arbitrary files and raw URL support

Users with permission may manage:
- configs;
- `defaultconfigs`;
- KubeJS;
- datapacks;
- resource packs;
- arbitrary Packwiz files.

Security requirements:
- normalize every path;
- reject absolute paths;
- reject `..`;
- reject NUL;
- reject symlinks/hardlink tricks in uploaded archives;
- configurable path allowlist for server integration;
- limit archive expansion count/size;
- atomic writes.

For remote URL imports:
- HTTP/HTTPS only;
- block loopback, private, link-local, multicast and metadata ranges;
- protect against DNS rebinding by validating resolved destinations used for the connection;
- cap redirects;
- cap size;
- time out;
- do not forward local service credentials;
- log sanitized destination information.

This SSRF protection is mandatory.

## 7. Publishing and revisions

Publishing must be transactional:

1. acquire a per-project lock;
2. validate project;
3. `packwiz refresh`;
4. verify every local/custom blob exists and hash matches;
5. create a staged immutable revision directory;
6. copy/hardlink only expected files;
7. generate revision manifest JSON;
8. fsync as appropriate;
9. atomically promote the revision;
10. atomically update the current pointer;
11. commit DB transaction;
12. emit audit event.

If any step fails, the previous published revision remains live.

Implement rollback by switching `current` to an already immutable revision, not by rewriting old revision contents.

Provide a changelog diff:
- added items;
- removed items;
- changed versions/hashes;
- side changes;
- changed raw files.

## 8. Public HTTP behavior

Management API:
- bind to loopback by default;
- protected with strong service token;
- CORS disabled or restrictive;
- no browser direct access.

Public pack endpoint:
- read-only;
- GET/HEAD only;
- immutable revision paths should emit long cache headers and ETags;
- stable current paths should use short cache/no-cache semantics suitable for updates;
- prevent directory listing;
- prevent path traversal;
- support range requests for large custom JARs where practical.

Endpoints should look like:

```text
GET /healthz
GET /readyz

# private management API
GET    /api/v1/projects
POST   /api/v1/projects
...
POST   /api/v1/projects/{id}/publish

# public
GET /public/{slug}/pack.toml
GET /public/{slug}/index.toml
GET /public/{slug}/releases/{revision}/...
GET /public/blobs/sha256/{digest}/{safe-filename}
```

## 9. Authentication and authorization

There are two trust boundaries.

### Pterodactyl → Packwiz service
The Blueprint PHP backend authenticates using a token stored in server configuration, never frontend JavaScript.

Use constant-time token validation.

### Pterodactyl user permissions
Do not rely solely on "has access to server".

Create permission concepts:
- `packwiz.read`
- `packwiz.edit`
- `packwiz.upload`
- `packwiz.publish`
- `packwiz.integration`

Use the best Blueprint/Pterodactyl-supported mapping possible. If Blueprint cannot add native Pterodactyl subuser permission strings cleanly, implement extension-level permission mapping controlled by admins and document that limitation rather than modifying core permission enums.

Admin-only operations:
- provider secrets;
- service endpoint/token;
- public host URL;
- storage settings;
- dangerous network import settings;
- global upload limits.

## 10. Pterodactyl server integration

The server route must be egg-filterable through Blueprint and intended for Minecraft servers.

Support linking a Packwiz project to a Pterodactyl server.

An "Install integration" workflow must:

1. verify project is published;
2. verify target server ownership/authorization;
3. upload a pinned, hash-verified `packwiz-installer-bootstrap.jar`;
4. write an idempotent `packwiz-start.sh` to server root;
5. configure the pack URL;
6. run Packwiz in non-GUI server mode:
   `java -jar packwiz-installer-bootstrap.jar -g -s server <PACK_URL>`;
7. start the server only if the Packwiz update succeeds;
8. never overwrite `world/`, logs, `server.properties`, ops/whitelist files or loader runtime files through Packwiz unless the user explicitly added them as managed files;
9. show current integration status.

Do not blindly rewrite Pterodactyl egg definitions.

If changing the server startup command requires administrator privileges or direct model changes, provide:
- an admin-gated "Apply startup integration" action;
- a preview of old/new startup values;
- audit logging;
- an exact revert path.

Prefer a wrapper command compatible with NeoForge/Fabric/Forge eggs.

## 11. UI requirements

The UI should feel native to Pterodactyl.

Server navigation route:
`/server/{identifier}/packwiz`

Pages/sections:
- Overview
- Mods
- Files
- Custom uploads
- Releases
- Server integration
- Settings (only if authorized)

### Overview
Show:
- project;
- MC version;
- loader;
- published revision;
- working tree status;
- public pack URL;
- server integration state;
- validation warnings.

### Mods
- search box;
- provider tabs;
- compatibility filters;
- installed list;
- update badge;
- provider badge;
- side selector;
- remove;
- inspect metadata.

### Upload custom JAR
Drag/drop plus:
- name;
- side;
- destination;
- hash after upload;
- warning that custom JARs execute server/client code.

### Releases
- revision;
- timestamp;
- actor;
- changelog;
- content digest;
- publish;
- rollback with confirmation.

Do not display raw secrets.

Use accessible labels, keyboard support, reasonable contrast, focus states and loading/error states.

## 12. Installer requirements

Maintain `installer/install.sh` as a production installer.

Supported deployment:
- Linux systemd host;
- amd64 and arm64;
- Blueprint native installation;
- Blueprint Docker installation.

The installer must:
- require root for host service installation;
- use `set -Eeuo pipefail`;
- never pipe remote scripts directly into a shell;
- resolve a GitHub release;
- download release assets to a temp dir;
- verify `SHA256SUMS`;
- detect architecture;
- install the service and bundled Packwiz binary;
- create an unprivileged `packwizmgr` system user;
- use `/etc/packwiz-manager` and `/srv/packwiz-manager`;
- generate a 32+ byte service token if absent;
- preserve existing config/secrets on upgrades;
- install a hardened systemd service;
- wait for `/healthz` and `/readyz`;
- install `packwizmanager.blueprint` only if Blueprint is detected;
- support `--service-only`;
- support `--version`;
- support `--repo`;
- support `--compose-file`;
- support `--extensions-dir`;
- support `--pack-host`;
- be idempotent;
- never replace a stock Pterodactyl image with Blueprint automatically.

Optional Cloudflare support:
- if the operator explicitly passes `--configure-cloudflare` and required environment variables, add `pack.<domain>` to an existing remotely managed tunnel without deleting existing ingress rules;
- use a scoped Cloudflare API token;
- preserve all existing tunnel ingress configuration;
- create/update the DNS CNAME to `<tunnel-id>.cfargotunnel.com`;
- require a final 404 catch-all ingress;
- do not put tunnel/API tokens in logs;
- do not configure Cloudflare unless explicitly requested.

Maintain `installer/uninstall.sh`:
- remove systemd service/binaries;
- optionally remove Blueprint extension;
- default to PRESERVING `/srv/packwiz-manager`;
- require `--purge-data` to delete project data;
- never delete Pterodactyl data.

## 13. GitHub Actions and release engineering

### CI (`.github/workflows/ci.yml`)
On PRs and pushes:
- Go fmt check;
- `go vet`;
- Go tests with race detector where supported;
- frontend lint/typecheck/test;
- PHP syntax/test;
- shellcheck installer/scripts;
- `bash -n`;
- build Go service;
- build pinned Packwiz;
- build Blueprint extension in a controlled Blueprint Docker environment;
- smoke-test release bundle;
- upload CI artifacts.

### Release (`.github/workflows/release.yml`)
On tags `v*`:
- verify `VERSION` matches tag;
- build service for linux/amd64 and linux/arm64;
- build Packwiz from pinned `PACKWIZ_REF`;
- package each architecture with service + Packwiz;
- build/export `packwizmanager.blueprint`;
- include installer scripts;
- generate `SHA256SUMS`;
- generate SBOMs (SPDX or CycloneDX);
- create GitHub Release;
- attach:
  - `packwiz-manager-linux-amd64.tar.gz`
  - `packwiz-manager-linux-arm64.tar.gz`
  - `packwizmanager.blueprint`
  - `install.sh`
  - `uninstall.sh`
  - SBOMs
  - `SHA256SUMS`

Use GitHub OIDC/attestations if practical.

Do not use long-lived secrets for normal release creation; use the repository `GITHUB_TOKEN`.

### CodeQL
Run CodeQL for Go, JavaScript/TypeScript and PHP where supported by GitHub.

## 14. Tests

At minimum implement:

### Go unit tests
- path normalization;
- SSRF IP/range rejection;
- redirect handling;
- SHA-256 upload;
- content-addressed blob storage;
- transaction rollback;
- revision atomic promotion;
- authorization middleware;
- Modrinth response parsing;
- CurseForge response parsing;
- Packwiz subprocess argument escaping;
- concurrent project locking.

### Integration tests
- initialize project → add fake custom JAR → publish → fetch pack.toml;
- revision rollback;
- failed publish leaves old revision current;
- project import;
- Modrinth API mocked;
- CurseForge API mocked;
- large upload rejected;
- traversal and symlink archive rejected;
- management API inaccessible without token.

### Installer tests
Use a container/VM fixture where possible:
- fresh install;
- upgrade preserves token/data;
- service-only;
- unsupported stock Pterodactyl Docker does not get modified;
- checksum mismatch aborts;
- systemd unit hardening syntax;
- amd64/arm64 asset resolution logic.

## 15. Security baseline

Treat this as hosting software.

Mandatory:
- no shell interpolation of user-supplied values;
- use `exec.CommandContext` with argument arrays;
- no `sh -c` for Packwiz commands;
- upload limits;
- rate limits for mutating APIs;
- CSRF/session protections inherited from authenticated Pterodactyl routes;
- SQL parameterization;
- constant-time service-token compare;
- restrictive file permissions;
- service token mode 0600;
- config mode 0640 or stricter;
- project data not writable by Pterodactyl game containers;
- never execute uploaded custom JARs in the manager;
- SSRF controls;
- ZIP bomb controls;
- no directory traversal;
- no public directory listing;
- audit destructive actions;
- redact provider/API secrets;
- structured logs with request IDs;
- graceful shutdown;
- DB backup/restore documentation.

Run a final security review of all upload, URL import, subprocess and path-handling code before considering v0.2 complete.

## 16. Documentation

README must contain:
- architecture diagram;
- prerequisites;
- Blueprint native install;
- Blueprint Docker install;
- service-only mode;
- Cloudflare Tunnel example;
- first Packwiz project;
- Modrinth;
- CurseForge API key;
- custom JAR upload;
- server integration;
- update workflow;
- release/rollback;
- backup;
- upgrade;
- uninstall;
- troubleshooting.

Document that custom JARs are executable Minecraft code and should only be uploaded from trusted sources.

Document how to use:
`packwiz-installer-bootstrap.jar -g -s server`.

## 17. Version target and compatibility

Initial target:
- Packwiz Manager `v0.2.0`;
- current stable Pterodactyl 1.x;
- current Blueprint beta/stable Docker environment;
- Java 21 Minecraft servers as a primary test target;
- NeoForge 1.21.1 as one integration fixture, without hardcoding the product to NeoForge.

Do not hardcode version claims without checking current upstream primary sources during implementation.

## 18. Definition of Done

v0.2 is complete only when all of the following work end-to-end:

1. Install Packwiz Manager service from GitHub Release using `install.sh`.
2. Install `packwizmanager.blueprint` on Blueprint Docker.
3. Open a Minecraft server and see a Packwiz route.
4. Create a Packwiz project.
5. Add a compatible Modrinth mod.
6. Upload an unpublished custom JAR, choose `both`, and see it represented in the pack.
7. Add/edit a managed config file.
8. Validate and publish revision 1.
9. Fetch public `pack.toml` and `index.toml`.
10. Change a mod and publish revision 2.
11. View revision diff.
12. Roll back to revision 1.
13. Link a Pterodactyl server.
14. Install the bootstrap integration.
15. Start the server and observe the server-side Packwiz update complete before Minecraft starts.
16. Client-only entries are excluded when bootstrap is run with `-s server`.
17. CI passes.
18. Tagged release produces all expected assets with valid SHA256SUMS.
19. Upgrade installer preserves projects and secrets.
20. Uninstall without `--purge-data` preserves project data.

## 19. Agent operating rules

- Work in small, reviewable commits.
- Never weaken security checks merely to make a test pass.
- Before using an upstream API/format, inspect current official documentation/source.
- Prefer boring, explicit code over clever abstractions.
- Do not leave TODO stubs for core v0.2 behavior.
- Do not fabricate successful integration tests; if a dependency cannot be exercised locally, create a deterministic mock test and document the remaining live validation.
- Keep a `docs/decisions/` ADR for important architecture choices.
- Update `README.md`, `CHANGELOG.md` and `VERSION`.
- At the end, provide:
  - a concise architecture summary;
  - exact installation command;
  - exact test commands;
  - known limitations;
  - release asset list;
  - a security review summary.

Start by inspecting current official Pterodactyl, Blueprint, Packwiz, Modrinth and CurseForge documentation, then create the repository skeleton and implement the service core before the UI.
