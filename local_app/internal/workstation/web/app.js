"use strict";

function columnsForDepth(depth, compact=false) {
  if (compact) return 1;
  const safe = Math.max(0, Math.min(3, Number(depth) || 0));
  return 4 - safe;
}

function stableJSON(value) { return JSON.stringify(value); }
function statesEqual(a, b) { return stableJSON(a) === stableJSON(b); }
function clone(value) { return JSON.parse(JSON.stringify(value)); }

function shortcutMatches(shortcut, event) {
  if (!shortcut) return false;
  const parts = shortcut.toLowerCase().split("+").map(x => x.trim()).filter(Boolean);
  const key = parts.pop();
  const needAlt = parts.includes("alt");
  const needCtrl = parts.includes("ctrl") || parts.includes("control");
  const needShift = parts.includes("shift");
  const needMeta = parts.includes("meta") || parts.includes("cmd");
  return event.key.toLowerCase() === key && !!event.altKey === needAlt && !!event.ctrlKey === needCtrl && !!event.shiftKey === needShift && !!event.metaKey === needMeta;
}

let saved = null;
let draft = null;
let repository = null;
let stateHash = "";
let depth = 0;
let compact = false;
let activeDocument = null;
let editingDocument = null;
let repositoryEdit = null;
let activeTools = {workspace: 1, project: 1, branch: 1};
let drawerState = "closed";
let pendingRefresh = false;
let commitMessageDraft = "";
let commitError = {action:"", code:"", status:"", message:""};
let gitControlInFlight = null;

// Phase 4 local Git control is a third authority layer. It is never implied by
// SAVE CHANGES or WRITE FILE, and it never implies a commit. These names are the
// only local Git actions the browser can dispatch.
const LOCAL_GIT_ACTIONS = {
  "stage-file": "STAGE FILE",
  "unstage-file": "UNSTAGE FILE",
  "commit-staged": "COMMIT STAGED"
};

function isDirty() { return saved && draft && !statesEqual(saved, draft); }

function repositoryDraftDirty(edit) {
  return !!edit && edit.content !== edit.originalContent;
}

function canStartRepositoryEdit(currentEdit, nextDocumentId) {
  return !repositoryDraftDirty(currentEdit) || currentEdit.documentId === nextDocumentId;
}

function makeRepositoryWriteRequest(edit) {
  if (!edit) return null;
  return {
    repository_id: edit.repositoryId,
    document_id: edit.documentId,
    path: edit.path,
    expected_content_sha256: edit.expectedContentSHA256,
    content: edit.content
  };
}

// Phase 4 browser helpers below are pure: they take the current repository
// snapshot (and at most one file relationship) and answer a question. They
// never touch the DOM, never perform a request, and are unit-testable from Node
// without a browser. They only decide which affordance is offered; the server
// re-verifies every identity and every eligibility rule.

// changeKind mirrors the server-side change classification so the browser reads
// the same status the snapshot reports. A missing status is "none", not
// "modified": an absent status is never eligible.
function changeKind(status) {
  if (!status) return "none";
  if (typeof status === "object") return status.kind || "none";
  const text = String(status);
  if (!text) return "none";
  switch (text[0]) {
    case "A": return "added";
    case "M": return "modified";
    case "D": return "deleted";
    case "R": return "renamed";
    case "C": return "copied";
    case "T": return "type_changed";
    case "U": return "unmerged";
    default: return "other";
  }
}

// localGitControlReady is the browser-side precondition for binding any
// mutating local Git request: the snapshot must carry an identity the server
// also re-verifies. Without it the expected_* fields cannot be bound at all.
function localGitControlReady(repository) {
  if (!repository) return false;
  return !!repository.repository_id
    && !!repository.head_commit
    && !!repository.branch
    && !!repository.index_sha256
    && !repository.detached
    && !repository.unborn;
}

// eligibleTrackedSource is the shared file gate: an existing regular non-symlink
// text file that Git already tracks. Untracked, missing, symlink, non-regular,
// binary, and oversized targets are all rejected before any status is read.
function eligibleTrackedSource(file) {
  if (!file || !file.path) return false;
  if (!file.tracked) return false;
  if (file.untracked) return false;
  if (!file.exists) return false;
  if (!file.regular) return false;
  if (file.symlink) return false;
  if (file.binary) return false;
  if (file.too_large) return false;
  return true;
}

// canStageFile is true only for an unstaged tracked modification. A file that
// already has a staged entry is not stageable: that is UNSTAGE territory.
function canStageFile(file, repository) {
  if (!eligibleTrackedSource(file)) return false;
  if (!localGitControlReady(repository)) return false;
  if (file.staged_status) return false;
  return changeKind(file.unstaged_status) === "modified";
}

// canUnstageFile is true only for a staged tracked modification. Worktree
// content is irrelevant to an index-only restore.
function canUnstageFile(file, repository) {
  if (!file || !file.path || !file.tracked) return false;
  if (!localGitControlReady(repository)) return false;
  return changeKind(file.staged_status) === "modified";
}

// canCommit requires an attached branch and a non-empty staged snapshot. A
// detached or unborn HEAD has no local branch to advance.
function canCommit(repository) {
  if (!localGitControlReady(repository)) return false;
  return Array.isArray(repository.staged) && repository.staged.length > 0;
}

// Request builders bind the observed snapshot identity only. They never invent
// a field, never trim or extend the commit message, and return null when the
// request could not bind a complete identity.
function makeStageRequest(file, repository) {
  if (!canStageFile(file, repository)) return null;
  if (!file.content_sha256) return null;
  return {
    repository_id: repository.repository_id,
    path: file.path,
    expected_head_commit: repository.head_commit,
    expected_branch: repository.branch,
    expected_index_sha256: repository.index_sha256,
    expected_worktree_sha256: file.content_sha256
  };
}

function makeUnstageRequest(file, repository) {
  if (!canUnstageFile(file, repository)) return null;
  return {
    repository_id: repository.repository_id,
    path: file.path,
    expected_head_commit: repository.head_commit,
    expected_branch: repository.branch,
    expected_index_sha256: repository.index_sha256
  };
}

function makeCommitRequest(repository, commitMessage) {
  if (!canCommit(repository)) return null;
  if (typeof commitMessage !== "string" || !commitMessage) return null;
  return {
    repository_id: repository.repository_id,
    expected_head_commit: repository.head_commit,
    expected_branch: repository.branch,
    expected_index_sha256: repository.index_sha256,
    commit_message: commitMessage
  };
}

// A dirty repository source draft is an unfinished worktree mutation. It blocks
// every local Git action until it is written or explicitly cancelled, so a
// stage/unstage/commit can never race an unsaved source edit.
function canRunLocalGitControl(action, edit) {
  if (!Object.prototype.hasOwnProperty.call(LOCAL_GIT_ACTIONS, action)) return false;
  return !repositoryDraftDirty(edit);
}

function sourceFileForPath(repository, path) {
  if (!repository || !Array.isArray(repository.files) || !path) return null;
  return repository.files.find(file => file.path === path) || null;
}

function stagedSummaryText(repository) {
  const staged = repository && Array.isArray(repository.staged) ? repository.staged : [];
  if (!staged.length) return "staged: 0\n(none)";
  return `staged: ${staged.length}\n${staged.map(change => `${change.status} ${change.path}`).join("\n")}`;
}

function setSaveIndicator(status, label) {
  const el = document.getElementById("save-state");
  el.dataset.state = status;
  el.textContent = label;
}

function updateDirtyIndicator() {
  if (isDirty()) setSaveIndicator("dirty", "UNSAVED EDITS");
  else setSaveIndicator("saved", "SAVED");
}

async function loadAuthoritative() {
  if (repositoryDraftDirty(repositoryEdit)) throw new Error("SOURCE DRAFT exists. WRITE FILE or CANCEL SOURCE EDIT before refresh.");
  const res = await fetch("/api/state", {cache:"no-store"});
  if (!res.ok) throw new Error(await res.text());
  const envelope = await res.json();
  saved = envelope.state;
  draft = clone(saved);
  repository = envelope.repository || null;
  stateHash = envelope.state_sha256;
  repositoryEdit = null;
  if (!activeDocument && draft.documents.length) activeDocument = draft.documents[0].id;
  renderAll();
}

async function saveChanges() {
  if (!draft) return;
  const res = await fetch("/api/save", {
    method:"POST",
    headers:{"Content-Type":"application/json"},
    body:JSON.stringify(draft)
  });
  if (res.status === 409) {
    setSaveIndicator("conflicted", "CONFLICTED");
    throw new Error(await res.text());
  }
  if (!res.ok) throw new Error(await res.text());
  const envelope = await res.json();
  saved = envelope.state;
  draft = clone(saved);
  repository = envelope.repository || repository;
  stateHash = envelope.state_sha256;
  pendingRefresh = false;
  document.getElementById("refresh-guard").hidden = true;
  renderAll();
}

function clearEdits() {
  if (!saved) return;
  draft = clone(saved);
  editingDocument = null;
  syncProjectInputs();
  renderAll();
}

function requestRefresh() {
  if (repositoryDraftDirty(repositoryEdit)) {
    showSourceError(new Error("SOURCE DRAFT exists. WRITE FILE or CANCEL SOURCE EDIT before refresh."));
    return;
  }
  if (!isDirty()) return loadAuthoritative().catch(showError);
  pendingRefresh = true;
  document.getElementById("refresh-guard").hidden = false;
}

function setDepth(next) {
  depth = Math.max(0, Math.min(3, Number(next) || 0));
  renderGeometry();
  window.setTimeout(centerActiveDocument, 0);
}

function renderGeometry() {
  compact = window.innerWidth < 900;
  document.body.dataset.compact = String(compact);
  const columns = columnsForDepth(depth, compact);
  const workspace = document.getElementById("workspace");
  workspace.dataset.documentColumns = String(columns);
  workspace.style.setProperty("--doc-cols", String(columns));
  document.getElementById("left-sliders").dataset.openSliders = String(depth);
  document.getElementById("compact-banner").hidden = !compact;
  document.querySelectorAll("[data-hub-slider]").forEach(slider => {
    const level = Number(slider.dataset.level);
    const open = level <= depth;
    slider.hidden = !open;
    slider.dataset.open = String(open);
  });
}

function sortedDocuments() {
  return [...draft.documents].sort((a,b) => a.order - b.order);
}

function renderDocuments() {
  const stream = document.getElementById("document-stream");
  stream.replaceChildren();
  for (const doc of sortedDocuments()) {
    const article = document.createElement("article");
    article.className = "document";
    article.dataset.hubDocument = "true";
    article.dataset.documentId = doc.id;
    article.dataset.documentKind = doc.kind;
    article.dataset.documentOrder = String(doc.order);
    article.dataset.authority = "local";
    article.dataset.editable = String(!!doc.editable);
    article.dataset.active = String(activeDocument === doc.id);
    article.dataset.editing = String(editingDocument === doc.id);
    article.dataset.state = saved.documents.find(x => x.id === doc.id)?.content === doc.content ? "saved" : "draft";

    const header = document.createElement("header");
    const kind = document.createElement("span"); kind.className="kind"; kind.textContent=doc.kind;
    const title = document.createElement("strong"); title.textContent=doc.title;
    const status = document.createElement("span"); status.className="doc-state"; status.textContent=article.dataset.state.toUpperCase();
    header.append(kind,title,status);
    if (doc.editable) {
      const edit = document.createElement("button");
      edit.type="button";
      edit.textContent = editingDocument === doc.id ? "VIEW" : "EDIT";
      edit.dataset.action = "toggle-edit";
      edit.addEventListener("click", ev => { ev.stopPropagation(); editingDocument = editingDocument === doc.id ? null : doc.id; activeDocument=doc.id; renderDocuments(); });
      header.append(edit);
    }
    article.append(header);
    article.addEventListener("click", () => {
      activeDocument=doc.id;
      if (!shouldRerenderAfterDocumentClick(doc.id, editingDocument)) return;
      renderDocuments();
    });

    if (editingDocument === doc.id && doc.editable) {
      const textarea = document.createElement("textarea");
      textarea.value = doc.content;
      textarea.setAttribute("aria-label", `Edit ${doc.title}`);
      textarea.addEventListener("input", () => {
        doc.content = textarea.value;
        article.dataset.state = "draft";
        status.textContent = "DRAFT";
        updateDirtyIndicator();
      });
      article.append(textarea);
    } else {
      const pre = document.createElement("pre");
      pre.className="fence";
      pre.dataset.language=doc.kind;
      pre.textContent=doc.content;
      article.append(pre);
    }
    stream.append(article);
  }
  if (repository) {
    for (const doc of repository.documents || []) {
      const editing = repositoryEdit?.documentId === doc.id;
      const sourceDirty = editing && repositoryDraftDirty(repositoryEdit);
      const article = document.createElement("article");
      article.className = "document";
      article.dataset.hubDocument = "true";
      article.dataset.documentId = doc.id;
      article.dataset.documentKind = doc.kind;
      article.dataset.authority = "repository";
      article.dataset.repositoryId = repository.repository_id;
      article.dataset.sourcePath = doc.path;
      article.dataset.editable = "true";
      article.dataset.active = String(activeDocument === doc.id);
      article.dataset.editing = String(editing);
      article.dataset.state = editing ? (sourceDirty ? "source-draft" : "source-edit") : "repository";

      const header = document.createElement("header");
      const kind = document.createElement("span"); kind.className="kind"; kind.textContent=doc.kind;
      const title = document.createElement("strong"); title.textContent=doc.path;
      const status = document.createElement("span");
      status.className="doc-state";
      status.textContent = editing ? (sourceDirty ? "SOURCE DRAFT" : "SOURCE EDIT") : "GIT WORKTREE";
      header.append(kind,title,status);

      const edit = document.createElement("button");
      edit.type="button";
      edit.dataset.action="toggle-source-edit";
      edit.textContent = editing ? "CANCEL SOURCE EDIT" : "EDIT SOURCE";
      edit.addEventListener("click", ev => {
        ev.stopPropagation();
        activeDocument=doc.id;
        if (editing) {
          repositoryEdit=null;
        } else {
          if (!canStartRepositoryEdit(repositoryEdit, doc.id)) {
            window.alert("SOURCE DRAFT exists. WRITE FILE or CANCEL SOURCE EDIT before editing another source.");
            return;
          }
          repositoryEdit={
            repositoryId: repository.repository_id,
            documentId: doc.id,
            path: doc.path,
            expectedContentSHA256: doc.content_sha256,
            originalContent: doc.content,
            content: doc.content,
            status: "editing",
            errorCode: ""
          };
        }
        renderDocuments();
      });
      header.append(edit);

      let write = null;
      if (editing) {
        write = document.createElement("button");
        write.type="button";
        write.dataset.action="write-file";
        write.textContent="WRITE FILE";
        write.disabled=!sourceDirty || repositoryEdit.status === "writing";
        write.addEventListener("click", ev => {
          ev.stopPropagation();
          writeRepositoryFile().catch(showSourceError);
        });
        header.append(write);
      }

      article.append(header);
      article.addEventListener("click", () => {
        activeDocument=doc.id;
        if (editing) return;
        renderDocuments();
      });

      if (editing) {
        const textarea = document.createElement("textarea");
        textarea.value=repositoryEdit.content;
        textarea.disabled=repositoryEdit.status === "writing";
        textarea.setAttribute("aria-label", `Edit source ${doc.path}`);
        textarea.addEventListener("input", ev => {
          repositoryEdit.content=ev.target.value;
          repositoryEdit.status="editing";
          repositoryEdit.errorCode="";
          article.dataset.state=repositoryDraftDirty(repositoryEdit) ? "source-draft" : "source-edit";
          status.textContent=repositoryDraftDirty(repositoryEdit) ? "SOURCE DRAFT" : "SOURCE EDIT";
          write.disabled=!repositoryDraftDirty(repositoryEdit);
        });
        article.append(textarea);
        if(repositoryEdit.errorCode){
          const error=document.createElement("div");
          error.className="source-write-error";
          error.dataset.errorCode=repositoryEdit.errorCode;
          error.textContent=`WRITE BLOCKED: ${repositoryEdit.errorCode}`;
          article.append(error);
        }
      } else {
        const pre = document.createElement("pre");
        pre.className="fence";
        pre.dataset.language=doc.kind;
        pre.textContent=doc.content;
        article.append(pre);
      }
      stream.append(article);
    }
  }
  document.getElementById("active-document-label").textContent=`active: ${activeDocument || "—"}`;
}

async function writeRepositoryFile() {
  if (!repositoryEdit || !repositoryDraftDirty(repositoryEdit)) return;
  repositoryEdit.status="writing";
  repositoryEdit.errorCode="";
  renderDocuments();

  const res = await fetch("/api/write-file", {
    method:"POST",
    headers:{"Content-Type":"application/json"},
    body:JSON.stringify(makeRepositoryWriteRequest(repositoryEdit))
  });
  if (!res.ok) {
    let code="WRITE_FAILED";
    let message=`WRITE FILE failed with HTTP ${res.status}`;
    try {
      const body=await res.json();
      code=body.code || code;
      message=body.error || message;
    } catch (_) {}
    repositoryEdit.status="conflicted";
    repositoryEdit.errorCode=code;
    renderDocuments();
    const error=new Error(message);
    error.code=code;
    throw error;
  }

  const result=await res.json();
  repository=result.repository;
  repositoryEdit=null;
  renderAll();
}

function showSourceError(err) {
  console.error(err);
  if(repositoryEdit){
    repositoryEdit.status="conflicted";
    repositoryEdit.errorCode=err.code || repositoryEdit.errorCode || "WRITE_FAILED";
    renderDocuments();
  }
  window.alert(err.message || String(err));
}

// Phase 4 local Git control. Every failure — 409 conflict, 400 invalid
// request, 422 unsupported target/state, 404, 500 — is surfaced with its code
// in #commit-error, and the local repository snapshot is deliberately NOT
// replaced, so a conflict can never silently refresh user state away.
function setCommitControlError(action, code, status, message) {
  commitError = {action: action || "", code: code || "", status: status || "", message: message || ""};
  renderCommitPanel();
}

function showGitControlError(err) {
  console.error(err);
  setCommitControlError(commitError.action, commitError.code || "GIT_CONTROL_FAILED", commitError.status, String(err.message || err));
  window.alert(err.message || String(err));
}

function guardLocalGitControl(action) {
  if (canRunLocalGitControl(action, repositoryEdit)) return true;
  window.alert(`SOURCE DRAFT exists. WRITE FILE or CANCEL SOURCE EDIT before ${LOCAL_GIT_ACTIONS[action]}.`);
  return false;
}

async function runLocalGitControl(action, endpoint, body, path) {
  if (!body) {
    setCommitControlError(action, "INVALID_REQUEST", "", `${LOCAL_GIT_ACTIONS[action]} could not bind the current repository identity.`);
    return;
  }
  gitControlInFlight = {action, path: path || ""};
  renderAll();
  let res;
  try {
    res = await fetch(endpoint, {
      method:"POST",
      headers:{"Content-Type":"application/json"},
      body:JSON.stringify(body)
    });
  } catch (err) {
    gitControlInFlight = null;
    setCommitControlError(action, "NETWORK_ERROR", "", String(err.message || err));
    return;
  }
  if (!res.ok) {
    let code = "GIT_CONTROL_FAILED";
    let message = `${LOCAL_GIT_ACTIONS[action]} failed with HTTP ${res.status}`;
    try {
      const payload = await res.json();
      code = payload.code || code;
      message = payload.error || message;
    } catch (_) {}
    gitControlInFlight = null;
    setCommitControlError(action, code, String(res.status), message);
    return;
  }
  const result = await res.json();
  repository = result.repository;
  gitControlInFlight = null;
  if (action === "commit-staged") commitMessageDraft = "";
  setCommitControlError("", "", "", "");
  renderAll();
}

async function stageRepositoryFile(file) {
  if (!guardLocalGitControl("stage-file")) return;
  await runLocalGitControl("stage-file", "/api/stage-file", makeStageRequest(file, repository), file && file.path);
}

async function unstageRepositoryFile(file) {
  if (!guardLocalGitControl("unstage-file")) return;
  await runLocalGitControl("unstage-file", "/api/unstage-file", makeUnstageRequest(file, repository), file && file.path);
}

async function commitStagedRepository() {
  if (!guardLocalGitControl("commit-staged")) return;
  await runLocalGitControl("commit-staged", "/api/commit", makeCommitRequest(repository, commitMessageDraft), "");
}

function localGitActionButton(action, file) {
  const button = document.createElement("button");
  button.type = "button";
  button.dataset.action = action;
  button.dataset.sourcePath = file.path;
  button.dataset.commitControl = "true";
  button.textContent = LOCAL_GIT_ACTIONS[action];
  button.disabled = !!gitControlInFlight;
  button.addEventListener("click", ev => {
    ev.stopPropagation();
    if (action === "stage-file") stageRepositoryFile(file).catch(showGitControlError);
    else unstageRepositoryFile(file).catch(showGitControlError);
  });
  return button;
}

function renderSpines() {
  for (const slider of draft.sliders) {
    const spine = document.querySelector(`[data-tool-spine="${slider.id}"]`);
    spine.replaceChildren();
    for (const tool of slider.tools) {
      const button = document.createElement("button");
      button.type="button";
      button.dataset.toolSlot=String(tool.slot);
      button.dataset.toolTarget=tool.target;
      button.dataset.enabled=String(!!tool.enabled);
      button.dataset.active=String(activeTools[slider.id] === tool.slot);
      button.disabled=!tool.enabled;
      button.title = `${tool.label}${tool.shortcut ? ` (${tool.shortcut})` : ""}`;
      button.setAttribute("aria-label", button.title);
      button.textContent=tool.icon || String(tool.slot);
      button.addEventListener("click", () => {
        activeTools[slider.id]=tool.slot;
        renderSpines();
        renderSliderViews();
      });
      spine.append(button);
    }
  }
}

function renderSliderViews() {
  for (const slider of draft.sliders) {
    const tool = slider.tools.find(x => x.slot === activeTools[slider.id]) || slider.tools[0];
    const target = document.querySelector(`[data-slider-view="${slider.id}"]`);
    target.replaceChildren();
    const strong=document.createElement("strong"); strong.textContent=tool.label;
    const p=document.createElement("p"); p.textContent=`${tool.target} · ${tool.tooltip || "No tooltip"}`;
    const hint=document.createElement("p"); hint.className="hint"; hint.textContent=`Slot ${tool.slot} · ${tool.shortcut || "no shortcut"}`;
    target.append(strong,p,hint);
  }
}

function renderToolEditor() {
  const sliderID=document.getElementById("tool-slider-select").value;
  const slider=draft.sliders.find(x=>x.id===sliderID);
  const root=document.getElementById("tool-editor");
  root.replaceChildren();
  if (!slider) return;
  for (const tool of slider.tools) {
    const row=document.createElement("div"); row.className="tool-edit-row";
    const slot=document.createElement("strong"); slot.textContent=String(tool.slot);
    const icon=input("text",tool.icon, v=>{tool.icon=v; changed();}); icon.setAttribute("aria-label",`Slot ${tool.slot} icon`);
    const label=input("text",tool.label, v=>{tool.label=v; changed();}); label.setAttribute("aria-label",`Slot ${tool.slot} label`);
    const target=input("text",tool.target, v=>{tool.target=v; changed();}); target.className="wide"; target.setAttribute("aria-label",`Slot ${tool.slot} target`);
    const shortcut=input("text",tool.shortcut, v=>{tool.shortcut=v; changed();}); shortcut.className="wide"; shortcut.setAttribute("aria-label",`Slot ${tool.slot} shortcut`);
    const enabled=document.createElement("label"); enabled.className="wide"; const check=document.createElement("input"); check.type="checkbox"; check.checked=tool.enabled; check.addEventListener("change",()=>{tool.enabled=check.checked;changed();}); enabled.append(check,document.createTextNode(" enabled"));
    row.append(slot,icon,label,target,shortcut,enabled); root.append(row);
  }
}

function input(type,value,onInput){const el=document.createElement("input");el.type=type;el.value=value;el.addEventListener("input",()=>onInput(el.value));return el;}
function changed(){updateDirtyIndicator();renderSpines();renderSliderViews();}

function syncProjectInputs() {
  document.getElementById("project-name-input").value=draft.project.name;
  document.getElementById("project-root-input").value=draft.project.root;
  document.getElementById("project-branch-input").value=draft.project.branch;
}

function renderMetadata() {
  document.getElementById("revision").textContent=`revision ${draft.revision}`;
  document.getElementById("workspace-name").textContent=draft.project.workspace;
  document.getElementById("project-name").textContent=draft.project.name;
  const branch = repository ? repositoryDisplayBranch(repository) : draft.project.branch;
  document.getElementById("branch-name").textContent=branch;
  document.getElementById("project-root").textContent=`repository root: ${repository?.root || "—"}`;
  document.getElementById("state-hash").textContent=`workstation state sha256: ${stateHash || "—"}`;
}

function repositoryDisplayBranch(repo) {
  if (repo.branch) return repo.branch;
  if (repo.detached) return "DETACHED";
  if (repo.unborn) return "UNBORN";
  return "UNKNOWN";
}

function shouldRerenderAfterDocumentClick(docId, editingId) {
  return editingId !== docId;
}

function renderRepository() {
  const panel = document.getElementById("repository-status");
  if (!repository) {
    panel.hidden = true;
    return;
  }
  panel.hidden = false;
  panel.dataset.repositoryId = repository.repository_id;
  panel.dataset.repositoryClean = String(repository.clean);
  panel.dataset.repositoryDetached = String(repository.detached);
  panel.dataset.repositoryUnborn = String(repository.unborn);
  document.getElementById("repository-id").textContent=`repository: ${repository.repository_id}`;
  document.getElementById("repository-head").textContent=`HEAD: ${repository.head_commit || "UNBORN"}`;
  document.getElementById("repository-branch").textContent=`branch: ${repositoryDisplayBranch(repository)}`;
  document.getElementById("repository-clean").textContent=`working tree: ${repository.clean ? "CLEAN" : "DIRTY"}`;
  document.getElementById("repository-counts").textContent=`staged: ${repository.staged.length} · unstaged: ${repository.unstaged.length} · untracked: ${repository.untracked.length}`;
  document.getElementById("repository-hash").textContent=`repo snapshot sha256: ${repository.snapshot_sha256}`;
  document.getElementById("staged-summary").textContent=`Staged changes (${repository.staged.length})`;
  document.getElementById("unstaged-summary").textContent=`Unstaged changes (${repository.unstaged.length})`;
  document.getElementById("untracked-summary").textContent=`Untracked files (${repository.untracked.length})`;
  document.getElementById("history-summary").textContent=`Recent local history (${repository.history.length})`;
  document.getElementById("relationships-summary").textContent=`Source relationships (${repository.files.length})`;
  renderChangeList(document.getElementById("staged-changes"), repository.staged, "unstage-file");
  renderChangeList(document.getElementById("unstaged-changes"), repository.unstaged, "stage-file");
  document.getElementById("staged-diff").textContent=repository.staged_diff || "<none>";
  document.getElementById("unstaged-diff").textContent=repository.unstaged_diff || "<none>";
  const untracked = document.getElementById("untracked-files"); untracked.replaceChildren();
  for (const path of repository.untracked) { const row=document.createElement("div"); row.className="repository-change"; row.textContent=`? ${path}`; untracked.append(row); }
  const history=document.getElementById("repository-history"); history.replaceChildren();
  for (const item of repository.history) { const row=document.createElement("div"); row.className="repository-history-row"; row.textContent=`${item.short} · ${item.date} · ${item.author} · ${item.subject}`; history.append(row); }
  const relationships=document.getElementById("source-relationships"); relationships.replaceChildren();
  for (const file of repository.files) {
    const row=document.createElement("div"); row.className="repository-file"; row.dataset.binary=String(file.binary); row.dataset.sourceId=file.id; row.dataset.sourcePath=file.path;
    const flags=[];
    if(file.staged_status) flags.push(`staged:${file.staged_status}`);
    if(file.unstaged_status) flags.push(`unstaged:${file.unstaged_status}`);
    if(file.untracked) flags.push("untracked");
    if(file.binary) flags.push("binary");
    if(file.too_large) flags.push("too-large");
    if(file.symlink) flags.push(`symlink:${file.symlink_target}`);
    const label=document.createElement("span");
    label.textContent=`${flags.length ? `[${flags.join(", ")}] ` : ""}${file.path}${file.previous_path ? ` ← ${file.previous_path}` : ""}${file.document_id ? ` → ${file.document_id}` : ""}`;
    row.append(label);
    if(canStageFile(file, repository)) row.append(localGitActionButton("stage-file", file));
    if(canUnstageFile(file, repository)) row.append(localGitActionButton("unstage-file", file));
    relationships.append(row);
  }
}

function renderCommitPanel() {
  const panel = document.getElementById("commit-panel");
  if (!panel) return;
  panel.hidden = !repository;
  const ready = canCommit(repository);
  panel.dataset.commitReady = String(ready);
  panel.dataset.commitInFlight = String(!!gitControlInFlight);
  document.getElementById("commit-branch").textContent = `branch: ${repository ? repositoryDisplayBranch(repository) : "—"}`;
  document.getElementById("commit-head").textContent = `HEAD: ${repository ? (repository.head_commit || "UNBORN") : "—"}`;
  document.getElementById("commit-index").textContent = `index sha256: ${repository && repository.index_sha256 ? repository.index_sha256 : "—"}`;
  const summary = document.getElementById("commit-staged-summary");
  summary.dataset.stagedCount = String(repository && Array.isArray(repository.staged) ? repository.staged.length : 0);
  summary.textContent = stagedSummaryText(repository);
  // The commit message is a browser-local draft. It is never workstation state,
  // so it survives every re-render and never marks the workstation dirty.
  const message = document.getElementById("commit-message");
  if (message.value !== commitMessageDraft) message.value = commitMessageDraft;
  message.disabled = !!(gitControlInFlight && gitControlInFlight.action === "commit-staged");
  const button = document.getElementById("commit-btn");
  button.disabled = !ready || !!gitControlInFlight;
  const error = document.getElementById("commit-error");
  error.hidden = !commitError.code;
  error.dataset.errorCode = commitError.code;
  error.dataset.errorStatus = commitError.status;
  error.dataset.errorAction = commitError.action;
  error.textContent = commitError.code
    ? `${commitError.action ? LOCAL_GIT_ACTIONS[commitError.action] || commitError.action : "LOCAL GIT CONTROL"} BLOCKED: ${commitError.code}${commitError.message ? ` — ${commitError.message}` : ""}`
    : "";
}

function renderChangeList(root, changes, action) {
  root.replaceChildren();
  for (const change of changes) {
    const row=document.createElement("div"); row.className="repository-change";
    row.dataset.sourcePath=change.path;
    const label=document.createElement("span");
    label.textContent=`${change.status} ${change.previous_path ? `${change.previous_path} → ` : ""}${change.path}`;
    row.append(label);
    const file=sourceFileForPath(repository, change.path);
    if(action === "stage-file" && canStageFile(file, repository)) row.append(localGitActionButton("stage-file", file));
    if(action === "unstage-file" && canUnstageFile(file, repository)) row.append(localGitActionButton("unstage-file", file));
    root.append(row);
  }
}

function renderAll() {
  renderGeometry();
  renderMetadata();
  renderRepository();
  renderCommitPanel();
  syncProjectInputs();
  renderSpines();
  renderSliderViews();
  renderDocuments();
  renderDrawer();
  renderToolEditor();
  updateDirtyIndicator();
}

function centerActiveDocument() {
  const el=document.querySelector(`[data-document-id="${CSS.escape(activeDocument || "")}"]`);
  if (!el) return;
  const behavior = window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth";
  el.scrollIntoView({block:"center",inline:"center",behavior});
}

function setDrawer(next) { drawerState=next; renderDrawer(); if(next.includes("system")) renderToolEditor(); }
function renderDrawer() {
  const drawer=document.getElementById("right-drawer"); drawer.dataset.drawerState=drawerState;
  const project=document.querySelector('[data-drawer-pane="project"]');
  const system=document.querySelector('[data-drawer-pane="system"]');
  project.hidden = !(drawerState === "project" || drawerState === "both");
  system.hidden = !(drawerState === "system" || drawerState === "both");
}

function showError(err) { console.error(err); setSaveIndicator("conflicted","ERROR"); window.alert(err.message || String(err)); }

function bind() {
  document.querySelectorAll("[data-depth]").forEach(btn=>btn.addEventListener("click",()=>setDepth(btn.dataset.depth)));
  document.getElementById("save-btn").addEventListener("click",()=>saveChanges().catch(showError));
  document.getElementById("clear-btn").addEventListener("click",clearEdits);
  document.getElementById("refresh-btn").addEventListener("click",requestRefresh);
  document.getElementById("settings-quick").addEventListener("click",()=>setDrawer(drawerState === "both" || drawerState === "system" ? "closed" : "both"));
  document.getElementById("documents-quick").addEventListener("click",()=>{setDepth(0);document.getElementById("workspace").focus?.();});
  document.getElementById("search-quick").addEventListener("click",()=>{setDepth(1);const s=draft.sliders[0];const tool=s.tools.find(x=>x.target==="search");if(tool){activeTools.workspace=tool.slot;renderSpines();renderSliderViews();}});
  document.querySelectorAll("[data-close-drawer]").forEach(btn=>btn.addEventListener("click",()=>setDrawer("closed")));
  document.querySelectorAll("[data-refresh-choice]").forEach(btn=>btn.addEventListener("click",async()=>{
    const choice=btn.dataset.refreshChoice;
    if(choice==="cancel"){pendingRefresh=false;document.getElementById("refresh-guard").hidden=true;return;}
    if(choice==="save"){try{await saveChanges();await loadAuthoritative();}catch(err){showError(err);}return;}
    if(choice==="clear"){clearEdits();await loadAuthoritative().catch(showError);document.getElementById("refresh-guard").hidden=true;}
  }));
  document.getElementById("project-name-input").addEventListener("input",e=>{draft.project.name=e.target.value;updateDirtyIndicator();renderMetadata();});
  document.getElementById("project-root-input").addEventListener("input",e=>{draft.project.root=e.target.value;updateDirtyIndicator();renderMetadata();});
  document.getElementById("project-branch-input").addEventListener("input",e=>{draft.project.branch=e.target.value;updateDirtyIndicator();renderMetadata();});
  // The commit message is a browser-local draft only: it never touches draft/saved
  // state, so it can never mark the workstation dirty or enable SAVE CHANGES.
  document.getElementById("commit-message").addEventListener("input",e=>{commitMessageDraft=e.target.value;});
  document.getElementById("commit-btn").addEventListener("click",()=>commitStagedRepository().catch(showGitControlError));
  document.getElementById("tool-slider-select").addEventListener("change",renderToolEditor);
  window.addEventListener("resize",renderGeometry);
  window.addEventListener("keydown",event=>{
    if(!draft || event.target.matches?.("input,textarea,select")) return;
    for(const slider of draft.sliders){
      for(const tool of slider.tools){
        if(tool.enabled && shortcutMatches(tool.shortcut,event)){
          event.preventDefault(); activeTools[slider.id]=tool.slot; setDepth(Math.max(depth,slider.level)); renderSpines();renderSliderViews(); return;
        }
      }
    }
  });
}

if (typeof document !== "undefined") {
  document.addEventListener("DOMContentLoaded",()=>{bind();loadAuthoritative().catch(showError);});
}
if (typeof module !== "undefined") module.exports={columnsForDepth,statesEqual,shortcutMatches,repositoryDisplayBranch,shouldRerenderAfterDocumentClick,repositoryDraftDirty,canStartRepositoryEdit,makeRepositoryWriteRequest,LOCAL_GIT_ACTIONS,changeKind,localGitControlReady,eligibleTrackedSource,canStageFile,canUnstageFile,canCommit,canRunLocalGitControl,makeStageRequest,makeUnstageRequest,makeCommitRequest,sourceFileForPath,stagedSummaryText};
