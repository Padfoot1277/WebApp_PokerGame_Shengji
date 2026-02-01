let ws = null;
let lastState = null;
let UI = null;
let lastPhase = null;
let lastVersion = null;
let MY_UID = null;

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
    const uid = window.myUID;
    if (!uid || !st?.seats) return getMyUID();
    for (let i = 0; i < st.seats.length; i++) {
        if (st.seats[i]?.uid === uid) return i;
    }
    return getMyUID();
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
        const canAct = seated && !isOwner && !already;

        setDisabled(UI.btnPass, !canAct);
        setPrimary(UI.btnPass, canAct);

        // 改主/攻主启用
        $("btnChangeTrump").disabled = !canAct;
        $("btnAttackTrump").disabled = !canAct;

        // 其他禁用
        setDisabled(UI.btnCallTrump, true);
        setDisabled(UI.btnPutBottom, true);
        setDisabled(UI.btnReady, true);
        setDisabled(UI.btnUnready, true);
        setDisabled(UI.btnStart, true);
        return;
    }

    if (st.phase === "play_trick") {
        const mySeat = mySeatIndex(st);
        const canPlay = (mySeat === st.trick.leaderSeat) && (mySeat === st.trick.turnSeat) && !leadPlayed(st);

        // 出牌按钮
        const btnPlay = document.getElementById("btnPlay");
        if (btnPlay) {
            btnPlay.disabled = !canPlay;
            setPrimary(btnPlay, canPlay);
            btnPlay.textContent = canPlay ? "先手出牌" : "出牌";
        }

        // 清空/其他按钮（你可按需）
        setDisabled(UI.btnCallTrump, true);
        setDisabled(UI.btnPutBottom, true);
        setDisabled(UI.btnPass, true);

        // 允许“清空选择”始终可点
        const btnClear = document.getElementById("btnClear");
        if (btnClear) btnClear.disabled = false;

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

    ws.onmessage = (e) => {
        const msg = JSON.parse(e.data);
        if (msg.type === "hello") {
            window.myUID = msg.uid;
            console.log("[hello] myUID =", window.myUID);
            setMyUID(msg.uid);
            return;
        }
        if (msg.type === "snapshot") {
            const st = msg.state;
            const phaseChanged = (lastPhase !== null && st.phase !== lastPhase);
            const leadBecameSet =
                lastState &&
                lastState.trick && !lastState.trick.lead &&
                st.trick && st.trick.lead;

            // 只在完成某一步骤后清空勾选
            if (phaseChanged || leadBecameSet) {
                selected.clear();
                log(`[ui] clear selection (phaseChanged=${phaseChanged}, leadBecameSet=${leadBecameSet})`);
            }
            lastVersion = st.version;
            lastPhase = st.phase;
            lastState = st;
            renderAll(st);

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
    const bar = document.getElementById("seatBar");
    if (!bar) return;
    bar.innerHTML = "";
    if (!st || !st.seats) return;

    const mySeat = mySeatIndex(st);
    const phase = st.phase;

    const trick = st.trick;
    const playsBySeat = buildPlaysBySeat(trick);

    for (let i = 0; i < st.seats.length; i++) {
        const s = st.seats[i];

        // 外层卡片
        const card = document.createElement("div");
        card.style.border = "1px solid #e5e7eb";
        card.style.borderRadius = "12px";
        card.style.padding = "10px";
        card.style.minWidth = "220px";

        // 顶部标题行：Seat + badges
        const title = document.createElement("div");
        title.style.display = "flex";
        title.style.alignItems = "center";
        title.style.justifyContent = "space-between";

        const left = document.createElement("div");
        left.innerHTML = `<b>Seat ${i}</b> ${s.uid ? "" : "(空)"}`;

        const badges = document.createElement("div");
        badges.style.display = "flex";
        badges.style.gap = "6px";
        badges.style.flexWrap = "wrap";

        // 你已有的标记：🟦 我 / 🟨 bottomOwner / 👉 turn...
        if (i === mySeat) badges.appendChild(makeBadge("🟦你"));
        if (typeof st.bottomOwnerSeat === "number" && i === st.bottomOwnerSeat) badges.appendChild(makeBadge("🟨坐家"));
        if (phase === "play_trick" && trick && i === trick.turnSeat) badges.appendChild(makeBadge("⏳轮到你"));
        if (phase === "play_trick" && trick && i === trick.leaderSeat) badges.appendChild(makeBadge("🎯先手"));

        title.appendChild(left);
        title.appendChild(badges);
        card.appendChild(title);

        // 中部：显示玩家状态（可选）
        const sub = document.createElement("div");
        sub.style.marginTop = "6px";
        sub.style.fontSize = "12px";
        sub.style.color = "#6b7280";
        sub.textContent = `hand: ${s.handCount ?? "?"}  | team: ${s.team ?? "?"}`;
        card.appendChild(sub);

        // ✅ 本回合出牌展示区
        const play = playsBySeat.get(i);
        const playBox = document.createElement("div");
        playBox.style.marginTop = "10px";

        const playTitle = document.createElement("div");
        playTitle.style.fontSize = "12px";
        playTitle.style.color = "#374151";
        playTitle.innerHTML = `<b>本回合出牌</b>`;
        playBox.appendChild(playTitle);

        if (!play || !play.actual || !(play.actual.cards && play.actual.cards.length)) {
            const none = document.createElement("div");
            none.style.fontSize = "12px";
            none.style.color = "#9ca3af";
            none.style.marginTop = "6px";
            none.textContent = "（未出牌）";
            playBox.appendChild(none);
        } else {
            // 显示最终出牌
            playBox.appendChild(renderCardsInline(play.actual.cards));

            // 甩牌失败提示：只对先手且失败
            if (play.type === "lead" && play.isThrow && !play.throwOk) {
                const warn = document.createElement("div");
                warn.style.marginTop = "8px";
                warn.style.padding = "8px";
                warn.style.borderRadius = "10px";
                warn.style.background = "#fee2e2";
                warn.style.color = "#991b1b";
                warn.style.fontSize = "12px";
                warn.innerHTML = `⚠️ 甩牌失败，已裁剪。${play.info ? "原因：" + escapeHtml(play.info) : ""}`;
                playBox.appendChild(warn);

                // 可选：再展示意图牌（置灰）
                if (play.intent && play.intent.cards && play.intent.cards.length) {
                    const intentLabel = document.createElement("div");
                    intentLabel.style.marginTop = "8px";
                    intentLabel.style.fontSize = "12px";
                    intentLabel.style.color = "#6b7280";
                    intentLabel.textContent = "原意图：";
                    playBox.appendChild(intentLabel);

                    const intentRow = renderCardsInline(play.intent.cards);
                    intentRow.style.opacity = "0.55";
                    playBox.appendChild(intentRow);
                }
            }
        }

        card.appendChild(playBox);
        bar.appendChild(card);
    }
}

function makeBadge(txt) {
    const b = document.createElement("span");
    b.textContent = txt;
    b.style.fontSize = "12px";
    b.style.padding = "2px 8px";
    b.style.borderRadius = "999px";
    b.style.background = "#eef2ff";
    return b;
}

function escapeHtml(s) {
    return String(s)
        .replaceAll("&", "&amp;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;")
        .replaceAll('"', "&quot;")
        .replaceAll("'", "&#039;");
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

function actionChangeTrump() {
    if (!lastState) return;
    const st = lastState;
    if (st.phase !== "trump_fight") return log("当前不在改主/攻主阶段");

    const mySeat = findMySeatIndex(st);
    if (mySeat < 0) return log("你还没坐下");
    if (st.bottomOwnerSeat === mySeat) return log("坐家不能改主/攻主");

    // 从右侧手牌选：1 joker + 2 level（同花色、同rank=本队级牌）
    const hand = st.myHand || [];
    const picked = hand.filter(c => selected.has(c.id));

    const joker = picked.find(c => c.kind === "joker_big" || c.kind === "joker_small");
    if (!joker) return log("改主需要选 1 张王");

    const myTeam = st.seats[mySeat].team;
    const myLevel = st.teams[myTeam].levelRank;

    const levels = picked.filter(c => c.kind === "normal" && c.rank === myLevel);
    if (levels.length !== 2) return log(`改主需要选 2 张本队级牌（rank=${myLevel}）`);

    if (levels[0].suit !== levels[1].suit) return log("两张级牌必须同花色（同一 suit）");

    send("game.change_trump", { jokerId: joker.id, levelIds: [levels[0].id, levels[1].id] });
}

function actionAttackTrump() {
    if (!lastState) return;
    const st = lastState;
    if (st.phase !== "trump_fight") return log("当前不在改主/攻主阶段");

    const mySeat = findMySeatIndex(st);
    if (mySeat < 0) return log("你还没坐下");
    if (st.bottomOwnerSeat === mySeat) return log("坐家不能改主/攻主");

    // 选 2 张王，且同 kind
    const hand = st.myHand || [];
    const picked = hand.filter(c => selected.has(c.id));
    const jokers = picked.filter(c => c.kind === "joker_big" || c.kind === "joker_small");
    if (jokers.length !== 2) return log("攻主需要选 2 张王");

    if (jokers[0].kind !== jokers[1].kind) return log("两张王必须同类型（大王对 或 小王对）");

    send("game.attack_trump", { jokerIds: [jokers[0].id, jokers[1].id] });
}

function renderLeadMove(st) {
    const info = $("leadInfo");
    const row = $("leadCards");
    if (!info || !row) return;

    info.textContent = "";
    row.innerHTML = "";

    if (!st || !st.trick || !st.trick.lead) {
        info.textContent = "（暂无）";
        return;
    }

    const lead = st.trick.lead;
    const seat = lead.seat;
    const ok = lead.throwOk;
    const isThrow = lead.isThrow;

    info.textContent =
        `Seat ${seat} 出牌：` +
        (isThrow ? (ok ? "甩牌成功" : "甩牌失败（已裁剪）") : "普通出牌") +
        (lead.reason ? ` | ${lead.reason}` : "");

    // 用 ActualIDs 找到对应 Card
    for (const c of (lead.actualCards || [])) {
        row.appendChild(makeCardButton(c, "bottom")); // bottom按钮样式=只展示
    }
}

function leadPlayed(st) {
    return st && st.trick && typeof st.trick.lead?.seat === "number" && st.trick.lead.seat !== -1;
}

function renderTurnHint(st) {
    const el = document.getElementById("turnHint");
    if (!el) return;

    if (!st || !st.phase) { el.textContent = ""; return; }

    if (st.phase !== "play_trick") {
        el.textContent = `当前阶段：${st.phase}`;
        return;
    }

    const leader = st.trick.leaderSeat;
    const turn = st.trick.turnSeat;

    // 你现在只实现先手出牌，所以 turn==leader 时表示等待先手
    if (!leadPlayed(st)) {
        el.textContent = `🟢 等待 Seat ${leader} 先手出牌（本版本未实现跟牌）`;
    } else {
        el.textContent = `✅ Seat ${st.trick.lead.seat} 已出牌。当前版本未实现跟牌/回合结算。`;
    }
}

function makeDisplayCard(c) {
    const btn = makeCardButton(c, "bottom"); // 复用你的展示样式
    btn.disabled = true;
    btn.classList.add("tableCard");
    return btn;
}

function renderLeadPanel(st) {
    const panel = document.getElementById("leadPanel");
    const leadCards = document.getElementById("leadCards");
    const intentCards = document.getElementById("intentCards");
    const badge = document.getElementById("leadBadge");
    const banner = document.getElementById("throwBanner");
    if (!panel || !leadCards || !intentCards || !badge || !banner) return;

    // 非 play_trick 也可以显示，但先清空
    leadCards.innerHTML = "";
    intentCards.innerHTML = "";
    badge.textContent = "";
    banner.classList.add("hidden");
    banner.classList.remove("danger");
    banner.textContent = "";

    if (!st || !st.trick) return;

    const lead = st.trick.lead;
    const played = leadPlayed(st);

    if (!played) {
        badge.textContent = "（尚未出牌）";
        return;
    }

    badge.textContent = `Seat ${lead.seat}`;

    // 最终桌面牌
    const finalCards = (lead.actualMove && lead.actualMove.cards) ? lead.actualMove.cards : [];
    for (const c of finalCards) leadCards.appendChild(makeDisplayCard(c));

    // 甩牌提示
    if (lead.isThrow) {
        if (lead.throwOk) {
            // 甩牌成功：可给个温和提示（可选）
            // banner.classList.remove("hidden");
            // banner.textContent = "甩牌成功";
        } else {
            banner.classList.remove("hidden");
            banner.classList.add("danger");
            banner.textContent = `⚠️ 甩牌失败，系统已裁剪出牌。${lead.info ? "原因：" + lead.info : ""}`;
        }

        // 原意图牌（置灰显示）
        const intent = (lead.intentMove && lead.intentMove.cards) ? lead.intentMove.cards : [];
        for (const c of intent) intentCards.appendChild(makeDisplayCard(c));
    }
}

function actionPlayLead() {
    if (!lastState) return;
    const st = lastState;

    if (st.phase !== "play_trick") return log("当前不在出牌阶段");
    if (leadPlayed(st)) return log("先手已出牌（未实现跟牌）");

    const mySeat = mySeatIndex(st);
    if (mySeat !== st.trick.leaderSeat || mySeat !== st.trick.turnSeat) {
        return log(`未轮到你出牌，应由 Seat ${st.trick.leaderSeat} 先手`);
    }

    const hand = st.myHand || [];
    const ids = hand.filter(c => selected.has(c.id)).map(c => c.id);
    if (ids.length === 0) return log("请选择要出的牌");

    send("game.play_cards", { cardIds: ids });
}

function setMyUID(uid) {
    MY_UID = String(uid);
    window.myUID = MY_UID;
    localStorage.setItem("upgrade_uid", MY_UID);
}

function getMyUID() {
    return MY_UID || window.myUID || localStorage.getItem("upgrade_uid");
}

function renderCardsInline(cards) {
    const wrap = document.createElement("div");
    wrap.style.display = "flex";
    wrap.style.flexWrap = "wrap";
    wrap.style.gap = "6px";

    for (const c of (cards || [])) {
        const btn = makeCardButton(c, "bottom"); // 复用你已有的按钮渲染（emoji花色/数字）
        btn.disabled = true;
        btn.style.opacity = "1";
        btn.style.cursor = "default";
        wrap.appendChild(btn);
    }
    return wrap;
}

function renderCardsInline(cards) {
    const wrap = document.createElement("div");
    wrap.style.display = "flex";
    wrap.style.flexWrap = "wrap";
    wrap.style.gap = "6px";

    for (const c of (cards || [])) {
        const btn = makeCardButton(c, "bottom"); // 复用你已有的按钮渲染（emoji花色/数字）
        btn.disabled = true;
        btn.style.opacity = "1";
        btn.style.cursor = "default";
        wrap.appendChild(btn);
    }
    return wrap;
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
    $("btnChangeTrump").addEventListener("click", actionChangeTrump);
    $("btnAttackTrump").addEventListener("click", actionAttackTrump);
    $("btnPlay").addEventListener("click", actionPlayLead);
    document.getElementById("btnPlay").addEventListener("click", actionPlayLead);

    renderAll(null);
    setWSStatus();
});
