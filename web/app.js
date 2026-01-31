let ws = null;
let lastState = null;
let UI = null;
let lastPhase = null;
let lastVersion = null;
const selected = new Set();

const $ = (id) => document.getElementById(id);
const log = (s) => { $("log").textContent = (s + "\n") + $("log").textContent; };

function initUIRefs() {
    UI = {
        btnReady: $("btnReady"),
        btnUnready: $("btnUnready"),
        btnCallTrump: $("btnCallTrump"),
        btnPass: $("btnPass"),
        btnPutBottom: $("btnPutBottom"),
        btnStart: $("btnStart"),
    };
}

function setPrimary(btn, on) {
    if (!btn) return;
    if (on) btn.classList.add("primary");
    else btn.classList.remove("primary");
}

function setDisabled(btn, on) {
    if (!btn) return;
    btn.disabled = !!on;
}

function mySeatIndex(st) {
    if (!st) return -1;
    const uid = $("uid").value.trim();
    for (let i = 0; i < 4; i++) {
        if ((st.seats[i].uid || "") === uid) return i;
    }
    return -1;
}

function mySeatReady(st, seat) {
    if (!st || seat < 0) return false;
    return !!st.seats[seat].ready;
}

function updateActionAvailability(st) {
    if (!UI) return;

    // 默认全部不高亮（由阶段决定）
    setPrimary(UI.btnReady, false);
    setPrimary(UI.btnCallTrump, false);
    setPrimary(UI.btnPutBottom, false);

    // 未连接/未收到snapshot：全部禁用
    if (!st) {
        setDisabled(UI.btnReady, true);
        setDisabled(UI.btnUnready, true);
        setDisabled(UI.btnStart, true);

        setDisabled(UI.btnCallTrump, true);
        setDisabled(UI.btnPass, true);
        setDisabled(UI.btnPutBottom, true);
        return;
    }

    const seat = mySeatIndex(st);
    const seated = seat >= 0;
    const readyNow = mySeatReady(st, seat);

    // ---- lobby 阶段：可坐下/准备/Start；不可定主 ----
    if (st.phase === "lobby") {
        setDisabled(UI.btnReady, !seated || readyNow);
        setDisabled(UI.btnUnready, !seated || !readyNow);
        setDisabled(UI.btnStart, true); // 可选：你也可以允许手动 start：seated && allReady

        setPrimary(UI.btnReady, seated && !readyNow); // 需要你去准备时高亮
        setPrimary(UI.btnCallTrump, false);

        setDisabled(UI.btnCallTrump, true);
        setDisabled(UI.btnPass, true);
        setDisabled(UI.btnPutBottom, true);
        return;
    }

    // ---- call_trump 阶段 ----
    if (st.phase === "call_trump") {
        setDisabled(UI.btnReady, true);
        setDisabled(UI.btnUnready, true);
        setDisabled(UI.btnStart, true);

        const mySeat = mySeatIndex(st);
        const seated = mySeat >= 0;

        const alreadyPassed = st.callPassedSeats ? !!st.callPassedSeats[mySeat] : false;

        let canAct = false;
        if (st.callMode === "race") {
            // ✅ 抢定主：starter未确定前，所有坐下且未pass的人都能操作
            canAct = seated && (st.starterSeat < 0) && !alreadyPassed;
        } else {
            // ✅ 顺位定主：轮到你且未pass
            canAct = seated && (st.callTurnSeat === mySeat) && !alreadyPassed;
        }

        setDisabled(UI.btnCallTrump, !canAct);
        setDisabled(UI.btnPass, !canAct);

        setPrimary(UI.btnCallTrump, canAct);
        setDisabled(UI.btnPutBottom, true);
        return;
    }


    // ---- bottom 阶段：只有坐家可以扣底（后端未实现也先按逻辑做）----
    if (st.phase === "bottom") {
        const mySeat = mySeatIndex(st);
        const seated = mySeat >= 0;
        const isOwner = seated && (st.bottomOwnerSeat === mySeat);

        setDisabled(UI.btnPutBottom, !isOwner);
        setPrimary(UI.btnPutBottom, isOwner);

        setDisabled(UI.btnCallTrump, true);
        setDisabled(UI.btnPass, true);
        setDisabled(UI.btnReady, true);
        setDisabled(UI.btnUnready, true);
        setDisabled(UI.btnStart, true);
        return;
    }

    if (st.phase === "trump_fight") {
        const mySeat = mySeatIndex(st);
        const seated = mySeat >= 0;
        const isOwner = seated && (st.bottomOwnerSeat === mySeat);

        // 坐家不参与跳过；其余三人未pass才能点
        const already = st.fightPassedSeats ? !!st.fightPassedSeats[mySeat] : false;
        const canPass = seated && !isOwner && !already;

        setDisabled(UI.btnPass, !canPass);
        setPrimary(UI.btnPass, canPass);

        // 其余按钮先禁用（改主/攻主占位）
        setDisabled(UI.btnCallTrump, true);
        setDisabled(UI.btnPutBottom, true);
        setDisabled(UI.btnReady, true);
        setDisabled(UI.btnUnready, true);
        setDisabled(UI.btnStart, true);
        return;
    }

    // ---- 其他阶段：都禁用（后续做出牌再开放）----
    setDisabled(UI.btnReady, true);
    setDisabled(UI.btnUnready, true);
    setDisabled(UI.btnStart, true);

    setDisabled(UI.btnCallTrump, true);
    setDisabled(UI.btnPass, true);
    setDisabled(UI.btnPutBottom, true);
}

function wsStatusText() {
    if (!ws) return "null";
    switch (ws.readyState) {
        case 0: return "CONNECTING";
        case 1: return "OPEN";
        case 2: return "CLOSING";
        case 3: return "CLOSED";
        default: return String(ws.readyState);
    }
}

function setWSStatus() {
    $("stWS").textContent = wsStatusText();
}

function connect() {
    const uid = $("uid").value.trim();
    const room = $("room").value.trim();
    const url = `ws://${location.host}/ws?uid=${encodeURIComponent(uid)}&room=${encodeURIComponent(room)}`;

    ws = new WebSocket(url);
    setWSStatus();
    log(`[ui] connect -> ${url}`);

    ws.onopen = () => {
        setWSStatus();
        log("[ws] open");
    };
    ws.onclose = () => {
        setWSStatus();
        log("[ws] close");
    };
    ws.onerror = () => {
        setWSStatus();
        log("[ws] error");
    };
    // ws.onmessage = (e) => {
    //     // 1) 先把原始帧打印出来（非常关键）
    //     log(`[recv raw] ${e.data}`);
    //
    //     let msg = null;
    //     try { msg = JSON.parse(e.data); }
    //     catch { return; }
    //
    //     // 2) 兼容不同后端消息形状
    //     const t = msg.type || msg.t || msg.kind;
    //
    //     if (t === "snapshot" || msg.state) {
    //         lastState = msg.state || msg;
    //         renderAll(lastState);
    //         return;
    //     }
    //
    //     if (t === "error" || t === "err" || msg.code || msg.message || msg.msg) {
    //         const code = msg.code || "ERR";
    //         const message = msg.message || msg.msg || JSON.stringify(msg);
    //         log(`[error] ${code}: ${message}`);
    //         return;
    //     }
    //
    //     log(`[recv] ${JSON.stringify(msg)}`);
    // };

    ws.onmessage = (e) => {
        const msg = JSON.parse(e.data);
        if (msg.type === "snapshot") {
            const st = msg.state;

            // 只在阶段切换时清空勾选
            if (lastPhase !== null && st.phase !== lastPhase) {
                selected.clear();
                log(`[ui] phase changed ${lastPhase} -> ${st.phase}, clear selection`);
            }

            // 可选：记录 version 变化
            lastVersion = st.version;
            lastPhase = st.phase;
            lastState = st;
            renderAll(lastState);
        } else if (msg.type === "error") {
            log(`[error] ${msg.code}: ${msg.message}`);
        } else {
            log("[msg] " + e.data);
        }
    };
}

function disconnect() {
    log("[ui] disconnect");
    if (ws) ws.close();
    ws = null;
    lastState = null;
    selected.clear();
    renderAll(null);
    setWSStatus();
}

function send(type, payload) {
    if (!ws || ws.readyState !== 1) {
        log(`[send] blocked (ws=${wsStatusText()}) type=${type}`);
        return;
    }
    const msg = { type, payload };
    log(`[send] ${type} ${JSON.stringify(payload)}`);
    ws.send(JSON.stringify(msg));
}

// ===== lobby actions =====
function sit(seat) { send("room.sit", { seat }); }
function leaveSeat() { send("room.leave_seat", {}); }
function ready() { send("room.ready", {}); }
function unready() { send("room.unready", {}); }
function start() { send("game.start", {}); }

// ===== call trump actions =====
function callPass() { send("game.call_pass", {}); }

function actionCallTrump() {
    if (!lastState) return log("[ui] no snapshot yet");
    const st = lastState;
    const hand = st.myHand || [];

    const mySeat = findMySeatIndex(st);
    if (mySeat < 0) return log("你还没坐下（请先坐下）");

    const myTeam = st.seats[mySeat].team;
    const myLevelRank = st.teams[myTeam].levelRank;

    const selectedCards = hand.filter(c => selected.has(c.id));

    const joker = selectedCards.find(c => c.kind === "joker_big" || c.kind === "joker_small");
    if (!joker) return log("请选择一张王（大王或小王）");

    const levels = selectedCards.filter(c => c.kind === "normal" && c.rank === myLevelRank);
    if (levels.length !== 1 && levels.length !== 2) {
        return log(`请选择 1 或 2 张本队级牌（rank=${myLevelRank}）`);
    }

    send("game.call_trump", { jokerId: joker.id, levelIds: levels.map(x => x.id) });
}

function actionPutBottom() {
    if (!lastState) return;
    const st = lastState;

    if (st.phase !== "bottom") return log("当前不在扣底阶段");
    const mySeat = findMySeatIndex(st);
    if (mySeat < 0) return log("你还没坐下");
    if (st.bottomOwnerSeat !== mySeat) return log("你不是坐家，不能扣底");

    // 只能从右侧手牌区选牌
    const hand = st.myHand || [];
    const selectedIds = hand.filter(c => selected.has(c.id)).map(c => c.id);

    if (selectedIds.length !== 8) {
        return log(`扣底需要选中 8 张牌，当前选中=${selectedIds.length}`);
    }

    send("game.put_bottom", { discardIds: selectedIds });
    // 提交后清空
    selected.clear();
    renderAll(lastState);
}

function clearSelection() {
    selected.clear();
    renderAll(lastState);
}

// ===== render =====
function renderAll(st) {
    renderStatus(st);
    renderSeatBar(st);
    renderCards(st);
    updateActionAvailability(st);
}

function renderSeatBar(st) {
    const el = $("seatBar");
    if (!el) return;

    el.innerHTML = "";
    if (!st) return;

    const mode = st.callMode; // "race" | "ordered"
    const starter = (typeof st.starterSeat === "number") ? st.starterSeat : -1;


    const me = findMySeatIndex(st);
    const turn = (typeof st.callTurnSeat === "number") ? st.callTurnSeat : -1;
    const owner = (typeof st.bottomOwnerSeat === "number") ? st.bottomOwnerSeat : -1;

    for (let i = 0; i < 4; i++) {
        const s = st.seats[i];
        const card = document.createElement("div");
        card.className = "seatCard";

        if (i === me) card.classList.add("me");

        if (st.phase === "call_trump") {
            if (mode === "ordered" && i === st.callTurnSeat) card.classList.add("turn");
            // race 模式可选：让所有“未pass的坐下玩家”有淡黄色边框，或者只强调“可抢”
            if (mode === "race" && starter < 0 && s.uid) card.classList.add("turn");
        }

        if (st.phase === "bottom" && i === st.bottomOwnerSeat) card.classList.add("owner");

        const badges = [];
        // race：starter 未确定前，所有坐下玩家都“可抢”
        if (st.phase === "call_trump" && mode === "race" && starter < 0) {
            badges.push("⚡可抢");
        }
        // starter 确定后标记
        if (i === starter && starter >= 0) badges.push("⚡Starter");
        // ordered 模式才显示 👉
        if (st.phase === "call_trump" && mode === "ordered" && i === st.callTurnSeat) badges.push("👉");
        // bottom 阶段坐家
        if (st.phase === "bottom" && i === st.bottomOwnerSeat) badges.push("🟨");
        // 我自己
        if (i === me) badges.push("🟦");
        // 已pass
        if (st.phase === "call_trump" && st.callPassedSeats && st.callPassedSeats[i]) badges.push("⛔pass");
        if (st.phase === "bottom" && i === st.bottomOwnerSeat) badges.push("🟨扣底中");
        if (st.phase === "trump_fight" && i !== st.bottomOwnerSeat) badges.push("⛳攻改窗口");
        if (st.phase === "trump_fight" && st.fightPassedSeats && st.fightPassedSeats[i]) badges.push("⛔跳过");

        const uid = s.uid || "(empty)";
        const online = !!s.online;
        const ready = !!s.ready;
        const team = (typeof s.team === "number") ? s.team : "?";
        const handCount = (typeof s.handCount === "number") ? s.handCount : 0;

        card.innerHTML = `
      <div class="seatTop">
        <div><b>Seat ${i}</b> <span class="uid">${escapeHtml(uid)}</span></div>
        <div class="seatBadges">${badges.join(" ")}</div>
      </div>
      <div style="margin-top:8px; display:flex; gap:8px; flex-wrap:wrap;">
        <span class="badge ${online ? "on" : ""}">online: ${online}</span>
        <span class="badge ${ready ? "on" : ""}">ready: ${ready}</span>
        <span class="badge">team: ${team}</span>
        <span class="badge">hand: ${handCount}</span>
      </div>
    `;
        el.appendChild(card);
    }
}

function escapeHtml(s) {
    return String(s)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
}


function renderStatus(st) {
    setWSStatus();

    if (!st) {
        $("stPhase").textContent = "-";
        $("stStarter").textContent = "-";
        $("stCallTurn").textContent = "-";
        $("stPass").textContent = "-";
        $("stTrump").textContent = "-";
        $("stHandN").textContent = "0";
        $("stBottomN").textContent = "0";
        $("handRow").innerHTML = "";
        $("bottomRow").innerHTML = "";
        return;
    }

    $("stPhase").textContent = st.callMode ? `${st.phase} (${st.callMode})` : st.phase;
    $("stStarter").textContent = String(st.starterSeat ?? "-");
    $("stCallTurn").textContent = String(st.callTurnSeat ?? "-");
    $("stPass").textContent = String(st.callPassCount ?? "-");

    $("stHandN").textContent = String((st.myHand || []).length);
    $("stBottomN").textContent = String((st.myBottom || []).length);

    const tr = st.trump || {};
    let trumpStr = "";
    if (tr.isHardTrump) {
        trumpStr = `硬主 level=${tr.levelRank || "?"} caller=${tr.callerSeat}`;
    } else if (tr.hasTrumpSuit) {
        trumpStr = `主=${suitEmoji(tr.suit)} level=${tr.levelRank || "?"} locked=${!!tr.locked} caller=${tr.callerSeat}`;
    } else {
        trumpStr = "未定主";
    }
    $("stTrump").textContent = trumpStr;
}

function renderCards(st) {
    const hand = (st && st.myHand) ? st.myHand : [];
    const bottom = (st && st.myBottom) ? st.myBottom : [];

    // 清理不可见选中
    const visibleIds = new Set([...hand, ...bottom].map(c => c.id));
    for (const id of [...selected]) {
        if (!visibleIds.has(id)) selected.delete(id);
    }

    $("handRow").innerHTML = "";
    for (const c of hand) $("handRow").appendChild(makeCardButton(c, "hand"));

    $("bottomRow").innerHTML = "";
    for (const c of bottom) $("bottomRow").appendChild(makeCardButton(c, "bottom"));
}

function makeCardButton(card, zone) {
    const btn = document.createElement("button");
    btn.className = "cardBtn";

    const colorCls = cardColorClass(card);
    btn.classList.add(colorCls);

    if (selected.has(card.id)) btn.classList.add("selected");

    btn.textContent = cardLabel(card);

    btn.addEventListener("click", () => {
        if (selected.has(card.id)) selected.delete(card.id);
        else selected.add(card.id);
        renderAll(lastState);
    });

    if (zone === "bottom") {
        btn.classList.add("small");
        btn.disabled = true;       // 永远只展示
        btn.onclick = null;        // 不允许选中
        return btn;
    }

    return btn;
}

// ===== card display helpers =====
function suitEmoji(suit) {
    switch (suit) {
        case "H": return "♥️";
        case "S": return "♠️";
        case "D": return "♦️";
        case "C": return "♣️";
        default: return "?";
    }
}

function cardColorClass(card) {
    if (card.kind === "joker_big" || card.kind === "joker_small") {
        return card.color === "red" ? "red" : "black";
    }
    return (card.suit === "H" || card.suit === "D") ? "red" : "black";
}

function cardLabel(card) {
    if (card.kind === "joker_big") return "🃏大王";   // 大王
    if (card.kind === "joker_small") return "🃟小王"; // 小王
    return `${suitEmoji(card.suit)}${card.rank}`;
}

function findMySeatIndex(st) {
    const uid = $("uid").value.trim();
    for (let i = 0; i < 4; i++) {
        if ((st.seats[i].uid || "") === uid) return i;
    }
    return -1;
}

// ===== bind buttons (no inline onclick) =====
window.addEventListener("DOMContentLoaded", () => {
    initUIRefs();
    $("btnConnect").addEventListener("click", connect);
    $("btnDisconnect").addEventListener("click", disconnect);

    $("btnSit0").addEventListener("click", () => sit(0));
    $("btnSit1").addEventListener("click", () => sit(1));
    $("btnSit2").addEventListener("click", () => sit(2));
    $("btnSit3").addEventListener("click", () => sit(3));
    $("btnLeave").addEventListener("click", leaveSeat);

    $("btnReady").addEventListener("click", ready);
    $("btnUnready").addEventListener("click", unready);
    $("btnStart").addEventListener("click", start);

    $("btnCallTrump").addEventListener("click", actionCallTrump);
    $("btnPass").addEventListener("click", callPass);
    $("btnPutBottom").addEventListener("click", actionPutBottom);
    $("btnClear").addEventListener("click", clearSelection);

    renderAll(null);
    setWSStatus();
});
