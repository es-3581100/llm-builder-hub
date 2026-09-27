#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path
import sys
import time

try:
    from playwright.sync_api import sync_playwright, TimeoutError as PlaywrightTimeoutError
except Exception as exc:
    print(f"PHASE3_BROWSER_AUTOMATION_BLOCKED: python playwright unavailable: {exc}", file=sys.stderr)
    sys.exit(2)


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def wait_no_dialog(page, action):
    messages = []

    def on_dialog(dialog):
        messages.append(dialog.message)
        dialog.dismiss()

    page.once("dialog", on_dialog)
    action()
    page.wait_for_timeout(150)
    return messages


def repo_article(page, path: str):
    return page.locator(f'article[data-authority="repository"][data-source-path="{path}"]')


def local_editable_article(page):
    articles = page.locator('article[data-authority="local"][data-editable="true"]')
    if articles.count() < 1:
        raise AssertionError("no editable workstation-authority document found")
    return articles.first


def assert_textarea_value(article, expected: str):
    textarea = article.locator("textarea")
    textarea.wait_for(state="visible")
    actual = textarea.input_value()
    if actual != expected:
        raise AssertionError(f"textarea mismatch: expected={expected!r} actual={actual!r}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument("--browser-executable", required=True)
    args = parser.parse_args()

    repo = Path(args.repo)
    evidence = Path(args.evidence_dir)
    evidence.mkdir(parents=True, exist_ok=True)
    source = repo / "source.txt"
    conflict = repo / "conflict-ui.txt"

    report = {
        "url": args.url,
        "browser_executable": args.browser_executable,
        "checks": {},
        "dialogs": [],
    }

    browser = None
    try:
        with sync_playwright() as p:
            browser = p.chromium.launch(
                headless=False,
                executable_path=args.browser_executable,
                args=["--no-first-run", "--no-default-browser-check"],
            )
            page = browser.new_page()
            page.goto(args.url, wait_until="domcontentloaded")
            page.locator("#repository-status").wait_for(state="visible")

            source_article = repo_article(page, "source.txt")
            source_article.wait_for(state="visible")
            source_article.locator('[data-action="toggle-source-edit"]').click()
            textarea = source_article.locator("textarea")
            textarea.fill("phase3 browser write\nline two\n")
            source_article.locator('[data-state="source-draft"]')
            if source_article.get_attribute("data-state") != "source-draft":
                raise AssertionError("source.txt did not enter source-draft state")
            report["checks"]["source_edit"] = True

            conflict_article = repo_article(page, "conflict-ui.txt")
            dialogs = wait_no_dialog(
                page,
                lambda: conflict_article.locator('[data-action="toggle-source-edit"]').click(),
            )
            report["dialogs"].extend(dialogs)
            if not any("SOURCE DRAFT exists" in message for message in dialogs):
                raise AssertionError("dirty source draft did not block editing a second source")
            assert_textarea_value(source_article, "phase3 browser write\nline two\n")
            if conflict_article.locator("textarea").count() != 0:
                raise AssertionError("second source editor opened despite dirty first draft")
            report["checks"]["source_switch_block"] = True

            dialogs = wait_no_dialog(page, lambda: page.locator("#refresh-btn").click())
            report["dialogs"].extend(dialogs)
            if not any("SOURCE DRAFT exists" in message for message in dialogs):
                raise AssertionError("REFRESH did not block dirty source draft")
            assert_textarea_value(source_article, "phase3 browser write\nline two\n")
            report["checks"]["source_refresh_block"] = True

            source_article.locator('[data-action="write-file"]').click()
            page.wait_for_function(
                """() => {
                    const a = document.querySelector('article[data-authority="repository"][data-source-path="source.txt"]');
                    return a && a.dataset.editing === "false" && !a.querySelector("textarea");
                }"""
            )
            if source.read_text() != "phase3 browser write\nline two\n":
                raise AssertionError("WRITE FILE did not produce exact source.txt bytes")
            source_article = repo_article(page, "source.txt")
            if "phase3 browser write\nline two\n" not in source_article.locator("pre").inner_text():
                raise AssertionError("fresh source.txt projection does not show written bytes")
            page.wait_for_function(
                """() => document.querySelector("#repository-status")?.dataset.repositoryClean === "false" """
            )
            report["checks"]["write_success"] = True

            conflict_article = repo_article(page, "conflict-ui.txt")
            conflict_article.locator('[data-action="toggle-source-edit"]').click()
            conflict_article.locator("textarea").fill("browser stale draft\n")
            if conflict_article.get_attribute("data-state") != "source-draft":
                raise AssertionError("conflict-ui.txt did not enter source-draft state")

            conflict.write_text("external edit wins\n")
            external_sha = sha256(conflict)
            report["external_conflict_sha256"] = external_sha

            dialogs = wait_no_dialog(page, lambda: page.locator("#refresh-btn").click())
            report["dialogs"].extend(dialogs)
            if not any("SOURCE DRAFT exists" in message for message in dialogs):
                raise AssertionError("REFRESH did not preserve stale conflict draft")
            assert_textarea_value(conflict_article, "browser stale draft\n")

            dialogs = wait_no_dialog(
                page,
                lambda: conflict_article.locator('[data-action="write-file"]').click(),
            )
            report["dialogs"].extend(dialogs)
            page.wait_for_function(
                """() => {
                    const a = document.querySelector('article[data-authority="repository"][data-source-path="conflict-ui.txt"]');
                    return a?.querySelector('.source-write-error')?.dataset.errorCode === "CONTENT_CONFLICT";
                }"""
            )
            if not any("source content conflict" in message.lower() for message in dialogs):
                raise AssertionError("CONTENT_CONFLICT did not surface through browser error")
            assert_textarea_value(conflict_article, "browser stale draft\n")
            if sha256(conflict) != external_sha or conflict.read_text() != "external edit wins\n":
                raise AssertionError("rejected stale write changed external bytes")
            report["checks"]["content_conflict"] = True
            report["checks"]["conflict_draft_preserved"] = True

            conflict_article.locator('[data-action="toggle-source-edit"]').click()
            if conflict_article.locator("textarea").count() != 0:
                raise AssertionError("CANCEL SOURCE EDIT did not close source editor")
            if conflict.read_text() != "external edit wins\n":
                raise AssertionError("CANCEL SOURCE EDIT mutated source bytes")
            report["checks"]["cancel_nonmutating"] = True

            local_article = local_editable_article(page)
            local_article.locator('[data-action="toggle-edit"]').click()
            local_textarea = local_article.locator("textarea")
            before_local = local_textarea.input_value()
            local_textarea.fill(before_local + "\nphase3 workstation save proof")
            before_source_sha = sha256(source)
            before_conflict_sha = sha256(conflict)
            before_revision = page.locator("#revision").inner_text()

            page.locator("#save-btn").click()
            page.wait_for_function(
                """before => document.querySelector("#revision")?.textContent !== before""",
                arg=before_revision,
            )
            if sha256(source) != before_source_sha or sha256(conflict) != before_conflict_sha:
                raise AssertionError("SAVE CHANGES mutated repository source bytes")
            report["checks"]["save_write_separation"] = True
            report["revision_before_browser_save"] = before_revision
            report["revision_after_browser_save"] = page.locator("#revision").inner_text()

            report["source_sha256_after"] = sha256(source)
            report["conflict_sha256_after"] = sha256(conflict)
            report["result"] = "PASS"
            (evidence / "browser-report.json").write_text(json.dumps(report, indent=2) + "\n")
            browser.close()
            print("PHASE3_BROWSER_AUTOMATION=PASS")
            return 0
    except PlaywrightTimeoutError as exc:
        report["result"] = "FAIL"
        report["error"] = f"timeout: {exc}"
    except Exception as exc:
        report["result"] = "FAIL"
        report["error"] = str(exc)
    finally:
        if browser is not None:
            try:
                browser.close()
            except Exception:
                pass

    (evidence / "browser-report.json").write_text(json.dumps(report, indent=2) + "\n")
    print(f"PHASE3_BROWSER_AUTOMATION=FAIL: {report.get('error','unknown')}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
