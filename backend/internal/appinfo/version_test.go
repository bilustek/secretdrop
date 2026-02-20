package appinfo_test

import (
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
)

func TestVersionIsSet(t *testing.T) {
	t.Parallel()

	if appinfo.Version == "" {
		t.Error("Version should not be empty")
	}

	if appinfo.Version != "0.0.0" {
		t.Errorf("Version = %q; want %q", appinfo.Version, "0.0.0")
	}
}
