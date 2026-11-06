export interface Preferences {
  sound: boolean;
  vibration: boolean;
  reducedMotion: boolean;
}
export const preferenceKey = "shengji-preferences-v1";
export function readPreferences(value: unknown): Preferences {
  const saved =
    value && typeof value === "object" ? (value as Partial<Preferences>) : {};
  return {
    sound: saved.sound === true,
    vibration: saved.vibration === true,
    reducedMotion: saved.reducedMotion === true,
  };
}
