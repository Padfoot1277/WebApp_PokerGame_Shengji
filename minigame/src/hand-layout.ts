// Keep ranks readable in the space beside the action rail, including 33-card hands.
export function handLayout(count: number) {
  const rows = count > 13 ? 2 : 1;
  const columns = Math.max(1, Math.ceil(count / rows));
  const step = Math.min(56, 710 / columns);
  return {
    rows, columns, step,
    slot: (i: number) => ({
      x: 28 + (i % columns) * step,
      y: rows === 2 ? 354 + Math.floor(i / columns) * 84 : 390,
      w: Math.min(58, step + 6),
      h: 80,
    }),
  };
}
