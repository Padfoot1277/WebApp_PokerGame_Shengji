type Rect = { left: number; top: number; right: number; bottom: number };
// Fit the entire logical table into the usable screen, including notch and gesture insets.
export function viewport(
  width: number,
  height: number,
  safe?: Rect,
  capsule?: Rect,
) {
  const left = Math.max(0, safe?.left ?? 0),
    right = Math.min(width, safe?.right ?? width);
  let top = Math.max(0, safe?.top ?? 0);
  const bottom = Math.min(height, safe?.bottom ?? height);
  if (capsule && capsule.bottom > top && capsule.top < bottom)
    top = Math.max(top, capsule.bottom + 6);
  const w = Math.max(1, right - left),
    h = Math.max(1, bottom - top);
  const scale = Math.min(w / 960, h / 540);
  return {
    scale,
    x: left + (w - 960 * scale) / 2,
    y: top + (h - 540 * scale) / 2,
  };
}
