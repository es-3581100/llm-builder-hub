#!/usr/bin/env python3
import json
import os
import shutil
import sys
import time
from pathlib import Path

try:
    from playwright.sync_api import sync_playwright
except Exception as exc:
    print(f"PHASE3_BROWSER_AUTOMATION_BLOCKED: Python Playwright unavailable: {exc}", file=sys.stderr)
    raise SystemExit(2)

if len(sys.argv) != 4:
    print("usage: host-accept-phase3.py <url> <test-repo> <evidence-dir>", file=sys.stderr)
    raise SystemExit(2)

URL = sys.argv[1].rstrip("/")
TEST_REPO = Path(sys.argv[2]).resolve()
EVIDENCE = Path(sys.argv[3]).resolve()
EVIDENCE.mkdir(parents=True, exist_ok=True)

HEADLESS = os.environ.get("PHASE3_BROWSER_HEADLESS", "1") != "0"
EXPLICIT_BROWSER = os.environ.get("PHASE3_BROWSER_EXECUTABLE", "").strip()

results = {}
dialogs = []
page = None


def record(name, value, detail=""):
    results[name] = {"pass": bool(value), "detail": detail}
    if not value:
        raise AssertionError(f"{name}: {detail}")


def find_browser():
    if EXPLICIT_BROWSER:
        p = Path(EXPLICIT_BROWSER)
        if p.exists():
            return str(p)
        resolved = shutil.which(EXPLICIT_BROWSER)
        if resolved:
            return resolved
        raise RuntimeError(f"PHASE3_BROWSER_EXECUTABLE not found: {EXPLICIT_BROWSER}")

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
    raise RuntimeError("no Chromium-class browser found; set PHASE3_BROWSER_EXECUTABLE")


def wait_until(predicate, message, timeout=8.0):
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


def article_selector(path):
    escaped = path.replace('\\', '\\\\').replace('"', '\\"')
    return f'article[data-authority="repository"][data-source-path="{escaped}"]'


def text(path):
    return Path(path).read_text(encoding="utf-8")


def expect_dialog(fragment, action):
    before = len(dialogs)
    action()
    wait_until(lambda: len(dialogs) > before, f"dialog containing {fragment!r}")
    message = dialogs[-1]
    record(
        f"dialog_{len(results)}",
        fragment in message,
        f"expected fragment={fragment!r}; actual={message!r}",
    )
    return message


browser_path = None
try:
    browser_path = find_browser()
    with sync_playwright() as p:
        browser = p.chromium.launch(
            executable_path=browser_path,
            headless=HEADLESS,
        )
        context = browser.new_context(viewport={"width": 1440, "height": 1000})
        page = context.new_page()

        def on_dialog(dialog):
            dialogs.append(dialog.message)
            dialog.dismiss()

        page.on("dialog", on_dialog)
        page.goto(URL + "/", wait_until="domcontentloaded")
        page.wait_for_selector("#repository-status")
        page.wait_for_selector(article_selector("source.txt"))

        record("loopback_loaded", "AUTHORITY: LOCAL" in page.locator("body").inner_text(), "authority banner missing")

        initial_revision = page.locator("#revision").inner_text()
        record("initial_revision_visible", initial_revision.startswith("revision "), initial_revision)

        # Successful source write.
        source_sel = article_selector("source.txt")
        source = page.locator(source_sel)
        source.locator('button[data-action="toggle-source-edit"]').click()
        source.locator("textarea").fill("phase3 browser write\nline two\n")

        record(
            "source_draft_state",
            source.get_attribute("data-state") == "source-draft",
            source.get_attribute("data-state") or "",
        )
        record(
            "source_draft_separate_from_workstation",
            page.locator("#save-state").inner_text() == "SAVED",
            page.locator("#save-state").inner_text(),
        )

        # Dirty source draft must not be replaced by editing another repository source.
        conflict_sel = article_selector("conflict-ui.txt")
        expect_dialog(
            "SOURCE DRAFT exists",
            lambda: page.locator(conflict_sel).locator('button[data-action="toggle-source-edit"]').click(),
        )
        record(
            "cross_document_switch_blocked",
            page.locator(source_sel).locator("textarea").input_value() == "phase3 browser write\nline two\n",
            "source draft changed while attempting cross-document edit",
        )

        # Refresh must be blocked while source draft is dirty.
        expect_dialog("SOURCE DRAFT exists", lambda: page.locator("#refresh-btn").click())
        record(
            "refresh_preserves_source_draft",
            page.locator(source_sel).locator("textarea").input_value() == "phase3 browser write\nline two\n",
            "source draft changed after blocked refresh",
        )

        revision_before_source_write = page.locator("#revision").inner_text()
        page.locator(source_sel).locator('button[data-action="write-file"]').click()

        wait_until(
            lambda: page.locator(source_sel).get_attribute("data-editing") == "false",
            "source editor to close after successful write",
        )
        wait_until(
            lambda: "source.txt" in page.locator("#unstaged-changes").inner_text(),
            "source.txt to appear in unstaged projection",
        )
        record(
            "source_bytes_written",
            text(TEST_REPO / "source.txt") == "phase3 browser write\nline two\n",
            repr(text(TEST_REPO / "source.txt")),
        )
        record(
            "source_write_revision_separation",
            page.locator("#revision").inner_text() == revision_before_source_write,
            f"before={revision_before_source_write} after={page.locator('#revision').inner_text()}",
        )

        # CANCEL SOURCE EDIT must be browser-only.
        conflict = page.locator(conflict_sel)
        conflict.locator('button[data-action="toggle-source-edit"]').click()
        conflict.locator("textarea").fill("cancel should not write\n")
        conflict.locator('button[data-action="toggle-source-edit"]').click()
        record(
            "cancel_source_edit_no_write",
            text(TEST_REPO / "conflict-ui.txt") == "conflict original\n",
            repr(text(TEST_REPO / "conflict-ui.txt")),
        )

        # Stale-content conflict.
        conflict = page.locator(conflict_sel)
        conflict.locator('button[data-action="toggle-source-edit"]').click()
        conflict.locator("textarea").fill("browser stale draft\n")
        record(
            "conflict_source_draft_state",
            conflict.get_attribute("data-state") == "source-draft",
            conflict.get_attribute("data-state") or "",
        )

        (TEST_REPO / "conflict-ui.txt").write_text("external edit wins\n", encoding="utf-8")

        expect_dialog("SOURCE DRAFT exists", lambda: page.locator("#refresh-btn").click())
        record(
            "stale_refresh_preserves_draft",
            page.locator(conflict_sel).locator("textarea").input_value() == "browser stale draft\n",
            "stale draft changed after blocked refresh",
        )

        revision_before_conflict = page.locator("#revision").inner_text()
        before_dialogs = len(dialogs)
        page.locator(conflict_sel).locator('button[data-action="write-file"]').click()

        wait_until(
            lambda: page.locator(conflict_sel).locator(".source-write-error").count() == 1,
            "visible source-write error after conflict",
        )
        error = page.locator(conflict_sel).locator(".source-write-error")
        record(
            "content_conflict_visible",
            error.get_attribute("data-error-code") == "CONTENT_CONFLICT",
            f"code={error.get_attribute('data-error-code')} text={error.inner_text()}",
        )
        record(
            "content_conflict_dialog",
            len(dialogs) > before_dialogs,
            "conflict did not surface a browser-visible dialog",
        )
        record(
            "stale_draft_preserved_after_conflict",
            page.locator(conflict_sel).locator("textarea").input_value() == "browser stale draft\n",
            page.locator(conflict_sel).locator("textarea").input_value(),
        )
        record(
            "external_edit_preserved",
            text(TEST_REPO / "conflict-ui.txt") == "external edit wins\n",
            repr(text(TEST_REPO / "conflict-ui.txt")),
        )
        record(
            "conflict_revision_separation",
            page.locator("#revision").inner_text() == revision_before_conflict,
            f"before={revision_before_conflict} after={page.locator('#revision').inner_text()}",
        )

        # Cancel the conflicted source draft, then prove SAVE CHANGES is workstation-only.
        page.locator(conflict_sel).locator('button[data-action="toggle-source-edit"]').click()
        source_before_save = text(TEST_REPO / "source.txt")
        conflict_before_save = text(TEST_REPO / "conflict-ui.txt")

        local_editable = page.locator('article[data-authority="local"][data-editable="true"]').first
        local_editable.locator('button[data-action="toggle-edit"]').click()
        local_textarea = local_editable.locator("textarea")
        local_textarea.fill(local_textarea.input_value() + "\nphase3 workstation save marker\n")
        wait_until(lambda: page.locator("#save-state").inner_text() == "UNSAVED EDITS", "workstation dirty indicator")

        revision_before_save = page.locator("#revision").inner_text()
        page.locator("#save-btn").click()
        wait_until(lambda: page.locator("#save-state").inner_text() == "SAVED", "workstation save completion")
        revision_after_save = page.locator("#revision").inner_text()

        record(
            "workstation_revision_advanced",
            revision_after_save != revision_before_save,
            f"before={revision_before_save} after={revision_after_save}",
        )
        record(
            "save_changes_does_not_write_source",
            text(TEST_REPO / "source.txt") == source_before_save and text(TEST_REPO / "conflict-ui.txt") == conflict_before_save,
            "repository bytes changed during SAVE CHANGES",
        )

        final = {
            "browser_executable": browser_path,
            "headless": HEADLESS,
            "initial_revision": initial_revision,
            "revision_before_source_write": revision_before_source_write,
            "revision_before_conflict": revision_before_conflict,
            "revision_before_save": revision_before_save,
            "revision_after_save": revision_after_save,
            "dialogs": dialogs,
            "checks": results,
        }
        (EVIDENCE / "browser-automation.json").write_text(json.dumps(final, indent=2), encoding="utf-8")
        page.screenshot(path=str(EVIDENCE / "browser-final.png"), full_page=True)
        context.close()
        browser.close()

except Exception as exc:
    failure = {
        "browser_executable": browser_path,
        "headless": HEADLESS,
        "error": repr(exc),
        "dialogs": dialogs,
        "checks": results,
    }
    (EVIDENCE / "browser-automation.json").write_text(json.dumps(failure, indent=2), encoding="utf-8")
    if page is not None:
        try:
            page.screenshot(path=str(EVIDENCE / "browser-failure.png"), full_page=True)
            (EVIDENCE / "browser-failure.html").write_text(page.content(), encoding="utf-8")
        except Exception:
            pass
    print(f"PHASE3_BROWSER_AUTOMATION=FAIL: {exc}", file=sys.stderr)
    raise SystemExit(1)

print(f"PHASE3_BROWSER_ENGINE={browser_path}")
print(f"PHASE3_BROWSER_HEADLESS={str(HEADLESS).lower()}")
print("PHASE3_BROWSER_AUTOMATION=PASS")
