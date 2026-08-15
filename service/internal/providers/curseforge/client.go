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
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Logo struct {
		ThumbnailURL string `json:"thumbnailUrl"`
	} `json:"logo"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
}
type response struct {
	Data []Mod `json:"data"`
}
type Client struct {
	HTTP            *http.Client
	BaseURL, APIKey string
}

func New(key string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}, BaseURL: "https://api.curseforge.com/v1", APIKey: key}
}
func (c *Client) Search(ctx context.Context, query, mc, loader string, classID, pageSize int) ([]Mod, error) {
	if c.APIKey == "" {
		return nil, errors.New("CurseForge is not configured")
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	u, _ := url.Parse(c.BaseURL + "/mods/search")
	q := u.Query()
	q.Set("gameId", "432")
	q.Set("searchFilter", query)
	q.Set("gameVersion", mc)
	q.Set("classId", strconv.Itoa(classID))
	if kind, ok := map[string]string{"forge": "1", "fabric": "4", "quilt": "5", "neoforge": "6"}[loader]; ok {
		q.Set("modLoaderType", kind)
	}
	q.Set("pageSize", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("CurseForge status %d", res.StatusCode)
	}
	var out response
	err = json.NewDecoder(res.Body).Decode(&out)
	return out.Data, err
}
