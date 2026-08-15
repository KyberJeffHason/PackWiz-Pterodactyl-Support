package packwiz

import (
	"reflect"
	"testing"
)

func TestArgumentsRemainSeparate(t *testing.T) {
	got := ModrinthAddArgs("id; touch /tmp/pwn", "v $(bad)")
	want := []string{"modrinth", "add", "--project-id", "id; touch /tmp/pwn", "--version-id", "v $(bad)", "-y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}
func TestArgumentsAllowPackwizCompatibleSelection(t *testing.T) {
	got := ModrinthAddArgs("safe-id", "")
	want := []string{"modrinth", "add", "--project-id", "safe-id", "-y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}
