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
let stateHash = "";
let depth = 0;
let compact = false;
let activeDocument = null;
let editingDocument = null;
let activeTools = {workspace: 1, project: 1, branch: 1};
let drawerState = "closed";
let pendingRefresh = false;

function isDirty() { return saved && draft && !statesEqual(saved, draft); }

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
  const res = await fetch("/api/state", {cache:"no-store"});
  if (!res.ok) throw new Error(await res.text());
  const envelope = await res.json();
  saved = envelope.state;
  draft = clone(saved);
  stateHash = envelope.state_sha256;
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
    article.addEventListener("click", () => { activeDocument=doc.id; renderDocuments(); });

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
  document.getElementById("active-document-label").textContent=`active: ${activeDocument || "—"}`;
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
  document.getElementById("branch-name").textContent=draft.project.branch;
  document.getElementById("project-root").textContent=`root: ${draft.project.root}`;
  document.getElementById("state-hash").textContent=`state sha256: ${stateHash || "—"}`;
}

function renderAll() {
  renderGeometry();
  renderMetadata();
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
if (typeof module !== "undefined") module.exports={columnsForDepth,statesEqual,shortcutMatches};
