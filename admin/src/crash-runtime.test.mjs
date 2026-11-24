import test from 'node:test';
import assert from 'node:assert/strict';
import {parseCrashLog,visibleCrashStack} from './crash-runtime.mjs';
test('runtime diagnostics are compact presentation only; original evidence and trace retained',()=>{
 const crash={Stack:Array.from({length:8},(_,i)=>`World.guard${i} at source.hx:${i+10}`).join('\n')+'\nGPU Model: Example GPU\nGPU Memory: 6 GB\nTracked GPU Memory: 200 MB\nScaleGrid Guard: incidental\nPhysics Contact Guard: incidental\nOther Guard: incidental\nScene: Main\nLevel: Hills'};
 const raw=crash.Stack,parsed=parseCrashLog(crash);
 assert.equal(parsed.frames.length,8);assert.equal(parsed.frames.slice(0,6).length,6);
 assert.deepEqual(parsed.details.map(x=>x.label),['GPU Model','GPU Memory','Tracked GPU Memory','Scene','Level']);
 assert(!visibleCrashStack(crash).includes('Guard:'));assert(visibleCrashStack(crash).includes('World.guard0'));assert.equal(crash.Stack,raw);assert.equal(parseCrashLog(crash,{includeGuards:true}).details.length,8);
});
test('diagnostics beginning with guards never become frames; genuine Guard stack methods remain',()=>{
 const c={Stack:'Physics.Guard.run at physics.hx:4\nScaleGrid Guard: yes\nPhysics Contact Guard: yes\nGPU Model: GPU'};
 assert.deepEqual(parseCrashLog(c).frames,['Physics.Guard.run at physics.hx:4']);
 assert.equal(visibleCrashStack(c),'Physics.Guard.run at physics.hx:4\nGPU Model: GPU');
 assert.equal(parseCrashLog({Stack:'PhysicsContactGuard.run:4'}).frames[0],'PhysicsContactGuard.run:4');
});
test('FilePos trace and empty or legacy stacks remain readable',()=>{
 assert.deepEqual(parseCrashLog({}).frames,[]);
 assert.deepEqual(parseCrashLog({Stack:'Frame1\r\nFrame2\n\nGPU Memory: unknown\nPhysics Contact Guard: x'}).details,[{label:'GPU Memory',value:'unknown'}]);
 assert.equal(parseCrashLog({Stack:'[FilePos(Method(World,step),world.hx,4,0), FilePos(Method(App,update),app.hx,9,0)]'}).frames.length,2);
});
