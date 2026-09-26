package build

import (
	"slices"
	"testing"
)

func TestCompilerArguments(t *testing.T) {
	t.Parallel()

	for _, compiler := range []string{"tinygo", "go"} {
		arguments := compilerArguments(compiler, "game.kart")

		want := []string{"build", "-buildmode=c-shared", "-o", "game.kart", "./src"}
		if compiler == "tinygo" {
			want = []string{"build", "-target=wasi", "-buildmode=c-shared", "-scheduler=none", "-o", "game.kart", "./src"}
		}

		if !slices.Equal(arguments, want) {
			t.Fatalf("%s arguments: got %v, want %v", compiler, arguments, want)
		}
	}
}
