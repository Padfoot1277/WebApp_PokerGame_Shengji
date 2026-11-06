/** All sounds are generated locally by tools/generate-sounds.mjs. No network audio. */
export type SoundKind = "tap" | "select" | "play" | "turn" | "win";

/** Minimal injectable subset of wx; absent or failing audio is always harmless. */
export interface AudioHost {
  createInnerAudioContext?: () => AudioContextLike;
  setInnerAudioOption?: (options: {
    obeyMuteSwitch: boolean;
    mixWithOther: boolean;
    fail?: () => void;
  }) => void;
}
export interface AudioContextLike {
  src: string;
  autoplay: boolean;
  loop: boolean;
  obeyMuteSwitch: boolean;
  volume: number;
  startTime: number;
  play(): void;
  stop(): void;
  destroy(): void;
  onError?(listener: () => void): void;
}
const sounds: Record<
  SoundKind,
  { duration: number; cooldown: number; priority: number }
> = {
  tap: { duration: 45, cooldown: 100, priority: 0 },
  select: { duration: 65, cooldown: 85, priority: 0 },
  play: { duration: 120, cooldown: 180, priority: 1 },
  turn: { duration: 220, cooldown: 900, priority: 2 },
  win: { duration: 420, cooldown: 1800, priority: 3 },
};

/**
 * Call unlock() only from a real touch handler. A restored preference alone never
 * creates audio. setEnabled is an explicit user preference, default false.
 * Call suspend/onHide and resume/onShow; resume never replays missed events.
 * The injected clock returns monotonic milliseconds (Date.now is the fallback).
 * One context is reused, so sounds cannot accumulate or play concurrently.
 */
export class GameAudio {
  private context: AudioContextLike | null = null;
  private enabled = false;
  private unlocked = false;
  private suspended = false;
  private destroyed = false;
  private failed = false;
  private busyUntil = -Infinity;
  private lastAt = -Infinity;
  private priority = -1;
  private played = new Map<SoundKind, number>();

  constructor(
    private readonly host: AudioHost = typeof wx === "undefined" ? {} : wx,
    private readonly now: () => number = Date.now,
  ) {}

  setEnabled(enabled: boolean) {
    if (this.destroyed) return;
    this.enabled = enabled;
    if (!enabled) this.stop();
    // Creation remains lazy even when the user turns the preference on.
  }

  unlock() {
    if (this.destroyed || this.suspended) return;
    this.unlocked = true;
    if (this.enabled) this.prepare();
  }

  private prepare(): AudioContextLike | null {
    if (this.context) return this.context;
    if (
      !this.enabled ||
      !this.unlocked ||
      this.suspended ||
      this.destroyed ||
      this.failed
    )
      return null;
    let created: AudioContextLike | null = null;
    try {
      const factory = this.host.createInnerAudioContext;
      if (typeof factory !== "function") {
        this.failed = true;
        return null;
      }
      // Since base library 2.3.0 the global option replaces context.obeyMuteSwitch.
      // Do not opt into useWebAudioImplement: it is incompatible with this option.
      this.host.setInnerAudioOption?.({
        obeyMuteSwitch: true,
        mixWithOther: true,
        fail: () => this.fail(),
      });
      if (this.failed) return null;
      created = factory.call(this.host);
      created.autoplay = false;
      created.loop = false;
      created.obeyMuteSwitch = true;
      created.volume = 0.24;
      created.startTime = 0;
      created.onError?.(() => this.fail());
      // An unusual host can synchronously notify an error during registration.
      if (this.failed) {
        try {
          created.destroy();
        } catch {}
        return null;
      }
      this.context = created;
      return created;
    } catch {
      try {
        created?.destroy();
      } catch {}
      this.failed = true;
      return null;
    }
  }

  play(kind: SoundKind): boolean {
    if (
      !this.enabled ||
      !this.unlocked ||
      this.suspended ||
      this.destroyed ||
      this.failed
    )
      return false;
    const sound = sounds[kind];
    if (!sound) return false;
    const time = this.now();
    if (time - (this.played.get(kind) ?? -Infinity) < sound.cooldown)
      return false;
    // Authoritative feedback must not be swallowed by the touch that caused it.
    if (time - this.lastAt < 65 && sound.priority <= this.priority)
      return false;
    if (time < this.busyUntil && sound.priority <= this.priority) return false;
    const context = this.prepare();
    if (!context) return false;
    try {
      context.stop();
      context.src = `assets/audio/${kind}.wav`;
      context.startTime = 0;
      context.play();
      if (this.failed) return false;
      this.played.set(kind, time);
      this.lastAt = time;
      this.priority = sound.priority;
      this.busyUntil = time + sound.duration + 35;
      return true;
    } catch {
      this.fail();
      return false;
    }
  }

  private stop() {
    this.busyUntil = -Infinity;
    this.priority = -1;
    try {
      this.context?.stop();
    } catch {
      /* Never break game lifecycle. */
    }
  }

  private fail() {
    this.failed = true;
    this.stop();
    const context = this.context;
    this.context = null;
    try {
      context?.destroy();
    } catch {}
  }

  suspend() {
    this.suspended = true;
    this.stop();
  }
  resume() {
    if (!this.destroyed) this.suspended = false;
  }
  destroy() {
    if (this.destroyed) return;
    this.destroyed = true;
    this.enabled = false;
    this.stop();
    const context = this.context;
    this.context = null;
    try {
      context?.destroy();
    } catch {}
  }
}
