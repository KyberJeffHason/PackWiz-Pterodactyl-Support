package files

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchRemoteCapsSize(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return response("12345"), nil })}
	if _, err := FetchRemote(context.Background(), client, "https://example.com/file", 4); err == nil {
		t.Fatal("large response accepted")
	}
}
func TestFetchRemoteRejectsPrivateURL(t *testing.T) {
	if _, err := FetchRemote(context.Background(), http.DefaultClient, "http://127.0.0.1/file", 10); err == nil {
		t.Fatal("private URL accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header)}
}
