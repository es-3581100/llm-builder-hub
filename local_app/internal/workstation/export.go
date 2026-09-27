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
:root{color-scheme:dark;--bg:#090d10;--panel:#11171c;--line:#33404a;--text:#e7eef2;--muted:#91a0aa;--accent:#7fd9df;--warn:#f0c66a;--good:#6fd49a;--max:42rem}*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.55 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}header.top{position:sticky;top:0;background:#0b1014;border-bottom:1px solid var(--line);padding:10px 16px;display:flex;gap:18px;align-items:center;z-index:2}.badge{border:1px solid var(--warn);color:var(--warn);padding:3px 8px}.muted{color:var(--muted)}main{padding:18px}.meta,.repo{border:1px solid var(--line);padding:12px;margin-bottom:16px;background:#0c1216}.repo strong{color:var(--accent)}.repo-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(16rem,1fr));gap:6px 16px;margin:8px 0}.repo details{border-top:1px solid var(--line);padding:8px 0}.repo pre{white-space:pre-wrap;overflow:auto;max-height:28rem;background:#0a1014;padding:8px}.documents{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,22rem),1fr));gap:12px;align-items:start}.doc{border:1px solid var(--line);background:var(--panel);max-width:var(--max);width:100%;justify-self:center}.doc>header{position:static;padding:7px 10px;background:#0d1317;border-bottom:1px solid var(--line);font-size:12px}.doc pre{margin:0;padding:12px;white-space:pre-wrap;overflow:auto;color:#dce9df}.kind{color:var(--accent)}.source{color:var(--muted);font-size:11px}script{display:none}</style>
</head>
<body>
<header class="top"><strong>LLM-HUB STATIC PROJECTION</strong><span class="badge">READ ONLY</span><span class="muted">workstation authority: LOCAL · projected revision {{.State.Revision}}</span></header>
<main data-project="{{.State.Project.Name}}" data-static-projection="true">
<section class="meta" data-project-metadata="true">
<strong>{{.State.Project.Name}}</strong><br>
<span class="muted">workspace={{.State.Project.Workspace}} · workstation_root_label={{.State.Project.Root}} · state_sha256={{.Hash}}</span>
</section>
{{if .Repository}}
<section class="repo" data-repository-authority="git" data-repository-id="{{.Repository.RepositoryID}}" data-repository-clean="{{.Repository.Clean}}" data-repository-detached="{{.Repository.Detached}}">
<strong>LOCAL GIT REPOSITORY · READ-ONLY INSPECTION</strong>
<div class="repo-grid">
<span>repository_id={{.Repository.RepositoryID}}</span>
<span>root={{.Repository.Root}}</span>
<span>HEAD={{if .Repository.HeadCommit}}{{.Repository.HeadCommit}}{{else}}UNBORN{{end}}</span>
<span>branch={{if .Repository.Branch}}{{.Repository.Branch}}{{else if .Repository.Detached}}DETACHED{{else}}UNBORN{{end}}</span>
<span>working_tree={{if .Repository.Clean}}CLEAN{{else}}DIRTY{{end}}</span>
<span>staged={{len .Repository.Staged}} · unstaged={{len .Repository.Unstaged}} · untracked={{len .Repository.Untracked}}</span>
<span>repository_snapshot_sha256={{.Repository.SnapshotSHA256}}</span>
</div>
<details><summary>Staged changes ({{len .Repository.Staged}})</summary>{{range .Repository.Staged}}<div>{{.Status}} {{if .PreviousPath}}{{.PreviousPath}} → {{end}}{{.Path}}</div>{{end}}<pre>{{.Repository.StagedDiff}}</pre></details>
<details><summary>Unstaged changes ({{len .Repository.Unstaged}})</summary>{{range .Repository.Unstaged}}<div>{{.Status}} {{if .PreviousPath}}{{.PreviousPath}} → {{end}}{{.Path}}</div>{{end}}<pre>{{.Repository.UnstagedDiff}}</pre></details>
<details><summary>Untracked files ({{len .Repository.Untracked}})</summary>{{range .Repository.Untracked}}<div>? {{.}}</div>{{end}}</details>
<details><summary>Recent local history ({{len .Repository.History}})</summary>{{range .Repository.History}}<div><code>{{.Short}}</code> {{.Date}} · {{.Author}} · {{.Subject}}</div>{{end}}</details>
<details><summary>Source relationships ({{len .Repository.Files}})</summary>{{range .Repository.Files}}<div data-source-file="{{.Path}}" data-source-id="{{.ID}}">{{if .StagedStatus}}[staged {{.StagedStatus}}] {{end}}{{if .UnstagedStatus}}[unstaged {{.UnstagedStatus}}] {{end}}{{if .Untracked}}[untracked] {{end}}{{if .Binary}}[binary] {{end}}{{if .TooLarge}}[too-large] {{end}}{{.Path}}{{if .PreviousPath}} ← {{.PreviousPath}}{{end}}{{if .DocumentID}} → {{.DocumentID}}{{end}}</div>{{end}}</details>
</section>
{{end}}
<section class="documents" data-document-stream="true">
{{range .Documents}}<article class="doc" data-hub-document="true" data-document-id="{{.ID}}" data-document-kind="{{.Kind}}" data-document-order="{{.Order}}" data-state="saved" data-authority="projection" data-editable="false">
<header><span class="kind">{{.Kind}}</span> — {{.Title}}</header><pre>{{.Content}}</pre>
</article>{{end}}
{{range .RepositoryDocuments}}<article class="doc" data-hub-document="true" data-document-id="{{.ID}}" data-document-kind="{{.Kind}}" data-state="repository" data-authority="repository" data-editable="false" data-source-path="{{.Path}}" data-repository-id="{{.RepositoryID}}">
<header><span class="kind">{{.Kind}}</span> — {{.Title}} <span class="source">git_worktree</span></header><pre>{{.Content}}</pre>
</article>{{end}}
</section>
<script id="hub-state" type="application/json">{{.StateJSON}}</script>
{{if .Repository}}<script id="hub-repository" type="application/json">{{.RepositoryJSON}}</script>{{end}}
</main></body></html>`

type exportData struct {
	State               State
	Documents           []Document
	Hash                string
	StateJSON           template.JS
	Repository          *RepositorySnapshot
	RepositoryDocuments []RepositoryDocument
	RepositoryJSON      template.JS
}

// ExportHTML preserves the Phase-1 API for callers that do not configure Git inspection.
func ExportHTML(state State, hash string) ([]byte, error) {
	return ExportHTMLWithRepository(state, hash, nil)
}

func ExportHTMLWithRepository(state State, hash string, repository *RepositorySnapshot) ([]byte, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	docs := append([]Document(nil), state.Documents...)
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].Order < docs[j].Order })
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	data := exportData{State: state, Documents: docs, Hash: hash, StateJSON: template.JS(raw), Repository: repository}
	if repository != nil {
		data.RepositoryDocuments = append([]RepositoryDocument(nil), repository.Documents...)
		sort.SliceStable(data.RepositoryDocuments, func(i, j int) bool { return data.RepositoryDocuments[i].Path < data.RepositoryDocuments[j].Path })
		repoRaw, err := json.Marshal(repository)
		if err != nil {
			return nil, err
		}
		data.RepositoryJSON = template.JS(repoRaw)
	}
	t, err := template.New("projection").Parse(exportTemplate)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
