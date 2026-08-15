package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"
)

var serverIDPattern = regexp.MustCompile(`^[A-Fa-f0-9-]{8,64}$`)

type serverLink struct {
	ProjectID        string `json:"project_id"`
	ServerUUID       string `json:"server_uuid"`
	ServerIdentifier string `json:"server_identifier"`
	UpdateOnStart    bool   `json:"update_on_start"`
	BootstrapState   string `json:"bootstrap_state"`
	BootstrapVersion string `json:"bootstrap_version"`
	StartupState     string `json:"startup_integration_state"`
	LastStatus       string `json:"last_sync_status"`
}

func (a *API) getServerLink(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.read") {
		return
	}
	var v serverLink
	var update int
	var version, status sql.NullString
	err := a.DB.QueryRowContext(r.Context(), `SELECT project_id,server_uuid,server_identifier,update_on_start,bootstrap_state,bootstrap_version,startup_integration_state,last_sync_status FROM server_links WHERE project_id=? AND server_uuid=?`, r.PathValue("id"), r.PathValue("server")).Scan(&v.ProjectID, &v.ServerUUID, &v.ServerIdentifier, &update, &v.BootstrapState, &version, &v.StartupState, &status)
	v.UpdateOnStart = update != 0
	v.BootstrapVersion = version.String
	v.LastStatus = status.String
	respond(w, v, err)
}
func (a *API) putServerLink(w http.ResponseWriter, r *http.Request) {
	if !permission(w, r, "packwiz.integration") {
		return
	}
	var in serverLink
	if err := decode(r, &in); err != nil {
		bad(w, err)
		return
	}
	in.ProjectID = r.PathValue("id")
	in.ServerUUID = r.PathValue("server")
	if !serverIDPattern.MatchString(in.ProjectID) || !serverIDPattern.MatchString(in.ServerUUID) || !publicPart.MatchString(in.ServerIdentifier) || in.BootstrapState == "" || in.StartupState == "" {
		bad(w, errors.New("incomplete integration state"))
		return
	}
	tx, err := a.DB.BeginTx(r.Context(), nil)
	if err != nil {
		respond(w, nil, err)
		return
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(r.Context(), `INSERT INTO server_links(project_id,server_uuid,server_identifier,update_on_start,side,bootstrap_state,bootstrap_version,startup_integration_state,last_sync_status) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(project_id,server_uuid) DO UPDATE SET server_identifier=excluded.server_identifier,update_on_start=excluded.update_on_start,bootstrap_state=excluded.bootstrap_state,bootstrap_version=excluded.bootstrap_version,startup_integration_state=excluded.startup_integration_state,last_sync_status=excluded.last_sync_status`, in.ProjectID, in.ServerUUID, in.ServerIdentifier, in.UpdateOnStart, "server", in.BootstrapState, in.BootstrapVersion, in.StartupState, in.LastStatus)
	if err == nil {
		metadata, _ := json.Marshal(map[string]string{"server": in.ServerUUID})
		_, err = tx.ExecContext(r.Context(), `INSERT INTO audit_events(actor,project_id,operation,request_id,metadata_json,created_at) VALUES(?,?,?,?,?,?)`, r.Header.Get("X-Pterodactyl-Actor"), in.ProjectID, "integration.update", r.Header.Get("X-Request-ID"), metadata, time.Now().UTC().Format(time.RFC3339Nano))
	}
	if err == nil {
		err = tx.Commit()
	}
	respond(w, in, err)
}
