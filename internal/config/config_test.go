package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRealModeRequiresLoopback(t *testing.T) {
	assets, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"0.0.0.0:8080", ":8080", "example.com:8080", "localhost:8080", "[::]:8080", "192.168.1.2:8080"} {
		if _, err := Parse([]string{"-assets", assets, "-listen", address}, io.Discard); err == nil {
			t.Fatalf("public address accepted: %s", address)
		}
	}
	for _, address := range []string{"127.0.0.1:0", "[::1]:0"} {
		if _, err := Parse([]string{"-assets", assets, "-listen", address}, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Parse([]string{"-assets", assets, "-listen", "0.0.0.0:8080", "-demo"}, io.Discard); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultsAndMissingAssets(t *testing.T) {
	assets, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := Parse([]string{"-assets", assets}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Demo || !settings.OpenBrowser || settings.Listen != "127.0.0.1:0" {
		t.Fatalf("unexpected defaults: %+v", settings)
	}
	if _, err := Parse([]string{"-assets", t.TempDir()}, io.Discard); err == nil {
		t.Fatal("missing assets accepted")
	}
	if _, err := os.Stat(settings.Assets); err != nil {
		t.Fatal(err)
	}
}
