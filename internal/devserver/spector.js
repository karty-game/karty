// Loaded before the game only when the preview URL includes spector=1.
(() => {
  if (!globalThis.SPECTOR) {
    console.warn("Spector.js could not load from the CDN; continuing without WebGL capture.");
    return;
  }
  try {
    const spector = new globalThis.SPECTOR.Spector();
    globalThis.spector = spector;
    spector.spyCanvases();
    const mount = () => {
      try {
        // The WASM host creates its canvas after DOM readiness. Use only
        // contexts Spector has already observed; probing getContext here can
        // claim a canvas before the host selects its graphics backend.
        const canvases = spector
          .getAvailableContexts()
          .map(({ canvas }) => canvas)
          .filter((canvas) => canvas instanceof HTMLCanvasElement && canvas.isConnected);
        if (canvases.length === 0) {
          setTimeout(mount, 100);
          return;
        }
        spector.displayUI(true);
        spector.getCaptureUI().updateCanvasesListInformation(
          canvases.map((canvas) => ({
            id: canvas.id || "Karty game",
            width: canvas.width,
            height: canvas.height,
            ref: canvas,
          })),
        );
      } catch (error) {
        console.warn("Spector.js capture controls could not open:", error);
      }
    };
    if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount, { once: true });
    else mount();
  } catch (error) {
    console.warn("Spector.js could not initialize; continuing without WebGL capture:", error);
  }
})();
