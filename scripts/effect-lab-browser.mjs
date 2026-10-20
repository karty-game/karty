import { chromium } from "playwright";

// Linux development PCs can use the render node without an X server. Never
// silently substitute SwiftShader: these runs are used for GPU measurements.
export function launchEffectLabBrowser() {
  const env = { ...process.env };
  delete env.LIBGL_ALWAYS_SOFTWARE;
  const args = ["--no-sandbox", "--enable-gpu", "--disable-software-rasterizer"];
  if (process.platform === "linux") {
    env.EGL_PLATFORM = "surfaceless";
    args.push("--use-gl=angle", "--use-angle=gl-egl", "--ozone-platform=headless");
  }
  return chromium.launch({
    headless: true,
    timeout: 5000,
    env,
    ignoreDefaultArgs: ["--enable-unsafe-swiftshader"],
    args,
  });
}
