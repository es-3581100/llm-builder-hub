const test = require('node:test');
const assert = require('node:assert/strict');
const { columnsForDepth, statesEqual, shortcutMatches } = require('../internal/workstation/web/app.js');

test('desktop slider depth maps exactly 0:4 1:3 2:2 3:1', () => {
  assert.equal(columnsForDepth(0,false),4);
  assert.equal(columnsForDepth(1,false),3);
  assert.equal(columnsForDepth(2,false),2);
  assert.equal(columnsForDepth(3,false),1);
});

test('compact mode deliberately uses one document column', () => {
  for (let depth=0; depth<=3; depth++) assert.equal(columnsForDepth(depth,true),1);
});

test('dirty comparison is structural', () => {
  assert.equal(statesEqual({a:1},{a:1}),true);
  assert.equal(statesEqual({a:1},{a:2}),false);
});

test('tool shortcuts use the same explicit mapping', () => {
  assert.equal(shortcutMatches('Alt+3',{key:'3',altKey:true,ctrlKey:false,shiftKey:false,metaKey:false}),true);
  assert.equal(shortcutMatches('Alt+3',{key:'3',altKey:false,ctrlKey:false,shiftKey:false,metaKey:false}),false);
});
