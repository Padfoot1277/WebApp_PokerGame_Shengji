type Rect = { left: number; top: number; right: number; bottom: number };
export function viewport(
  width: number,
  height: number,
  safe?: Rect,
  capsule?: Rect,
) {
  const left = Math.max(0, safe?.left ?? 0),
    right = Math.min(width, safe?.right ?? width);
  const top = Math.max(0, safe?.top ?? 0),
    bottom = Math.min(height, safe?.bottom ?? height);
  const fit = (l: number, t: number, r: number, b: number) => {
    const w = Math.max(1, r - l),
      h = Math.max(1, b - t),
      scale = Math.min(w / 960, h / 540);
    return {
      scale,
      x: l + (w - 960 * scale) / 2,
      y: t + (h - 540 * scale) / 2,
    };
  };
  const base = fit(left, top, right, bottom);
  if (
    !capsule ||
    capsule.right <= base.x ||
    capsule.left >= base.x + 960 * base.scale ||
    capsule.bottom <= base.y ||
    capsule.top >= base.y + 62 * base.scale
  )
    return base;
  // Wide phones can place the capsule beside the table without shrinking every card.
  const beside = fit(left, top, Math.min(right, capsule.left - 8), bottom);
  if (
    beside.scale >= base.scale * 0.9 &&
    beside.x + 960 * beside.scale <= capsule.left - 8 + 0.01
  )
    return beside;
  return fit(left, Math.max(top, capsule.bottom + 6), right, bottom);
}
