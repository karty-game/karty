// Hand-authored routes and landmark placement, in world metres.
// The beach trail, timber approach and terrain bearing use the same endpoints.
export const PIER_APPROACH = Object.freeze({
  x0: 9.6,
  x1: 13.2,
  centerX: 11.4,
  deckEndY: 15.8,
  rampStartY: 18.2,
  rampEndY: 21.2,
  deckHeight: 1.08,
  toeHeight: 0.86,
  pathToeHeight: 0.98,
});
export const BEACH_PATH = [
  [PIER_APPROACH.centerX, PIER_APPROACH.rampEndY, PIER_APPROACH.pathToeHeight],
  [PIER_APPROACH.centerX, 22.3, 1],
  [10.9, 23.1, 1],
  [9.8, 23.4, 1],
  [8.6, 23.2, 1],
  [7, 22.6, 1],
  [0, 18, 1],
  [-8, 17, 1.8],
  [-14, 18, 4.2],
  [-20, 17, 6],
  [-25, 15, 6.65],
  [-28, 13, 6.192],
];
export const LIGHTHOUSE_PATH = [
  [-20, 17, 6],
  [-22, 23, 7.8],
  [-33, 24, 9],
  [-39, 20, 9.8],
  [-39, 15, 12],
  [-38, 10, 15],
  [-36, 5, 18],
  [-36, -3, 19.8],
  [-36, -5, 20.1],
];
export const LIGHTHOUSE = { x: -36, y: -9, z: 20.1 };
export function nearestRoute(x, y, points) {
  let best = { distance: Infinity, height: 0 };
  for (let i = 1; i < points.length; i++) {
    const a = points[i - 1],
      b = points[i],
      dx = b[0] - a[0],
      dy = b[1] - a[1];
    const t = Math.max(0, Math.min(1, ((x - a[0]) * dx + (y - a[1]) * dy) / (dx * dx + dy * dy)));
    const distance = Math.hypot(x - a[0] - t * dx, y - a[1] - t * dy);
    if (distance < best.distance) best = { distance, height: a[2] + t * (b[2] - a[2]) };
  }
  return best;
}

export const ROCK_PLACEMENTS = [
  ["coastal-slab", 25, 23, 18, 0.8],
  ["coastal-boulder", 6, 25.5, 41, 0.6],
  ["coastal-split-stone", 3, 23.5, 135, 0.75],
  ["coastal-slab", -1, 20, 64, 0.9],
  ["coastal-boulder", -11, 12, 23, 0.7],
  ["coastal-split-stone", -18, 20, 92, 0.65],
  ["coastal-slab", -24, 18, 12, 0.7],
  ["coastal-boulder", -31, 15, 73, 0.8],
  ["coastal-split-stone", -26, 29, 38, 0.8],
  ["coastal-slab", -34, 30, 110, 1.1],
  ["coastal-boulder", -42, 22, 85, 0.7],
  ["coastal-split-stone", -42, 15, 18, 0.75],
  ["coastal-slab", -42, 8, 52, 1],
  ["coastal-boulder", -40, 6, 155, 0.8],
  ["coastal-split-stone", -32, 3, 12, 0.7],
  ["coastal-slab", -40, -16, 75, 0.8],
  ["coastal-boulder", 29, 15, 115, 0.6],
  ["coastal-split-stone", 33, 18, 35, 0.75],
];
