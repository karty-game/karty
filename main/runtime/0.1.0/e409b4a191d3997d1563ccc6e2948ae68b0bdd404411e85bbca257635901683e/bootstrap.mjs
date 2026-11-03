// Production game shell. The CLI supplies #world, #status and the pinned hash.
import { startHost, play } from "./runtime.mjs";
const status = document.querySelector("#status");
let failed = false;
let presented = false;
function showReady() {
  if (failed || presented) return;
  presented = true;
  document.documentElement.dataset.kartyState = "running";
  if (status) status.textContent = "";
  if (status) status.hidden = true;
  window.dispatchEvent(new CustomEvent("karty-host-ready"));
}
window.addEventListener("karty-host-presented", showReady, { once: true });
function fail(error) {
  failed = true;
  window.removeEventListener("karty-host-presented", showReady);
  globalThis.kartyWebGPUStartupError ??= String(error?.message ?? error);
  if (status) {
    status.hidden = false;
    status.textContent = `Game loading failed: ${error?.message ?? error}`;
  }
  document.documentElement.dataset.kartyState = "error";
  window.dispatchEvent(new CustomEvent("karty-host-error", { detail: { message: String(error?.message ?? error) } }));
  console.error(error);
}
try {
  await startHost({
    sha256: document.documentElement.dataset.wasmSha256,
    onStage: (name) => {
      if (status) status.textContent = `Loading: ${name}…`;
      window.dispatchEvent(new CustomEvent("karty-host-stage", { detail: { stage: name } }));
    },
    onError: fail,
  });
  await play(document.documentElement.dataset.cartridge || "./game.kart");
  if (!globalThis.kartyWebGPUHasPresentationEvents || globalThis.kartyWebGPUPresentationReady) showReady();
  else window.dispatchEvent(new CustomEvent("karty-host-stage", { detail: { stage: "world" } }));
} catch (error) {
  fail(error);
}
