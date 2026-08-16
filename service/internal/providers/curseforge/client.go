package curseforge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Mod struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Summary string `json:"summary"`
	Logo    struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"logo"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
}
type File struct {
	ID           int      `json:"id"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	ReleaseType  int      `json:"releaseType"`
	FileDate     string   `json:"fileDate"`
	GameVersions []string `json:"gameVersions"`
}
type pagination struct {
	Index       int `json:"index"`
	PageSize    int `json:"pageSize"`
	ResultCount int `json:"resultCount"`
	TotalCount  int `json:"totalCount"`
}
type response struct {
	Data       []Mod      `json:"data"`
	Pagination pagination `json:"pagination"`
}
type fileResponse struct {
	Data       []File     `json:"data"`
	Pagination pagination `json:"pagination"`
}
type SearchResult struct {
	Mods  []Mod
	Total int
}
type FileResult struct {
	Files []File
	Total int
}
type Client struct {
	HTTP            *http.Client
	BaseURL, APIKey string
}

func New(key string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, BaseURL: "https://api.curseforge.com/v1", APIKey: key}
}
func (c *Client) Search(ctx context.Context, query, mc, loader string, classID, pageSize, index int) (SearchResult, error) {
	if c.APIKey == "" {
		return SearchResult{}, errors.New("CurseForge is not configured")
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	if index < 0 {
		index = 0
	}
	u, _ := url.Parse(c.BaseURL + "/mods/search")
	q := u.Query()
	q.Set("gameId", "432")
	q.Set("searchFilter", query)
	q.Set("gameVersion", mc)
	q.Set("classId", strconv.Itoa(classID))
	if kind, ok := loaderType(loader); ok {
		q.Set("modLoaderType", kind)
	}
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("index", strconv.Itoa(index))
	u.RawQuery = q.Encode()
	var out response
	if err := c.get(ctx, u.String(), &out); err != nil {
		return SearchResult{}, err
	}
	total := out.Pagination.TotalCount
	if total == 0 && len(out.Data) > 0 {
		total = len(out.Data)
	}
	return SearchResult{Mods: out.Data, Total: total}, nil
}
func (c *Client) Files(ctx context.Context, modID, mc, loader string, pageSize, index int) (FileResult, error) {
	if c.APIKey == "" {
		return FileResult{}, errors.New("CurseForge is not configured")
	}
	id, err := strconv.Atoi(modID)
	if err != nil || id <= 0 {
		return FileResult{}, errors.New("invalid CurseForge mod id")
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 50
	}
	if index < 0 {
		index = 0
	}
	u, _ := url.Parse(fmt.Sprintf("%s/mods/%d/files", c.BaseURL, id))
	q := u.Query()
	if mc != "" {
		q.Set("gameVersion", mc)
	}
	if kind, ok := loaderType(loader); ok {
		q.Set("modLoaderType", kind)
	}
	q.Set("pageSize", strconv.Itoa(pageSize))
	q.Set("index", strconv.Itoa(index))
	u.RawQuery = q.Encode()
	var out fileResponse
	if err = c.get(ctx, u.String(), &out); err != nil {
		return FileResult{}, err
	}
	total := out.Pagination.TotalCount
	if total == 0 && len(out.Data) > 0 {
		total = len(out.Data)
	}
	return FileResult{Files: out.Data, Total: total}, nil
}
func loaderType(loader string) (string, bool) {
	kind, ok := map[string]string{"forge": "1", "fabric": "4", "quilt": "5", "neoforge": "6"}[loader]
	return kind, ok
}
func (c *Client) get(ctx context.Context, endpoint string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("CurseForge status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}
