// karty-sdk-verify authenticates local SDK metadata with the CLI's release key.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/karty-game/karty/internal/sdk"
)

const maxManifestBytes = 1 << 20
const signatureBytes = 64

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("karty-sdk-verify", flag.ContinueOnError)
	version := flags.String("version", "", "exact SDK version")
	manifest := flags.String("manifest", "", "local signed release manifest")
	signature := flags.String("signature", "", "local detached Ed25519 signature")

	if err := flags.Parse(arguments); err != nil {
		return err
	}

	if flags.NArg() != 0 || *version == "" || *manifest == "" || *signature == "" {
		return fmt.Errorf("usage: karty-sdk-verify --version VERSION --manifest FILE --signature FILE: %w", os.ErrInvalid)
	}

	contents, err := readBounded(*manifest, maxManifestBytes)
	if err != nil {
		return err
	}

	proof, err := readBounded(*signature, signatureBytes)
	if err != nil {
		return err
	}

	_, err = sdk.VerifyPublishedManifest(*version, contents, proof)

	return err
}

func readBounded(path string, maximum int64) ([]byte, error) {
	// #nosec G304 -- This maintenance command intentionally reads caller-selected local metadata.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	contents, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}

	if int64(len(contents)) > maximum {
		return nil, fmt.Errorf("release metadata %q exceeds its byte bound: %w", path, os.ErrInvalid)
	}

	return contents, nil
}
