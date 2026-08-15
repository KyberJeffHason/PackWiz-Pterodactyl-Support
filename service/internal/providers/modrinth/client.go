package modrinth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Hit struct {
	ProjectID   string   `json:"project_id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	IconURL     string   `json:"icon_url"`
	Versions    []string `json:"versions"`
	Categories  []string `json:"categories"`
}
type searchResponse struct {
	Hits  []Hit `json:"hits"`
	Total int   `json:"total_hits"`
}
type Client struct {
	HTTP               *http.Client
	BaseURL, UserAgent string
}

func New(version string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, BaseURL: "https://api.modrinth.com/v2", UserAgent: "packwiz-manager/" + version + " (https://github.com/packwiz-manager/packwiz-manager)"}
}
func (c *Client) Search(ctx context.Context, query, mc, loader string, limit int) ([]Hit, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	facets, _ := json.Marshal([][]string{{"project_type:mod"}, {"versions:" + mc}, {"categories:" + loader}})
	u, _ := url.Parse(c.BaseURL + "/search")
	q := u.Query()
	q.Set("query", query)
	q.Set("facets", string(facets))
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()
	var out searchResponse
	if err := c.get(ctx, u.String(), &out); err != nil {
		return nil, err
	}
	return out.Hits, nil
}
func (c *Client) get(ctx context.Context, endpoint string, out any) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			last = err
			continue
		}
		defer res.Body.Close()
		if res.StatusCode == 429 || res.StatusCode >= 500 {
			last = fmt.Errorf("provider status %d", res.StatusCode)
			time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
			continue
		}
		if res.StatusCode != 200 {
			return fmt.Errorf("provider status %d", res.StatusCode)
		}
		return json.NewDecoder(res.Body).Decode(out)
	}
	return last
}
