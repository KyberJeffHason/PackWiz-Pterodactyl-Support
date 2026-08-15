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
		w.Write([]byte(`{"hits":[{"project_id":"abc","title":"Test","author":"A"}],"total_hits":1}`))
	}))
	defer s.Close()
	c := New("test")
	c.BaseURL = s.URL
	hits, err := c.Search(context.Background(), "test", "1.21.1", "neoforge", 20)
	if err != nil || len(hits) != 1 || hits[0].ProjectID != "abc" {
		t.Fatalf("%+v %v", hits, err)
	}
}
