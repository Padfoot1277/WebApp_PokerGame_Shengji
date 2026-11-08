// Node.js 22+. Three ordinary clients; room is intentionally fixed to test.
const fs = require('node:fs');
const path = require('node:path');
const { randomUUID } = require('node:crypto');
const { spawnSync } = require('node:child_process');
const dir = path.join(__dirname, '..', 'release', 'bots');
fs.mkdirSync(dir, { recursive: true });
const tokenFile = path.join(dir, 'tokens.json');
let tokens = [];
try { tokens = JSON.parse(fs.readFileSync(tokenFile, 'utf8')); }
catch {
  // Import identities used by the earlier temporary runner.
  try { tokens = JSON.parse(fs.readFileSync(path.join(require('node:os').tmpdir(), 'shengji-bot-tokens.json'), 'utf8')); } catch {}
}
const address = 'ws://120.53.18.44:8080/ws';
const bots = [];
let stopping = false;
const log = (b, message) => console.log(new Date().toLocaleTimeString(), b.name, message);
function persist() {
  fs.writeFileSync(tokenFile + '.tmp', JSON.stringify(tokens), { mode: 0o600 });
  fs.renameSync(tokenFile + '.tmp', tokenFile);
}
function decision(b) {
  const s = b.state;
  if (!s) return;
  const seat = s.mySeat;
  if (seat < 0) {
    if (s.phase !== 'lobby') return;
    const empty = s.seats.findIndex(x => !x.uid);
    if (empty >= 0) return ['room.sit', { seat: empty }];
    return;
  }
  const hand = (s.myHand || []).flat();
  switch (s.phase) {
    case 'lobby':
      if (!s.seats[seat].ready) return ['room.ready', {}];
      break;
    case 'call_trump':
      if (!s.callPassedSeats[seat] && (s.callMode === 'race' || s.callTurnSeat === seat)) return ['policy', {}];
      break;
    case 'trump_fight':
      if (s.fightTurnSeat === seat && !s.fightPassedSeats[seat]) return ['policy', {}];
      break;
    case 'bottom':
      if (s.bottomOwnerSeat === seat && hand.length >= 8) return ['policy', {}];
      break;
    case 'play_trick':
      if (s.trick.turnSeat === seat && hand.length) return ['play', {}];
      break;
    case 'round_settle':
      if (s.nextStarterSeat === seat) return ['game.start_next_round', {}];
  }
}
function tick(b) {
  if (!b.hello || b.pending || b.timer || b.ws.readyState !== WebSocket.OPEN || !decision(b)) return;
  // Recompute after the delay: another player may have changed the state.
  const version = b.state.version;
  if (b.blocked === version) return;
  b.timer = setTimeout(() => {
    b.timer = null;
    if (!b.hello || b.pending || b.ws.readyState !== WebSocket.OPEN || b.state.version !== version) return;
    let action = decision(b);
    if (!action) return;
    if (action[0] === 'play' || action[0] === 'policy') {
      const result = spawnSync(path.join(dir, 'bot-policy.exe'), action[0] === 'policy' ? ['--action'] : [], { input: JSON.stringify({ ...b.state, botHistory: (b.history || []).filter(Boolean) }), encoding: 'utf8', timeout: 15000 });
      if (result.status !== 0) { b.blocked = version; log(b, '策略暂停：' + (result.stderr || result.error || 'policy failed')); return; }
      const picked = JSON.parse(result.stdout);
      action = action[0] === 'policy' ? [picked.type, picked.payload] : ['game.play_cards', { cardIds: picked }];
    }
    const request = { type: action[0], payload: action[1], reqId: randomUUID() };
    b.pending = { request, version, sent: Date.now() };
    b.ws.send(JSON.stringify(request));
    log(b, request.type + '（等待至少1秒）');
  }, 1000);
}
function connect(b) {
  if (stopping) return;
  b.hello = false; b.state = null; b.blocked = null;
  const url = new URL(address);
  url.searchParams.set('room', 'test'); url.searchParams.set('name', b.name);
  if (tokens[b.index]) url.searchParams.set('token', tokens[b.index]);
  const ws = b.ws = new WebSocket(url);
  ws.onmessage = e => {
    if (b.ws !== ws) return;
    let m; try { m = JSON.parse(e.data); } catch { return; }
    if (m.type === 'hello') {
      tokens[b.index] = m.token; persist(); b.hello = true;
      log(b, '已连接 test');
      if (b.pending) { b.pending.sent = Date.now(); ws.send(JSON.stringify(b.pending.request)); }
    }
    if (m.type === 'snapshot') {
      const state = m.state;
      if (b.historyRound !== state.roundIndex || b.historyRoom !== state.roomId || state.phase === 'lobby') {
        b.history = []; b.historyRound = state.roundIndex; b.historyRoom = state.roomId;
      }
      // LastMoves survives the server clearing a completed trick; retain only public plays.
      if (state.trickIndex > 0 && state.trick.lastMoves?.some(Boolean)) {
        b.history[state.trickIndex - 1] = { leader: state.trick.lastLeaderSeat, plays: state.trick.lastMoves };
      }
      b.state = state;
      fs.writeFileSync(path.join(dir, `status-${b.index + 1}.json`), JSON.stringify({ phase: m.state.phase, seat: m.state.mySeat, seats: m.state.seats, time: new Date().toISOString() }));
    }
    if (m.type === 'ack' && b.pending?.request.reqId === m.reqId) {
      if (!m.ok) { b.blocked = b.pending.version; log(b, '请求被拒绝：' + m.message); }
      b.pending = null;
    }
    if (m.type === 'error') log(b, m.message);
  };
  ws.onclose = e => {
    if (b.ws !== ws) return;
    b.hello = false; clearTimeout(b.timer); b.timer = null;
    log(b, '已断开 ' + e.code);
    if (e.code === 4001) { log(b, '身份在其他位置连接，停止抢占'); return; }
    if (e.code === 4003) { tokens[b.index] = null; b.pending = null; persist(); }
    if (!stopping) b.reconnect = setTimeout(() => connect(b), 2000);
  };
  ws.onerror = () => ws.close();
}
for (let index = 0; index < 3; index++) {
  const b = { index, name: '测试机器人' + (index + 1), pending: null, timer: null };
  bots.push(b); connect(b);
}
const ticker = setInterval(() => {
  for (const b of bots) {
    if (b.pending && Date.now() - b.pending.sent > 10000 && b.ws.readyState === WebSocket.OPEN) { b.pending.sent = Date.now(); b.ws.close(); }
    tick(b);
  }
}, 200);
function stop() { stopping = true; clearInterval(ticker); for (const b of bots) { clearTimeout(b.timer); clearTimeout(b.reconnect); b.ws.close(); } setTimeout(() => process.exit(), 300); }
process.on('SIGINT', stop); process.on('SIGTERM', stop);
console.log('Three bots running in test. Ctrl+C stops them.');
