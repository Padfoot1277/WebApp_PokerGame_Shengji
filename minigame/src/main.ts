import { isTrumpCard } from "../../frontend/src/utils/card";
import {
  unavailableCard,
  floatingFeedback,
} from "../../frontend/src/utils/play-feedback";
import {
  showingLast,
  moveTag,
  seatOffsets,
  lobbySeatOrder,
} from "../../frontend/src/utils/table";
import { GameConnection } from "./connection";
import { guide, relative, seatName, seatRole, teamName, turn } from "./model";
import { helpPages } from "./help";
import { selectionError } from "./selection";
import { viewport, portraitLayout } from "./viewport";
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
let width = 390,
  height = 844,
  logicalHeight = 844,
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
    floatError = true;
    floatLines = wrap(value, 404).slice(0, 3);
    floatUntil = Date.now() + 3200;
    clearTimeout(floatTimer);
    floatTimer = setTimeout(() => {
      floatLines = [];
      floatError = false;
      draw();
    }, 3200);
  }
  if (messages[messages.length - 1]?.text !== value)
    messages.push({ text: value, tone });
  if (messages.length > 8) messages.shift();
}
const localServer = "ws://192.168.124.19:8080/ws";
const savedServer = wx.getStorageSync("shengji-server");
let server = savedServer || localServer;
if (server !== savedServer) wx.setStorageSync("shengji-server", server);
let room = wx.getStorageSync("shengji-room") || "好友房";
let nickname = wx.getStorageSync("shengji-name") || "牌友";
const selected = new Set<number>();
let previous = false,
  lastHand = "",
  handScroll = 0,
  handScrollMax = 0,
  panelScroll = 0,
  panelScrollMax = 0;
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
        handScroll = 0;
        gesture = null;
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
      if (
        floating.length &&
        visible &&
        !(floatError && Date.now() < floatUntil)
      ) {
        floatError = false;
        floatLines = floating;
        floatUntil = Date.now() + 2200;
        clearTimeout(floatTimer);
        floatTimer = setTimeout(() => {
          floatLines = [];
          draw();
        }, 2200);
      }
      applyFeedback(feedback.observe(next));
    } else if (msg.type === "ack") {
      if (!msg.ok) notify(msg.message, "error");
      else if (messageTone === "error") notify("操作已确认");
    } else if (msg.type === "notice" || msg.type === "error")
      notify(msg.message, msg.type === "error" ? "error" : "info");
    else if (msg.type === "hello") {
      floatBaseline = true;
      floatLines = [];
      clearTimeout(floatTimer);
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
  color = "#20382d",
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
      ? "#23785f"
      : tone === "sunny"
        ? "#ffcc70"
        : tone === "field"
          ? "#ffffff"
          : "#e9f0e9";
  const pressKey = `${x}:${y}:${label}`;
  const down = pressed === pressKey && motion.active("press");
  if (enabled && tone !== "field")
    box(
      x,
      y + 4,
      w,
      44,
      tone === "primary" ? "#b8d4c6" : tone === "sunny" ? "#e4ad55" : "#d5e2d8",
    );
  box(x, y + (down ? 3 : 0), w, 44, fill);
  ctx.font = "16px sans-serif";
  const labelX =
    tone === "field" || w > 300
      ? x + 14
      : x + Math.max(14, (w - ctx.measureText(label).width) / 2);
  text(
    label,
    labelX,
    y + 22 + (down ? 3 : 0),
    16,
    !enabled ? "#a49baa" : tone === "primary" ? "#ffffff" : "#315b48",
    w - 28,
  );
  if (enabled) targets.push({ x, y, w, h: 44, action, label: pressKey });
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
    seat % 2 ? "#c73e61" : "#23785f",
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
  const disabled =
    interactive &&
    (previous || link.busy || unavailableCard(view, c, [...selected]));
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
  ctx.strokeStyle = "#d7e2d9";
  ctx.lineWidth = 1;
  ctx.stroke();
  if (isTrumpCard(c, view?.trump)) {
    ctx.strokeStyle = "#ffe033";
    ctx.lineWidth = 2;
    ctx.stroke();
  }
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
    disabled && !picked
      ? red
        ? "#c78996"
        : "#88838f"
      : red
        ? "#dc4965"
        : "#20382d",
    w - 6,
  );
  text(
    c.rank.replace("大王", "王").replace("小王", "王"),
    x + 4,
    top + (h <= 44 ? 27 : 40),
    h <= 44 ? 18 : 28,
    disabled && !picked
      ? red
        ? "#c78996"
        : "#88838f"
      : red
        ? "#dc4965"
        : "#20382d",
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
function openPanel(next: typeof panel) {
  panel = next;
  panelScroll = 0;
  draw();
}
function paragraph(
  value: string,
  x: number,
  y: number,
  max = 338,
  size = 14,
  color = "#65796f",
) {
  const lines = wrap(value, max, size);
  lines.forEach((line, i) =>
    text(line, x, y + i * (size + 7), size, color, max),
  );
  return lines.length * (size + 7);
}
function confirmSurrender(s: ViewState) {
  const cancel = s.surrenderVotes[s.mySeat];
  wx.showModal({
    title: cancel ? "撤回弃局" : "申请弃局",
    content: `当前打家已得 ${s.points} 分。同队两人同意后结束本局，仅计算已结算分数，不额外抠底。`,
    success: (r) => {
      if (r.confirm) {
        panel = null;
        send(cancel ? "game.cancel_surrender" : "game.surrender");
      }
    },
  });
}
function drawPanel() {
  if (!panel) return;
  targets = [];
  const h = logicalHeight,
    bottom = h - 82,
    clipTop = 154;
  box(0, 0, 390, h, "rgba(15,35,28,0.56)");
  box(12, 94, 366, h - 108, "#ffffff");
  const titles = {
    rules: "游戏规则",
    room: "房间菜单",
    connection: "连接设置",
    move: "出牌详情",
    messages: "牌桌消息",
    trick: "末墩回放",
    record: "公开记牌",
    preferences: "声音与动效",
  };
  text(titles[panel], 26, 121, 21, "#20382d", 244);
  button("关闭", 288, 102, 76, () => openPanel(null), true, "quiet");
  const startTargets = targets.length;
  ctx.save();
  ctx.beginPath();
  ctx.rect(22, clipTop, 346, bottom - clipTop);
  ctx.clip();
  let y = 170 - panelScroll;
  if (panel === "rules") {
    const page = helpPages[helpPage]!;
    text(page.title, 26, y, 18, "#20382d", 338);
    y += 32;
    for (const line of page.lines)
      y += paragraph("• " + line, 26, y, 338, 14) + 14;
  } else if (panel === "preferences") {
    for (const [key, title, detail] of [
      ["sound", "轻快音效", "点选、出牌和轮到你；遵循系统静音"],
      ["vibration", "回合轻震", "只在轮到你时提醒，不随选牌振动"],
      ["reducedMotion", "减少动态效果", "关闭位移与光圈，保留静态提示"],
    ] as const) {
      text(title, 26, y, 17);
      y += 26;
      y += paragraph(detail, 26, y, 338, 13);
      button(
        key === "reducedMotion"
          ? preferences[key]
            ? "已减少"
            : "标准动态"
          : preferences[key]
            ? "已开启"
            : "已关闭",
        26,
        y,
        338,
        () => togglePreference(key),
        true,
        preferences[key] ? "primary" : "quiet",
      );
      y += 62;
    }
    button(
      "试听音效",
      26,
      y,
      338,
      () => audio.play("turn"),
      preferences.sound,
      "quiet",
    );
    y += 62;
    y += paragraph("本机保存 · 音效遵循系统静音设置", 26, y);
  } else if (panel === "room") {
    text(view?.roomId || room, 26, y, 28, "#23785f", 338);
    y += 36;
    y += paragraph("对家是队友 · 四人准备后自动开局", 26, y) + 16;
    button("复制房间码", 26, y, 160, copyRoom);
    button("邀请好友", 202, y, 162, () =>
      wx.shareAppMessage({
        title: "一起来打升级",
        query: `room=${encodeURIComponent(view?.roomId || room)}`,
      }),
    );
    y += 58;
    for (const [label, next] of [
      ["消息记录", "messages"],
      ["声音与动效", "preferences"],
      ["连接设置", "connection"],
    ] as const) {
      button(label, 26, y, 338, () => openPanel(next), true, "quiet");
      y += 52;
    }
    if (view && !view.hideRecord) {
      button(
        "公开记牌",
        26,
        y,
        338,
        () => openPanel("record"),
        view.phase !== "lobby",
        "quiet",
      );
      y += 52;
    }
    if (
      view &&
      view.mySeat >= 0 &&
      ["bottom", "trump_fight", "play_trick"].includes(view.phase)
    ) {
      const s = view;
      button(
        s.surrenderVotes[s.mySeat] ? "撤回弃局" : "申请弃局",
        26,
        y,
        338,
        () => confirmSurrender(s),
        !link.busy,
      );
      y += 56;
    }
    y += paragraph("分享后以房间里实际入座的玩家为准。", 26, y);
  } else if (panel === "connection") {
    text("服务器地址", 26, y, 16);
    y += 20;
    button(
      server,
      26,
      y,
      338,
      () =>
        edit("服务器地址", server, (v) => {
          server = v;
          wx.setStorageSync("shengji-server", v);
        }),
      true,
      "field",
    );
    y += 66;
    y += paragraph(
      "本地体验需与电脑连接同一 Wi-Fi，并保持后端运行。异地连接需配置可访问的 WSS 地址。",
      26,
      y,
    );
  } else if (panel === "messages") {
    if (!messages.length)
      y += paragraph("暂无消息，当前操作提示显示在牌桌上方。", 26, y);
    for (const m of messages) {
      y +=
        paragraph(
          m.text,
          26,
          y,
          338,
          14,
          m.tone === "error" ? "#b14539" : "#315b48",
        ) + 22;
    }
  } else if (panel === "record") {
    const s = view;
    if (!s || s.hideRecord || !s.record)
      y += paragraph("本局未开放记牌信息", 26, y);
    else {
      y += paragraph("仅统计已结算的公开出牌。", 26, y) + 18;
      text("花色", 26, y, 14);
      text("张数 / A / K / 10", 152, y, 14);
      y += 34;
      for (const [label, key] of [
        ["黑桃", "spade"],
        ["红桃", "heart"],
        ["梅花", "club"],
        ["方块", "diamond"],
      ] as const) {
        const r = s.record[key];
        text(label, 26, y, 16);
        text(`${r.num}  /  ${r.a}  /  ${r.k}  /  ${r.ten}`, 152, y, 16);
        y += 44;
      }
      y += paragraph(
        `大王已出 ${s.record.bigJoker} 张 · 小王已出 ${s.record.smallJoker} 张`,
        26,
        y,
      );
    }
  } else if (panel === "move" && inspectedMove) {
    text(
      `${relative(inspectedMove.seat, view?.mySeat ?? -1)} · ${inspectedMove.followMove.cards.length} 张`,
      26,
      y,
      16,
    );
    y += 30;
    inspectedMove.followMove.cards.forEach((c, i) =>
      card(c, 26 + (i % 7) * 48, y + Math.floor(i / 7) * 74, 42, 64),
    );
    y += Math.ceil(inspectedMove.followMove.cards.length / 7) * 74 + 14;
    y += paragraph(inspectedMove.info || "本次实际出牌，服务器已确认", 26, y);
  } else if (panel === "trick") {
    (view?.trick.lastMoves ?? []).forEach((move, seat) => {
      text(
        `${relative(seat, view?.mySeat ?? -1)} · ${view?.seats[seat]?.name || "牌友"}${view?.trick.winnerSeat === seat ? " · 赢墩" : ""}`,
        26,
        y,
        15,
      );
      y += 20;
      if (move) {
        move.followMove.cards
          .slice(0, 7)
          .forEach((c, i) => card(c, 26 + i * 48, y, 42, 60));
        y += 72;
        button(
          "查看完整出牌",
          26,
          y,
          338,
          () => {
            inspectedMove = move;
            openPanel("move");
          },
          true,
          "quiet",
        );
        y += 56;
      } else {
        y += paragraph("本墩尚未出牌", 26, y) + 16;
      }
    });
  }
  ctx.restore();
  panelScrollMax = Math.max(
    0,
    y + panelScroll - clipTop - (bottom - clipTop) + 20,
  );
  panelScroll = Math.min(panelScroll, panelScrollMax);
  targets = targets.filter(
    (t, i) => i < startTargets || (t.y >= clipTop && t.y + t.h <= bottom),
  );
  if (panelScrollMax > 0) text("上下滑动查看更多", 26, 146, 12, "#65796f", 210);
  if (panel === "rules") {
    button(
      "上一页",
      26,
      h - 70,
      100,
      () => {
        helpPage--;
        panelScroll = 0;
        draw();
      },
      helpPage > 0,
      "quiet",
    );
    text(`${helpPage + 1} / ${helpPages.length}`, 174, h - 50, 13);
    button(
      "下一页",
      264,
      h - 70,
      100,
      () => {
        helpPage++;
        panelScroll = 0;
        draw();
      },
      helpPage < helpPages.length - 1,
    );
  }
}
function drawSeat(
  s: ViewState,
  seat: number,
  x: number,
  y: number,
  lobby: boolean,
) {
  const p = s.seats[seat]!,
    last = showingLast(s, previous),
    shown = last ? s.trick.lastMoves : s.trick.playedMoves,
    move = shown[seat];
  const active = !previous && seat === turn(s),
    big = !!move && seat === (last ? s.trick.winnerSeat : s.trick.biggerSeat);
  box(
    x,
    y,
    177,
    lobby ? 114 : 100,
    big ? "#e7f1d4" : active ? "#fff0cd" : "#ffffff",
  );
  if (active) {
    ctx.strokeStyle = "#c59a46";
    ctx.lineWidth = 2;
    ctx.stroke();
  }
  avatar(seat, x + 16, y + 17);
  text(
    `${p.name || "等待入座"}${p.isBot ? " · 机器人" : ""}`,
    x + 33,
    y + 17,
    14,
    "#20382d",
    136,
  );
  text(
    `${lobby ? seatName(seat) : relative(seat, s.mySeat)} · ${seatRole(s, seat)}${p.uid && !p.online ? " · 离线" : ""}`,
    x + 10,
    y + 38,
    11,
    "#65796f",
    158,
  );
  if (lobby) {
    text(
      p.uid ? (p.ready ? "已准备" : p.online ? "未准备" : "暂时离线") : "空位",
      x + 10,
      y + 58,
      12,
      p.ready ? "#23785f" : "#65796f",
    );
    if (!p.uid)
      button(
        "入座",
        x + 10,
        y + 70,
        157,
        () => send("room.sit", { seat }),
        !link.busy,
      );
    if (seat === s.mySeat) {
      button(
        p.ready ? "取消准备" : "准备",
        x + 10,
        y + 70,
        88,
        () => send(p.ready ? "room.unready" : "room.ready"),
        !link.busy,
      );
      button(
        "离座",
        x + 104,
        y + 70,
        63,
        () => send("room.leave_seat"),
        !link.busy,
        "quiet",
      );
    }
  } else if (move) {
    const arrival = previous
      ? 0
      : 6 * (1 - easeOut(motion.progress(`move:${seat}`)));
    move.followMove.cards
      .slice(0, 5)
      .forEach((c, i) =>
        card(c, x + 10 + i * 31, y + 44, 28, 40, false, arrival),
      );
    const tags = [
      big ? "最大" : "",
      seat === (last ? s.trick.lastLeaderSeat : s.trick.leaderSeat)
        ? "先手"
        : "",
      moveTag(move)?.label,
      move.followMove.cards.length > 5
        ? `共${move.followMove.cards.length}张`
        : "",
    ].filter(Boolean);
    text(tags.join(" · "), x + 10, y + 92, 10, "#23785f", 158);
    targets.push({
      x,
      y,
      w: 177,
      h: 100,
      action: () => {
        inspectedMove = move;
        openPanel("move");
      },
    });
  } else {
    text(
      (s.phase === "call_trump" && s.callPassedSeats[seat]) ||
        (s.phase === "trump_fight" && s.fightPassedSeats[seat])
        ? "已跳过"
        : active
          ? "正在操作…"
          : "等待出牌",
      x + 10,
      y + 67,
      13,
      "#65796f",
      158,
    );
  }
}
function drawSettlement(s: ViewState, busy: boolean) {
  if (motion.active("celebrate")) {
    ctx.save();
    ctx.globalAlpha = 1 - motion.progress("celebrate");
    sparkle(350, 148, "#c59a46");
    ctx.restore();
  }
  const over = s.phase === "game_over",
    y = 114;
  box(12, y, 366, logicalHeight - 240, "#ffffff");
  text(
    over
      ? `${teamName(s.winnerTeam)}升 A 获胜`
      : `第 ${s.roundIndex + 1} 局结算`,
    26,
    y + 28,
    23,
    "#20382d",
    338,
  );
  text(s.surrendered ? "弃局结算" : "本局已结束", 26, y + 58, 13);
  text(String(s.roundPointsFinal), 26, y + 110, 46, "#23785f");
  text("打家最终得分", 152, y + 111, 15);
  text(s.roundResultLabel, 26, y + 160, 22, "#23785f", 338);
  text(
    `坐家升 ${s.callerDelta} 级 · 打家升 ${s.defenderDelta} 级`,
    26,
    y + 192,
    16,
  );
  const mine = myUpgrade(s);
  if (mine > 0) text(`我方升级 +${mine}`, 26, y + 226, 20, "#23785f", 300);
  paragraph(
    s.surrendered
      ? "同队确认弃局 · 未完成墩不计分，不额外抠底"
      : `底牌 ${s.bottomPoints} 分 · 倍率 ×${s.bottomMul} · 抠底实得 ${s.bottomAward} 分`,
    26,
    y + 262,
    338,
    13,
  );
  (s.bottomReveal ?? []).forEach((c, i) =>
    card(c, 26 + i * 42, y + 302, 37, 56),
  );
  button(
    s.surrendered ? "查看最近完整墩" : "查看末墩",
    26,
    logicalHeight - 166,
    338,
    () => openPanel("trick"),
    s.trick.lastMoves.some(Boolean),
    "quiet",
  );
  if (over) button("返回大厅", 12, logicalHeight - 104, 366, leaveRoom, !busy);
  else if (s.mySeat === s.nextStarterSeat)
    button(
      "下一局",
      12,
      logicalHeight - 104,
      366,
      () => send("game.start_next_round"),
      !busy,
    );
  else
    text(
      `等待${relative(s.nextStarterSeat, s.mySeat)}「${s.seats[s.nextStarterSeat]?.name || "牌友"}」开启下一局`,
      22,
      logicalHeight - 83,
      15,
      "#65796f",
      346,
    );
}
function drawActions(s: ViewState, busy: boolean) {
  const y = portraitLayout(logicalHeight).footer;
  box(0, y - 4, 390, 114, "#ffffff");
  const own = turn(s) === s.mySeat,
    actions: { label: string; run: () => void }[] = [];
  if (s.mySeat >= 0) {
    if (
      s.phase === "call_trump" &&
      !s.callPassedSeats[s.mySeat] &&
      (s.callMode === "race" || own)
    )
      actions.push(
        { label: "定主", run: () => submit("game.call_trump") },
        { label: "跳过", run: () => send("game.call_pass") },
      );
    if (s.phase === "bottom" && own)
      actions.push({
        label: "扣底 8 张",
        run: () => submit("game.put_bottom"),
      });
    if (s.phase === "trump_fight" && own) {
      if (!s.trump.locked)
        actions.push({ label: "改主", run: () => submit("game.change_trump") });
      actions.push(
        { label: "攻主", run: () => submit("game.attack_trump") },
        { label: "跳过", run: () => send("game.call_pass") },
      );
    }
    if (s.phase === "play_trick" && own)
      actions.push({ label: "出牌", run: () => submit("game.play_cards") });
  }
  if (actions.length) {
    const w = (366 - 8 * (actions.length - 1)) / actions.length;
    actions.forEach((a, i) =>
      button(
        a.label,
        12 + i * (w + 8),
        y,
        w,
        a.run,
        !busy && !previous,
        a.label === "跳过" ? "quiet" : "primary",
      ),
    );
  } else
    text(
      previous
        ? "回放中 · 回当前墩后继续操作"
        : busy
          ? "正在恢复连接或确认操作…"
          : s.mySeat < 0
            ? "正在观战 · 不显示玩家手牌"
            : "等待其他牌友操作",
      20,
      y + 20,
      14,
      "#65796f",
      350,
    );
  const hasHand = (s.myHand ?? []).flat().length > 0;
  button(
    "选对子",
    12,
    y + 50,
    116,
    selectPair,
    hasHand && !busy && !previous,
    "quiet",
  );
  button(
    "清空选牌",
    136,
    y + 50,
    116,
    () => {
      selected.clear();
      draw();
    },
    hasHand && !busy && !previous,
    "quiet",
  );
  if (s.phase === "play_trick")
    button(
      previous ? "回当前墩" : "看上一墩",
      260,
      y + 50,
      118,
      () => {
        previous = !previous;
        draw();
      },
      previous || s.trick.lastMoves.some(Boolean),
      "quiet",
    );
  else if (s.phase === "bottom" && own)
    button(
      "查看原底牌",
      260,
      y + 50,
      118,
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
  else
    button(
      "对局消息",
      260,
      y + 50,
      118,
      () => openPanel("messages"),
      true,
      "quiet",
    );
}
function draw() {
  if (!visible) return;
  targets = [];
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#f4f6f2";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.setTransform(
    scale * pixelRatio,
    0,
    0,
    scale * pixelRatio,
    offsetX * pixelRatio,
    offsetY * pixelRatio,
  );
  box(0, 0, 390, logicalHeight, "#f4f6f2");
  text("升级 · 好友房", 14, 22, 21);
  text(
    `${link.status}${view ? ` · 房间 ${view.roomId}` : " · 四人搭档 / 两副扑克"}`,
    14,
    43,
    12,
    "#65796f",
    360,
  );
  button("规则", 12, 56, 116, () => openPanel("rules"), true, "quiet");
  button(
    view ? "菜单" : "声音与动效",
    136,
    56,
    116,
    () => openPanel(view ? "room" : "preferences"),
    true,
    "quiet",
  );
  if (view) button("返回大厅", 260, 56, 118, leaveRoom, true, "quiet");
  else
    button(
      "连接设置",
      260,
      56,
      118,
      () => openPanel("connection"),
      true,
      "quiet",
    );
  if (!view) {
    box(12, 114, 366, 130, "#23785f");
    text("好牌，一起打。", 28, 154, 29, "#ffffff", 330);
    text("和朋友同桌，也可以添加机器人。", 28, 192, 16, "#e7f0e9", 330);
    text("对家是队友 · 四人准备即开局", 28, 222, 13, "#e7f0e9", 330);
    box(12, 258, 366, 292, "#ffffff");
    text("开始一桌", 26, 285, 22);
    for (const [label, value, y, save] of [
      ["昵称", nickname, 316, (v: string) => (nickname = v)],
      ["房间码", room, 392, (v: string) => (room = v)],
    ] as const) {
      text(label, 26, y, 13, "#65796f");
      button(
        value,
        26,
        y + 16,
        338,
        () => edit(label, value, save),
        true,
        "field",
      );
    }
    button("创建房间", 26, 476, 160, () => connect(true));
    button("加入房间", 202, 476, 162, () => connect());
    button(
      `新房记牌器：${createRecord ? "开启" : "关闭"}`,
      12,
      566,
      366,
      () => {
        createRecord = !createRecord;
        draw();
      },
      true,
      "quiet",
    );
    text("输入好友的房间码，加入同一桌。", 18, 626, 13, "#65796f", 354);
  } else {
    const s = view,
      busy = link.busy || link.status !== "已连接",
      lobby = s.phase === "lobby";
    box(12, 108, 366, 44, "#ffffff");
    text(
      `第 ${s.roundIndex + 1} 局 · ${s.mySeat < 0 ? "观战" : `我方${s.teams[s.mySeat % 2]?.levelRank}级 / 对方${s.teams[(s.mySeat + 1) % 2]?.levelRank}级`}`,
      24,
      123,
      12,
      "#65796f",
      260,
    );
    text(`打家 ${s.points} 分`, 24, 142, 16, "#23785f", 160);
    text(
      lobby
        ? "等待准备"
        : s.phase === "call_trump"
          ? "待定主"
          : `${s.trump.hasTrumpSuit ? s.trump.suit : "硬主"} ${s.trump.levelRank}${s.trump.locked ? " · 锁主" : ""}`,
      226,
      137,
      14,
      "#23785f",
      140,
    );
    if (["round_settle", "game_over"].includes(s.phase))
      drawSettlement(s, busy);
    else {
      box(12, 160, 366, 30, "#e7f0e9");
      text(
        previous ? "上一墩回放 · 选牌与出牌已暂停" : guide(s),
        22,
        175,
        12,
        "#315b48",
        346,
      );
      const base = Math.max(0, s.mySeat);
      const seatOrder = lobby
        ? lobbySeatOrder
        : seatOffsets.map((off) => (base + off) % 4);
      seatOrder.forEach((seat, i) =>
        drawSeat(
          s,
          seat,
          12 + (i % 2) * 189,
          196 + Math.floor(i / 2) * (lobby ? 122 : 108),
          lobby,
        ),
      );
      if (lobby) {
        text(
          `已入座 ${s.seats.filter((p) => p.uid).length}/4 · 已准备 ${s.seats.filter((p) => p.uid && p.ready).length}/4`,
          18,
          461,
          15,
          "#65796f",
          354,
        );
        button(
          "增加机器人",
          12,
          482,
          177,
          () => send("room.add_bot"),
          !busy && s.seats.some((p) => !p.uid),
        );
        button(
          "减少机器人",
          201,
          482,
          177,
          () => send("room.remove_bot"),
          !busy && s.seats.some((p) => p.isBot),
          "quiet",
        );
        button("复制房间码", 12, 538, 177, copyRoom);
        button(
          "邀请好友",
          201,
          538,
          177,
          () => openPanel("room"),
          true,
          "quiet",
        );
        text("顺时针：A1 → B1 → A2 → B2 → A1", 18, 604, 13, "#65796f", 354);
      } else {
        const cards = (s.myHand ?? []).flat(),
          layout = portraitLayout(logicalHeight),
          hand = handLayout(
            cards.length,
            444,
            (s.myHand ?? []).map((g) => g.length),
          );
        text(s.mySeat < 0 ? "观战模式" : "我的手牌", 18, 425, 17);
        text(
          `已选 ${selected.size} / ${cards.length}${hand.contentHeight > layout.handBottom - layout.handTop ? " · 上下滑动" : ""}`,
          146,
          425,
          12,
          "#65796f",
          228,
        );
        handScrollMax = Math.max(
          0,
          hand.contentHeight - (layout.handBottom - layout.handTop),
        );
        handScroll = Math.min(handScroll, handScrollMax);
        box(12, 438, 366, Math.max(30, layout.handBottom - 438), "#e9f0e9");
        const start = targets.length;
        ctx.save();
        ctx.beginPath();
        ctx.rect(12, layout.handTop, 366, layout.handBottom - layout.handTop);
        ctx.clip();
        hand.headers.forEach((header, i) =>
          text(
            (s.myHand ?? [])[i]?.[0]?.suit_class || "手牌",
            20,
            header.y - handScroll,
            11,
            "#65796f",
            340,
          ),
        );
        cards.forEach((c, i) => {
          const slot = hand.slot(i),
            y = slot.y - handScroll;
          if (y + slot.h < layout.handTop || y - 12 > layout.handBottom) return;
          card(c, slot.x, y, slot.w, slot.h, true);
        });
        ctx.restore();
        targets = targets
          .filter(
            (t, i) =>
              i < start ||
              (t.y + t.h > layout.handTop && t.y < layout.handBottom),
          )
          .map((t, i) =>
            i < start
              ? t
              : {
                  ...t,
                  y: Math.max(t.y, layout.handTop),
                  h:
                    Math.min(t.y + t.h, layout.handBottom) -
                    Math.max(t.y, layout.handTop),
                },
          );
        if (s.mySeat < 0)
          paragraph("仅显示公开出牌和积分，不显示玩家手牌。", 26, 470, 338, 14);
        if (handScrollMax > 0) {
          const track = layout.handBottom - layout.handTop;
          box(
            372,
            layout.handTop + (handScroll / handScrollMax) * (track - 24),
            3,
            24,
            "#8caf9e",
          );
        }
        drawActions(s, busy);
      }
    }
    if (link.status !== "已连接")
      button(
        "连接中断 · 点击重连",
        12,
        108,
        366,
        () => link.retryNow(),
        true,
        "quiet",
      );
  }
  if (!view || view.phase === "lobby")
    text(
      messageTone === "error" ? message : "随时可从菜单查看规则和消息",
      14,
      logicalHeight - 12,
      11,
      messageTone === "error" ? "#b14539" : "#65796f",
      362,
    );
  drawPanel();
  if (Date.now() < floatUntil && floatLines.length) {
    const lines = floatLines.flatMap((line) => wrap(line, 330, 15)).slice(0, 4);
    box(
      18,
      158,
      354,
      lines.length * 24 + 20,
      floatError ? "#fff0ed" : "#214c3b",
    );
    lines.forEach((line, i) =>
      text(line, 30, 180 + i * 24, 15, floatError ? "#b14539" : "#ffffff", 330),
    );
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
  logicalHeight = layout.height;
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
type Gesture = {
  x: number;
  y: number;
  target: Target | undefined;
  region: "hand" | "panel" | null;
  scroll: number;
  before: Set<number>;
  mode: "pending" | "scroll" | "select";
  paint: boolean;
};
let gesture: Gesture | null = null;
function restoreSelection(ids: Set<number>) {
  selected.clear();
  ids.forEach((id) => selected.add(id));
}
wx.onTouchStart((e) => {
  if (!visible || !e.touches[0]) return;
  audio.unlock();
  const p = point(e.touches[0]),
    layout = portraitLayout(logicalHeight);
  const region =
    panel && p.y >= 154 && p.y <= logicalHeight - 82
      ? "panel"
      : !panel &&
          view &&
          view.phase !== "lobby" &&
          p.y >= layout.handTop &&
          p.y <= layout.handBottom
        ? "hand"
        : null;
  const target = hit(p.x, p.y);
  gesture = {
    ...p,
    target,
    region,
    scroll: region === "panel" ? panelScroll : handScroll,
    before: new Set(selected),
    mode: "pending",
    paint: target?.card === undefined || !selected.has(target.card),
  };
  touched.clear();
  if (target?.label) {
    pressed = target.label;
    motion.start("press", 140);
    draw();
  }
});
wx.onTouchMove((e) => {
  const g = gesture;
  if (!g || !e.touches[0]) return;
  const p = point(e.touches[0]),
    dx = p.x - g.x,
    dy = p.y - g.y;
  if (g.region && Math.abs(dy) > 8 && Math.abs(dy) > Math.abs(dx) * 1.1) {
    if (g.mode === "select") restoreSelection(g.before);
    g.mode = "scroll";
  }
  if (!g.region && Math.hypot(dx, dy) > 8) g.mode = "scroll";
  if (g.mode === "scroll") {
    if (!g.region) return;
    if (g.region === "panel")
      panelScroll = Math.max(0, Math.min(panelScrollMax, g.scroll - dy));
    else handScroll = Math.max(0, Math.min(handScrollMax, g.scroll - dy));
    draw();
    return;
  }
  if (g.target?.card !== undefined && Math.abs(dx) > 8 && Math.abs(dy) < 20) {
    if (g.mode === "pending") {
      g.mode = "select";
      touched.add(g.target.card);
      if (g.paint) selected.add(g.target.card);
      else selected.delete(g.target.card);
    }
    const target = hit(p.x, p.y);
    if (target?.card !== undefined && !touched.has(target.card)) {
      touched.add(target.card);
      if (g.paint) selected.add(target.card);
      else selected.delete(target.card);
      motion.start(`select:${target.card}`, 110);
      audio.play("select");
    }
    draw();
  }
});
wx.onTouchEnd(() => {
  const g = gesture;
  gesture = null;
  if (!g) return;
  if (g.mode === "pending" && g.target) {
    audio.play(g.target.card !== undefined ? "select" : "tap");
    g.target.action();
  }
  pressed = null;
});
wx.onTouchCancel(() => {
  if (gesture?.mode === "select") restoreSelection(gesture.before);
  gesture = null;
  pressed = null;
  draw();
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
  floatLines = [];
  clearTimeout(floatTimer);
  gesture = null;
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
