package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesDefaultWithoutConfigOrEnv(t *testing.T) {
	result, err := Load(t.TempDir(), map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.APIURL != DefaultAPIURL {
		t.Fatalf("api url = %q", result.Config.APIURL)
	}
	if result.Source.APIURL != "default" {
		t.Fatalf("api url source = %q", result.Source.APIURL)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Config{APIURL: "http://localhost:8080/", Token: "psg_test"}); err != nil {
		t.Fatal(err)
	}
	result, err := Load(dir, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.APIURL != "http://localhost:8080" {
		t.Fatalf("api url = %q", result.Config.APIURL)
	}
	if result.Config.Token != "psg_test" {
		t.Fatalf("token = %q", result.Config.Token)
	}
	info, err := os.Stat(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %v, want 0600", got)
	}
}

func TestSaveTightensExistingConfigPermissions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := Path(dir)
	if err := os.WriteFile(path, []byte(`{"api_url":"http://localhost:8080","token":"old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, Config{APIURL: "http://localhost:8080", Token: "psg_new"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %v, want 0600", got)
	}
}

func TestEnvOverridesSavedConfig(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Config{APIURL: "http://localhost:8080", Token: "psg_saved"}); err != nil {
		t.Fatal(err)
	}
	result, err := Load(dir, map[string]string{
		EnvAPIURL: "https://example.test/",
		EnvToken:  "psg_env",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Config.APIURL != "https://example.test" {
		t.Fatalf("api url = %q", result.Config.APIURL)
	}
	if result.Config.Token != "psg_env" {
		t.Fatalf("token = %q", result.Config.Token)
	}
	if result.Source.APIURL != "env" || result.Source.Token != "env" {
		t.Fatalf("source = %#v", result.Source)
	}
}

func TestSaveRejectsInvalidConfig(t *testing.T) {
	if err := Save(t.TempDir(), Config{APIURL: "localhost:8080", Token: "psg_test"}); err == nil {
		t.Fatal("Save accepted URL without scheme")
	}
	if err := Save(t.TempDir(), Config{APIURL: "http://localhost:8080", Token: ""}); err == nil {
		t.Fatal("Save accepted empty token")
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir, map[string]string{})
	if err == nil {
		t.Fatal("Load accepted invalid JSON")
	}
}

func TestDefaultDirUsesPassageHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".passage")
	if dir != want {
		t.Fatalf("dir = %q, want %q", dir, want)
	}
	if Path(dir) != filepath.Join(home, ".passage", "auth.json") {
		t.Fatalf("path = %q", Path(dir))
	}
}

func TestRedactToken(t *testing.T) {
	got := RedactToken("psg_abcdefghijklmnopqrstuvwxyz")
	if !strings.HasPrefix(got, "psg_...") {
		t.Fatalf("redacted = %q", got)
	}
	if strings.Contains(got, "abcdefghijklmnopqrstuv") {
		t.Fatalf("redacted leaked token = %q", got)
	}
	if RedactToken("short") != "****" {
		t.Fatal("short token was not fully redacted")
	}
}
