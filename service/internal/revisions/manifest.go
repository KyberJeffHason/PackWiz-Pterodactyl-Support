package revisions

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}
type Manifest struct {
	RevisionID     string `json:"revision_id"`
	Revision       int64  `json:"revision"`
	ProjectID      string `json:"project_id"`
	ContentDigest  string `json:"content_digest"`
	ManagerVersion string `json:"manager_version"`
	PackwizCommit  string `json:"packwiz_commit"`
	PackwizSHA256  string `json:"packwiz_sha256"`
	Files          []File `json:"files"`
}
