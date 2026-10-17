package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

func TestSettingsRoundTripAndReplacement(t *testing.T) {
	store := JSON{Path: filepath.Join(t.TempDir(), "nested", "settings.json")}
	document, err := store.Load()
	if err != nil || document.Preferences != domain.DefaultPreferences() {
		t.Fatalf("defaults: %+v %v", document, err)
	}
	for _, delay := range []int{1, 10} {
		document.Preferences.DelayMinutes = delay
		if err := store.Save(document); err != nil {
			t.Fatal(err)
		}
		actual, err := store.Load()
		if err != nil || actual.Preferences != document.Preferences {
			t.Fatalf("round trip: %+v %v", actual, err)
		}
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".settings-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files: %v %v", files, err)
	}
}

func TestCorruptSettingsReturnError(t *testing.T) {
	store := JSON{Path: filepath.Join(t.TempDir(), "settings.json")}
	if err := os.WriteFile(store.Path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("corrupt JSON accepted")
	}
}
