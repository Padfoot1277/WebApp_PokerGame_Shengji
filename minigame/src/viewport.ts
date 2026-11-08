type Rect = { left: number; top: number; right: number; bottom: number };
export const portraitWidth = 390;
// A single portrait column, like the mobile web table. Never rotate the canvas.
export function viewport(
  width: number,
  height: number,
  safe?: Rect,
  capsule?: Rect,
) {
  const left = Math.max(0, safe?.left ?? 0),
    right = Math.min(width, safe?.right ?? width);
  const top = Math.max(0, safe?.top ?? 0, capsule ? capsule.bottom + 8 : 0),
    bottom = Math.min(height, safe?.bottom ?? height);
  const usableWidth = Math.max(1, right - left),
    usableHeight = Math.max(1, bottom - top);
  const scale = Math.min(usableWidth / portraitWidth, usableHeight / 640, 1.5);
  return {
    scale,
    x: Math.max(left, left + (usableWidth - portraitWidth * scale) / 2),
    y: top,
    width: portraitWidth,
    height: usableHeight / scale,
  };
}
export function portraitLayout(height: number) {
  const footer = height - 110;
  return {
    tableTop: 196,
    tableHeight: 208,
    handTop: 444,
    handBottom: footer - 8,
    footer,
  };
}
