let ws = null;

const $ = (id) => document.getElementById(id);
const log = (s) => { $("log").textContent = (s + "\n") + $("log").textContent; };

function connect() {
    const uid = $("uid").value.trim();
    const room = $("room").value.trim();
    const url = `ws://${location.host}/ws?uid=${encodeURIComponent(uid)}&room=${encodeURIComponent(room)}`;

    ws = new WebSocket(url);
    ws.onopen = () => log("[ws] open");
    ws.onclose = () => log("[ws] close");
    ws.onerror = (e) => log("[ws] error " + (e?.message || ""));
    ws.onmessage = (e) => {
        const msg = JSON.parse(e.data);
        if (msg.type === "snapshot") {
            $("snapshot").textContent = JSON.stringify(msg.state, null, 2);
            log(`phase=${msg.state.phase} myHand=${(msg.state.myHand||[]).length} bottomCount=${msg.state.bottomCount}`);
        } else if (msg.type === "error") {
            log(`[error] ${msg.code}: ${msg.message}`);
        } else {
            log("[msg] " + e.data);
        }
    };
}

function disconnect() {
    if (ws) ws.close();
    ws = null;
}

function send(type, payload) {
    if (!ws || ws.readyState !== 1) return log("not connected");
    ws.send(JSON.stringify({ type, payload }));
}

function sit(seat) { send("room.sit", { seat }); }
function leaveSeat() { send("room.leave_seat", {}); }
function ready() { send("room.ready", {}); }
function unready() { send("room.unready", {}); }
function start() { send("game.start", {}); }

$("btnConnect").onclick = connect;
$("btnDisconnect").onclick = disconnect;
