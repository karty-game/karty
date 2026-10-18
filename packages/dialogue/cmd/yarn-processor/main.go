// yarn-processor keeps compiler build diagnostics out of the KPR1 data stream.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	build := exec.Command("dotnet", "build", "compiler/Compiler.csproj", "--nologo", "--verbosity", "quiet")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	compiler := exec.Command("dotnet", "dist/bin/Debug/net9.0/Compiler.dll")
	compiler.Stdin = os.Stdin
	compiler.Stdout = os.Stdout
	compiler.Stderr = os.Stderr
	if err := compiler.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
