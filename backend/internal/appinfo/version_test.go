package appinfo_test

import (
	"regexp"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/appinfo"
)

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestVersionIsSet(t *testing.T) {
	t.Parallel()

	if appinfo.Version == "" {
		t.Error("Version should not be empty")
	}

	if !semverRe.MatchString(appinfo.Version) {
		t.Errorf("Version = %q; want semver format (x.y.z)", appinfo.Version)
	}
}
