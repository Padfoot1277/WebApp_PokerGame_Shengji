import { GameConnection } from "./connection";
import { guide, relative, seatName, turn } from "./model";
import { helpPages } from "./help";
import { selectionError, selectionSummary } from "./selection";
import { viewport } from "./viewport";
import type { Card, ViewState } from "../../frontend/src/types/view";

const canvas = wx.createCanvas();
const ctx = canvas.getContext("2d") as unknown as CanvasRenderingContext2D;
let width = 960,
  height = 540,
  scale = 1,
  offsetX = 0,
  offsetY = 0;
let view: ViewState | null = null;
let panel: "rules" | "room" | "connection" | null = null;
let helpPage = 0;
let message = "局域网开发体验 · 需与电脑连接同一 Wi-Fi";
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
type Target = {
  x: number;
  y: number;
  w: number;
  h: number;
  action: () => void;
  card?: number;
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
        view?.phase !== next.phase
      )
        previous = false;
      view = next;
    } else if (msg.type === "ack") {
      if (!msg.ok) message = msg.message;
    } else if (msg.type === "notice" || msg.type === "error")
      message = msg.message;
    else if (msg.type === "hello") {
      nickname = msg.name;
      wx.setStorageSync("shengji-name", nickname);
    }
  },
  () => draw(),
);
function text(
  value: string,
  x: number,
  y: number,
  size = 16,
  color = "#263747",
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
function button(
  label: string,
  x: number,
  y: number,
  w: number,
  action: () => void,
  enabled = true,
  tone: "primary" | "quiet" | "field" = "primary",
) {
  const fill = !enabled
    ? "#e0e5e8"
    : tone === "primary"
      ? "#276b74"
      : tone === "field"
        ? "#ffffff"
        : "#e7edef";
  box(x, y, w, 40, fill);
  text(
    label,
    x + 14,
    y + 20,
    16,
    !enabled ? "#89969f" : tone === "primary" ? "#ffffff" : "#344c5c",
    w - 28,
  );
  if (enabled) targets.push({ x, y, w, h: 40, action });
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
function connect(create = false) {
  if (!/^wss?:\/\/[^\s?#]+$/i.test(server)) {
    message = "地址需类似 ws://电脑IP:8080/ws";
    draw();
    return;
  }
  if (create) room = Math.random().toString(36).slice(2, 8).toUpperCase();
  room = room.trim();
  if (!room || [...room].length > 32) {
    message = "请输入 1–32 个字符的房间码";
    draw();
    return;
  }
  wx.setStorageSync("shengji-server", server);
  wx.setStorageSync("shengji-room", room);
  wx.setStorageSync("shengji-name", nickname);
  view = null;
  selected.clear();
  link.connect(server, room, nickname);
}
function leaveRoom() {
  const leave = () => {
    panel = null;
    previous = false;
    link.close();
    view = null;
    selected.clear();
    message = "座位已保留，加入原房间可继续";
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
      message = "房间码已复制，发给好友即可加入";
      draw();
    },
    fail: () => {
      message = "复制失败，请手动记下房间码";
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
    message = seed ? "这张牌没有同花色同点数的另一张" : "手牌中没有对子";
    draw();
    return;
  }
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
    text(page.title, 128, 111, 26, "#213a4b", 600);
    page.lines.forEach((line, i) =>
      text(line, 128, 170 + i * 34, 17, "#344c5c", 704),
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
    text(`${helpPage + 1} / ${helpPages.length}`, 449, 440, 16, "#647b88");
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
  } else if (panel === "room") {
    text("好友房", 128, 111, 26, "#213a4b");
    text(view?.roomId || room, 128, 180, 38, "#276b74", 650);
    text("对家是队友 · 四人准备后自动开局", 128, 235, 18, "#647b88");
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
      "#647b88",
    );
  } else {
    text("连接设置", 128, 111, 26, "#213a4b");
    text("服务器地址", 128, 178, 17, "#647b88");
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
      "#344c5c",
    );
    text("异地体验需要配置可访问的 WSS 服务地址。", 128, 328, 17, "#344c5c");
  }
}
function send(type: string, payload: unknown = {}) {
  if (!link.send(type, payload)) {
    message = link.busy ? "上一操作正在确认，请稍候" : "尚未连接，请检查网络";
  }
  draw();
}
function submit(type: string) {
  if (!view) return;
  const cards = (view?.myHand ?? []).flat().filter((c) => selected.has(c.id));
  const error = selectionError(type, cards, view);
  if (error) {
    message = error;
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
) {
  const picked = interactive && selected.has(c.id);
  const top = y - (picked ? 9 : 0);
  box(x, top, w, h, picked ? "#b9dfde" : "#ffffff");
  const red = /[♥♦]/.test(c.suit) || c.suit === "BJ";
  const suit =
    c.suit === "BJ"
      ? "大"
      : c.suit === "SJ"
        ? "小"
        : c.suit.replace(/\uFE0F/g, "");
  text(suit, x + 4, top + 12, 12, red ? "#bc3b48" : "#243c32", w - 6);
  text(
    c.rank.replace("大王", "王").replace("小王", "王"),
    x + 4,
    top + 33,
    17,
    red ? "#bc3b48" : "#243c32",
    w - 6,
  );
  if (interactive) {
    if (view?.usedTrumpIds.includes(c.id))
      text("已亮", x + 2, top + h - 5, 8, "#805a26", w - 4);
    if (!link.busy && !previous)
      targets.push({
        x,
        y: top,
        w,
        h,
        card: c.id,
        action: () => {
          if (selected.has(c.id)) selected.delete(c.id);
          else selected.add(c.id);
          draw();
        },
      });
  }
}
function draw() {
  if (!ctx) return;
  ctx.setTransform(1, 0, 0, 1, 0, 0);
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = "#eef1f2";
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  const dpr = wx.getSystemInfoSync().pixelRatio || 1;
  ctx.setTransform(
    dpr * scale,
    0,
    0,
    dpr * scale,
    dpr * offsetX,
    dpr * offsetY,
  );
  targets = [];
  box(0, 0, 960, 540, "#eef1f2");
  box(16, 14, 928, 48, "#ffffff");
  text("升 级 · 好友房", 34, 38, 23, "#213a4b");
  text(
    `${link.status}${view ? "  ·  房间 " + view.roomId : ""}`,
    258,
    38,
    15,
    "#647b88",
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
      "房间",
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
    box(28, 82, 330, 384, "#213e50");
    text("SHENGJI", 56, 122, 14, "#a8c9d0");
    text("好牌，", 56, 179, 42, "#ffffff");
    text("一起打。", 56, 233, 42, "#ffffff");
    text("四人搭档 · 两副扑克", 56, 299, 18, "#c9dce1");
    text("邀上好友，坐下就开局", 56, 331, 16, "#a8c9d0");
    text("♠   ♥   ♣   ♦", 56, 411, 30, "#c9dce1");
    box(378, 82, 554, 384, "#e3e9ec");
    text("开始一桌", 406, 118, 26, "#213a4b");
    for (const [label, value, y, save] of [
      ["昵称", nickname, 150, (v: string) => (nickname = v)],
      ["房间码", room, 212, (v: string) => (room = v)],
    ] as const) {
      text(label, 408, y + 20, 15, "#526c7b");
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
    text("首次使用可在此设置服务器", 674, 294, 14, "#647b88", 225);
    button("创建房间", 408, 344, 232, () => connect(true));
    button("加入房间", 656, 344, 248, () => connect());
    text(
      "点击上方信息可编辑 · 输入好友房间码即可加入",
      408,
      422,
      14,
      "#647b88",
      495,
    );
  } else {
    const s = view;
    const busy = link.busy || link.status !== "已连接";
    text(
      `第 ${s.roundIndex + 1} 局   东西队 ${s.teams[0]?.levelRank} 级 / 南北队 ${s.teams[1]?.levelRank} 级   打家 ${s.points} 分`,
      28,
      88,
      16,
    );
    const trump =
      s.phase === "call_trump"
        ? "待定主"
        : `${s.trump.hasTrumpSuit ? s.trump.suit : "硬主"} · 级牌 ${s.trump.levelRank}${s.trump.locked ? " · 已锁主" : ""}`;
    text(trump, 660, 88, 15, "#276b74", 270);
    box(24, 106, 912, 32, "#dce9eb");
    const partnerVote =
      s.mySeat >= 0 && s.surrenderVotes[(s.mySeat + 2) % 4] && !s.surrendered;
    text(
      previous
        ? `上一墩回放 · ${seatName(s.trick.winnerSeat)}赢墩 · 点击“回当前墩”继续操作`
        : `${partnerVote ? "队友已申请弃局 · " : ""}${guide(s)}`,
      38,
      122,
      15,
      "#27616c",
      880,
    );
    const base = Math.max(0, s.mySeat);
    const positions = [
      [330, 150, 300, 90],
      [24, 188, 278, 118],
      [658, 188, 278, 118],
      [330, 252, 300, 90],
    ];
    [2, 3, 1, 0].forEach((off, i) => {
      const seat = (base + off) % 4;
      const [x, y, w, h] = positions[i]!;
      const player = s.seats[seat]!;
      box(x!, y!, w!, h!, seat === turn(s) ? "#d7e9e8" : "#ffffff");
      text(
        `${relative(seat, s.mySeat)} · ${player.name || "空位"} · ${player.handCount}张${!player.online && player.uid ? " · 离线" : ""}`,
        x! + 8,
        y! + 15,
        16,
        "#263747",
        w! - 16,
      );
      if (s.phase === "lobby") {
        text(
          seatName(seat) + (player.ready ? " · 已准备" : " · 未准备"),
          x! + 8,
          y! + 38,
          13,
          "#70838e",
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
        const move = (previous ? s.trick.lastMoves : s.trick.playedMoves)[seat];
        if (move) {
          const step = Math.min(
            28,
            (w! - 50) / Math.max(1, move.followMove.cards.length),
          );
          move.followMove.cards.forEach((c, j) =>
            card(c, x! + 8 + j * step, y! + 28, 30, 44),
          );
        } else
          text(
            (s.phase === "call_trump" && s.callPassedSeats[seat]) ||
              (s.phase === "trump_fight" && s.fightPassedSeats[seat])
              ? "已跳过"
              : s.phase === "play_trick"
                ? seat === turn(s)
                  ? "轮到此席出牌"
                  : "尚未出牌"
                : "等待操作",
            x! + 8,
            y! + 42,
            12,
            "#70838e",
          );
      }
    });
    if (s.phase !== "lobby" && s.mySeat >= 0) {
      box(16, 350, 928, 120, "#e0e6e9");
      box(16, 488, 928, 48, "#ffffff");
      const groups = s.myHand ?? [];
      const count = groups.flat().length;
      if (s.phase === "bottom" && s.bottomOwnerSeat === s.mySeat) {
        button(
          "查看原底牌",
          658,
          308,
          156,
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
      if (s.phase === "round_settle" || s.phase === "game_over") {
        box(16, 350, 928, 120, "#ffffff");
        text(
          s.phase === "game_over" ? "整场结束" : "本局结算",
          34,
          375,
          21,
          "#213a4b",
        );
        text(
          `${s.roundPointsFinal} 分 · ${s.roundResultLabel}`,
          34,
          408,
          24,
          "#276b74",
          390,
        );
        text(
          s.surrendered
            ? "同队确认弃局"
            : `底牌 ${s.bottomPoints} 分 · 倍率 ×${s.bottomMul} · 计入 ${s.bottomAward} 分`,
          34,
          443,
          14,
          "#647b88",
          400,
        );
        text("公开底牌", 468, 373, 14, "#647b88");
        (s.bottomReveal ?? []).forEach((c, i) =>
          card(c, 468 + i * 54, 390, 46, 62),
        );
      }
      const step = Math.min(42, 850 / Math.max(1, count));
      let index = 0;
      if (count > 0) text("手牌 · 点选或滑动选牌", 32, 363, 14, "#647b88", 320);
      groups.forEach((group) => {
        group.forEach((c) => {
          card(c, 32 + index * step, 388, Math.min(44, step + 5), 80, true);
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
          190 + i * 128,
          492,
          116,
          a.run,
          !busy && !previous,
          a.label === "清空选牌" || a.label === "跳过" ? "quiet" : "primary",
        ),
      );
      if (count > 0)
        text(
          selectionSummary(
            groups.flat().filter((c) => selected.has(c.id)),
            s,
          ),
          400,
          363,
          14,
          "#276b74",
          520,
        );
      if (count > 0)
        button("选对子", 28, 492, 144, selectPair, !busy && !previous, "quiet");
      if (busy) text("正在确认…", 28, 513, 14, "#647b88", 150);
      if (["bottom", "trump_fight", "play_trick"].includes(s.phase))
        button(
          s.surrenderVotes[s.mySeat] ? "撤回弃局" : "申请弃局",
          827,
          492,
          110,
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
        818,
        308,
        118,
        () => {
          previous = !previous;
          draw();
        },
        s.trick.lastMoves.some(Boolean),
        "quiet",
      );
    if (s.phase === "round_settle" || s.phase === "game_over") {
      text(
        s.surrendered
          ? "同队已确认弃局 · 未完成墩不计分、不抠底"
          : `底牌 ${s.bottomPoints} 分 · 实得 ${s.bottomAward} 分`,
        28,
        327,
        15,
        "#276b74",
      );
    }
  }
  text(message, 28, 479, 12, "#647b88", 900);
  drawPanel();
}
function resize() {
  const info = wx.getSystemInfoSync();
  width = info.windowWidth;
  height = info.windowHeight;
  const dpr = info.pixelRatio || 1;
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
  const t = e.touches[0];
  if (!t) return;
  const p = point(t);
  const target = hit(p.x, p.y);
  if (!target) return;
  if (target.card !== undefined) {
    dragging = true;
    paint = !selected.has(target.card);
    touched.clear();
    touched.add(target.card);
  }
  target.action();
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
  if (options.query?.room && !view) {
    room = options.query.room;
    draw();
  }
  link.resume();
});
wx.onHide(() => {
  dragging = false;
});
wx.onWindowResize(resize);
wx.showShareMenu({ menus: ["shareAppMessage"] });
wx.onShareAppMessage(() => ({
  title: "一起来打升级，四人好友房",
  query: `room=${encodeURIComponent(view?.roomId || room)}`,
}));
resize();
