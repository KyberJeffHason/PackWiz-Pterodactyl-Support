PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS projects (
 id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, display_name TEXT NOT NULL,
 minecraft_version TEXT NOT NULL, loader TEXT NOT NULL, loader_version TEXT NOT NULL,
 pack_version TEXT NOT NULL, working_directory TEXT NOT NULL UNIQUE,
 current_revision_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS blobs (
 sha256 TEXT PRIMARY KEY, byte_size INTEGER NOT NULL, mime_type TEXT NOT NULL,
 storage_path TEXT NOT NULL UNIQUE, original_filename TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS items (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 kind TEXT NOT NULL, provider TEXT NOT NULL, provider_project_id TEXT, provider_version_id TEXT,
 display_name TEXT NOT NULL, target_path TEXT NOT NULL, filename TEXT NOT NULL,
 side TEXT NOT NULL CHECK(side IN ('client','server','both')), expected_sha256 TEXT,
 source_url TEXT, blob_id TEXT REFERENCES blobs(sha256), metadata_json TEXT NOT NULL DEFAULT '{}',
 enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS revisions (
 id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 revision_number INTEGER NOT NULL, pack_version TEXT NOT NULL, content_digest TEXT NOT NULL,
 release_directory TEXT NOT NULL, changelog TEXT NOT NULL DEFAULT '', created_by TEXT, created_at TEXT NOT NULL,
 UNIQUE(project_id, revision_number)
);
CREATE TABLE IF NOT EXISTS server_links (
 project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 server_uuid TEXT NOT NULL, server_identifier TEXT NOT NULL, update_on_start INTEGER NOT NULL DEFAULT 1,
 side TEXT NOT NULL DEFAULT 'server', bootstrap_state TEXT NOT NULL DEFAULT 'absent',
 bootstrap_version TEXT, startup_integration_state TEXT NOT NULL DEFAULT 'not-applied', last_sync_status TEXT,
 PRIMARY KEY(project_id, server_uuid)
);
CREATE TABLE IF NOT EXISTS audit_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, actor TEXT, project_id TEXT, operation TEXT NOT NULL,
 request_id TEXT NOT NULL, metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_items_project ON items(project_id);
CREATE INDEX IF NOT EXISTS idx_revisions_project ON revisions(project_id, revision_number);
