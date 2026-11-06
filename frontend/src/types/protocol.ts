import type { ViewState } from "./view";
export type ServerMessage =
  | { type: "hello"; uid: string; token: string; name: string }
  | { type: "snapshot"; state: ViewState }
  | { type: "error" | "notice"; message: string; code?: string }
  | {
      type: "ack";
      reqId: string;
      ok: boolean;
      code: string;
      message: string;
      version: number;
    };
export interface ClientEvent<T = unknown> {
  type: string;
  payload: T;
  reqId: string;
}
