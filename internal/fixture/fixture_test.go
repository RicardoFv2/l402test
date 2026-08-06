package fixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWrite(t *testing.T) {
	t.Parallel()
	filename := filepath.Join(t.TempDir(), "fixture.json")
	want := File{Version: 1, Scenario: "happy", BaseURL: "http://127.0.0.1:1234"}
	if err := Write(filename, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got File
	if err := json.Unmarshal(contents, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixture = %#v, want %#v", got, want)
	}
	want.Scenario = "credential-refresh"
	if err := Write(filename, want); err != nil {
		t.Fatalf("replace fixture: %v", err)
	}
}
