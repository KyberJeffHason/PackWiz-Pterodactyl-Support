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
		w.Write([]byte(`{"data":[{"id":42,"name":"Example","slug":"example"}]}`))
	}))
	defer s.Close()
	c := New("secret")
	c.BaseURL = s.URL
	mods, err := c.Search(context.Background(), "ex", "1.21.1", "neoforge", 6, 20)
	if err != nil || len(mods) != 1 || mods[0].ID != 42 {
		t.Fatalf("%+v %v", mods, err)
	}
}
