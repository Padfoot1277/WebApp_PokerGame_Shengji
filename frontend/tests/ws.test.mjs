import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
import {webcrypto} from 'node:crypto';
function setup(){
 const storage=new Map();const timers=new Map();let timer=0;
 class Socket {static OPEN=1;readyState=1;sent=[];constructor(url){this.url=url;Socket.instances.push(this)}static instances=[];send(raw){this.sent.push(JSON.parse(raw))}close(){this.readyState=3}message(msg){this.onmessage?.({data:JSON.stringify(msg)})}}
 const context={exports:{},WebSocket:Socket,URL,location:{href:'http://localhost/'},sessionStorage:{getItem:key=>storage.get(key)??null,setItem:(key,value)=>storage.set(key,value),removeItem:key=>storage.delete(key)},localStorage:{getItem:key=>storage.get("local:"+key)??null,setItem:(key,value)=>storage.set("local:"+key,value),removeItem:key=>storage.delete("local:"+key)},crypto:webcrypto,setTimeout:fn=>{timers.set(++timer,fn);return timer},clearTimeout:id=>timers.delete(id)};
 vm.runInNewContext(ts.transpileModule(readFileSync(new URL('../src/services/ws.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,context);
 const service=context.exports.wsService;const messages=[];const statuses=[];service.connect('ws://localhost/ws',msg=>messages.push(msg),status=>statuses.push(status));const socket=Socket.instances[0];socket.message({type:'hello',token:'test',uid:'one',name:'one'});
 return {service,socket,messages,statuses,timers,storage,Socket};
}
test('unrelated acknowledgement cannot release the pending command',()=>{const s=setup();assert.equal(s.service.send('game.play_cards',{cardIds:[1]}),true);const id=s.socket.sent[0].reqId;s.socket.message({type:'ack',reqId:'obsolete',ok:true});assert.equal(s.messages.filter(m=>m.type==='ack').length,0);assert.equal(s.timers.size,1);assert.equal(s.service.send('room.ready',{}),false);s.socket.message({type:'ack',reqId:id,ok:true});assert.equal(s.timers.size,0);assert.equal(s.service.send('room.ready',{}),true)});
test('callbacks from replaced sockets are ignored',()=>{const s=setup();const before=s.messages.length;s.service.connect('ws://localhost/ws',m=>s.messages.push(m),v=>s.statuses.push(v));s.socket.message({type:'notice',message:'stale'});s.socket.onclose({code:1006});assert.equal(s.messages.length,before);assert.equal(s.timers.size,0)});
test('manual close reports idle and clears timers',()=>{const s=setup();s.service.send('room.ready',{});s.service.close();assert.equal(s.statuses.at(-1),'idle');assert.equal(s.timers.size,0)});
test('expired session stops automatic refresh reconnect',()=>{const s=setup();s.storage.set('shengji-url','ws://localhost/ws');s.socket.onclose({code:4003});assert.equal(s.storage.has('shengji-url'),false);assert.equal(s.timers.size,0);assert.equal(s.messages.at(-1).type,'error')});

test('identity survives closing a tab and rejoining the same room',()=>{
 const s=setup();
 s.storage.delete('shengji-token');
 s.service.connect('ws://localhost/ws',()=>{});
 // The persisted identity is available independently of tab storage.
 assert.equal(new URL(s.Socket.instances.at(-1).url).searchParams.get('token'),'test');
});
