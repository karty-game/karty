package toolchain

import "testing"

func TestKartyHomeUsesSharedSDKEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KARTY_HOME", root)

	resolved, err := kartyHome("")
	if err != nil || resolved != root {
		t.Fatalf("cache root=%q err=%v", resolved, err)
	}

	explicit := t.TempDir()

	resolved, err = kartyHome(explicit)
	if err != nil || resolved != explicit {
		t.Fatalf("explicit cache root=%q err=%v", resolved, err)
	}
}
