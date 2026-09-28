#!/usr/bin/env python3
"""Phase 4 automated host acceptance, browser half.

Drives the real loopback service in a real Chromium-class browser and proves,
from inside the page, that the local Git control chain is reachable and that
every authority layer stays separate. Every fact this script records is
observed; nothing is inferred. Any failed assertion aborts the run with the
partial evidence already on disk.
"""
import hashlib
import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

try:
    from playwright.sync_api import sync_playwright
except Exception as exc:
    print(f"PHASE4_BROWSER_AUTOMATION_BLOCKED: Python Playwright unavailable: {exc}", file=sys.stderr)
    raise SystemExit(2)

if len(sys.argv) != 4:
    print("usage: host-accept-phase4.py <url> <test-repo> <evidence-dir>", file=sys.stderr)
    raise SystemExit(2)

URL = sys.argv[1].rstrip("/")
TEST_REPO = Path(sys.argv[2]).resolve()
EVIDENCE = Path(sys.argv[3]).resolve()
EVIDENCE.mkdir(parents=True, exist_ok=True)

HEADLESS = os.environ.get("PHASE4_BROWSER_HEADLESS", "1") != "0"
EXPLICIT_BROWSER = os.environ.get("PHASE4_BROWSER_EXECUTABLE", "").strip()

# Exact bytes and strings the shell half also writes to evidence. They are
# passed in so the two halves can never drift apart on what "correct" means.
EXPECTED_SOURCE = os.environ.get("PHASE4_EXPECTED_SOURCE")
SOURCE_BASELINE = os.environ.get("PHASE4_SOURCE_BASELINE")
UNRELATED_BASELINE = os.environ.get("PHASE4_UNRELATED_BASELINE")
EXTERNAL_REWRITE = os.environ.get("PHASE4_EXTERNAL_REWRITE")
COMMIT_MESSAGE = os.environ.get("PHASE4_COMMIT_MESSAGE")

CONTRACT = {
    "PHASE4_EXPECTED_SOURCE": EXPECTED_SOURCE,
    "PHASE4_SOURCE_BASELINE": SOURCE_BASELINE,
    "PHASE4_UNRELATED_BASELINE": UNRELATED_BASELINE,
    "PHASE4_EXTERNAL_REWRITE": EXTERNAL_REWRITE,
    "PHASE4_COMMIT_MESSAGE": COMMIT_MESSAGE,
}
missing = [name for name, value in CONTRACT.items() if value is None]
if missing:
    print(
        f"PHASE4_BROWSER_AUTOMATION_BLOCKED: missing harness contract env vars: {', '.join(missing)}",
        file=sys.stderr,
    )
    raise SystemExit(2)

results = {}
dialogs = []
api_requests = []
git_observations = []
steps = []
page = None


def record(name, value, detail=""):
    results[name] = {"pass": bool(value), "detail": detail}
    if not value:
        raise AssertionError(f"{name}: {detail}")
    return detail


def step(label, **data):
    entry = {"step": label}
    entry.update(data)
    steps.append(entry)
    return entry


def find_browser():
    if EXPLICIT_BROWSER:
        p = Path(EXPLICIT_BROWSER)
        if p.exists():
            return str(p)
        resolved = shutil.which(EXPLICIT_BROWSER)
        if resolved:
            return resolved
        raise RuntimeError(f"PHASE4_BROWSER_EXECUTABLE not found: {EXPLICIT_BROWSER}")

    candidates = [
        "/usr/bin/thorium-browser",
        "thorium-browser",
        "chromium",
        "chromium-browser",
        "google-chrome",
        "google-chrome-stable",
    ]
    for candidate in candidates:
        if candidate.startswith("/") and Path(candidate).exists():
            return candidate
        resolved = shutil.which(candidate)
        if resolved:
            return resolved
    raise RuntimeError("no Chromium-class browser found; set PHASE4_BROWSER_EXECUTABLE")


def wait_until(predicate, message, timeout=15.0):
    deadline = time.monotonic() + timeout
    last = None
    while time.monotonic() < deadline:
        try:
            last = predicate()
            if last:
                return last
        except Exception:
            pass
        if page is not None:
            page.wait_for_timeout(50)
        else:
            time.sleep(0.05)
    raise AssertionError(f"timeout: {message}; last={last!r}")


# --- independent Git observation, taken from the filesystem, not the UI -----


def git(*args):
    # GIT_OPTIONAL_LOCKS=0 keeps every observation here read-only. Without it
    # `git status` refreshes and rewrites .git/index, which would change the
    # index identity this script is trying to measure.
    return subprocess.run(
        ["git", "-C", str(TEST_REPO), *args],
        check=True,
        capture_output=True,
        text=True,
        env={**os.environ, "GIT_OPTIONAL_LOCKS": "0"},
    ).stdout


def git_lines(*args):
    return sorted(line for line in git(*args).splitlines() if line)


def index_path():
    try:
        candidate = git("rev-parse", "--path-format=absolute", "--git-path", "index").strip()
        if candidate:
            return Path(candidate)
    except subprocess.CalledProcessError:
        pass
    return TEST_REPO / ".git" / "index"


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def index_sha():
    return sha256_bytes(index_path().read_bytes())


def file_bytes(name):
    return (TEST_REPO / name).read_bytes()


def observe(label):
    """Snapshot the whole repository identity from outside the browser."""
    entry = {
        "label": label,
        "head": git("rev-parse", "HEAD").strip(),
        "branch": git("branch", "--show-current").strip(),
        "branch_ref": git("symbolic-ref", "--quiet", "HEAD").strip(),
        "index_path": str(index_path()),
        "index_sha256": index_sha(),
        "source_bytes": file_bytes("source.txt").decode("utf-8"),
        "unrelated_bytes": file_bytes("unrelated.txt").decode("utf-8"),
        "staged": git_lines("diff", "--cached", "--name-only"),
        "unstaged": git_lines("diff", "--name-only"),
        "status": git("status", "--porcelain=v1", "--untracked-files=all"),
    }
    git_observations.append(entry)
    return entry


# --- DOM helpers -----------------------------------------------------------


def article_selector(path):
    escaped = path.replace("\\", "\\\\").replace('"', '\\"')
    return f'article[data-authority="repository"][data-source-path="{escaped}"]'


def article(path):
    return page.locator(article_selector(path))


def article_body(path):
    article(path).wait_for(state="visible")
    return article(path).inner_text()


def expand(root_id):
    """Open the disclosure that holds a change list so its rows are clickable."""
    details = page.locator(f"details:has(#{root_id})").first
    details.wait_for(state="attached")
    if not details.evaluate("node => node.open"):
        details.locator("summary").click()
    wait_until(lambda: details.evaluate("node => node.open"), f"details for #{root_id} to open")


def index_dom():
    return page.locator("#commit-index").inner_text().split("index sha256:")[-1].strip()


def head_dom():
    return page.locator("#commit-head").inner_text().split("HEAD:")[-1].strip()


def staged_count():
    return page.locator("#commit-staged-summary").get_attribute("data-staged-count")


def save_state():
    return page.locator("#save-state").get_attribute("data-state")


def commit_error():
    element = page.locator("#commit-error")
    return {
        "visible": element.is_visible(),
        "code": element.get_attribute("data-error-code"),
        "status": element.get_attribute("data-error-status"),
        "action": element.get_attribute("data-error-action"),
        "text": element.inner_text(),
    }


def expect_dialog(fragment, action, label):
    before = len(dialogs)
    requests_before = len(api_requests)
    action()
    wait_until(lambda: len(dialogs) > before, f"dialog containing {fragment!r}")
    message = dialogs[-1]
    record(
        f"dialog_{label}",
        fragment in message,
        f"expected fragment={fragment!r}; actual={message!r}",
    )
    record(
        f"dialog_{label}_no_request",
        len(api_requests) == requests_before,
        f"guard issued {len(api_requests) - requests_before} local Git request(s)",
    )
    return message


def click_with_response(locator, fragment, label):
    with page.expect_response(lambda response: fragment in response.url, timeout=20000) as info:
        locator.click()
    response = info.value
    step(f"{label}_http", status=response.status, url=response.url)
    return response


def shot(name):
    page.screenshot(path=str(EVIDENCE / name), full_page=True)


browser_path = None
try:
    browser_path = find_browser()
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=browser_path, headless=HEADLESS)
        context = browser.new_context(viewport={"width": 1440, "height": 1000})
        page = context.new_page()

        def on_dialog(dialog):
            dialogs.append(dialog.message)
            dialog.dismiss()

        def on_request(request):
            if any(endpoint in request.url for endpoint in ("/api/stage-file", "/api/unstage-file", "/api/commit")):
                api_requests.append({"method": request.method, "url": request.url})

        page.on("dialog", on_dialog)
        page.on("request", on_request)

        page.goto(URL + "/", wait_until="domcontentloaded")
        page.wait_for_selector("#repository-status")
        page.wait_for_selector("#commit-panel[data-commit-panel='true']")
        page.wait_for_selector(article_selector("source.txt"))
        page.wait_for_selector(article_selector("unrelated.txt"))
        wait_until(lambda: save_state() == "saved", "workstation to finish its initial load")

        record("authority_banner", "AUTHORITY: LOCAL" in page.locator("body").inner_text(), "authority banner missing")
        record("initial_snapshot_loaded", bool(index_dom()) and head_dom() != "—", f"index={index_dom()} head={head_dom()}")
        record("initial_unrelated_unstaged", "unrelated.txt" in (page.locator("#unstaged-changes").text_content() or ""), "unrelated.txt not projected as unstaged")
        record("initial_nothing_staged", staged_count() == "0", f"staged={staged_count()}")
        record("initial_commit_disabled", page.locator("#commit-btn").is_disabled(), "COMMIT STAGED enabled with an empty index")
        record("initial_index_matches_disk", index_dom() == index_sha(), f"dom={index_dom()} disk={index_sha()}")
        shot("01-loaded.png")

        # ---- Authority separation: SAVE CHANGES is workstation JSON only ----
        before_save = observe("before-save")
        local_doc = page.locator('article[data-authority="local"][data-editable="true"]').first
        local_doc.locator('button[data-action="toggle-edit"]').click()
        local_textarea = local_doc.locator("textarea")
        local_textarea.fill(local_textarea.input_value() + "\nphase4 acceptance workstation save marker\n")
        wait_until(lambda: save_state() == "dirty", "workstation dirty indicator")
        page.locator("#save-btn").click()
        wait_until(lambda: save_state() == "saved", "workstation save completion")
        after_save = observe("after-save")
        record(
            "save_changes_leaves_git_untouched",
            after_save["head"] == before_save["head"]
            and after_save["branch"] == before_save["branch"]
            and after_save["index_sha256"] == before_save["index_sha256"]
            and after_save["source_bytes"] == before_save["source_bytes"]
            and after_save["unrelated_bytes"] == before_save["unrelated_bytes"]
            and after_save["staged"] == before_save["staged"]
            and after_save["status"] == before_save["status"],
            f"before={before_save} after={after_save}",
        )
        record(
            "save_changes_left_source_baseline",
            after_save["source_bytes"] == SOURCE_BASELINE,
            repr(after_save["source_bytes"]),
        )
        step("save-changes", before=before_save, after=after_save)

        # ---- Authority separation: the commit message never dirties the app --
        page.locator("#commit-message").fill("typing never marks the workstation dirty")
        wait_until(
            lambda: page.locator("#commit-message").input_value() == "typing never marks the workstation dirty",
            "commit message draft",
        )
        record("commit_message_keeps_workstation_saved", save_state() == "saved", f"save-state={save_state()}")
        record(
            "commit_message_never_enables_save_dirty_state",
            save_state() != "dirty" and page.locator("#commit-btn").is_disabled(),
            f"save-state={save_state()}",
        )
        page.locator("#commit-message").fill("")
        record("commit_message_kept_after_typing", save_state() == "saved", f"save-state={save_state()}")
        shot("02-after-save.png")

        # ---- WRITE FILE -> worktree bytes on disk, nothing staged ----------
        before_write = observe("before-write-file")
        source = article("source.txt")
        source.locator('button[data-action="toggle-source-edit"]').click()
        source.locator("textarea").fill(EXPECTED_SOURCE)
        record("source_draft_dirty", source.get_attribute("data-state") == "source-draft", source.get_attribute("data-state") or "")
        source.locator('button[data-action="write-file"]').click()
        wait_until(
            lambda: article("source.txt").get_attribute("data-editing") == "false",
            "source editor to close after the write",
        )
        wait_until(
            lambda: "source.txt" in (page.locator("#unstaged-changes").text_content() or ""),
            "source.txt to appear in the unstaged projection",
        )
        after_write = observe("after-write-file")
        written_bytes = file_bytes("source.txt")
        record(
            "worktree_bytes_after_write_file",
            written_bytes == EXPECTED_SOURCE.encode("utf-8"),
            f"expected={EXPECTED_SOURCE!r} actual={written_bytes.decode('utf-8', 'replace')!r}",
        )
        (EVIDENCE / "source-expected.txt").write_text(EXPECTED_SOURCE, encoding="utf-8")
        record(
            "write_file_does_not_stage",
            after_write["index_sha256"] == before_write["index_sha256"]
            and after_write["head"] == before_write["head"]
            and after_write["staged"] == []
            and after_write["unstaged"] == ["source.txt", "unrelated.txt"],
            f"before={before_write} after={after_write}",
        )
        record("write_file_staged_count_zero", staged_count() == "0", f"staged={staged_count()}")
        record("write_file_index_identity_unchanged", index_dom() == index_sha(), f"dom={index_dom()} disk={index_sha()}")
        record("commit_button_still_disabled", page.locator("#commit-btn").is_disabled(), "COMMIT STAGED enabled with nothing staged")
        step("write-file", before=before_write, after=after_write)
        shot("03-after-write-file.png")

        # ---- STAGE FILE ----------------------------------------------------
        expand("unstaged-changes")
        expand("staged-changes")
        expand("source-relationships")
        stage_button = page.locator('#unstaged-changes button[data-action="stage-file"][data-source-path="source.txt"]')
        stage_button.wait_for(state="visible")
        response = click_with_response(stage_button, "/api/stage-file", "stage-file-1")
        record("stage_file_http_200", response.status == 200, f"status={response.status}")
        stage_payload = response.json()
        wait_until(lambda: staged_count() == "1", "staged count to reach 1 after STAGE FILE")
        after_stage = observe("after-stage-file")
        record(
            "stage_changes_index_only",
            after_stage["index_sha256"] != before_write["index_sha256"]
            and after_stage["head"] == before_write["head"]
            and after_stage["branch"] == before_write["branch"]
            and after_stage["staged"] == ["source.txt"]
            and after_stage["unstaged"] == ["unrelated.txt"]
            and after_stage["source_bytes"] == EXPECTED_SOURCE,
            f"before={before_write} after={after_stage}",
        )
        record(
            "stage_index_sha_matches_disk",
            after_stage["index_sha256"] == stage_payload["after_index_sha256"] == index_dom() == index_sha(),
            f"disk={after_stage['index_sha256']} api={stage_payload['after_index_sha256']} dom={index_dom()}",
        )
        record("stage_head_dom_unchanged", head_dom() == before_write["head"], f"dom={head_dom()}")
        record("stage_error_cleared", commit_error()["code"] == "", json.dumps(commit_error()))
        step("stage-file", before=before_write, after=after_stage, api_index=stage_payload["after_index_sha256"])
        shot("04-after-stage-file.png")

        # ---- STAGE FILE is not COMMIT STAGED -------------------------------
        record("stage_does_not_commit", after_stage["head"] == before_write["head"], f"head={after_stage['head']}")
        record("commit_ready_with_staged", page.locator("#commit-panel").get_attribute("data-commit-ready") == "true", "commit panel not ready with a staged entry")
        record("commit_button_enabled_when_staged", not page.locator("#commit-btn").is_disabled(), "COMMIT STAGED still disabled")

        # ---- UNSTAGE FILE --------------------------------------------------
        before_unstage = after_stage
        unstage_button = page.locator('#staged-changes button[data-action="unstage-file"][data-source-path="source.txt"]')
        unstage_button.wait_for(state="visible")
        response = click_with_response(unstage_button, "/api/unstage-file", "unstage-file")
        record("unstage_file_http_200", response.status == 200, f"status={response.status}")
        unstage_payload = response.json()
        wait_until(lambda: staged_count() == "0", "staged count to return to 0 after UNSTAGE FILE")
        after_unstage = observe("after-unstage-file")
        record(
            "unstage_changes_index_only",
            after_unstage["index_sha256"] != before_unstage["index_sha256"]
            and after_unstage["head"] == before_unstage["head"]
            and after_unstage["branch"] == before_unstage["branch"]
            and after_unstage["staged"] == []
            and after_unstage["unstaged"] == ["source.txt", "unrelated.txt"]
            and after_unstage["source_bytes"] == EXPECTED_SOURCE,
            f"before={before_unstage} after={after_unstage}",
        )
        record(
            "unstage_index_sha_matches_disk",
            after_unstage["index_sha256"] == unstage_payload["after_index_sha256"] == index_dom() == index_sha(),
            f"disk={after_unstage['index_sha256']} api={unstage_payload['after_index_sha256']} dom={index_dom()}",
        )
        record("unstage_preserves_worktree_bytes", after_unstage["source_bytes"] == EXPECTED_SOURCE, repr(after_unstage["source_bytes"]))
        record("commit_blocked_with_empty_index", page.locator("#commit-btn").is_disabled(), "COMMIT STAGED enabled with nothing staged")
        step("unstage-file", before=before_unstage, after=after_unstage, api_index=unstage_payload["after_index_sha256"])
        shot("05-after-unstage-file.png")

        # ---- STAGE FILE again, so a commit is reachable --------------------
        stage_button = page.locator('#unstaged-changes button[data-action="stage-file"][data-source-path="source.txt"]')
        stage_button.wait_for(state="visible")
        response = click_with_response(stage_button, "/api/stage-file", "stage-file-2")
        record("restage_http_200", response.status == 200, f"status={response.status}")
        restage_payload = response.json()
        wait_until(lambda: staged_count() == "1", "staged count to reach 1 after the second STAGE FILE")
        after_restage = observe("after-restage-file")
        record(
            "restage_changes_index_only",
            after_restage["index_sha256"] != after_unstage["index_sha256"]
            and after_restage["head"] == after_unstage["head"]
            and after_restage["staged"] == ["source.txt"],
            f"before={after_unstage} after={after_restage}",
        )
        record(
            "restage_index_sha_matches_disk",
            after_restage["index_sha256"] == restage_payload["after_index_sha256"] == index_sha(),
            f"disk={after_restage['index_sha256']} api={restage_payload['after_index_sha256']}",
        )
        step("restage-file", before=after_unstage, after=after_restage, api_index=restage_payload["after_index_sha256"])
        shot("06-after-restage-file.png")

        # ---- A dirty source draft blocks every local Git action -------------
        unrelated = article("unrelated.txt")
        unrelated.locator('button[data-action="toggle-source-edit"]').click()
        unrelated.locator("textarea").fill("phase4 draft that must never reach git\n")
        record("conflict_draft_dirty", unrelated.get_attribute("data-state") == "source-draft", unrelated.get_attribute("data-state") or "")

        # A non-empty browser-local commit message draft must survive a blocked
        # commit: the blocked request must not consume or clear it.
        blocked_message = "draft message that must survive a blocked commit"
        page.locator("#commit-message").fill(blocked_message)
        record("blocked_commit_message_does_not_dirty", save_state() == "saved", f"save-state={save_state()}")

        before_blocks = observe("before-draft-block")
        blocked_stage = page.locator('#unstaged-changes button[data-action="stage-file"][data-source-path="unrelated.txt"]')
        blocked_stage.wait_for(state="visible")
        expect_dialog("SOURCE DRAFT exists", lambda: blocked_stage.click(), "stage_blocked")
        after_stage_block = observe("after-draft-block-stage")
        record(
            "draft_blocks_stage_without_mutation",
            after_stage_block["index_sha256"] == before_blocks["index_sha256"]
            and after_stage_block["head"] == before_blocks["head"]
            and after_stage_block["branch"] == before_blocks["branch"]
            and after_stage_block["staged"] == before_blocks["staged"]
            and after_stage_block["unstaged"] == before_blocks["unstaged"]
            and after_stage_block["source_bytes"] == before_blocks["source_bytes"]
            and after_stage_block["unrelated_bytes"] == before_blocks["unrelated_bytes"]
            and after_stage_block["status"] == before_blocks["status"],
            f"before={before_blocks} after={after_stage_block}",
        )
        draft_stage_error = commit_error()
        record(
            "draft_block_short_circuits_before_request",
            draft_stage_error["code"] == "" and not draft_stage_error["visible"],
            f"commit-error={json.dumps(draft_stage_error)}",
        )
        shot("07-draft-block-stage.png")

        before_blocks = observe("before-draft-block-unstage")
        blocked_unstage = page.locator('#staged-changes button[data-action="unstage-file"][data-source-path="source.txt"]')
        blocked_unstage.wait_for(state="visible")
        expect_dialog("SOURCE DRAFT exists", lambda: blocked_unstage.click(), "unstage_blocked")
        after_unstage_block = observe("after-draft-block-unstage")
        record(
            "draft_blocks_unstage_without_mutation",
            after_unstage_block["index_sha256"] == before_blocks["index_sha256"]
            and after_unstage_block["head"] == before_blocks["head"]
            and after_unstage_block["branch"] == before_blocks["branch"]
            and after_unstage_block["staged"] == before_blocks["staged"]
            and after_unstage_block["unstaged"] == before_blocks["unstaged"]
            and after_unstage_block["source_bytes"] == before_blocks["source_bytes"]
            and after_unstage_block["unrelated_bytes"] == before_blocks["unrelated_bytes"]
            and after_unstage_block["status"] == before_blocks["status"],
            f"before={before_blocks} after={after_unstage_block}",
        )
        record(
            "draft_block_unstage_no_error_and_no_request",
            commit_error()["code"] == "",
            f"commit-error={json.dumps(commit_error())}",
        )
        shot("07-draft-block-unstage.png")

        before_blocks = observe("before-draft-block-commit")
        expect_dialog("SOURCE DRAFT exists", lambda: page.locator("#commit-btn").click(), "commit_blocked")
        after_commit_block = observe("after-draft-block-commit")
        record(
            "draft_blocks_commit_without_mutation",
            after_commit_block["index_sha256"] == before_blocks["index_sha256"]
            and after_commit_block["head"] == before_blocks["head"]
            and after_commit_block["branch"] == before_blocks["branch"]
            and after_commit_block["staged"] == before_blocks["staged"]
            and after_commit_block["unstaged"] == before_blocks["unstaged"]
            and after_commit_block["source_bytes"] == before_blocks["source_bytes"]
            and after_commit_block["unrelated_bytes"] == before_blocks["unrelated_bytes"]
            and after_commit_block["status"] == before_blocks["status"],
            f"before={before_blocks} after={after_commit_block}",
        )
        record(
            "draft_block_commit_no_error_and_no_request",
            commit_error()["code"] == "",
            f"commit-error={json.dumps(commit_error())}",
        )
        record(
            "commit_message_survives_blocked_commit",
            page.locator("#commit-message").input_value() == blocked_message,
            f"expected={blocked_message!r} actual={page.locator('#commit-message').input_value()!r}",
        )
        step("draft-block", observations=[before_blocks, after_commit_block])
        shot("07-draft-block-commit.png")

        # CANCEL SOURCE EDIT is browser-only: the draft never reached disk.
        unrelated.locator('button[data-action="toggle-source-edit"]').click()
        wait_until(
            lambda: article("unrelated.txt").get_attribute("data-editing") == "false",
            "source editor to close on cancel",
        )
        after_cancel = observe("after-draft-cancel")
        record(
            "cancel_draft_writes_nothing",
            after_cancel["unrelated_bytes"] == UNRELATED_BASELINE and after_cancel["index_sha256"] == before_blocks["index_sha256"],
            f"unrelated={after_cancel['unrelated_bytes']!r}",
        )

        # ---- 409 conflict: desynchronize the browser from the worktree -----
        pre_conflict_dom = {
            "index": index_dom(),
            "snapshot": page.locator("#repository-hash").inner_text(),
            "counts": page.locator("#repository-counts").inner_text(),
        }
        pre_conflict_body = article_body("unrelated.txt")
        (TEST_REPO / "unrelated.txt").write_text(EXTERNAL_REWRITE, encoding="utf-8")
        after_desync = observe("after-external-rewrite")
        record(
            "external_rewrite_desynchronized_browser",
            after_desync["unrelated_bytes"] == EXTERNAL_REWRITE and after_desync["index_sha256"] == before_blocks["index_sha256"],
            f"unrelated={after_desync['unrelated_bytes']!r}",
        )
        conflict_button = page.locator('#unstaged-changes button[data-action="stage-file"][data-source-path="unrelated.txt"]')
        conflict_button.wait_for(state="visible")
        response = click_with_response(conflict_button, "/api/stage-file", "stage-conflict")
        record("conflict_http_409", response.status == 409, f"status={response.status}")
        conflict_payload = response.json()
        wait_until(lambda: commit_error()["visible"], "visible #commit-error after a 409")
        conflict_error = commit_error()
        record("conflict_code_visible", conflict_error["code"] == "WORKTREE_CONFLICT", json.dumps(conflict_error))
        record("conflict_status_visible", conflict_error["status"] == "409", json.dumps(conflict_error))
        record("conflict_action_visible", conflict_error["action"] == "stage-file", json.dumps(conflict_error))
        record("conflict_api_code", conflict_payload["code"] == "WORKTREE_CONFLICT", json.dumps(conflict_payload))
        record("conflict_error_readable", "WORKTREE_CONFLICT" in conflict_error["text"], conflict_error["text"])
        record(
            "conflict_does_not_replace_local_snapshot",
            index_dom() == pre_conflict_dom["index"]
            and page.locator("#repository-hash").inner_text() == pre_conflict_dom["snapshot"]
            and page.locator("#repository-counts").inner_text() == pre_conflict_dom["counts"]
            and article_body("unrelated.txt") == pre_conflict_body,
            f"before={pre_conflict_dom} after={{index={index_dom()}, snapshot={page.locator('#repository-hash').inner_text()}, counts={page.locator('#repository-counts').inner_text()}}}",
        )
        after_conflict = observe("after-conflict-stage")
        record(
            "conflict_preserves_external_bytes",
            file_bytes("unrelated.txt").decode("utf-8") == EXTERNAL_REWRITE,
            repr(file_bytes("unrelated.txt").decode("utf-8", "replace")),
        )
        record(
            "conflict_performs_no_git_mutation",
            after_conflict["index_sha256"] == after_desync["index_sha256"]
            and after_conflict["head"] == after_desync["head"]
            and after_conflict["staged"] == after_desync["staged"],
            f"before={after_desync} after={after_conflict}",
        )
        step("stage-conflict", response=conflict_payload, error=conflict_error, observation=after_conflict)
        shot("08-after-conflict.png")

        # ---- COMMIT STAGED --------------------------------------------------
        pre_commit = observe("before-commit")
        page.locator("#commit-message").fill(COMMIT_MESSAGE)
        record("commit_message_keeps_workstation_saved_again", save_state() == "saved", f"save-state={save_state()}")
        record("commit_message_kept_verbatim", page.locator("#commit-message").input_value() == COMMIT_MESSAGE, page.locator("#commit-message").input_value())
        response = click_with_response(page.locator("#commit-btn"), "/api/commit", "commit-staged")
        record("commit_http_200", response.status == 200, f"status={response.status}")
        commit_payload = response.json()
        wait_until(lambda: staged_count() == "0", "staged count to be zero after COMMIT STAGED")
        after_commit = observe("after-commit")
        record(
            "commit_advances_head_only",
            after_commit["head"] == commit_payload["new_head_commit"] == head_dom() == git("rev-parse", "HEAD").strip(),
            f"disk={after_commit['head']} api={commit_payload['new_head_commit']} dom={head_dom()}",
        )
        record(
            "commit_one_parent_is_previous_head",
            commit_payload["parent_commit"] == pre_commit["head"] == commit_payload["expected_head_commit"],
            f"api_parent={commit_payload['parent_commit']} pre_commit_head={pre_commit['head']}",
        )
        parent_line = git("rev-list", "--parents", "-n", "1", after_commit["head"]).strip().split()
        record(
            "commit_object_has_exactly_one_parent",
            len(parent_line) == 2 and parent_line[1] == pre_commit["head"],
            f"rev-list={parent_line}",
        )
        record(
            "commit_keeps_branch",
            after_commit["branch"] == pre_commit["branch"] and after_commit["branch_ref"] == pre_commit["branch_ref"],
            f"before={pre_commit['branch']} after={after_commit['branch']}",
        )
        record(
            "commit_leaves_index_untouched",
            after_commit["index_sha256"] == pre_commit["index_sha256"] == commit_payload["after_index_sha256"] == index_sha(),
            f"disk={after_commit['index_sha256']} api={commit_payload['after_index_sha256']}",
        )
        record(
            "commit_consumes_the_staged_snapshot",
            after_commit["staged"] == [] and after_commit["unstaged"] == ["unrelated.txt"],
            f"staged={after_commit['staged']} unstaged={after_commit['unstaged']}",
        )
        record("staged_count_zero_after_commit", staged_count() == "0", f"staged={staged_count()}")
        record(
            "unrelated_unstaged_survives_commit",
            file_bytes("unrelated.txt").decode("utf-8") == EXTERNAL_REWRITE
            and after_commit["unrelated_bytes"] == EXTERNAL_REWRITE
            and "unrelated.txt" in (page.locator("#unstaged-changes").text_content() or ""),
            repr(after_commit["unrelated_bytes"]),
        )
        record("commit_never_dirties_workstation", save_state() == "saved", f"save-state={save_state()}")
        record("commit_clears_commit_error", commit_error()["code"] == "" and not commit_error()["visible"], json.dumps(commit_error()))
        record("commit_clears_message_draft", page.locator("#commit-message").input_value() == "", page.locator("#commit-message").input_value())
        record("commit_button_disabled_again", page.locator("#commit-btn").is_disabled(), "COMMIT STAGED enabled after the commit")
        record("no_push_control", page.locator('button:has-text("PUSH")').count() == 0 and "PUSH" not in page.locator("body").inner_text(), "a PUSH affordance exists")
        step("commit-staged", before=pre_commit, after=after_commit, response=commit_payload)
        shot("09-after-commit.png")

        final = {
            "browser_executable": browser_path,
            "headless": HEADLESS,
            "url": URL,
            "test_repo": str(TEST_REPO),
            "source_baseline": SOURCE_BASELINE,
            "expected_source": EXPECTED_SOURCE,
            "external_rewrite": EXTERNAL_REWRITE,
            "commit_message": COMMIT_MESSAGE,
            "pre_commit_head": pre_commit["head"],
            "branch": after_commit["branch"],
            "commit": commit_payload,
            "stage": stage_payload,
            "unstage": unstage_payload,
            "restage": restage_payload,
            "conflict": conflict_payload,
            "dialogs": dialogs,
            "api_requests": api_requests,
            "steps": steps,
            "git_observations": git_observations,
            "checks": results,
        }
        (EVIDENCE / "browser-automation.json").write_text(json.dumps(final, indent=2), encoding="utf-8")
        context.close()
        browser.close()

except Exception as exc:
    failure = {
        "browser_executable": browser_path,
        "headless": HEADLESS,
        "error": repr(exc),
        "dialogs": dialogs,
        "api_requests": api_requests,
        "steps": steps,
        "git_observations": git_observations,
        "checks": results,
    }
    (EVIDENCE / "browser-automation.json").write_text(json.dumps(failure, indent=2), encoding="utf-8")
    if page is not None:
        try:
            page.screenshot(path=str(EVIDENCE / "browser-failure.png"), full_page=True)
            (EVIDENCE / "browser-failure.html").write_text(page.content(), encoding="utf-8")
        except Exception:
            pass
    print(f"PHASE4_BROWSER_AUTOMATION=FAIL: {exc}", file=sys.stderr)
    raise SystemExit(1)

print(f"PHASE4_BROWSER_ENGINE={browser_path}")
print(f"PHASE4_BROWSER_HEADLESS={str(HEADLESS).lower()}")
print(f"PHASE4_BROWSER_CHECKS={len(results)}")
print("PHASE4_BROWSER_AUTOMATION=PASS")
