#!/usr/bin/env python3
"""Update CCX Desktop Microsoft Store submission from GitHub Release MSIX assets.

Default mode is dry-run. Use --submit only after explicit release approval.
"""

from __future__ import annotations

import argparse
import fnmatch
import hashlib
import json
import os
import re
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any

GITHUB_API = "https://api.github.com"
STORE_RESOURCE = "https://manage.devcenter.microsoft.com"
STORE_BASE = "https://manage.devcenter.microsoft.com/v1.0/my/applications"
DEFAULT_REPO = "BenedictKing/ccx"
DEFAULT_PACKAGE_GLOB = "CCX-Desktop-*-windows-*-store.msix"
REQUIRED_ARCHES = {"amd64", "arm64"}
SUCCESS_STATUSES = {
    "PreProcessing",
    "Certification",
    "Release",
    "PendingPublication",
    "Publishing",
    "Published",
    "Completed",
}
FAILURE_STATUSES = {
    "CommitFailed",
    "PreProcessingFailed",
    "CertificationFailed",
    "ReleaseFailed",
    "PublishFailed",
    "Failed",
}


@dataclass(frozen=True)
class ReleaseAsset:
    name: str
    url: str
    size: int | None


@dataclass(frozen=True)
class PackageAsset:
    arch: str
    asset: ReleaseAsset
    path: Path
    sha256: str
    sha_status: str


class StoreUpdateError(RuntimeError):
    pass


def log(message: str) -> None:
    print(message, flush=True)


def load_env_file(path: Path) -> None:
    if not path.exists():
        raise StoreUpdateError(f"env 文件不存在: {path}")
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        os.environ.setdefault(key, value)


def request_json(method: str, url: str, *, token: str | None = None, body: Any = None) -> Any:
    headers = {"Accept": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            payload = resp.read().decode("utf-8")
            return json.loads(payload) if payload else {}
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise StoreUpdateError(f"HTTP {exc.code} {method} {url}: {detail[:2000]}") from exc
    except urllib.error.URLError as exc:
        raise StoreUpdateError(f"请求失败 {method} {url}: {exc}") from exc


def request_form(url: str, form: dict[str, str]) -> Any:
    data = urllib.parse.urlencode(form).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise StoreUpdateError(f"获取 Azure AD token 失败 HTTP {exc.code}: {detail[:2000]}") from exc


def download_file(url: str, dest: Path, *, max_retries: int = 4) -> None:
    headers = {"User-Agent": "ccx-store-update"}
    token = os.environ.get("GITHUB_TOKEN")
    if token and "github.com" in urllib.parse.urlparse(url).netloc:
        headers["Authorization"] = f"Bearer {token}"

    last_error: Exception | None = None
    mode = "wb"
    existing = 0
    for attempt in range(1, max_retries + 1):
        if dest.exists() and attempt > 1:
            existing = dest.stat().st_size
            headers["Range"] = f"bytes={existing}-"
            mode = "ab"
        else:
            headers.pop("Range", None)
            mode = "wb"
            existing = 0

        req = urllib.request.Request(url, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                expected = resp.headers.get("Content-Length")
                if existing and resp.status == 200:
                    # 服务端不支持 Range，整文件重下
                    mode = "wb"
                    existing = 0
                with dest.open(mode) as fh:
                    while True:
                        chunk = resp.read(1024 * 1024)
                        if not chunk:
                            break
                        fh.write(chunk)
                if expected:
                    total = int(expected) + existing if mode == "ab" else int(expected)
                    if dest.stat().st_size < total:
                        last_error = StoreUpdateError(f"下载不完整 {url}: {dest.stat().st_size}/{total}")
                        if attempt < max_retries:
                            log(f"下载不完整，重试 {attempt + 1}/{max_retries}: {url}")
                            time.sleep(min(2 ** attempt, 16))
                            continue
                        raise last_error
            return
        except urllib.error.HTTPError as exc:
            if exc.code in (408, 429, 500, 502, 503, 504) and attempt < max_retries:
                detail = exc.read().decode("utf-8", errors="replace")
                last_error = StoreUpdateError(f"下载可重试 HTTP {exc.code} {url}: {detail[:500]}")
                log(f"下载可重试 HTTP {exc.code}，重试 {attempt + 1}/{max_retries}: {url}")
                time.sleep(min(2 ** attempt, 16))
                continue
            detail = exc.read().decode("utf-8", errors="replace")
            raise StoreUpdateError(f"下载失败 HTTP {exc.code} {url}: {detail[:1000]}") from exc
        except (urllib.error.URLError, TimeoutError, ConnectionError) as exc:
            last_error = exc
            if attempt < max_retries:
                log(f"下载网络错误，重试 {attempt + 1}/{max_retries}: {url}: {exc}")
                time.sleep(min(2 ** attempt, 16))
                continue
            raise StoreUpdateError(f"下载失败 {url}: {exc}") from exc

    raise StoreUpdateError(f"下载失败，已重试 {max_retries} 次: {url}: {last_error}")


def github_release(repo: str, tag: str | None) -> dict[str, Any]:
    endpoint = f"{GITHUB_API}/repos/{repo}/releases/tags/{urllib.parse.quote(tag)}" if tag else f"{GITHUB_API}/repos/{repo}/releases/latest"
    headers = {"Accept": "application/vnd.github+json", "User-Agent": "ccx-store-update"}
    token = os.environ.get("GITHUB_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(endpoint, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise StoreUpdateError(f"读取 GitHub Release 失败 HTTP {exc.code}: {detail[:1000]}") from exc


def parse_assets(release: dict[str, Any], package_glob: str) -> tuple[list[ReleaseAsset], dict[str, ReleaseAsset]]:
    packages: list[ReleaseAsset] = []
    sha_assets: dict[str, ReleaseAsset] = {}
    for item in release.get("assets", []):
        name = item.get("name")
        url = item.get("browser_download_url")
        if not name or not url:
            continue
        asset = ReleaseAsset(name=name, url=url, size=item.get("size"))
        if fnmatch.fnmatch(name, package_glob):
            packages.append(asset)
        elif name.endswith(".sha256"):
            sha_assets[name.removesuffix(".sha256")] = asset
    return packages, sha_assets


def arch_from_name(name: str) -> str:
    match = re.search(r"windows-(amd64|arm64)-store\.msix$", name)
    if not match:
        raise StoreUpdateError(f"无法从 MSIX 文件名识别架构: {name}")
    return match.group(1)


def parse_sha256(text: str) -> str | None:
    match = re.search(r"\b[a-fA-F0-9]{64}\b", text)
    return match.group(0).lower() if match else None


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def download_packages(release: dict[str, Any], package_glob: str, download_dir: Path) -> list[PackageAsset]:
    packages, sha_assets = parse_assets(release, package_glob)
    if len(packages) != 2:
        names = ", ".join(sorted(asset.name for asset in packages)) or "无"
        raise StoreUpdateError(f"期望恰好 2 个 Store MSIX，实际 {len(packages)} 个: {names}")

    seen_arches: set[str] = set()
    result: list[PackageAsset] = []
    for asset in sorted(packages, key=lambda item: item.name):
        arch = arch_from_name(asset.name)
        if arch in seen_arches:
            raise StoreUpdateError(f"重复架构 MSIX: {arch}")
        seen_arches.add(arch)

        dest = download_dir / asset.name
        log(f"下载 MSIX: {asset.name}")
        download_file(asset.url, dest)
        digest = file_sha256(dest)

        sha_status = "release 中没有对应 .sha256，已计算本地 sha256"
        sha_asset = sha_assets.get(asset.name)
        if sha_asset:
            sha_dest = download_dir / sha_asset.name
            log(f"下载 sha256: {sha_asset.name}")
            download_file(sha_asset.url, sha_dest)
            expected = parse_sha256(sha_dest.read_text(encoding="utf-8", errors="replace"))
            if not expected:
                raise StoreUpdateError(f"无法解析 sha256 文件: {sha_asset.name}")
            if expected != digest:
                raise StoreUpdateError(f"sha256 不匹配: {asset.name}, expected={expected}, actual={digest}")
            sha_status = "sha256 校验通过"

        result.append(PackageAsset(arch=arch, asset=asset, path=dest, sha256=digest, sha_status=sha_status))

    if seen_arches != REQUIRED_ARCHES:
        raise StoreUpdateError(f"MSIX 架构集合错误: actual={sorted(seen_arches)}, expected={sorted(REQUIRED_ARCHES)}")
    return result


def create_zip(packages: list[PackageAsset], dest: Path) -> Path:
    with zipfile.ZipFile(dest, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        for package in packages:
            zf.write(package.path, arcname=package.asset.name)
    return dest


def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        raise StoreUpdateError(f"缺少环境变量: {name}")
    return value


def get_access_token(tenant_id: str, client_id: str, client_secret: str) -> str:
    token_url = f"https://login.microsoftonline.com/{urllib.parse.quote(tenant_id)}/oauth2/token"
    payload = request_form(token_url, {
        "grant_type": "client_credentials",
        "client_id": client_id,
        "client_secret": client_secret,
        "resource": STORE_RESOURCE,
    })
    token = payload.get("access_token")
    if not token:
        raise StoreUpdateError(f"Azure AD 响应中没有 access_token: {json.dumps(payload)[:1000]}")
    return token


def upload_zip_to_sas(file_upload_url: str, zip_path: Path) -> None:
    data = zip_path.read_bytes()
    req = urllib.request.Request(
        file_upload_url,
        data=data,
        headers={
            "Content-Type": "application/zip",
            "Content-Length": str(len(data)),
            "x-ms-blob-type": "BlockBlob",
        },
        method="PUT",
    )
    try:
        with urllib.request.urlopen(req, timeout=600) as resp:
            if resp.status not in (200, 201):
                raise StoreUpdateError(f"上传 ZIP 到 SAS URL 返回异常状态: {resp.status}")
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise StoreUpdateError(f"上传 ZIP 到 SAS URL 失败 HTTP {exc.code}: {detail[:2000]}") from exc


def build_package_entries(packages: list[PackageAsset]) -> list[dict[str, str]]:
    return [
        {
            "fileName": package.asset.name,
            "fileStatus": "PendingUpload",
            "minimumDirectXVersion": "None",
            "minimumSystemRam": "None",
        }
        for package in sorted(packages, key=lambda item: item.arch)
    ]


def normalize_release_notes(markdown: str, max_chars: int, truncate: bool) -> str:
    text = markdown.replace("\r\n", "\n").replace("\r", "\n").strip()
    if not text:
        return ""

    lines: list[str] = []
    skip_rest = False
    for raw_line in text.split("\n"):
        line = raw_line.strip()
        lower = line.lower()
        if lower.startswith("**full changelog**") or lower.startswith("full changelog"):
            skip_rest = True
        if skip_rest or line == "---":
            continue
        line = re.sub(r"^#{1,6}\s*", "", line)
        line = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r"\1", line)
        line = re.sub(r"[*_`]+", "", line)
        line = re.sub(r"<[^>]+>", "", line)
        lines.append(line)

    normalized = "\n".join(lines).strip()
    normalized = re.sub(r"\n{3,}", "\n\n", normalized)
    if len(normalized) <= max_chars:
        return normalized
    if not truncate:
        raise StoreUpdateError(
            f"Store releaseNotes 超过 {max_chars} 字符（实际 {len(normalized)}）。"
            "请用 --store-release-notes/--release-notes-file 提供精简内容，或显式加 --truncate-release-notes。"
        )
    suffix = "\n…"
    return normalized[: max_chars - len(suffix)].rstrip() + suffix


def resolve_store_release_notes(args: argparse.Namespace, release: dict[str, Any]) -> tuple[str, str]:
    if args.no_release_notes:
        return "", "disabled"
    if args.store_release_notes is not None:
        source = "--store-release-notes"
        raw = args.store_release_notes
    elif args.release_notes_file:
        source = f"--release-notes-file {args.release_notes_file}"
        raw = args.release_notes_file.read_text(encoding="utf-8")
    else:
        source = "GitHub Release body"
        raw = str(release.get("body") or "")
    notes = normalize_release_notes(raw, args.release_notes_max_chars, args.truncate_release_notes)
    return notes, source


def apply_release_notes_to_listings(submission: dict[str, Any], release_notes: str) -> list[str]:
    if not release_notes:
        return []
    listings = submission.get("listings")
    if not isinstance(listings, dict) or not listings:
        raise StoreUpdateError("submission 中没有 listings，无法自动填写 Store 更新内容 releaseNotes")

    updated_languages: list[str] = []
    for language, listing in listings.items():
        if not isinstance(listing, dict):
            continue
        base_listing = listing.get("baseListing")
        if not isinstance(base_listing, dict):
            continue
        base_listing["releaseNotes"] = release_notes
        updated_languages.append(str(language))

    if not updated_languages:
        raise StoreUpdateError("submission listings 中没有 baseListing，无法自动填写 releaseNotes")
    return updated_languages


def submit_to_store(
    application_id: str,
    token: str,
    packages: list[PackageAsset],
    zip_path: Path,
    certification_notes: str,
    release_notes: str,
    poll: bool,
    poll_timeout: int,
    poll_interval: int,
) -> dict[str, Any]:
    app_base = f"{STORE_BASE}/{urllib.parse.quote(application_id)}"
    log("创建 Microsoft Store submission...")
    submission = request_json("POST", f"{app_base}/submissions", token=token)
    submission_id = submission.get("id")
    upload_url = submission.get("fileUploadUrl")
    if not submission_id or not upload_url:
        raise StoreUpdateError(f"创建 submission 响应缺少 id 或 fileUploadUrl: {json.dumps(submission)[:2000]}")

    submission["applicationPackages"] = build_package_entries(packages)
    updated_release_note_languages = apply_release_notes_to_listings(submission, release_notes)
    if certification_notes:
        submission["notesForCertification"] = certification_notes

    log(f"更新 submission package 列表与 Store release notes: {submission_id}")
    updated = request_json("PUT", f"{app_base}/submissions/{urllib.parse.quote(str(submission_id))}", token=token, body=submission)
    upload_url = updated.get("fileUploadUrl") or upload_url

    log(f"上传 ZIP 到 Microsoft 提供的 SAS URL: {zip_path.name}")
    upload_zip_to_sas(upload_url, zip_path)

    log(f"提交 submission: {submission_id}")
    commit_result = request_json("POST", f"{app_base}/submissions/{urllib.parse.quote(str(submission_id))}/commit", token=token)

    status_result: dict[str, Any] | None = None
    if poll:
        status_result = poll_submission_status(app_base, str(submission_id), token, poll_timeout, poll_interval)

    return {
        "submissionId": submission_id,
        "updateStatus": updated.get("status"),
        "releaseNoteLanguages": updated_release_note_languages,
        "commitResult": commit_result,
        "statusResult": status_result,
    }


def poll_submission_status(app_base: str, submission_id: str, token: str, timeout_seconds: int, interval_seconds: int) -> dict[str, Any]:
    deadline = time.monotonic() + timeout_seconds
    last: dict[str, Any] = {}
    status_url = f"{app_base}/submissions/{urllib.parse.quote(submission_id)}/status"
    while time.monotonic() < deadline:
        last = request_json("GET", status_url, token=token)
        status = str(last.get("status", ""))
        log(f"submission status: {status or 'unknown'}")
        if status in SUCCESS_STATUSES or status in FAILURE_STATUSES:
            return last
        time.sleep(interval_seconds)
    raise StoreUpdateError(f"轮询超时，最后状态: {json.dumps(last, ensure_ascii=False)[:2000]}")


def print_summary(
    release: dict[str, Any],
    packages: list[PackageAsset],
    zip_path: Path,
    mode: str,
    release_notes: str,
    release_notes_source: str,
    store_result: dict[str, Any] | None,
) -> None:
    print("\n📦 CCX Desktop Store 更新准备结果")
    print(f"- 模式: {mode}")
    print(f"- Release: {release.get('tag_name')} ({release.get('html_url')})")
    print(f"- ZIP: {zip_path}")
    print(f"- Store 更新内容来源: {release_notes_source}")
    if release_notes:
        preview = release_notes[:500] + ("…" if len(release_notes) > 500 else "")
        print(f"- Store 更新内容长度: {len(release_notes)} 字符")
        print("- Store 更新内容预览:")
        for line in preview.splitlines() or [preview]:
            print(f"  {line}")
    else:
        print("- Store 更新内容: 未填写")
    print("- MSIX:")
    for package in sorted(packages, key=lambda item: item.arch):
        print(f"  - {package.arch}: {package.asset.name}")
        print(f"    sha256: {package.sha256}")
        print(f"    校验: {package.sha_status}")
    if store_result:
        print("- Partner Center:")
        print(f"  - submissionId: {store_result.get('submissionId')}")
        print(f"  - updateStatus: {store_result.get('updateStatus')}")
        release_note_languages = store_result.get("releaseNoteLanguages") or []
        if release_note_languages:
            print(f"  - releaseNotes 写入语言: {', '.join(release_note_languages)}")
        status_result = store_result.get("statusResult") or {}
        if status_result:
            print(f"  - finalStatus: {status_result.get('status')}")
            details = status_result.get("statusDetails")
            if details:
                print(f"  - statusDetails: {json.dumps(details, ensure_ascii=False)[:1000]}")
    else:
        print("- Partner Center: dry-run 未调用 Microsoft API")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Update CCX Desktop Microsoft Store submission from GitHub Release MSIX assets.")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--dry-run", action="store_true", help="download and validate assets only; this is the default")
    mode.add_argument("--submit", action="store_true", help="create/update/commit Microsoft Store submission")
    parser.add_argument("--repo", default=os.environ.get("MS_STORE_GITHUB_REPO", DEFAULT_REPO), help=f"GitHub repo, default: {DEFAULT_REPO}")
    parser.add_argument("--tag", help="release tag; default uses latest release")
    parser.add_argument("--allow-prerelease", action="store_true", help="allow submitting a prerelease; default rejects prereleases")
    parser.add_argument("--package-glob", default=os.environ.get("MS_STORE_PACKAGE_GLOB", DEFAULT_PACKAGE_GLOB), help=f"MSIX asset glob, default: {DEFAULT_PACKAGE_GLOB}")
    parser.add_argument("--download-dir", type=Path, help="directory to store downloaded files; default uses a temporary directory")
    parser.add_argument("--env-file", type=Path, help="optional env file for MS_STORE_* variables")
    parser.add_argument("--notes", default="CCX Desktop automated Store package update.", help="notesForCertification value")
    parser.add_argument("--store-release-notes", help="override Store listing releaseNotes; default reads GitHub Release body")
    parser.add_argument("--release-notes-file", type=Path, help="read Store listing releaseNotes from a local text/markdown file")
    parser.add_argument("--no-release-notes", action="store_true", help="do not update Store listing releaseNotes")
    parser.add_argument("--release-notes-max-chars", type=int, default=1000, help="maximum Store releaseNotes length; default: 1000")
    parser.add_argument("--truncate-release-notes", action="store_true", help="truncate releaseNotes when over the max length instead of failing")
    parser.add_argument("--no-poll", action="store_true", help="do not poll submission status after commit")
    parser.add_argument("--poll-timeout-seconds", type=int, default=1800)
    parser.add_argument("--poll-interval-seconds", type=int, default=30)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.env_file:
        load_env_file(args.env_file)

    temp_ctx = None
    if args.download_dir:
        download_dir = args.download_dir
        download_dir.mkdir(parents=True, exist_ok=True)
    else:
        temp_ctx = tempfile.TemporaryDirectory(prefix="ccx-store-update-")
        download_dir = Path(temp_ctx.name)

    try:
        mode = "submit" if args.submit else "dry-run"
        log(f"读取 GitHub Release: repo={args.repo}, tag={args.tag or 'latest'}")
        release = github_release(args.repo, args.tag)
        if release.get("draft"):
            raise StoreUpdateError("最新 Release 是 Draft，不参与 Store 提交。请先正式发布 Release。")
        if release.get("prerelease") and not args.allow_prerelease:
            raise StoreUpdateError("最新 Release 是 Prerelease，默认不提交 Store。如需提交请显式加 --allow-prerelease。")
        release_notes, release_notes_source = resolve_store_release_notes(args, release)
        packages = download_packages(release, args.package_glob, download_dir)
        safe_tag = re.sub(r"[^A-Za-z0-9_.-]+", "-", str(release.get("tag_name") or "latest"))
        zip_path = create_zip(packages, download_dir / f"ccx-desktop-store-{safe_tag}.zip")

        store_result = None
        if args.submit:
            tenant_id = require_env("MS_STORE_TENANT_ID")
            client_id = require_env("MS_STORE_CLIENT_ID")
            client_secret = require_env("MS_STORE_CLIENT_SECRET")
            application_id = require_env("MS_STORE_APPLICATION_ID")
            log("获取 Azure AD access token...")
            token = get_access_token(tenant_id, client_id, client_secret)
            store_result = submit_to_store(
                application_id=application_id,
                token=token,
                packages=packages,
                zip_path=zip_path,
                certification_notes=args.notes,
                release_notes=release_notes,
                poll=not args.no_poll,
                poll_timeout=args.poll_timeout_seconds,
                poll_interval=args.poll_interval_seconds,
            )

        print_summary(
            release=release,
            packages=packages,
            zip_path=zip_path,
            mode=mode,
            release_notes=release_notes,
            release_notes_source=release_notes_source,
            store_result=store_result,
        )
        if temp_ctx:
            print("\n提示: 未指定 --download-dir，临时下载目录会在脚本退出后删除。需要审计文件时请使用 --download-dir。")
        return 0
    except StoreUpdateError as exc:
        print(f"\n❌ Store 更新失败: {exc}", file=sys.stderr)
        return 1
    finally:
        if temp_ctx:
            temp_ctx.cleanup()


if __name__ == "__main__":
    raise SystemExit(main())
