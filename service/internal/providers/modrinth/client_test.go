package modrinth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchParsing(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing UA")
		}
		if r.URL.Query().Get("offset") != "20" {
			t.Errorf("unexpected offset: %s", r.URL.Query().Get("offset"))
		}
		w.Write([]byte(`{"hits":[{"project_id":"abc","title":"Test","author":"A","icon_url":"https://cdn.example/icon.png"}],"total_hits":37}`))
	}))
	defer s.Close()
	c := New("test")
	c.BaseURL = s.URL
	result, err := c.Search(context.Background(), "test", "1.21.1", "neoforge", 20, 20)
	if err != nil || len(result.Hits) != 1 || result.Hits[0].ProjectID != "abc" || result.Total != 37 {
		t.Fatalf("%+v %v", result, err)
	}
}
