package files

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/packwiz-manager/packwiz-manager/service/internal/security"
)

func FetchRemote(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if err = security.ValidateRemoteURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "packwiz-manager/0.2")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("remote server returned non-200 status")
	}
	if res.ContentLength > maxBytes {
		return nil, errors.New("remote file exceeds configured limit")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, errors.New("remote file exceeds configured limit")
	}
	return b, nil
}
