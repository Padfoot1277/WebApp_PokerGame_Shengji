import { isTrumpCard } from "../../frontend/src/utils/card";
import { unavailableCard, floatingFeedback } from "../../frontend/src/utils/play-feedback";
import { showingLast, moveTag } from "../../frontend/src/utils/table";
import { GameConnection } from "./connection";
import { guide, relative, seatName, seatRole, teamName, turn } from "./model";
import { helpPages } from "./help";
import { selectionError } from "./selection";
import { viewport } from "./viewport";
import { handLayout } from "./hand-layout";
import {
  preferenceKey,
  readPreferences,
  type Preferences,
} from "./preferences";
import { Motion, easeOut } from "./motion";
import { GameAudio } from "./audio";
import {
  FeedbackTracker,
  myUpgrade,
  lastTrickSummary,
  type Feedback,
} from "./feedback";
import type { Card, Move, ViewState } from "../../frontend/src/types/view";

const canvas = wx.createCanvas();
const ctx = canvas.getContext("2d") as unknown as CanvasRenderingContext2D;
let width = 960,
  height = 540,
  pixelRatio = 1,
  scale = 1,
  offsetX = 0,
  offsetY = 0;
let view: ViewState | null = null;
let panel:
  | "rules"
  | "room"
  | "connection"
  | "move"
  | "messages"
  | "trick"
  | "record"
  | "preferences"
  | null = null;
let inspectedMove: Move | null = null;
let messageTone: "info" | "error" = "info";
const messages: { text: string; tone: "info" | "error" }[] = [];
let helpPage = 0;
let message = "局域网开发体验 · 需与电脑连接同一 Wi-Fi";
function notify(value: string, tone: "info" | "error" = "info") {
  message = value;
  messageTone = tone;
  if (tone === "error" && view && view.phase !== "lobby") {
    floatError = true; floatLines = wrap(value, 404).slice(0, 3); floatUntil = Date.now() + 3200;
    clearTimeout(floatTimer); floatTimer = setTimeout(() => { floatLines = []; floatError = false; draw(); }, 3200);
  }
  if (messages[messages.length - 1]?.text !== value)
    messages.push({ text: value, tone });
  if (messages.length > 8) messages.shift();
}
const localServer = "ws://192.168.124.19:8080/ws";
const savedServer = wx.getStorageSync("shengji-server");
let server =
  !savedServer ||
  /^wss?:\/\/(localhost|127(?:\.\d+){3}|0\.0\.0\.0|\[::1\])(?=[:/]|$)/i.test(
    savedServer,
  )
    ? localServer
    : savedServer;
if (server !== savedServer) wx.setStorageSync("shengji-server", server);
let room = wx.getStorageSync("shengji-room") || "好友房";
let nickname = wx.getStorageSync("shengji-name") || "牌友";
const selected = new Set<number>();
let previous = false,
  lastHand = "",
  dragging = false,
  paint = true;
const touched = new Set<number>();
let pressed: string | null = null;
const preferences = readPreferences(wx.getStorageSync(preferenceKey));
const audio = new GameAudio();
audio.setEnabled(preferences.sound);
const motion = new Motion(() => draw());
motion.setEnabled(!preferences.reducedMotion);
let visible = true;
let floatLines: string[] = [];
let floatUntil = 0;
let floatError = false;
let floatTimer: ReturnType<typeof setTimeout> | undefined;
let floatBaseline = true;
const feedback = new FeedbackTracker();
let dealtIds: number[] = [];
let lastVibration = -Infinity;
function applyFeedback(events: Feedback[]) {
  if (!visible || panel || link.status !== "已连接") return;
  for (const event of events) {
    if (event.kind === "hand") {
      dealtIds = event.ids;
      motion.start("deal", 440);
    }
    // A fourth move has already left playedMoves: give it the trick result cue.
    if (event.kind === "move" && !view?.trick.resolved)
      motion.start(`move:${event.seat}`, 220);
    if (event.kind === "trick") motion.start("trick", 650);
    if (event.kind === "celebrate") motion.start("celebrate", 700);
    if (event.kind === "ready") motion.start(`ready:${event.seat}`, 300);
    if (event.kind === "turn") {
      motion.start("turn", 700);
      if (preferences.vibration && Date.now() - lastVibration >= 1000) {
        lastVibration = Date.now();
        try {
          wx.vibrateShort?.({ type: "light", fail: () => {} });
        } catch {
          /* Optional feedback. */
        }
      }
    }
  }
  if (events.some((e) => e.kind === "celebrate")) audio.play("win");
  else if (events.some((e) => e.kind === "turn")) audio.play("turn");
  else if (events.some((e) => e.kind === "move")) audio.play("play");
}
function togglePreference(key: keyof Preferences) {
  preferences[key] = !preferences[key];
  try {
    wx.setStorageSync(preferenceKey, preferences);
  } catch {
    /* Session preference still works. */
  }
  motion.setEnabled(!preferences.reducedMotion);
  audio.setEnabled(preferences.sound);
  if (key === "sound" && preferences.sound) audio.play("turn");
  draw();
}
type Target = {
  x: number;
  y: number;
  w: number;
  h: number;
  action: () => void;
  card?: number;
  label?: string;
};
let targets: Target[] = [];
const link = new GameConnection(
  (msg) => {
    if (msg.type === "snapshot") {
      const next = msg.state;
      const key = `${next.roomId}:${next.roundIndex}:${next.phase}:${next.myHand
        ?.flat()
        .map((c) => c.id)
        .sort((a, b) => a - b)}`;
      if (key !== lastHand) {
        selected.clear();
        lastHand = key;
      }
      if (
        view?.roundIndex !== next.roundIndex ||
        view?.trickIndex !== next.trickIndex ||
        view?.phase !== next.phase ||
        !next.trick.lastMoves.some(Boolean)
      )
        previous = false;
      const floating = floatingFeedback(floatBaseline ? null : view, next);
      floatBaseline = false;
      view = next;
      if (floating.length && visible && !(floatError && Date.now() < floatUntil)) {
        floatError = false; floatLines = floating; floatUntil = Date.now() + 2200;
        clearTimeout(floatTimer); floatTimer = setTimeout(() => { floatLines = []; draw(); }, 2200);
      }
      applyFeedback(feedback.observe(next));
    } else if (msg.type === "ack") {
      if (!msg.ok) notify(msg.message, "error");
      else if (messageTone === "error") notify("操作已确认");
    } else if (msg.type === "notice" || msg.type === "error")
      notify(msg.message, msg.type === "error" ? "error" : "info");
    else if (msg.type === "hello") {
      floatBaseline = true; floatLines = []; clearTimeout(floatTimer);
      feedback.reset();
      motion.clear();
      nickname = msg.name;
      wx.setStorageSync("shengji-name", nickname);
    }
  },
  () => {
    if (link.status !== "已连接") {
      feedback.reset();
      motion.clear();
      audio.suspend();
      if (visible) audio.resume();
    }
    draw();
  },
);
function text(
  value: string,
  x: number,
  y: number,
  size = 16,
  color = "#30314f",
  max = 900,
) {
  ctx.fillStyle = color;
  ctx.font = `${size}px sans-serif`;
  ctx.textBaseline = "middle";
  let output = value;
  while (output.length > 1 && ctx.measureText(output).width > max)
    output = output.slice(0, -2) + "…";
  ctx.fillText(output, x, y);
}
function wrap(value: string, max: number, size = 16): string[] {
  ctx.font = `${size}px sans-serif`;
  const lines: string[] = [];
  let line = "";
  for (const char of value) {
    if (line && ctx.measureText(line + char).width > max) {
      lines.push(line);
      line = "";
    }
    line += char;
  }
  if (line) lines.push(line);
  return lines;
}
function box(x: number, y: number, w: number, h: number, color: string) {
  ctx.fillStyle = color;
  const r = Math.min(12, w / 2, h / 2);
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
  ctx.fill();
}
function sparkle(x: number, y: number, color: string) {
  ctx.beginPath();
  ctx.moveTo(x, y - 8);
  for (const [dx, dy] of [
    [3, -3],
    [8, 0],
    [3, 3],
    [0, 8],
    [-3, 3],
    [-8, 0],
    [-3, -3],
  ])
    ctx.lineTo(x + dx!, y + dy!);
  ctx.closePath();
  ctx.fillStyle = color;
  ctx.fill();
}
function button(
  label: string,
  x: number,
  y: number,
  w: number,
  action: () => void,
  enabled = true,
  tone: "primary" | "quiet" | "field" | "sunny" = "primary",
) {
  const fill = !enabled
    ? "#e8e4e9"
    : tone === "primary"
      ? "#6652df"
      : tone === "sunny"
        ? "#ffcc70"
        : tone === "field"
          ? "#ffffff"
          : "#eee8ff";
  const pressKey = `${x}:${y}:${label}`;
  const down = pressed === pressKey && motion.active("press");
  if (enabled && tone !== "field")
    box(
      x,
      y + 4,
      w,
      40,
      tone === "primary" ? "#b5a5ee" : tone === "sunny" ? "#e4ad55" : "#ded4ed",
    );
  box(x, y + (down ? 3 : 0), w, 40, fill);
  ctx.font = "16px sans-serif";
  const labelX =
    tone === "field" || w > 300
      ? x + 14
      : x + Math.max(14, (w - ctx.measureText(label).width) / 2);
  text(
    label,
    labelX,
    y + 20 + (down ? 3 : 0),
    16,
    !enabled ? "#a49baa" : tone === "primary" ? "#ffffff" : "#4d496b",
    w - 28,
  );
  if (enabled) targets.push({ x, y, w, h: 40, action, label: pressKey });
}
function avatar(seat: number, x: number, y: number) {
  const colors = ["#ffe08a", "#ffd1df", "#d6ef9a", "#d9d0ff"];
  ctx.beginPath();
  ctx.fillStyle = colors[seat]!;
  ctx.arc(x, y, 11, 0, Math.PI * 2);
  ctx.fill();
  text(
    ["♠", "♥", "♣", "♦"][seat]!,
    x - 5,
    y,
    13,
    seat % 2 ? "#c73e61" : "#51409a",
    16,
  );
  const ring =
    seat === view?.mySeat && seat === turn(view) && motion.active("turn")
      ? "turn"
      : seat === view?.trick.winnerSeat && motion.active("trick")
        ? "trick"
        : motion.active(`ready:${seat}`)
          ? `ready:${seat}`
          : "";
  if (ring) {
    const p = motion.progress(ring);
    ctx.save();
    ctx.globalAlpha = (1 - p) * 0.85;
    ctx.strokeStyle = ring === "turn" ? "#ef9956" : "#8d75d9";
    ctx.lineWidth = 3;
    ctx.beginPath();
    ctx.arc(x, y, 14 + 8 * p, 0, Math.PI * 2);
    ctx.stroke();
    ctx.restore();
  }
}
function heroCards() {
  for (const [x, angle, suit, color] of [
    [110, -0.17, "♠", "#6652df"],
    [174, 0.05, "♥", "#dc4965"],
    [238, 0.19, "♣", "#6652df"],
  ] as const) {
    ctx.save();
    ctx.translate(x, 407);
    ctx.rotate(angle);
    box(-24, -35, 50, 76, "#fffdf7");
    text("A", -15, -17, 18, color, 40);
    text(suit, -12, 12, 26, color, 40);
    ctx.restore();
  }
}
function edit(title: string, current: string, save: (v: string) => void) {
  wx.showModal({
    title,
    editable: true,
    placeholderText: current,
    success: (r) => {
      if (r.confirm && r.content?.trim()) {
        save(r.content.trim());
        draw();
      }
    },
  });
}
let createRecord = false;
function connect(create = false) {
  if (!/^wss?:\/\/[^\s?#]+$/i.test(server)) {
    notify("地址需类似 ws://电脑IP:8080/ws");
    draw();
    return;
  }
  if (create) room = Math.random().toString(36).slice(2, 8).toUpperCase();
  room = room.trim();
  if (!room || [...room].length > 32) {
    notify("请输入 1–32 个字符的房间码");
    draw();
    return;
  }
  wx.setStorageSync("shengji-server", server);
  wx.setStorageSync("shengji-room", room);
  wx.setStorageSync("shengji-name", nickname);
  view = null;
  selected.clear();
  link.connect(server, room, nickname, create && createRecord);
}
function leaveRoom() {
  const leave = () => {
    panel = null;
    previous = false;
    link.close();
    view = null;
    selected.clear();
    notify("座位已保留，加入原房间可继续");
    draw();
  };
  if (
    view &&
    view.mySeat >= 0 &&
    !["lobby", "round_settle", "game_over"].includes(view.phase)
  ) {
    wx.showModal({
      title: "返回大厅？",
      content: "这会断开当前连接，座位仍保留。重新加入原房间可以继续对局。",
      success: (r) => {
        if (r.confirm) leave();
      },
    });
  } else leave();
}
function copyRoom() {
  wx.setClipboardData({
    data: view?.roomId || room,
    success: () => {
      notify("房间码已复制，发给好友即可加入");
      draw();
    },
    fail: () => {
      notify("复制失败，请手动记下房间码");
      draw();
    },
  });
}
function selectPair() {
  if (!view || link.busy || previous) return;
  const hand = (view.myHand ?? []).flat();
  const seed =
    hand.find((c) => selected.has(c.id)) ??
    hand.find((c) =>
      hand.some(
        (other) =>
          other.id !== c.id && c.suit === other.suit && c.rank === other.rank,
      ),
    );
  const pair = seed
    ? hand.filter((c) => c.suit === seed.suit && c.rank === seed.rank)
    : [];
  if (pair.length !== 2) {
    notify(seed ? "这张牌没有同花色同点数的另一张" : "手牌中没有对子");
    draw();
    return;
  }
  const ids = new Set(pair.map((c) => c.id));
  for (const id of new Set([...selected, ...ids]))
    if (selected.has(id) !== ids.has(id)) motion.start(`select:${id}`, 110);
  selected.clear();
  pair.forEach((c) => selected.add(c.id));
  draw();
}
function drawPanel() {
  if (!panel) return;
  // Modal targets replace table targets so touches cannot fall through.
  targets = [];
  box(0, 0, 960, 540, "rgba(15,35,48,0.65)");
  box(100, 72, 760, 408, "#ffffff");
  button(
    "关闭",
    750,
    88,
    88,
    () => {
      panel = null;
      draw();
    },
    true,
    "quiet",
  );
  if (panel === "rules") {
    const page = helpPages[helpPage]!;
    text(page.title, 128, 111, 26, "#30314f", 600);
    const lines = page.lines.flatMap((line) => wrap("• " + line, 704, 16));
    lines.forEach((line, i) =>
      text(
        line,
        128,
        160 + i * Math.min(29, 244 / lines.length),
        16,
        "#4d496b",
        704,
      ),
    );
    button(
      "上一页",
      128,
      420,
      116,
      () => {
        helpPage--;
        draw();
      },
      helpPage > 0,
      "quiet",
    );
    text(`${helpPage + 1} / ${helpPages.length}`, 449, 440, 16, "#74708d");
    button(
      "下一页",
      712,
      420,
      116,
      () => {
        helpPage++;
        draw();
      },
      helpPage < helpPages.length - 1,
    );
  } else if (panel === "trick") {
    text(
      view?.surrendered ? "最近完整墩" : "末墩出牌",
      128,
      111,
      26,
      "#30314f",
    );
    (view?.trick.lastMoves ?? []).forEach((move, seat) => {
      const x = 128 + (seat % 2) * 360,
        y = 148 + Math.floor(seat / 2) * 145;
      box(x, y, 340, 130, "#fff7e7");
      text(
        `${relative(seat, view?.mySeat ?? -1)} · ${view?.seats[seat]?.name || "牌友"}${view?.trick.winnerSeat === seat ? " · 赢墩" : ""}`,
        x + 12,
        y + 20,
        16,
        "#30314f",
        316,
      );
      if (move) {
        move.followMove.cards
          .slice(0, 8)
          .forEach((c, i) => card(c, x + 12 + i * 42, y + 40, 36, 62));
        text(
          `${move.followMove.cards.length} 张 · 点击查看完整出牌`,
          x + 12,
          y + 116,
          13,
          "#6652df",
        );
        targets.push({
          x,
          y,
          w: 340,
          h: 130,
          action: () => {
            inspectedMove = move;
            panel = "move";
            draw();
          },
        });
      } else text("本墩尚未出牌", x + 12, y + 70, 16, "#74708d");
    });
  } else if (panel === "move" && inspectedMove) {
    text(`${relative(inspectedMove.seat, view?.mySeat ?? -1)} · 出牌详情`, 128, 111, 26, "#30314f");
    text(
      `${inspectedMove.followMove.cards.length} 张 · ${inspectedMove.suitClass}`,
      128,
      158,
      17,
      "#74708d",
    );
    inspectedMove.followMove.cards.forEach((c, i) =>
      card(c, 128 + (i % 14) * 50, 190 + Math.floor(i / 14) * 80, 44, 70),
    );
    wrap(inspectedMove.info || "本次实际出牌，服务器已确认", 704).forEach(
      (line, i) => text(line, 128, 406 + i * 23, 16, inspectedMove?.info?.startsWith("甩牌被阻止") ? "#c53939" : "#6652df", 704),
    );
  } else if (panel === "record") {
    text("公开记牌", 128, 111, 26, "#30314f");
    const s = view;
    if (!s || s.hideRecord || !s.record)
      text("本局未开放记牌信息", 128, 190, 18, "#74708d");
    else {
      text("只统计已结算出牌，不含未公开手牌与底牌", 128, 158, 16, "#74708d");
      text("花色", 140, 202, 17, "#30314f");
      ["已出张数", "A", "K", "10"].forEach((label, j) =>
        text(label, 360 + j * 112, 202, 17, "#30314f"),
      );
      (
        [
          ["黑桃", "spade"],
          ["红桃", "heart"],
          ["梅花", "club"],
          ["方块", "diamond"],
        ] as const
      ).forEach(([name, key], i) => {
        const row = s.record[key],
          y = 240 + i * 38;
        box(128, y - 17, 704, 34, i % 2 === 0 ? "#fff7e7" : "#ffffff");
        text(name, 140, y, 18, /[红方]/.test(name) ? "#dc4965" : "#4d496b");
        [row.num, row.a, row.k, row.ten].forEach((n, j) =>
          text(String(n), 360 + j * 112, y, 18, "#4d496b"),
        );
      });
      text(
        `大王已出 ${s.record.bigJoker} 张 · 小王已出 ${s.record.smallJoker} 张`,
        140,
        418,
        17,
        "#6652df",
      );
    }
  } else if (panel === "messages") {
    text("牌桌消息", 128, 111, 26, "#30314f");
    if (!messages.length)
      text("暂无消息，当前操作提示会显示在牌桌上方。", 128, 180, 17, "#74708d");
    messages.slice(-5).forEach((m, i) =>
      wrap(m.text, 704, 16)
        .slice(0, 2)
        .forEach((line, j) =>
          text(
            line,
            128,
            160 + i * 56 + j * 23,
            16,
            m.tone === "error" ? "#a54747" : "#4d496b",
            704,
          ),
        ),
    );
  } else if (panel === "preferences") {
    text("声音与动效", 128, 111, 26, "#30314f");
    const rows = [
      ["sound", "轻快音效", "轻点、出牌、轮到你与升级；遵循系统静音"],
      ["vibration", "回合轻震", "只在轮到你时提醒一次，不随选牌振动"],
      ["reducedMotion", "减少动态效果", "关闭位移、光圈与彩纸，保留静态提示"],
    ] as const;
    rows.forEach(([key, title, detail], i) => {
      const y = 154 + i * 82;
      box(120, y - 8, 720, 72, i % 2 ? "#fff7e7" : "#f3efff");
      text(title, 140, y + 11, 20, "#30314f", 490);
      text(detail, 140, y + 42, 15, "#74708d", 490);
      button(
        key === "reducedMotion"
          ? preferences[key]
            ? "已减少"
            : "标准动态"
          : preferences[key]
            ? "已开启"
            : "已关闭",
        692,
        y + 6,
        126,
        () => togglePreference(key),
        true,
        preferences[key] ? "primary" : "quiet",
      );
    });
    text(
      "本机保存 · 开启音效会试听一次 · 初始静音",
      140,
      434,
      15,
      "#74708d",
      490,
    );
    button(
      "试听音效",
      692,
      414,
      126,
      () => audio.play("turn"),
      preferences.sound,
      "quiet",
    );
  } else if (panel === "room") {
    text("好友房", 128, 111, 26, "#30314f");
    text(view?.roomId || room, 128, 180, 38, "#6652df", 650);
    text("对家是队友 · 四人准备后自动开局", 128, 235, 18, "#74708d");
    button("复制房间码", 128, 278, 200, copyRoom);
    button("邀请好友", 348, 278, 200, () =>
      wx.shareAppMessage({
        title: "一起来打升级",
        query: `room=${encodeURIComponent(view?.roomId || room)}`,
      }),
    );
    text(
      "好友需能访问同一服务器；分享成功状态由微信处理。",
      128,
      358,
      16,
      "#74708d",
    );
    button(
      "消息记录",
      128,
      404,
      160,
      () => {
        panel = "messages";
        draw();
      },
      true,
      "quiet",
    );
    if (view && !view.hideRecord) button(
      "公开记牌",
      308,
      404,
      160,
      () => {
        panel = "record";
        draw();
      },
      !!view && view.phase !== "lobby",
      "quiet",
    );
    button(
      "声音与动效",
      488,
      404,
      180,
      () => {
        panel = "preferences";
        draw();
      },
      true,
      "quiet",
    );
  } else {
    text("连接设置", 128, 111, 26, "#30314f");
    text("服务器地址", 128, 178, 17, "#74708d");
    button(
      server,
      128,
      204,
      700,
      () =>
        edit("服务器地址", server, (v) => {
          server = v;
          wx.setStorageSync("shengji-server", v);
        }),
      true,
      "quiet",
    );
    text(
      "本地体验：手机与电脑连接同一 Wi-Fi，保持电脑后端运行。",
      128,
      292,
      17,
      "#4d496b",
    );
    text("异地体验需要配置可访问的 WSS 服务地址。", 128, 328, 17, "#4d496b");
  }
}
function send(type: string, payload: unknown = {}) {
  if (!link.send(type, payload)) {
    notify(link.busy ? "上一操作正在确认，请稍候" : "尚未连接，请检查网络");
  }
  draw();
}
function submit(type: string) {
  if (!view) return;
  const cards = (view?.myHand ?? []).flat().filter((c) => selected.has(c.id));
  const error = selectionError(type, cards, view);
  if (error) {
    notify(error, "error");
    draw();
    return;
  }
  const jokers = cards.filter((c) => c.suit === "SJ" || c.suit === "BJ");
  const levels = cards.filter((c) => c.suit !== "SJ" && c.suit !== "BJ");
  if (type === "game.play_cards") {
    send(type, { cardIds: [...selected] });
    return;
  }
  if (type === "game.put_bottom") {
    send(type, { discardIds: [...selected] });
    return;
  }
  if (type === "game.attack_trump") {
    send(type, { jokerIds: jokers.map((c) => c.id) });
    return;
  }
  send(type, { jokerId: jokers[0]!.id, levelIds: levels.map((c) => c.id) });
}
function card(
  c: Card,
  x: number,
  y: number,
  w = 36,
  h = 48,
  interactive = false,
  arrival = 0,
) {
  const disabled = interactive && (previous || link.busy || unavailableCard(view, c, [...selected]));
  const picked = interactive && selected.has(c.id);
  const hitTop = y - (picked ? 9 : 0);
  const selectionLift = interactive
    ? (picked ? 9 : -9) * (1 - easeOut(motion.progress(`select:${c.id}`)))
    : 0;
  const dealIndex = interactive ? dealtIds.indexOf(c.id) : -1;
  const dealProgress = Math.min(
    1,
    Math.max(0, (motion.progress("deal") * 440 - dealIndex * 4) / 260),
  );
  const top =
    hitTop +
    selectionLift +
    arrival -
    (dealIndex >= 0 ? 10 * (1 - easeOut(dealProgress)) : 0);
  box(x, top, w, h, picked ? "#ecf7ef" : "#ffffff");
  if (isTrumpCard(c, view?.trump)) { ctx.strokeStyle = "#ffe033"; ctx.lineWidth = 2; ctx.stroke(); }
  if (picked) {
    ctx.strokeStyle = "#379b80";
    ctx.lineWidth = 2;
    ctx.stroke();
  }
  const red = /[♥♦]/.test(c.suit) || c.suit === "BJ";
  const suit =
    c.suit === "BJ"
      ? "大"
      : c.suit === "SJ"
        ? "小"
        : c.suit.replace(/\uFE0F/g, "");
  text(
    suit,
    x + 4,
    top + (h <= 44 ? 9 : 14),
    h <= 44 ? 10 : 18,
    disabled && !picked ? (red ? "#c78996" : "#88838f") : red ? "#dc4965" : "#30314f",
    w - 6,
  );
  text(
    c.rank.replace("大王", "王").replace("小王", "王"),
    x + 4,
    top + (h <= 44 ? 27 : 40),
    h <= 44 ? 18 : 28,
    disabled && !picked ? (red ? "#c78996" : "#88838f") : red ? "#dc4965" : "#30314f",
    w - 6,
  );
  if (interactive) {
    if (view?.usedTrumpIds.includes(c.id))
      text("亮", x + Math.max(15, w - 13), top + 10, 8, "#805a26", 12);
    if (!link.busy && !previous)
      targets.push({
        x,
        y: hitTop,
        w,
        h,
        card: c.id,
        action: () => {
          motion.start(`select:${c.id}`, 110);
          if (selected.has(c.id)) selected.delete(c.id);
          else selected.add(c.id);
          draw();
        },
      });
  }
}
function drawSettlement(s: ViewState, busy: boolean) {
  targets = targets.filter((t) => t.y < 140 || t.y >= 488);
  box(24, 150, 912, 316, "#ffffff");
  if (motion.active("celebrate")) {
    const p = motion.progress("celebrate");
    ctx.save();
    ctx.beginPath();
    ctx.rect(684, 150, 244, 62);
    ctx.clip();
    ctx.globalAlpha = 1 - p;
    const colors = ["#f0aa49", "#a28ae8", "#e887a7", "#91bd67"];
    for (let i = 0; i < 12; i++) {
      const angle = (i * Math.PI) / 6;
      ctx.fillStyle = colors[i % colors.length]!;
      ctx.save();
      ctx.translate(
        806 + Math.cos(angle) * (60 + 52 * easeOut(p)),
        180 + Math.sin(angle) * (16 + 26 * easeOut(p)),
      );
      ctx.rotate(angle + p);
      ctx.fillRect(-3, -2, 6, 4);
      ctx.restore();
    }
    ctx.restore();
  }
  const over = s.phase === "game_over";
  text(
    over
      ? `${s.mySeat < 0 ? teamName(s.winnerTeam) : s.winnerTeam === s.mySeat % 2 ? "我方" : "对方"}升 A 获胜`
      : `第 ${s.roundIndex + 1} 局结算`,
    48,
    181,
    26,
    "#30314f",
    500,
  );
  const mine = myUpgrade(s);
  const won = over && s.mySeat >= 0 && s.mySeat % 2 === s.winnerTeam;
  box(706, 160, 204, 38, mine > 0 || won ? "#ffe08a" : "#eee8ff");
  if (mine > 0 || won) sparkle(725, 180, "#8a64cf");
  text(
    over
      ? s.mySeat < 0
        ? "整场结束"
        : won
          ? "我方获胜！"
          : "下次再战！"
      : s.mySeat < 0
        ? "本局结束"
        : mine > 0
          ? `我方升级 +${mine}`
          : "下一局加油！",
    mine > 0 || won ? 742 : 722,
    180,
    17,
    "#51409a",
    mine > 0 || won ? 158 : 176,
  );
  text(String(s.roundPointsFinal), 48, 244, 52, "#6652df", 220);
  text("打家最终得分", 48, 288, 17, "#74708d");
  text(s.roundResultLabel, 286, 233, 26, "#6652df", 600);
  text(
    `坐家升 ${s.callerDelta} 级 · 打家升 ${s.defenderDelta} 级`,
    286,
    274,
    18,
    "#4d496b",
    600,
  );
  text(
    over
      ? "整场结束，返回大厅可创建新房间"
      : `下一局由${relative(s.nextStarterSeat, s.mySeat)}「${s.seats[s.nextStarterSeat]?.name || "牌友"}」开启`,
    286,
    308,
    17,
    "#74708d",
    610,
  );
  text(
    s.surrendered
      ? "同队确认弃局 · 未完成墩不计分，不额外抠底"
      : `底牌 ${s.bottomPoints} 分 · 倍率 ×${s.bottomMul} · 抠底实得 ${s.bottomAward} 分`,
    48,
    342,
    16,
    "#74708d",
    840,
  );
  (s.bottomReveal ?? []).forEach((c, i) => card(c, 48 + i * 62, 362, 52, 88));
  if (!(s.bottomReveal ?? []).length)
    text("本局未公开底牌", 48, 400, 17, "#74708d");
  button(
    s.surrendered ? "最近完整墩" : "查看末墩",
    738,
    394,
    172,
    () => {
      panel = "trick";
      draw();
    },
    s.trick.lastMoves.some(Boolean),
    "quiet",
  );
  if (s.mySeat !== s.nextStarterSeat && !over)
    text(
      s.mySeat < 0
        ? "等待牌友开启下一局，你正在观战"
        : "等待下一局开启，你的座位已保留",
      342,
      512,
      16,
      "#74708d",
      570,
    );
  if (over) button("返回大厅", 738, 490, 172, leaveRoom, !busy);
}
function draw() {
  if (!ctx || !visible) return;
  if (panel) motion.clear();
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#fff7e7";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.setTransform(
    pixelRatio * scale,
    0,
    0,
    pixelRatio * scale,
    pixelRatio * offsetX,
    pixelRatio * offsetY,
  );
  targets = [];
  box(0, 0, 960, 540, "#fff7e7");
  box(16, 14, 928, 48, "#ffffff");
  text("升 级 · 好友房", 34, 38, 23, "#30314f");
  text(
    `${link.status}${view ? "  ·  房间 " + view.roomId : ""}`,
    258,
    38,
    15,
    "#74708d",
    334,
  );
  button(
    "规则",
    view ? 602 : 818,
    18,
    88,
    () => {
      panel = "rules";
      draw();
    },
    true,
    "quiet",
  );
  if (view)
    button(
      "菜单",
      702,
      18,
      104,
      () => {
        panel = "room";
        draw();
      },
      true,
      "quiet",
    );
  else
    button(
      "声音与动效",
      672,
      18,
      134,
      () => {
        panel = "preferences";
        draw();
      },
      true,
      "quiet",
    );
  if (view && link.status !== "已连接") {
    button(
      "连接中断 · 点击重连",
      248,
      18,
      342,
      () => link.retryNow(),
      true,
      "quiet",
    );
  }
  if (view) button("返回大厅", 818, 18, 112, leaveRoom, true, "quiet");
  if (!view) {
    box(28, 82, 330, 384, "#6856e9");
    text("SHENGJI", 56, 122, 14, "#d9d0ff");
    text("好牌，", 56, 179, 42, "#ffffff");
    text("一起打。", 56, 233, 42, "#ffffff");
    text("四人搭档 · 两副扑克", 56, 299, 18, "#f8f0ff");
    text("邀上好友，坐下就开局", 56, 331, 16, "#d9d0ff");
    heroCards();
    box(378, 82, 554, 384, "#eee8ff");
    text("开始一桌", 406, 118, 26, "#30314f");
    for (const [label, value, y, save] of [
      ["昵称", nickname, 150, (v: string) => (nickname = v)],
      ["房间码", room, 212, (v: string) => (room = v)],
    ] as const) {
      text(label, 408, y + 20, 15, "#74708d");
      button(value, 498, y, 406, () => edit(label, value, save), true, "field");
    }
    button(
      "连接设置",
      498,
      274,
      160,
      () => {
        panel = "connection";
        draw();
      },
      true,
      "quiet",
    );
    text("首次使用可在此设置服务器", 674, 294, 14, "#74708d", 225);
    button("创建房间", 408, 344, 232, () => connect(true));
    button(`新房记牌器：${createRecord ? "开启" : "关闭"}`, 408, 450, 496, () => { createRecord = !createRecord; draw(); }, true, "quiet");
    button("加入房间", 656, 344, 248, () => connect(), true, "sunny");
    text(
      "点击上方信息可编辑 · 输入好友房间码即可加入",
      408,
      422,
      14,
      "#74708d",
      495,
    );
  } else {
    const s = view;
    const busy = link.busy || link.status !== "已连接";
    text(
      `第 ${s.roundIndex + 1} 局   ${s.mySeat >= 0 ? `我方 ${s.teams[s.mySeat % 2]?.levelRank} 级 / 对方 ${s.teams[(s.mySeat + 1) % 2]?.levelRank} 级` : `A组 ${s.teams[0]?.levelRank} 级 / B组 ${s.teams[1]?.levelRank} 级`}   打家 ${s.points} 分`,
      28,
      88,
      16,
    );
    const trump =
      s.phase === "lobby"
        ? "四人准备后发牌"
        : s.phase === "call_trump"
          ? "待定主"
          : `${s.trump.hasTrumpSuit ? s.trump.suit : "硬主"} · 级牌 ${s.trump.levelRank}${s.trump.locked ? " · 已锁主" : ""}`;
    text(trump, 660, 88, 15, "#6652df", 270);
    box(24, 106, 912, 32, "#e5edbd");
    const partnerVote =
      s.mySeat >= 0 && s.surrenderVotes[(s.mySeat + 2) % 4] && !s.surrendered;
    text(
      previous
        ? `上一墩回放 · ${relative(s.trick.winnerSeat, s.mySeat)}赢墩 · 点击“回当前墩”继续操作`
        : `${partnerVote ? "队友已申请弃局 · " : ""}${guide(s)}`,
      38,
      122,
      15,
      "#4c5730",
      880,
    );
    const base = Math.max(0, s.mySeat);
    const positions = s.phase === "lobby" ? [
      [330, 150, 300, 90],
      [24, 188, 278, 118],
      [658, 188, 278, 118],
      [330, 252, 300, 90],
    ] : [[24, 146, 370, 94], [24, 244, 370, 94], [404, 146, 370, 94], [404, 244, 370, 94]];
    [2, 3, 1, 0].forEach((off, i) => {
      const seat = (base + off) % 4;
      const [x, y, w, h] = positions[i]!;
      const player = s.seats[seat]!;
      const shown = showingLast(s, previous) ? s.trick.lastMoves : s.trick.playedMoves;
      const maximum = !!shown[seat] && seat === (showingLast(s, previous) ? s.trick.winnerSeat : s.trick.biggerSeat);
      box(x!, y!, w!, h!, maximum ? "#dfceff" : seat === turn(s) ? "#e6f4b6" : "#ffffff");
      if (seat === turn(s)) {
        ctx.strokeStyle = "#f5a14f";
        ctx.lineWidth = 2;
        ctx.stroke();
      }
      avatar(seat, x! + 20, y! + 17);
      text(
        `${player.name || "等待入座"}${!player.online && player.uid ? " · 离线" : ""}`,
        x! + 38,
        y! + 15,
        16,
        "#30314f",
        w! - 170,
      );
      if (seat === turn(s) && s.phase !== "play_trick") {
        box(x! + w! - 70, y! + 5, 62, 23, "#ffe08a");
        text(
          seat === s.mySeat ? "轮到你!" : "操作中",
          x! + w! - 64,
          y! + 16,
          12,
          "#674919",
          58,
        );
      }
      if (s.phase === "lobby") {
        text(
          `${seatName(seat)} · ${seatRole(s, seat)}${player.uid ? (player.ready ? " · 已准备" : " · 未准备") : ""}`,
          x! + 8,
          y! + 38,
          13,
          player.ready ? "#6652df" : "#807896",
        );
        if (!player.uid)
          button(
            "入座",
            x! + 8,
            y! + 46,
            90,
            () => send("room.sit", { seat }),
            !busy,
          );
        if (seat === s.mySeat) {
          button(
            player.ready ? "取消准备" : "准备",
            x! + 8,
            y! + 46,
            100,
            () => send(player.ready ? "room.unready" : "room.ready"),
            !busy,
          );
          button(
            "离座",
            x! + 120,
            y! + 46,
            80,
            () => send("room.leave_seat"),
            !busy,
            "quiet",
          );
        }
      } else {
        const move = (showingLast(s, previous) ? s.trick.lastMoves : s.trick.playedMoves)[seat];
        const last = showingLast(s, previous);
        const tags = [
          !previous && seat === turn(s) ? "⏳" : "",
          seat === (last ? s.trick.lastLeaderSeat : s.trick.leaderSeat) ? "🚩" : "",
          move && seat === (last ? s.trick.winnerSeat : s.trick.biggerSeat) ? "👑" : "",
          moveTag(move) ? moveTag(move)!.label : "",
        ].filter(Boolean);
        text(tags.join(" · "), x! + w! - 126, y! + 15, 12, "#6652df", 118);

        if (move) {
          const arrival = previous
            ? 0
            : 10 * (1 - easeOut(motion.progress(`move:${seat}`)));
          move.followMove.cards
            .slice(0, 8)
            .forEach((c, j) =>
              card(c, x! + 8 + j * 44, y! + 38, 43, 52, false, arrival),
            );
          text(
            (move.info && !["垫牌", "杂牌"].includes(move.info) ? move.info : "") || (move.followMove.cards.length > 8
              ? `共 ${move.followMove.cards.length} 张 · 查看全部`
              : ""),
            x! + 8,
            y! + h! - 5,
            10,
            move.info?.startsWith("甩牌被阻止") ? "#c53939" : "#6652df",
            w! - 16,
          );
          targets.push({
            x: x!,
            y: y!,
            w: w!,
            h: h!,
            action: () => {
              inspectedMove = move;
              panel = "move";
              draw();
            },
          });
        } else
          text(
            (s.phase === "call_trump" && s.callPassedSeats[seat]) ||
              (s.phase === "trump_fight" && s.fightPassedSeats[seat])
              ? "已跳过"
              : s.phase === "play_trick"
                ? s.trick.resolved && s.trick.lastMoves.every(Boolean)
                  ? seat === s.trick.winnerSeat
                    ? `上墩赢墩 · ${lastTrickSummary(s)}`
                    : ""
                  : ""
                : "",
            x! + 8,
            y! + 42,
            12,
            "#807896",
          );
      }
    });
    if (s.phase === "lobby") {
      box(24, 366, 912, 98, "#ffffff");
      text(
        `已入座 ${s.seats.filter((p) => p.uid).length} / 4 人`,
        44,
        393,
        22,
        "#30314f",
      );
      s.seats.forEach((seat, i) => {
        const x = 342 + i * 38;
        box(x, 381, 28, 26, seat.uid && seat.ready ? "#d9edaf" : "#eee8ff");
        if (seat.uid && seat.ready) {
          ctx.save();
          ctx.strokeStyle = "#557334";
          ctx.lineWidth = 2.5;
          ctx.lineCap = "round";
          ctx.beginPath();
          ctx.moveTo(x + 7, 394);
          ctx.lineTo(x + 12, 399);
          ctx.lineTo(x + 21, 388);
          ctx.stroke();
          ctx.restore();
        } else
          text(
            ["A1", "B1", "A2", "B2"][i]!,
            x + 6,
            394,
            16,
            seat.uid ? "#51409a" : "#9a92aa",
            22,
          );
      });
      text(
        `已准备 ${s.seats.filter((p) => p.uid && p.ready).length} / 4 人 · 对家是队友`,
        44,
        429,
        17,
        "#74708d",
      );
      button("复制房间码", 526, 395, 180, copyRoom);
      button(
        "邀请好友",
        726,
        395,
        180,
        () => {
          panel = "room";
          draw();
        },
        true,
        "quiet",
      );
    }
    if (s.phase !== "lobby" && s.mySeat >= 0) {
      if (!["round_settle", "game_over"].includes(s.phase))
        box(16, 344, 764, 182, "#f0e7fb");
      box(794, 144, 150, 382, "#ffffff");
      const groups = s.myHand ?? [];
      const count = groups.flat().length;
      if (s.phase === "bottom" && s.bottomOwnerSeat === s.mySeat) {
        button(
          "查看原底牌",
          802,
          206,
          134,
          () =>
            wx.showModal({
              title: "本次收取的底牌",
              content: (s.myBottom ?? [])
                .map((c) => `${c.suit}${c.rank}`)
                .join("  "),
              showCancel: false,
            }),
          !busy,
          "quiet",
        );
      }
      const handPositions = handLayout(count);
      let index = 0;
      groups.forEach((group) => {
        group.forEach((c) => {
          const slot = handPositions.slot(index);
          card(c, slot.x, slot.y, slot.w, slot.h, true);
          index++;
        });
      });
      const actions: { label: string; run: () => void }[] = [];
      const own = turn(s) === s.mySeat;
      if (
        s.phase === "call_trump" &&
        !s.callPassedSeats[s.mySeat] &&
        (s.callMode === "race" || own)
      ) {
        actions.push(
          { label: "定主", run: () => submit("game.call_trump") },
          { label: "跳过", run: () => send("game.call_pass") },
        );
      }
      if (s.phase === "bottom" && own)
        actions.push({
          label: "扣底 8 张",
          run: () => submit("game.put_bottom"),
        });
      if (s.phase === "trump_fight" && own) {
        if (!s.trump.locked)
          actions.push({
            label: "改主",
            run: () => submit("game.change_trump"),
          });
        actions.push(
          { label: "攻主", run: () => submit("game.attack_trump") },
          { label: "跳过", run: () => send("game.call_pass") },
        );
      }
      if (s.phase === "play_trick" && own)
        actions.push({ label: "出牌", run: () => submit("game.play_cards") });
      if (s.phase === "round_settle" && s.nextStarterSeat === s.mySeat)
        actions.push({
          label: "下一局",
          run: () => send("game.start_next_round"),
        });
      if (count > 0)
        actions.push({
          label: "清空选牌",
          run: () => {
            selected.clear();
            draw();
          },
        });
      actions.forEach((a, i) =>
        button(
          a.label,
          802,
          482 - i * 54,
          134,
          a.run,
          !busy && !previous,
          a.label === "清空选牌" || a.label === "跳过" ? "quiet" : "primary",
        ),
      );
      if (count > 0)
        button("选对子", 802, 482 - actions.length * 54, 134, selectPair, !busy && !previous, "quiet");
      if (busy)
        text(
          link.status !== "已连接"
            ? "连接恢复后可继续操作"
            : "正在确认操作，请稍候",
          700,
          363,
          14,
          "#74708d",
          224,
        );
      if (["bottom", "trump_fight", "play_trick"].includes(s.phase))
        button(
          s.surrenderVotes[s.mySeat] ? "撤回弃局" : "申请弃局",
          802,
          152,
          134,
          () => {
            const cancel = s.surrenderVotes[s.mySeat];
            wx.showModal({
              title: cancel
                ? "撤回弃局"
                : partnerVote
                  ? "确认与队友一起弃局？"
                  : "申请弃局",
              content: `当前打家已得 ${s.points} 分。${partnerVote ? "你确认后将立即结束本局。" : "同队两人同意后结束本局。"}仅计算已结算分数，不额外抠底。`,
              success: (r) => {
                if (r.confirm)
                  send(cancel ? "game.cancel_surrender" : "game.surrender");
              },
            });
          },
          !busy,
          "quiet",
        );
    }
    if (s.phase === "play_trick")
      button(
        previous ? "回当前墩" : "看上一墩",
        802,
        206,
        134,
        () => {
          previous = !previous;
          draw();
        },
        previous || s.trick.lastMoves.some(Boolean),
        "quiet",
      );
    if (["round_settle", "game_over"].includes(s.phase))
      drawSettlement(s, busy);
  }
  text(
    messageTone === "error" ? "" : message,
    28,
    533,
    11,
    messageTone === "error" ? "#a54747" : "#74708d",
    900,
  );
  targets.push({
    x: 24,
    y: 526,
    w: 912,
    h: 18,
    action: () => {
      panel = "messages";
      draw();
    },
  });
  drawPanel();
  if (Date.now() < floatUntil && floatLines.length) {
    box(260, 142, 440, floatLines.length * 30 + 16, floatError ? "#fff0ed" : "#403569");
    floatLines.forEach((line, i) => text(line, 278, 166 + i * 30, 18, floatError ? "#b14539" : "#ffffff", 404));
  }
}
function resize() {
  const info = wx.getSystemInfoSync();
  width = info.windowWidth;
  height = info.windowHeight;
  const dpr = info.pixelRatio || 1;
  pixelRatio = dpr;
  canvas.width = width * dpr;
  canvas.height = height * dpr;
  let capsule: WechatMinigame.ClientRect | undefined;
  try {
    capsule = wx.getMenuButtonBoundingClientRect?.();
  } catch {
    /* Older runtimes may not expose capsule bounds. */
  }
  const layout = viewport(width, height, info.safeArea, capsule);
  scale = layout.scale;
  offsetX = layout.x;
  offsetY = layout.y;
  draw();
}
function hit(x: number, y: number) {
  return [...targets]
    .reverse()
    .find((t) => x >= t.x && x <= t.x + t.w && y >= t.y && y <= t.y + t.h);
}
function point(t: WechatMinigame.Touch) {
  return { x: (t.clientX - offsetX) / scale, y: (t.clientY - offsetY) / scale };
}
wx.onTouchStart((e) => {
  if (!visible) return;
  const t = e.touches[0];
  if (!t) return;
  audio.unlock();
  const p = point(t);
  const target = hit(p.x, p.y);
  if (!target) return;
  audio.play(target.card !== undefined ? "select" : "tap");
  if (target.card !== undefined) {
    dragging = true;
    paint = !selected.has(target.card);
    touched.clear();
    touched.add(target.card);
  }
  target.action();
  if (target.label) {
    pressed = target.label;
    if (!panel) motion.start("press", 140);
    draw();
  }
});
wx.onTouchMove((e) => {
  if (!dragging) return;
  const t = e.touches[0];
  if (!t) return;
  const p = point(t);
  const target = hit(p.x, p.y);
  if (target?.card !== undefined && !touched.has(target.card)) {
    touched.add(target.card);
    if (paint) selected.add(target.card);
    else selected.delete(target.card);
    motion.start(`select:${target.card}`, 110);
    audio.play("select");
    draw();
  }
});
wx.onTouchEnd(() => {
  dragging = false;
});
wx.onTouchCancel(() => {
  dragging = false;
});
wx.onShow((options) => {
  feedback.reset();
  visible = true;
  motion.resume();
  audio.resume();
  if (options.query?.room && !view) {
    room = options.query.room;
    draw();
  }
  link.resume();
  draw();
});
wx.onHide(() => {
  floatLines = []; clearTimeout(floatTimer);
  dragging = false;
  visible = false;
  motion.suspend();
  audio.suspend();
  pressed = null;
  feedback.reset();
});
wx.onWindowResize(resize);
wx.showShareMenu({ menus: ["shareAppMessage"] });
wx.onShareAppMessage(() => ({
  title: "一起来打升级，四人好友房",
  query: `room=${encodeURIComponent(view?.roomId || room)}`,
}));
resize();
