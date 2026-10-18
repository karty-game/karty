package toolchain

import "testing"

func TestGoVersionRequiresExactCompilerToken(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, output string
		valid        bool
	}{
		{"linux", "go version go1.27.1 linux/amd64\n", true},
		{"windows", "go version go1.27.1 windows/arm64\r\n", true},
		{"darwin whitespace", "  go version go1.27.1 darwin/arm64  \n", true},
		{"different patch", "go version go1.27.2 linux/amd64", false},
		{"patch prefix", "go version go1.27.10 linux/amd64", false},
		{"release candidate", "go version go1.27.1rc1 linux/amd64", false},
		{"custom suffix", "go version go1.27.1-custom linux/amd64", false},
		{"development compiler", "go version devel go1.27.1-hash linux/amd64", false},
		{"different tool", "tinygo version go1.27.1 linux/amd64", false},
		{"embedded expected token", "go version go1.27.2 linux/amd64 go1.27.1", false},
		{"missing compiler prefix", "go version 1.27.1 linux/amd64", false},
		{"incomplete output", "go version go1.27.1", false},
		{"empty output", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := matchesGoVersion(test.output, "go1.27.1"); got != test.valid {
				t.Fatalf("matchesGoVersion(%q) = %t, want %t", test.output, got, test.valid)
			}
		})
	}
}
