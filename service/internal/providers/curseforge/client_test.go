package curseforge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchParsing(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "secret" {
			t.Error("missing key")
		}
		if r.URL.Query().Get("index") != "20" {
			t.Errorf("unexpected index: %s", r.URL.Query().Get("index"))
		}
		w.Write([]byte(`{"data":[{"id":42,"name":"Example","slug":"example","summary":"Example summary","logo":{"thumbnailUrl":"https://cdn.example/icon.png"}}],"pagination":{"index":20,"pageSize":20,"resultCount":1,"totalCount":55}}`))
	}))
	defer s.Close()
	c := New("secret")
	c.BaseURL = s.URL
	result, err := c.Search(context.Background(), "ex", "1.21.1", "neoforge", 6, 20, 20)
	if err != nil || len(result.Mods) != 1 || result.Mods[0].ID != 42 || result.Total != 55 {
		t.Fatalf("%+v %v", result, err)
	}
}
