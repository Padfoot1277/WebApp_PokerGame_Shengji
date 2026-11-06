import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";
import ts from "typescript";

const require = createRequire(import.meta.url);
const { createPinia, setActivePinia } = require("pinia");
function boot() {
  const sent = [];
  const wsService = { send: (type,payload) => { sent.push({type,payload}); return true; }, close() {}, connect() {} };
  const module = {exports:{}};
  const source = ts.transpileModule(readFileSync(new URL("../src/store/game.ts",import.meta.url),"utf8"), { compilerOptions:{module:ts.ModuleKind.CommonJS} }).outputText;
  vm.runInNewContext(source,{module,exports:module.exports,console,sessionStorage:{setItem(){},removeItem(){}},require: name => name === "../services/ws" ? {wsService} : require(name)});
  setActivePinia(createPinia());
  return { game: module.exports.useGameStore(), sent };
}
const state = () => ({roomId:"room",roundIndex:0,phase:"play_trick",trickIndex:1,trick:{lastMoves:[{seat:0},null,null,null]}});
test("history mode blocks table commands but permits deliberate surrender",()=>{
  const {game,sent}=boot();game.reviewPrevious=true;
  game.sendEvent("game.play_cards",{cardIds:[1]});
  assert.equal(sent.length,0);assert.equal(game.pending,false);
  assert.match(game.messages.at(-1).text,/上一墩/);
  game.sendEvent("game.surrender",{});assert.equal(sent[0].type,"game.surrender");
});

test("leader starting next trick clears history and releases play even without trick index change",()=>{
  const {game,sent}=boot();
  game.handleServerMessage({type:"snapshot",state:state()});
  game.reviewPrevious=true;
  game.handleServerMessage({type:"snapshot",state:{...state(),trick:{lastMoves:[null,null,null,null]}}});
  assert.equal(game.reviewPrevious,false);
  game.sendEvent("game.play_cards",{cardIds:[1]});
  assert.equal(sent.length,1);
  assert.equal(sent[0].type,"game.play_cards");
});
test("history remains stable for same trick but resets on room, phase, round or trick transition",()=>{
  for (const [key,value] of [["roomId","another"],["phase","round_settle"],["roundIndex",1],["trickIndex",2]]) {
    const {game}=boot();game.handleServerMessage({type:"snapshot",state:state()});
    game.reviewPrevious=true;game.handleServerMessage({type:"snapshot",state:state()});assert.equal(game.reviewPrevious,true);
    game.handleServerMessage({type:"snapshot",state:{...state(),[key]:value}});assert.equal(game.reviewPrevious,false,key);
  }
});
