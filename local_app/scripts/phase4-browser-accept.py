#!/usr/bin/env python3
import argparse
import json
from pathlib import Path
import sys
import urllib.request

try:
    from playwright.sync_api import sync_playwright, TimeoutError as PlaywrightTimeoutError
except Exception as exc:
    print(f"PHASE4_BROWSER_AUTOMATION_BLOCKED: python playwright unavailable: {exc}", file=sys.stderr)
    sys.exit(2)


def api_state(base_url: str):
    with urllib.request.urlopen(f"{base_url.rstrip('/')}/api/state", timeout=10) as response:
        return json.load(response)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--evidence-dir", required=True)
    parser.add_argument("--browser-executable", required=True)
    parser.add_argument("--prompt-file", required=True)
    parser.add_argument("--model", required=True)
    parser.add_argument("--variant", default="")
    parser.add_argument("--timeout-seconds", type=int, default=300)
    args = parser.parse_args()

    evidence = Path(args.evidence_dir)
    prompt = Path(args.prompt_file).read_text()
    report = {
        "url": args.url,
        "browser_executable": args.browser_executable,
        "model": args.model,
        "variant": args.variant,
        "checks": {},
        "dialogs": [],
    }
    browser = None
    try:
        before = api_state(args.url)
        before_revision = int(before["state"]["revision"])
        before_repository = before["repository"]
        if not before_repository["clean"]:
            raise AssertionError("repository was not clean before browser execution")

        with sync_playwright() as p:
            browser = p.chromium.launch(
                headless=False,
                executable_path=args.browser_executable,
                args=["--no-first-run", "--no-default-browser-check"],
            )
            page = browser.new_page()
            page.on("dialog", lambda dialog: (report["dialogs"].append(dialog.message), dialog.dismiss()))
            page.goto(args.url, wait_until="domcontentloaded")
            page.locator("#execution-panel").wait_for(state="visible")

            state = page.locator("#execution-state").inner_text()
            if state != "READY":
                raise AssertionError(f"execution surface was not READY: {state!r}")
            report["checks"]["execution_surface_ready"] = True

            page.locator("#execution-model").fill(args.model)
            page.locator("#execution-variant").fill(args.variant)
            page.locator("#execution-timeout").fill(str(args.timeout_seconds))
            if page.locator("#execution-auto").is_checked():
                page.locator("#execution-auto").uncheck()
            page.locator("#execution-prompt").fill(prompt)

            page.locator("#run-opencode-btn").click()

            page.wait_for_function(
                """() => {
                    const result = document.querySelector("#execution-result")?.textContent || "";
                    const state = document.querySelector("#execution-state")?.textContent || "";
                    return state !== "RUNNING" &&
                           !result.startsWith("No execution result") &&
                           !result.startsWith("OpenCode is running");
                }""",
                timeout=(args.timeout_seconds + 60) * 1000,
            )

            result_text = page.locator("#execution-result").inner_text()
            report["result_text"] = result_text
            if "status: PASS" not in result_text:
                raise AssertionError(f"browser execution did not PASS: {result_text!r}")
            if "run:" not in result_text or "prompt sha256:" not in result_text or "evidence:" not in result_text:
                raise AssertionError(f"browser result missing execution identity: {result_text!r}")
            report["checks"]["browser_run_pass"] = True

            after = api_state(args.url)
            after_revision = int(after["state"]["revision"])
            if after_revision != before_revision:
                raise AssertionError(f"RUN advanced workstation revision {before_revision} -> {after_revision}")
            report["checks"]["workstation_revision_unchanged"] = True

            if not after["repository"]["clean"]:
                raise AssertionError("no-op OpenCode acceptance left repository dirty")
            if after["repository"]["head_commit"] != before_repository["head_commit"]:
                raise AssertionError("no-op OpenCode acceptance changed repository HEAD")
            report["checks"]["repository_remained_clean"] = True
            report["checks"]["head_unchanged"] = True

            report["revision_before"] = before_revision
            report["revision_after"] = after_revision
            report["repository_id"] = before_repository["repository_id"]
            report["head_commit"] = before_repository["head_commit"]
            report["result"] = "PASS"
            (evidence / "browser-report.json").write_text(json.dumps(report, indent=2) + "\n")
            browser.close()
            print("PHASE4_BROWSER_AUTOMATION=PASS")
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
    print(f"PHASE4_BROWSER_AUTOMATION=FAIL: {report.get('error','unknown')}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
