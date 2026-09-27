package workstation

import (
	"bytes"
	"encoding/json"
	"html/template"
	"sort"
)

const exportTemplate = `<!doctype html>
<html lang="en" data-hub-projection="static" data-authority="projection">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.State.Project.Name}} — LLM-Hub Static Projection</title>
<style>
:root{color-scheme:dark;--bg:#090d10;--panel:#11171c;--line:#33404a;--text:#e7eef2;--muted:#91a0aa;--accent:#7fd9df;--warn:#f0c66a;--max:42rem}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.55 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}header{position:sticky;top:0;background:#0b1014;border-bottom:1px solid var(--line);padding:10px 16px;display:flex;gap:18px;align-items:center;z-index:2}.badge{border:1px solid var(--warn);color:var(--warn);padding:3px 8px}.muted{color:var(--muted)}main{padding:18px}.meta{border:1px solid var(--line);padding:12px;margin-bottom:16px;background:#0c1216}.documents{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,22rem),1fr));gap:12px;align-items:start}.doc{border:1px solid var(--line);background:var(--panel);max-width:var(--max);width:100%;justify-self:center}.doc>header{position:static;padding:7px 10px;background:#0d1317;border-bottom:1px solid var(--line);font-size:12px}.doc pre{margin:0;padding:12px;white-space:pre-wrap;overflow:auto;color:#dce9df}.kind{color:var(--accent)}script{display:none}</style>
</head>
<body>
<header><strong>LLM-HUB STATIC PROJECTION</strong><span class="badge">READ ONLY</span><span class="muted">source authority: LOCAL · projected revision {{.State.Revision}}</span></header>
<main data-project="{{.State.Project.Name}}" data-static-projection="true">
<section class="meta" data-project-metadata="true">
<strong>{{.State.Project.Name}}</strong><br>
<span class="muted">workspace={{.State.Project.Workspace}} · branch={{.State.Project.Branch}} · root={{.State.Project.Root}} · state_sha256={{.Hash}}</span>
</section>
<section class="documents" data-document-stream="true">
{{range .Documents}}<article class="doc" data-hub-document="true" data-document-id="{{.ID}}" data-document-kind="{{.Kind}}" data-document-order="{{.Order}}" data-state="saved" data-authority="projection" data-editable="false">
<header><span class="kind">{{.Kind}}</span> ─ {{.Title}}</header><pre>{{.Content}}</pre>
</article>{{end}}
</section>
<script id="hub-state" type="application/json">{{.StateJSON}}</script>
</main></body></html>`

type exportData struct {
	State     State
	Documents []Document
	Hash      string
	StateJSON template.JS
}

func ExportHTML(state State, hash string) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	docs := append([]Document(nil), state.Documents...)
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].Order < docs[j].Order })
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	t, err := template.New("projection").Parse(exportTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, exportData{State: state, Documents: docs, Hash: hash, StateJSON: template.JS(raw)}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
