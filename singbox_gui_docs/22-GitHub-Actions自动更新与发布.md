# 22 GitHub Actions 自动更新与发布

本章是待实现的工作流契约。目标为“自动发现上游更新、自动准备候选、验证通过后发布”。协议未变不代替运行验证；核心同步与 Apple 界面同步分别记录（16 章）。

## 22.1 工作流总览

| 文件 | 触发 | 作用 |
|---|---|---|
| `ci.yml` | 推送、PR、`workflow_call` | 构建、静态检查、单元/组件测试、协议再生成、许可与临时签名测试；可复用入口接收源提交 `ref` |
| `upstream-core.yml` | 每日 UTC 01:00（新加坡 09:00）、手动 | 分通道发现核心标签，生成差异报告，准备同步提交，CI 通过后打候选标签 |
| `upstream-apple.yml` | 每日 UTC 01:30（新加坡 09:30）、手动 | 跟踪 Apple 提交，生成移植清单，开议题或 PR |
| `ui-sync-release.yml` | 已合并且带 `upstream-apple` 标签的 PR | 在已验收提交上分配 `+w.N` 修订号，CI 通过后打标签 |
| `release.yml` | 推送 `v*` 标签、手动重试指定标签 | 固定候选提交，CI、构建、签名、打包、摘要签名、扫描，创建草稿；不直接公开 |
| `promote.yml` | 手动指定候选 run ID 与证据，或可信实机测试成功事件 | 校验候选和实机证据，公开同一草稿的同一批资产 |
| `cert-expiry.yml` | 每周、手动 | 检查代码签名证书剩余有效期（21.1.3） |

标签推送是自动准备候选的唯一入口。上游工作流使用 GitHub App 令牌推送标签后，不再 `workflow_call` 调用 `release.yml`，以免重复构建。手动重试用于恢复失败候选，不能覆盖已公开标签或资产。

定时工作流在默认分支运行，但分别读取 `main`、`beta` 的锁定状态。上游处理串行；所有会写同步分支或分配版本的工作流共享同一同步锁，避免核心更新与界面修订互相覆盖；正式晋级另设全局锁。不取消正在签名、打标签或晋级的任务。

## 22.2 上游核心：`upstream-core.yml`

### 22.2.1 发现与恢复
1. 获取官方标签，接受 `^v\d+\.\d+\.\d+(-(alpha|beta|rc)\.\d+)?$`，解析带注释标签对应的提交。手动输入也须验证格式、来源及提交，不直接插入 shell 命令。
2. 稳定通道对照 `main` 锁定项；测试通道对照 `beta` 锁定项，分别选择最新候选，先处理稳定再处理测试。`beta` 合并 `main` 不得回退已有核心版本；冲突则开议题，不强行覆盖。
3. 无公开版本的候选区分：未准备、适配受阻、构建失败、等待验证、可晋级。已存在议题或标签不代表已发布；重试更新原议题，不重复打标签。已公开且校验一致才算完成。
4. 比较该通道旧提交和候选提交；检测失败、源码不可获取、清单无法解析均失败关闭，不能当成“无变化”。

### 22.2.2 协议和行为比对
- 使用 16.1 的 `scripts/proto/manifest.json` 及同一依赖解析器，检查 `daemon/*.proto`、`experimental/boxdd/desktop_service.proto` 和全部递归导入。内容、路径、导入或生成工具版本变化均须审查，不能只检查 `daemon/`。
- 检查 `experimental/boxdd/**` 全部变化：签名验证及调用链、父子进程校验、worker 参数、管道、安全目录、服务注册、所有权、更新安装等。可用 AST（抽象语法树，用于比较代码结构）辅助分类，但不能仅比对几个函数名或常量就宣告兼容。
- 同时检查 `cmd/internal/build_boxdd/**`、`cmd/internal/build_shared/**`、`release/DEFAULT_BUILD_TAGS*`、核心 `go.mod`/`go.sum`、16.3 的配置结构与数据语义文件。构建依赖、驱动、工具链变化均登记。
- `api-diff.md` 包含变更路径、接口/行为影响、章节、所需适配、负责人和结论。未分类变化均待审查；范围外的核心变化仍须阅读发布说明和真实冒烟，路径检查不能证明行为等价。
- `api-diff.go` 返回 0 表示监视集合无变化，1 表示需要适配审查，其他值表示检测失败。1 开议题；其他值终止并报告故障；二者都不能进入无人审查的同步路径。

### 22.2.3 准备候选
1. 无待审查变化时，机器人更新对应分支的核心子模块与锁定项，保留 Apple 基线；有变化时由适配 PR 完成。提交记录来源和差异报告。
2. 在确定提交上运行 `ci.yml`，`ref` 必须是完整提交 SHA，不能为空或浮动分支名；通过后在**同一 SHA** 上打附注标签。
3. GitHub App 推送标签触发 `release.yml`。打标签任务显式 `needs: [detect, bump, test]`，不能从间接依赖读取 `needs.bump.outputs.ref`。
4. 公开发布完成后才关闭跟版议题；草稿阶段记录候选链接、验证状态及阻塞原因。

### 22.2.4 适配议题
标题 `[upstream-core] <标签>：兼容性适配`，包含官方来源、旧/新 SHA、完整差异产物、16.5 清单和测试。可修改前端适配层，不允许给官方后端打补丁；必须改后端时按 A-29 报告目标冲突。

## 22.3 Apple 界面同步
1. 检测官方 Apple 新提交，记录待移植目标，不直接更新“已对齐”锁定项。
2. 对所有变更路径分类，重点覆盖 16.3 的视图、模型、资源、本地化；未映射路径待审查，不因目录白名单而忽略，移动/删除文件同样处理。
3. 生成 Windows 模块清单、五种语言文案差异、行为变化及明确的平台例外。逐项完成实现、文档、测试，按 3.9/20.4 对照截图。
4. 验收完成才更新 Apple 子模块及 `apple_client.commit`/`version`；确实不影响本应用的提交须附依据，可走仅更新基线的 PR。
5. `ui-sync-release.yml` 在确定提交上运行 CI，按通道串行分配 `+w.N` 并推送标签。核心为预发布时修订仍属测试通道。PR 合并只是触发条件，必须明确检出待发布 SHA，不能隐式使用默认分支当前 HEAD。
6. 发布说明分开写核心版本、已对齐 Apple 提交、未完成差异。核心先更新不代表 Apple 新功能已经移植。

## 22.4 候选构建与晋级

### 22.4.1 `release.yml` 作业
| 作业 | 环境 | 必须完成 |
|---|---|---|
| resolve | 托管 Linux | 校验标签/通道、解析完整 SHA、核对锁定项和子模块；已有正式发布则拒绝覆盖 |
| ci | 可复用 CI | 对 resolve 的 SHA 执行 22.7 |
| build | 托管 Windows，amd64/arm64 矩阵，`release` 环境 | 检出同一 SHA；分别安装 GUI/核心 Go 工具链；官方脚本构建后端、MyGo 构建 GUI、签名、NSIS 打包、签名者核验 |
| checksums | 托管 Linux，`release` 环境 | 汇集产物、完整源码包（显式包含子模块，不能仅 git archive 父仓库）、SHA256SUMS、SHA256SUMS.unsigned、Ed25519 签名及自检 |
| scan | 托管 Windows | 更新 Defender 特征并扫描最终产物；有检出失败 |
| draft | 托管 Linux | 创建或恢复候选草稿，上传资产和来源；不设置 latest，不公开 |

- 资产：`sing-box-<版本>-windows-amd64-setup.exe`、`sing-box-<版本>-windows-arm64-setup.exe`、`sing-box-<版本>-src.tar.gz`、`SHA256SUMS`、`SHA256SUMS.unsigned`、`SHA256SUMS.sig`、`candidate.json`。版本资源按 16.2。
- `candidate.json` 由可信工作流生成，记录标签、源 SHA、上游提交、工具链/构建标签、候选 run ID、证书指纹及安装包摘要。先生成候选记录，再把它列入签名的 SHA256SUMS，避免循环摘要；20.7 证据另绑定 SHA256SUMS 摘要。
- 最终产物冻结后才实测。重构建或重签名视为新候选 run，使旧验证失效。已公开资产不得替换，修复须发布新修订号。
- `release` 环境限制可信分支/标签；PR 和 fork 不得获取签名私钥。构建不等于批准公开发布。

### 22.4.2 默认：人工实测、受保护晋级
1. 自动生成草稿和产物；维护者下载最终候选，在 20.7 环境实测，填写 `validation.json` 并附日志。
2. 运行 `promote.yml`，输入候选 run ID 与证据位置。核验来源工作流、仓库、成功状态、标签 SHA、签名清单、全部资产摘要、证据环境及用例。失败、缺失、重构建后旧证据或包哈希不匹配均终止。
3. 晋级使用独立 `release-publish` 环境，默认设置必需审阅者核对实测记录；**审批不能代替证据**。该环境只需发布权限，不需签名私钥。
4. 再检查草稿候选与资产未变，原地转正式或 prerelease，不重新编译、签名、打包。稳定版仅在版本确实高于当前 latest 时更新 latest，避免晚完成的旧版占据入口。
5. 首次公开发布另检查 A-10、A-26 和第 17 章条件；未完成时可继续构建验证，但不公开。

### 22.4.3 可选：自动实测与晋级
可信交互式 Windows 运行器覆盖 20.7 矩阵后，可自动执行相同测试、生成证据并晋级。运行器只接收可信候选，不执行 PR 任意代码，不持有签名私钥；不能在会话 0 把认证拒绝算作通过。覆盖不足的环境仍人工实测，不能仅移除审批就宣称已实现全自动发布。

### 22.4.4 失败恢复
- 构建/扫描失败不公开；草稿失败保留故障记录，可清理不完整草稿资产，不影响正式版本。
- 测试失败保留候选与日志，修复后重新构建验证，不向客户端推送。
- 晋级幂等重试，先确认是否已公开和资产是否一致，不能删除已成功公开版本。客户端不读取草稿。

## 22.5 证书到期
每周检查 `keys/codesign.cer`，剩余不足 18 个月开议题，不足 12 个月标为 urgent 并启动轮换（21.1.4）。在 `release` 环境核对 PFX 证书与仓库公钥指纹。私钥不得进入日志和产物。

## 22.6 权限与机密
| 名称 | 作用域 | 用途 |
|---|---|---|
| CODESIGN_PFX_BASE64、CODESIGN_PFX_PASSWORD | release 环境机密 | 签署 GUI、后端、卸载器、安装包 |
| RELEASE_ED25519_PRIVATE_KEY | release 环境机密 | 签署 SHA256SUMS |
| UPSTREAM_BOT_APP_ID | 仓库变量 | 同步 App 标识 |
| UPSTREAM_BOT_PRIVATE_KEY | 仓库机密 | 同步提交和标签；限制仓库及分支权限 |
| GITHUB_TOKEN | 自动 | 默认 contents: read；开议题/PR、草稿和晋级在对应任务声明最小写权限 |

签名机密另有离线加密备份。第三方 Action 固定完整 SHA。可复用工作流不能提升调用者权限；增加调用层时显式传递所需权限，不能只在被调用工作流中升级。证据上传和晋级入口不能向普通 PR 开放。

## 22.7 CI 与发布验证
- 格式、静态分析、单元/组件测试、更新版本比较、签名校验、国际化、许可检查。
- 16.1 全部协议再生成无差异，依赖清单/描述符与源码一致；tag、commit、子模块、MyGo 依赖一致；后端无源码修改。
- 双架构构建、临时证书自测：相同/不同签名者、未签名、篡改、过期、未生效、缺少代码签名用途。
- 假服务只验证应用逻辑，不代替 worker、服务认证和升级。真实发布验证统一按 20.7，每个候选均执行。
- 工作流回归：只改桌面 proto 会触发适配；递归依赖被识别；检测失败不放行；CI SHA 与标签 SHA 一致；标签只触发一次候选构建；失败可重试；通道不串线；缺失/错误哈希证据不能晋级；已公开资产不能覆盖。

## 22.8 工作流依赖骨架

以下是**不完整片段**，仅表示依赖与参数契约，不是可直接部署的工作流。开发者须实现 22.1–22.7 的步骤、脚本和权限，用 actionlint 等校验最终 YAML；省略步骤、Action SHA、输入检查完成前不得启用公开晋级。

```yaml
# upstream-core.yml 的关键依赖；detect/bump 及 steps 按 22.2 实现
jobs:
  test:
    needs: bump
    uses: ./.github/workflows/ci.yml
    with:
      ref: ${{ needs.bump.outputs.ref }} # 完整候选 SHA
  tag:
    needs: [detect, bump, test]          # 必须直接依赖 bump
    runs-on: ubuntu-latest
    env:
      CANDIDATE_SHA: ${{ needs.bump.outputs.ref }}
      CANDIDATE_TAG: ${{ needs.detect.outputs.tag }}
    # steps: 校验输入、检出 SHA、确认 CI SHA、打标签并以 App 推送
    # 推送后结束，不再调用 release.yml
```

`release.yml` 入口只保留如下两种，不设置 `workflow_call`：

```yaml
name: release-candidate
on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      tag:
        type: string
        required: true
concurrency:
  group: candidate-${{ inputs.tag || github.ref_name }}
  cancel-in-progress: false
permissions:
  contents: read
# jobs: resolve -> ci -> build -> checksums -> scan -> draft
# 后续 jobs 显式依赖 resolve，使用其完整 SHA；通道由校验后的标签解析。
# draft 单独声明 contents: write；不存在公开 publish 作业。
```

`promote.yml` 使用独立全局晋级锁保护 latest，按 22.4.2 从可信运行记录读取候选，不信任任意 URL 或 JSON 自称的 SHA。脚本通过结构化参数或环境变量接收输入，不把外部文本直接插入 shell。

## 22.9 客户端更新路径
| 场景 | 路径 | UAC（Windows 管理员授权提示） |
|---|---|---|
| 新核心版本，同证书且 ProductName 为 sing-box | 守护进程 InstallUpdate | 无 |
| 同核心 GUI 修订 +w.N | NOT_NEWER 后运行安装包 | 有 |
| 证书轮换 | SIGNER_MISMATCH 后运行安装包 | 有 |
| 后端不可用或产品名不是 sing-box | 运行安装包 | 有 |

两条路径均先验证 SHA256SUMS 的 Ed25519 签名及安装包哈希。只读公开通道，按 12.5 比较核心版本和修订号。公开发布与客户端检查/用户安装是不同步骤，不因跟版取消客户端确认。
