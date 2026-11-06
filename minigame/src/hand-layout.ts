// A 33-card burying hand uses two rows so ranks and touch targets stay legible.
export function handLayout(count: number) {
  const rows = count > 25 ? 2 : 1;
  const columns = rows === 2 ? Math.ceil(count / 2) : count;
  const step = Math.min(rows === 2 ? 52 : 42, 850 / Math.max(1, columns));
  return {
    rows,
    columns,
    step,
    slot: (i: number) => ({
      x: 32 + (i % Math.max(1, columns)) * step,
      y: rows === 2 ? 382 + Math.floor(i / columns) * 44 : 388,
      w: rows === 2 ? Math.min(50, step - 3) : Math.min(44, step + 5),
      h: rows === 2 ? 40 : 80,
    }),
  };
}
