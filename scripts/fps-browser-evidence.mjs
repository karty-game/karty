// These accessors only wrap calls to record the stable wire-15/16 command stream.
// Every original function receives the original arguments exactly once.
export function observeController() {
  const state = (globalThis.fpsEvidence = {
    frame: 0,
    camera: 0,
    yaw: 0,
    moving: 0,
    velocity: null,
    body: null,
    commandVersion: 0,
    pose: null,
    poseUpdates: 0,
    vectors: [],
  });
  let update, submit;
  Object.defineProperty(globalThis, "kartyClientUpdate", {
    configurable: true,
    get: () => update,
    set(original) {
      update = (frame, batch) => {
        state.frame = Number(frame);
        const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
        let offset = 24;
        for (let index = 0; index < view.getUint32(16, true); index++) {
          const tag = view.getUint8(offset),
            payload = offset + 3,
            length = view.getUint16(offset + 1, true);
          // Wire15/16: authoritative entity pose (11) and physical body pose (19).
          const entityOffset = tag === 11 ? 0 : tag === 19 ? 4 : -1;
          if (entityOffset >= 0 && view.getUint32(payload + entityOffset, true) === state.camera) {
            const position = payload + (tag === 11 ? 4 : 8);
            state.pose = {
              x: view.getFloat32(position, true),
              y: view.getFloat32(position + 4, true),
              z: view.getFloat32(position + 8, true),
              yaw: view.getFloat32(payload + (tag === 11 ? 16 : 32), true),
            };
            state.poseUpdates++;
          }
          offset += 3 + length;
        }
        return original(frame, batch);
      };
    },
  });
  Object.defineProperty(globalThis, "kartyHostSubmitCommands", {
    configurable: true,
    get: () => submit,
    set(original) {
      submit = (batch) => {
        const view = new DataView(batch.buffer, batch.byteOffset, batch.byteLength);
        state.commandVersion = view.getUint16(4, true);
        let offset = 24;
        for (let index = 0; index < view.getUint32(16, true); index++) {
          const tag = view.getUint8(offset),
            payload = offset + 3,
            length = view.getUint16(offset + 1, true);
          const id = length >= 4 ? view.getUint32(payload, true) : 0;
          if (tag === 10 && !state.vectors.includes(id)) state.vectors.push(id);
          if (tag === 27) {
            state.camera = id;
            state.pose = null;
            state.velocity = null;
            state.body = null;
            state.yaw = view.getFloat32(payload + 24, true);
          }
          if (tag === 28 && id === state.camera) state.yaw = view.getFloat32(payload + 20, true);
          if (tag === 42 && id === state.camera) {
            state.body = { flags: view.getUint32(payload + 40, true), mass: view.getFloat32(payload + 16, true) };
          }
          if (tag === 45 && id === state.camera) {
            const x = view.getFloat32(payload + 4, true),
              y = view.getFloat32(payload + 8, true);
            state.velocity = { x, y, flags: view.getUint32(payload + 16, true), frame: state.frame };
            if (Math.hypot(x, y) > 0.001) state.moving++;
          }
          offset += 3 + length;
        }
        return original(batch);
      };
    },
  });
}
