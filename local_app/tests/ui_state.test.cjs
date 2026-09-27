const test = require('node:test');
const assert = require('node:assert/strict');
const { columnsForDepth, statesEqual, shortcutMatches, repositoryDisplayBranch, shouldRerenderAfterDocumentClick, repositoryDraftDirty, canStartRepositoryEdit, makeRepositoryWriteRequest } = require('../internal/workstation/web/app.js');

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

test('repository branch labels preserve branch detached and unborn states', () => {
  assert.equal(repositoryDisplayBranch({branch:'phase2',detached:false,unborn:false}),'phase2');
  assert.equal(repositoryDisplayBranch({branch:'',detached:true,unborn:false}),'DETACHED');
  assert.equal(repositoryDisplayBranch({branch:'',detached:false,unborn:true}),'UNBORN');
  assert.equal(repositoryDisplayBranch({branch:'',detached:false,unborn:false}),'UNKNOWN');
});

test('clicking inside the open editor never rebuilds the document stream', () => {
  // Regression: the article click used to call renderDocuments() unconditionally, which
  // began with stream.replaceChildren() and destroyed the focused textarea before any
  // input event could fire, so document edits could never reach the dirty indicator.
  assert.equal(shouldRerenderAfterDocumentClick('project-readme','project-readme'),false);
});

test('clicking a document that is not in EDIT mode still rerenders', () => {
  assert.equal(shouldRerenderAfterDocumentClick('project-readme',null),true);
  assert.equal(shouldRerenderAfterDocumentClick('project-readme','build-ledger'),true);
});

test('repository source draft is independent and dirty only when content changes', () => {
  const edit = {
    repositoryId:'repo-1',
    documentId:'git-doc',
    path:'source.txt',
    expectedContentSHA256:'abc',
    originalContent:'before\n',
    content:'before\n'
  };
  assert.equal(repositoryDraftDirty(edit),false);
  edit.content='after\n';
  assert.equal(repositoryDraftDirty(edit),true);
  assert.equal(repositoryDraftDirty(null),false);
});

test('WRITE FILE request binds repository document path and original content hash', () => {
  const edit = {
    repositoryId:'repo-1',
    documentId:'git-doc',
    path:'source.txt',
    expectedContentSHA256:'0123456789abcdef',
    originalContent:'before\n',
    content:'after\n',
    status:'editing',
    errorCode:''
  };
  assert.deepEqual(makeRepositoryWriteRequest(edit),{
    repository_id:'repo-1',
    document_id:'git-doc',
    path:'source.txt',
    expected_content_sha256:'0123456789abcdef',
    content:'after\n'
  });
  assert.equal(makeRepositoryWriteRequest(null),null);
});

test('dirty repository source draft cannot be silently replaced by editing another source', () => {
  const dirty = {
    documentId:'git-a',
    originalContent:'before\n',
    content:'draft\n'
  };
  assert.equal(canStartRepositoryEdit(dirty,'git-b'),false);
  assert.equal(canStartRepositoryEdit(dirty,'git-a'),true);
  dirty.content='before\n';
  assert.equal(canStartRepositoryEdit(dirty,'git-b'),true);
  assert.equal(canStartRepositoryEdit(null,'git-b'),true);
});
