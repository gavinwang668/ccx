---
name: desktop-store-update
description: 在 CCX Desktop 发布后更新 Microsoft Store。用户提到 Store 上架、Microsoft Partner Center、MSIX、从 GitHub Release 下载两个 store.msix、提交 CCX Desktop 商店更新、发布后同步 Windows Store、从 release 填写商店更新内容时必须使用此技能。该技能会下载最新 GitHub Release 的 amd64/arm64 MSIX，校验 sha256，从 Release body 生成 Store listing releaseNotes，并用 Microsoft Store submission API 创建/更新/提交 app submission；默认 dry-run，只有用户明确要求 submit/提交商店更新时才执行真实 Partner Center 提交。
version: 1.0.0
author: https://github.com/BenedictKing/ccx/
allowed-tools: Bash, Read
context: fork
---

# CCX Desktop Microsoft Store 更新技能

## 适用场景

当用户要求把 CCX Desktop 发布后的 Windows Store/MSIX 包提交到 Microsoft Store 或 Partner Center 时使用本技能。典型输入：

- “发布后把两个 msix 提交到 store”
- “更新 Microsoft Store 上的 CCX Desktop”
- “从最新 release 下载 MSIX 并调用 Partner API”
- “跑一下 desktop store update”

## 安全边界

Microsoft Store 提交是对外发布操作，默认只允许 dry-run。只有用户明确包含以下意图时，才允许加 `--submit`：

- “提交”、“真实提交”、“更新 Store”、“submit”
- 明确要求调用 Partner Center / Microsoft Store API 完成提交

如果用户没有明确授权真实提交，只运行 dry-run 并输出将要执行的步骤。

不要在命令行中明文传入 client secret。凭据只能从环境变量或本地配置文件读取。

## 凭据与配置

脚本读取以下环境变量：

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `MS_STORE_TENANT_ID` | submit 必填 | Entra ID tenant id |
| `MS_STORE_CLIENT_ID` | submit 必填 | Partner Center 关联应用 client id |
| `MS_STORE_CLIENT_SECRET` | submit 必填 | client secret |
| `MS_STORE_APPLICATION_ID` | submit 必填 | Partner Center app applicationId |
| `MS_STORE_GITHUB_REPO` | 可选 | 默认 `BenedictKing/ccx` |
| `MS_STORE_PACKAGE_GLOB` | 可选 | 默认 `CCX-Desktop-*-windows-*-store.msix` |

也可用 `--env-file <path>` 读取本地 env 文件。env 文件不能提交到仓库。

## 默认产物匹配

当前 release workflow 生成两个 Store MSIX：

- `CCX-Desktop-${VERSION}-windows-amd64-store.msix`
- `CCX-Desktop-${VERSION}-windows-arm64-store.msix`

脚本要求最新 GitHub Release 中恰好匹配 amd64 与 arm64 两个 `.msix`，并优先读取同名 `.sha256` 进行校验。

## 执行流程

### 1. Dry-run 检查最新 Release 和包完整性

```bash
python3 .claude/skills/desktop-store-update/scripts/update_store_submission.py --dry-run
```

dry-run 会：

1. 通过 GitHub API 读取最新 release。
2. 从 GitHub Release body 生成 Store listing `releaseNotes`，并输出预览。
3. 下载两个 MSIX 和对应 `.sha256` 到临时目录。
4. 校验架构集合必须为 `amd64` + `arm64`。
5. 校验 sha256（如果 release 中存在 `.sha256`）。
6. 创建将上传给 Store API 的 zip 包。
7. 输出 Partner Center 提交计划，但不调用 Microsoft API。

### 2. 真实提交 Store 更新

真实提交前必须确认用户已授权。执行：

```bash
MS_STORE_TENANT_ID="..." \
MS_STORE_CLIENT_ID="..." \
MS_STORE_CLIENT_SECRET="..." \
MS_STORE_APPLICATION_ID="..." \
python3 .claude/skills/desktop-store-update/scripts/update_store_submission.py --submit
```

如果用户提供的是 env 文件：

```bash
python3 .claude/skills/desktop-store-update/scripts/update_store_submission.py --submit --env-file /path/to/store.env
```

真实提交会：

1. 获取 Azure AD access token，resource 为 `https://manage.devcenter.microsoft.com`。
2. `POST /v1.0/my/applications/{applicationId}/submissions` 创建新 submission。
3. 将 `applicationPackages` 替换为两个 `PendingUpload` 包条目。
4. 将 Release body 转换为纯文本并写入 submission 中所有 Store listing 的 `baseListing.releaseNotes` 字段。
5. `PUT /v1.0/my/applications/{applicationId}/submissions/{submissionId}` 更新 submission。
6. 上传包含两个 MSIX 的 zip 到 submission 返回的 `fileUploadUrl`。
7. `POST /v1.0/my/applications/{applicationId}/submissions/{submissionId}/commit` 提交。
8. 轮询 `/status`，直到进入成功、失败或超时状态。

## 重要风险提示

- Microsoft 官方文档提醒：通过 Store submission API 创建的 submission，后续只能继续用 API 修改；不要再在 Partner Center UI 中编辑同一 submission，否则可能导致 submission 不能提交，需要删除重建。
- `--submit` 会创建并提交 Store ingestion/certification 流程，属于对外发布操作。
- 如果 Partner Center 中已有进行中的 submission，API 可能返回冲突或复制上次提交。不要盲目删除远端 submission；先把错误输出给用户。
- 前置条件：目标 app 必须已在 Partner Center 完成至少一次人工提交并填写 age ratings，否则 API 无法创建后续 submission。
- 默认拒绝 Draft 和 Prerelease Release；只有显式 `--allow-prerelease` 才允许 prerelease。
- 本技能用 Release 内的 `.sha256` 资产校验下载完整性。如需更强的供应链验证（Sigstore `checksums-windows.txt` bundle），目前需要人工执行 `cosign verify-blob`，脚本暂不集成。

## 常用参数

```bash
python3 .claude/skills/desktop-store-update/scripts/update_store_submission.py --help
```

常用选项：

- `--tag <tag>`：指定 release tag，不使用 latest。
- `--allow-prerelease`：允许 prerelease Release，默认拒绝。
- `--repo owner/name`：指定 GitHub 仓库。
- `--download-dir <dir>`：保留下载文件和 zip，便于审计。
- `--notes "..."`：提交 notesForCertification。
- `--store-release-notes "..."`：手动覆盖 Store listing 更新内容。
- `--release-notes-file <file>`：从本地文件读取 Store listing 更新内容。
- `--no-release-notes`：不更新 Store listing `releaseNotes`。
- `--release-notes-max-chars 1000`：Store 更新内容最大长度，默认 1000。
- `--truncate-release-notes`：超过长度时显式截断；默认超过长度会失败，避免静默丢内容。
- `--poll-timeout-seconds 1800`：提交后轮询超时。
- `--no-poll`：提交后不轮询状态。

## 输出要求

完成后输出中文摘要，至少包含：

- GitHub release tag 和 URL。
- Store 更新内容来源、长度和预览。
- 两个 MSIX 文件名、架构和 sha256 校验状态。
- dry-run 或 submit 模式。
- submit 模式下的 submission id、releaseNotes 写入语言、commit/status 结果。
- 如果失败，保留 HTTP 状态码和 Partner Center 返回的错误摘要。

## 官方参考

需要解释 API 行为时，先读 `references/store-submission-api.md`。
