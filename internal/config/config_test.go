package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicSavePreservesSecretsAndPermissions(t *testing.T) {
	fixture, err := os.ReadFile("../auth/testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Config Config }
	if err = json.Unmarshal(fixture, &data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	encoded, _ := json.Marshal(data.Config)
	if err = os.WriteFile(path, encoded, 0640); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.PasswordSalt = "replacement-salt"
	if err = Save(path, c); err != nil {
		t.Fatal(err)
	}
	actual, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual != c {
		t.Fatal("atomic save changed other secrets")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Fatal("config permissions changed")
	}
	if err = os.WriteFile(path, []byte(`{"passwordHash":"oops"}`), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err = Load(path); err == nil {
		t.Fatal("invalid startup config accepted")
	}
}
