// Non-overlapping cards grouped by suit, with vertical scrolling for long hands.
export function handLayout(
  count: number,
  top = 444,
  groups: readonly number[] = [count],
) {
  const columns = 8,
    step = 44,
    slots: { x: number; y: number; w: number; h: number }[] = [],
    headers: { y: number }[] = [];
  let y = top + 12;
  for (const size of groups) {
    headers.push({ y });
    y += 16;
    for (let i = 0; i < size; i++)
      slots.push({
        x: 20 + (i % columns) * step,
        y: y + Math.floor(i / columns) * 76,
        w: 40,
        h: 64,
      });
    y += Math.ceil(size / columns) * 76 + 8;
  }
  return {
    rows: Math.ceil(count / columns),
    columns,
    step,
    headers,
    contentHeight: y - top,
    slot: (i: number) => slots[i]!,
  };
}
