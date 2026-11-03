package build

import (
	"context"
	"os"

	"github.com/karty-game/karty/internal/toolchain"
)

const hostBundleName = toolchain.HostBundleName
const hostBundleFileLimit = toolchain.HostBundleFileLimit

type hostBundle = toolchain.HostBundle
type hostBundleFile = toolchain.HostBundleFile

func readHostBundle(ctx context.Context, selected, target, platform string) (*hostBundle, error) {
	return toolchain.ReadHostBundle(ctx, selected, target, platform)
}
func ValidateLocalHost(ctx context.Context, selected, target, platform string) (bool, error) {
	return toolchain.ValidateLocalHost(ctx, selected, target, platform)
}
func ResolveLocalHostBundle(ctx context.Context, selected, target, platform string) (string, bool, error) {
	return toolchain.ResolveLocalHostBundle(ctx, selected, target, platform)
}
func webGPUHostFiles() []string { return toolchain.WebGPUHostFiles() }
func verifyBundleFile(ctx context.Context, root *os.Root, name string, entry hostBundleFile) error {
	return toolchain.VerifyHostBundleFile(ctx, root, name, entry)
}
