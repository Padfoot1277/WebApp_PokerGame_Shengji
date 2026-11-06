// Short, event-driven drawing only. No idle loop; never changes game state or hit boxes.
export class Motion {
  private marks = new Map<string, { start: number; duration: number }>();
  private timer: ReturnType<typeof setTimeout> | undefined;
  private enabled = true;
  private visible = true;
  constructor(
    private redraw: () => void,
    private now = () => Date.now(),
    private schedule = (f: () => void) => setTimeout(f, 33),
    private cancel = (id: ReturnType<typeof setTimeout>) => clearTimeout(id),
  ) {}
  start(key: string, duration: number) {
    if (!this.enabled || !this.visible) return;
    if (this.marks.size >= 48 && !this.marks.has(key))
      this.marks.delete(this.marks.keys().next().value!);
    this.marks.set(key, {
      start: this.now(),
      duration: Math.min(1200, Math.max(1, duration)),
    });
    this.queue();
  }
  progress(key: string) {
    const mark = this.marks.get(key);
    return mark
      ? Math.min(1, Math.max(0, (this.now() - mark.start) / mark.duration))
      : 1;
  }
  active(key: string) {
    return this.progress(key) < 1;
  }
  private queue() {
    if (this.timer !== undefined || !this.marks.size) return;
    this.timer = this.schedule(() => {
      this.timer = undefined;
      for (const key of this.marks.keys())
        if (!this.active(key)) this.marks.delete(key);
      this.redraw();
      this.queue();
    });
  }
  clear() {
    if (this.timer !== undefined) this.cancel(this.timer);
    this.timer = undefined;
    this.marks.clear();
  }
  setEnabled(value: boolean) {
    this.enabled = value;
    if (!value) this.clear();
  }
  suspend() {
    this.visible = false;
    this.clear();
  }
  resume() {
    this.visible = true;
  }
}
export const easeOut = (t: number) => 1 - (1 - t) ** 3;
