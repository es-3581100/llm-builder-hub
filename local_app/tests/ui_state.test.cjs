const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { columnsForDepth, statesEqual, shortcutMatches, repositoryDisplayBranch, shouldRerenderAfterDocumentClick, repositoryDraftDirty, canStartRepositoryEdit, makeRepositoryWriteRequest, LOCAL_GIT_ACTIONS, changeKind, localGitControlReady, eligibleTrackedSource, canStageFile, canUnstageFile, canCommit, canRunLocalGitControl, makeStageRequest, makeUnstageRequest, makeCommitRequest, sourceFileForPath, stagedSummaryText } = require('../internal/workstation/web/app.js');

const WEB_DIR = path.join(__dirname, '..', 'internal', 'workstation', 'web');
const APP_PATH = path.join(WEB_DIR, 'app.js');
const HTML_PATH = path.join(WEB_DIR, 'index.html');

// Phase 4 fixtures. Every snapshot carries a complete identity because
// localGitControlReady refuses to bind any request without one, and a fixture
// that quietly omitted head_commit/index_sha256 would make the eligibility
// matrix pass for the wrong reason.
function readyRepository(overrides) {
  return Object.assign({
    repository_id: 'repo-1',
    root: '/tmp/repo',
    head_commit: 'head-sha-1',
    branch: 'phase4/local-git-commit-control',
    index_sha256: 'index-sha-1',
    snapshot_sha256: 'snapshot-sha-1',
    detached: false,
    unborn: false,
    clean: false,
    staged: [],
    unstaged: [],
    untracked: [],
    history: [],
    files: [],
    documents: []
  }, overrides || {});
}

function modifiedSource(overrides) {
  return Object.assign({
    id: 'src-1',
    path: 'alpha.txt',
    document_id: 'git-alpha',
    tracked: true,
    untracked: false,
    exists: true,
    regular: true,
    symlink: false,
    symlink_target: '',
    binary: false,
    too_large: false,
    content_sha256: 'worktree-sha-1',
    staged_status: '',
    unstaged_status: 'M'
  }, overrides || {});
}

const STAGE_KEYS = ['expected_branch','expected_head_commit','expected_index_sha256','expected_worktree_sha256','path','repository_id'];
const UNSTAGE_KEYS = ['expected_branch','expected_head_commit','expected_index_sha256','path','repository_id'];
const COMMIT_KEYS = ['commit_message','expected_branch','expected_head_commit','expected_index_sha256','repository_id'];

// Structural helpers. Source text is read and scanned with a brace-balanced
// extractor rather than a regex, because a function body contains nested
// objects, template literals, and string literals full of characters that a
// naive /function name[\s\S]*?\n}/ match would truncate or overrun.
function readSource(file) { return fs.readFileSync(file, 'utf8'); }

function functionBody(source, name) {
  const signature = 'function ' + name + '(';
  const start = source.indexOf(signature);
  assert.notEqual(start, -1, 'expected to find declaration of ' + name);
  const open = source.indexOf('{', start);
  assert.notEqual(open, -1, 'expected a body for ' + name);
  let depth = 0;
  let index = open;
  let state = 'code';
  while (index < source.length) {
    const ch = source[index];
    const next = source[index + 1];
    if (state === 'line-comment') {
      if (ch === '\n') state = 'code';
    } else if (state === 'block-comment') {
      if (ch === '*' && next === '/') { state = 'code'; index += 1; }
    } else if (state === 'string') {
      if (ch === '\\') index += 1;
      else if (ch === state) state = 'code';
    } else {
      if (ch === '/' && next === '/') { state = 'line-comment'; index += 1; }
      else if (ch === '/' && next === '*') { state = 'block-comment'; index += 1; }
      else if (ch === '"' || ch === "'" || ch === '`') state = ch;
      else if (ch === '{') depth += 1;
      else if (ch === '}') {
        depth -= 1;
        if (depth === 0) return source.slice(open, index + 1);
      }
    }
    index += 1;
  }
  throw new Error('unbalanced braces extracting ' + name);
}

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

test('local Git request builders send exactly the documented field sets', () => {
  const repository = readyRepository({staged: [{status:'M', path:'alpha.txt'}]});
  const staged = makeStageRequest(modifiedSource(), repository);
  assert.deepEqual(Object.keys(staged).sort(), STAGE_KEYS);
  assert.deepEqual(staged, {
    repository_id: 'repo-1',
    path: 'alpha.txt',
    expected_head_commit: 'head-sha-1',
    expected_branch: 'phase4/local-git-commit-control',
    expected_index_sha256: 'index-sha-1',
    expected_worktree_sha256: 'worktree-sha-1'
  });

  const unstaged = makeUnstageRequest(modifiedSource({staged_status:'M', unstaged_status:''}), repository);
  assert.deepEqual(Object.keys(unstaged).sort(), UNSTAGE_KEYS);
  assert.deepEqual(unstaged, {
    repository_id: 'repo-1',
    path: 'alpha.txt',
    expected_head_commit: 'head-sha-1',
    expected_branch: 'phase4/local-git-commit-control',
    expected_index_sha256: 'index-sha-1'
  });

  const commit = makeCommitRequest(repository, 'docs: seal browser local git control');
  assert.deepEqual(Object.keys(commit).sort(), COMMIT_KEYS);
  assert.deepEqual(commit, {
    repository_id: 'repo-1',
    expected_head_commit: 'head-sha-1',
    expected_branch: 'phase4/local-git-commit-control',
    expected_index_sha256: 'index-sha-1',
    commit_message: 'docs: seal browser local git control'
  });
});

test('request builders refuse to bind an incomplete identity or message', () => {
  const repository = readyRepository({staged: [{status:'M', path:'alpha.txt'}]});
  // STAGE cannot name a worktree hash it has not observed.
  assert.equal(makeStageRequest(modifiedSource({content_sha256:''}), repository), null);
  // UNSTAGE has no worktree field, so a staged modification is enough.
  assert.notEqual(makeUnstageRequest(modifiedSource({staged_status:'M'}), repository), null);
  // A commit message is never invented, trimmed, or extended.
  assert.equal(makeCommitRequest(repository, ''), null);
  assert.equal(makeCommitRequest(repository, null), null);
  assert.equal(makeCommitRequest(repository, '  padded  ').commit_message, '  padded  ');
  // Detached and unborn snapshots bind no branch to advance.
  assert.equal(makeStageRequest(modifiedSource(), readyRepository({detached:true})), null);
  assert.equal(makeCommitRequest(readyRepository({unborn:true, staged:[{status:'M', path:'alpha.txt'}]}), 'msg'), null);
});

test('STAGE FILE is offered only for an unstaged tracked modification', () => {
  const repository = readyRepository();
  // unstaged_status arrives from the snapshot as the single-letter string "M".
  assert.equal(changeKind('M'), 'modified');
  assert.equal(canStageFile(modifiedSource(), repository), true);
  assert.equal(eligibleTrackedSource(modifiedSource()), true);

  const rejected = {
    'untracked file': {untracked:true, tracked:false},
    'symlink': {symlink:true, symlink_target:'../alpha.txt'},
    'binary': {binary:true},
    'too large': {too_large:true},
    'missing target': {exists:false},
    'non-regular': {regular:false}
  };
  for (const [label, overrides] of Object.entries(rejected)) {
    assert.equal(eligibleTrackedSource(modifiedSource(overrides)), false, label);
    assert.equal(canStageFile(modifiedSource(overrides), repository), false, label);
  }

  // A staged entry is not a file-shape problem, so the file stays an eligible
  // tracked source; what it loses is the STAGE affordance, which belongs to
  // UNSTAGE territory.
  const alreadyStaged = modifiedSource({staged_status:'M'});
  assert.equal(eligibleTrackedSource(alreadyStaged), true, 'already staged');
  assert.equal(canStageFile(alreadyStaged, repository), false, 'already staged');
  assert.equal(canUnstageFile(alreadyStaged, repository), true, 'already staged');

  for (const status of ['A','D','R','T','U']) {
    const file = modifiedSource({unstaged_status: status});
    assert.equal(eligibleTrackedSource(file), true, status);
    assert.equal(canStageFile(file, repository), false, 'unstaged_status ' + status + ' is not a modification');
  }
  assert.equal(canStageFile(modifiedSource(), readyRepository({detached:true})), false);
  assert.equal(localGitControlReady(readyRepository({index_sha256:''})), false);
});

test('UNSTAGE FILE is offered only for a staged tracked modification', () => {
  const repository = readyRepository();
  assert.equal(canUnstageFile(modifiedSource({staged_status:'M', unstaged_status:''}), repository), true);
  for (const status of ['A','D','R','T','U']) {
    assert.equal(canUnstageFile(modifiedSource({staged_status: status}), repository), false, 'staged_status ' + status + ' is not a modification');
  }
  assert.equal(canUnstageFile(modifiedSource({tracked:false}), repository), false);
  assert.equal(canUnstageFile(null, repository), false);
  assert.equal(canUnstageFile(modifiedSource({staged_status:'M'}), readyRepository({unborn:true})), false);
});

test('COMMIT STAGED requires an attached branch and a non-empty staged snapshot', () => {
  const staged = [{status:'M', path:'alpha.txt'}];
  assert.equal(canCommit(readyRepository({staged: staged})), true);
  assert.equal(canCommit(readyRepository({staged: [], detached: true})), false);
  assert.equal(canCommit(readyRepository({staged: [], unborn: true})), false);
  assert.equal(canCommit(readyRepository({staged: []})), false);
  assert.equal(canCommit(readyRepository({staged: null})), false);
  assert.equal(canCommit(null), false);
});

test('WRITE FILE never implies STAGE FILE', () => {
  const body = functionBody(readSource(APP_PATH), 'writeRepositoryFile');
  assert.ok(body.includes('/api/write-file'), 'WRITE FILE must call the write endpoint');
  for (const forbidden of ['/api/stage-file', 'stageRepositoryFile', 'makeStageRequest']) {
    assert.equal(body.includes(forbidden), false, 'WRITE FILE must not reach local Git control via ' + forbidden);
  }
});

test('the WRITE FILE button binding is declared once in enclosing scope before every use', () => {
  // Regression: the WRITE FILE element was declared with `const` inside the first
  // `if (editing) {` block and then read from a sibling `if (editing) {` block, so
  // under "use strict" every keystroke threw `ReferenceError: write is not defined`
  // and the button's `disabled` flag was never cleared. The exported helpers are
  // pure and cannot observe DOM construction, so this asserts the binding's shape
  // in the source instead.
  const body = functionBody(readSource(APP_PATH), 'renderDocuments');

  // A bare identifier only. Word characters plus the quote, backtick and hyphen
  // characters that cannot appear inside this identifier are all excluded, so
  // `writeRepositoryFile` (a different function), `"write-file"` and
  // `"source-write-error"` (string data) are not counted. A trailing "." stays
  // allowed because `write.type` is a genuine use of the binding.
  const identifier = /(?<![A-Za-z0-9_$'"`\-.])write(?![A-Za-z0-9_$'"`-])/g;
  const declarations = [];
  const usages = [];
  for (const match of body.matchAll(identifier)) {
    const declared = /\b(?:let|const|var)\s+$/.test(body.slice(0, match.index));
    (declared ? declarations : usages).push(match.index);
  }

  const lines = body.split('\n');
  const lineAt = index => body.slice(0, index).split('\n').length;
  const found = index => 'line ' + lineAt(index) + ': ' + lines[lineAt(index) - 1].trim();
  const indentAt = index => lines[lineAt(index) - 1].match(/^[ \t]*/)[0].length;

  assert.equal(
    declarations.length, 1,
    'renderDocuments must declare the write binding exactly once; found ' +
    (declarations.length ? declarations.map(found).join(' | ') : 'no declaration')
  );
  // Non-vacuity: the binding the regression concerns has to be observable.
  assert.ok(usages.length > 0, 'expected renderDocuments to use the write binding');

  // The guarding block is the `if (editing)` that introduces WRITE FILE, i.e. the
  // last one before the first use. The first `if (editing)` in the body belongs to
  // the EDIT SOURCE toggle listener and would be the wrong anchor.
  const declaration = declarations[0];
  const guard = body.lastIndexOf('if (editing)', usages[0]);
  assert.notEqual(guard, -1, 'expected an if (editing) block that builds WRITE FILE');
  assert.ok(
    declaration < guard,
    'the write binding must be declared before the if (editing) block that builds WRITE FILE; ' +
    'declared at ' + found(declaration) + ' but the block starts at ' + found(guard)
  );
  for (const usage of usages) {
    assert.ok(usage > declaration, 'the write binding is used before it is declared at ' + found(usage));
  }
  // The enclosing-scope claim, as a nesting check: a binding declared inside the
  // `if (editing) {` body is indented deeper than the `if` statement itself.
  assert.ok(
    indentAt(declaration) <= indentAt(guard),
    'the write binding must live in the scope enclosing the if (editing) block, not inside it; ' +
    'declared at ' + found(declaration) + ' (indent ' + indentAt(declaration) + ') which is nested ' +
    'deeper than ' + found(guard) + ' (indent ' + indentAt(guard) + ')'
  );
});

test('STAGE FILE never implies COMMIT STAGED', () => {
  const body = functionBody(readSource(APP_PATH), 'stageRepositoryFile');
  assert.ok(body.includes('/api/stage-file'), 'STAGE FILE must call the stage endpoint');
  for (const forbidden of ['/api/commit', 'commitStagedRepository', 'makeCommitRequest']) {
    assert.equal(body.includes(forbidden), false, 'STAGE FILE must not reach commit control via ' + forbidden);
  }
});

test('a dirty source draft blocks every local Git action', () => {
  const dirty = {documentId:'git-a', originalContent:'before\n', content:'draft\n'};
  const clean = {documentId:'git-a', originalContent:'before\n', content:'before\n'};
  for (const action of ['stage-file', 'unstage-file', 'commit-staged']) {
    assert.equal(canRunLocalGitControl(action, dirty), false, action);
    assert.equal(canRunLocalGitControl(action, clean), true, action);
    assert.equal(canRunLocalGitControl(action, null), true, action);
  }
  // Local Git control is a closed set: there is no third action, remote or not.
  assert.deepEqual(Object.keys(LOCAL_GIT_ACTIONS).sort(), ['commit-staged', 'stage-file', 'unstage-file']);
  assert.equal(canRunLocalGitControl('push', clean), false);
});

test('bind wires the commit message draft and the COMMIT STAGED button', () => {
  const body = functionBody(readSource(APP_PATH), 'bind');
  assert.ok(body.includes('document.getElementById("commit-message").addEventListener("input"'), 'commit message input is unbound');
  assert.ok(body.includes('document.getElementById("commit-btn").addEventListener("click"'), 'COMMIT STAGED is unbound');
  assert.ok(body.includes('commitStagedRepository()'), 'COMMIT STAGED must dispatch the commit control');
  assert.ok(body.includes('showGitControlError'), 'commit failures must surface in #commit-error');
});

test('the commit message stays browser-local and never marks the workstation dirty', () => {
  const line = readSource(APP_PATH).split('\n')
    .find(text => text.includes('getElementById("commit-message")') && text.includes('addEventListener'));
  assert.ok(line, 'expected a commit message input listener');
  for (const forbidden of ['draft', 'saved', 'isDirty', 'updateDirtyIndicator', 'setSaveIndicator', 'saveChanges', 'pendingRefresh']) {
    assert.equal(line.includes(forbidden), false, 'commit message input must not touch ' + forbidden);
  }
  const commitAction = readSource(APP_PATH).split('\n')
    .find(text => text.includes('getElementById("commit-btn")'));
  assert.equal(commitAction.includes('saveChanges'), false, 'COMMIT STAGED must not imply SAVE CHANGES');
});

test('no push, fetch, or pull affordance exists in the browser layer', () => {
  for (const file of [APP_PATH, HTML_PATH]) {
    const text = readSource(file);
    for (const endpoint of ['/api/push', '/api/fetch', '/api/pull', 'data-action="push"']) {
      assert.equal(text.includes(endpoint), false, path.basename(file) + ' must not contain ' + endpoint);
    }
    assert.equal(/PUSH|FETCH|PULL/.test(text), false, path.basename(file) + ' must not label a control PUSH/FETCH/PULL');
  }
  const html = readSource(HTML_PATH);
  assert.equal(/push/i.test(html), false, 'index.html must not mention push at all');
  // Every remaining lowercase "push" in app.js is an Array.prototype.push call,
  // so a remote-synchronization endpoint cannot hide behind a matching name.
  const residual = readSource(APP_PATH).replace(/\.push\(/g, '');
  assert.equal(residual.includes('push'), false, 'app.js may only use push as an array method');
  // The assertion is not vacuous: the local-only controls it protects exist.
  assert.ok(html.includes('>COMMIT STAGED<'));
  assert.ok(html.includes('data-commit-control="true"'));
  assert.ok(readSource(APP_PATH).includes('"COMMIT STAGED"'));
});

test('staged summary text and source lookup stay pure snapshot readers', () => {
  assert.equal(stagedSummaryText(readyRepository({staged: []})), 'staged: 0\n(none)');
  assert.equal(stagedSummaryText(null), 'staged: 0\n(none)');
  const repository = readyRepository({
    staged: [{status:'M', path:'alpha.txt'}, {status:'A', path:'beta.txt'}],
    files: [modifiedSource()]
  });
  assert.equal(stagedSummaryText(repository), 'staged: 2\nM alpha.txt\nA beta.txt');
  assert.equal(sourceFileForPath(repository, 'alpha.txt').path, 'alpha.txt');
  assert.equal(sourceFileForPath(repository, 'missing.txt'), null);
  assert.equal(sourceFileForPath(null, 'alpha.txt'), null);
});
