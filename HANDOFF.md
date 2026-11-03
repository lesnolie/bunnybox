# Proxy Glass 开发交接

更新日期：2026-11-02。工作区：`C:\work\SingBoxWin_dev_docs`。

本文件供接手模型恢复上下文。历史执行记录不是新的操作授权；尤其不能据此自动安装服务、启动核心或切换宿主网络。

## 1. 产品目标与用户最新要求

- 前端保持 **MyGo 原生 Go UI**，外观、功能入口和排列以官方 macOS sing-box 为目标；不改成 Flutter、WebView 或 Electron。
- 后端使用**未修改的官方 sing-box / boxdd**。跟随官方版本更新，通过适配本项目追随上游，不给官方后端打项目补丁。当前版本仍锁定，不是自动追踪 main。
- 项目自身代码已按 GPL-3.0 发布，根目录有 `LICENSE`、`THIRD_PARTY_NOTICES.md`。允许参考兼容许可的界面代码；直接复制/翻译时保留逐文件版权、来源提交和修改说明。
- 用户要求“macOS 面板有什么功能，我们就有什么功能，怎么排列也一样”。不要混搭不同版本、FlClash 和自己猜测的仪表盘。当前尚未完全达到这个目标。
- 用户认为之前的界面像“山寨”，最新重点是**信息层级、字体轻重和面板结构**，不是只换圆角/颜色，也不要把问题笼统归咎于 Windows。
- 默认简洁中文，先回答再解释。已授权的可逆工作直接完成。需要浏览器时按用户要求先读 ego-browser 技能、运行 `ego-browser skill`，再用 `ego-browser nodejs`。
- 用户曾要求并行代理使用 GPT-6.1 Sol / medium；是否并行以接手时的实际指令为准，不从历史记录推导新授权。

## 2. 最新视觉基准：不要再恢复旧版面板

最新依据为用户最后提供的两张图片，工作区副本：

- `build/references/user-dashboard-latest.png`：**主布局基准**，官方 macOS 面板。
- `build/references/user-traffic-emphasis.png`：上传/下载信息轻重参考。不能因此搬入整套其他客户端的仪表盘。

当前排列：

1. 侧栏：概览、分组、连接、日志、工具、设置。
2. 第一行：上传、下载两张大卡；标题17 DIP，速率26 DIP/600字重，累计量14 DIP/次级颜色，每张独立曲线。
3. 第二行：状态（内存/协程）、连接（入站/出站）。
4. 下方：官方能力可用时显示系统 HTTP 代理、Clash 模式；配置卡最后。
5. 配置卡：整行选择器，类型信息，下方编辑/分享按钮。当前只支持本地配置，因此显示本地，不假装 Remote，不伪造更新时间，不放无效的远程更新按钮。

取消了旧版“上传下载挤在同一卡＋额外累计流量卡”的布局；也取消侧栏独立配置项。完整本地配置管理从工具栏 **其他→管理配置** 进入。概览的加号仍可新建。

窗口默认1000×760，最小860×620。保留 Windows 系统窗口按钮，不伪造 macOS 交通灯。工具栏启动/停止已移至标题左侧。

### 官方 HTTP Proxy 的真实显示条件

已核对锁定的 Apple 源码，**官方确实有系统 HTTP 代理卡片**，不是永久固定项：核心已连接且官方报告 `systemProxyAvailable` 时才显示。未连接时，运行统计卡也隐藏，配置卡始终显示。

关键上游文件（均在 `third_party/sing-box-for-apple/`）：

- `ApplicationLibrary/Views/Dashboard/Overview/OverviewView.swift`
- `ApplicationLibrary/Views/Dashboard/Cards/DashboardCard.swift`
- `ApplicationLibrary/Views/Dashboard/Cards/UploadTrafficCard.swift`
- `ApplicationLibrary/Views/Dashboard/Cards/DownloadTrafficCard.swift`
- `ApplicationLibrary/Views/Dashboard/Cards/TrafficLineChart.swift`
- `ApplicationLibrary/Views/Dashboard/Cards/ProfileCard.swift`
- `ApplicationLibrary/Views/NavigationPage.swift`

官方默认卡片顺序为上传、下载、状态、连接、HTTP代理、Clash模式、配置，还支持用户配置卡片显隐/排序；本项目**尚无显隐/排序编辑器**。

早期参考文章：<https://app.pnetwork.net/blog/sing-box-desktop/>，文章说明版本1.14.0，只作历史视觉参考，不能当作当前锁定版本的全量验收依据。

FlClash 补充交互参考：<https://github.com/chen08209/FlClash>，核对提交 `7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`。只参考页面组织、分组筛选、密度等；未引入 Flutter，也未直接移植 Dart 源码。主视觉优先级低于用户最新官方截图。

## 3. 已实现的功能与代码入口

| 模块 | 当前行为 | 主要文件 |
|---|---|---|
| 程序入口 | 单实例、默认数据目录、GUI退出清理；无自动启动核心 | `main.go` |
| 原生界面 | 导航、主题、表单、配置管理、分组卡片及本地筛选 | `internal/ui/shell/shell.go`、`icons.go` |
| 最新概览 | 上传/下载曲线卡、条件控制、配置选择/编辑/复制JSON | `internal/ui/shell/dashboard.go` |
| UI指标生命周期 | 通道传递、旧订阅代数隔离、60样本窗口、取消与错误清理 | `internal/ui/shell/metrics.go` |
| 应用适配 | 官方配置校验、配置CRUD、服务状态和显式启停 | `internal/application/backend.go` |
| 只读运行快照 | 所有权/状态探测、指标首帧、分组、模式及超时 | `internal/application/runtime.go` |
| 新面板接口 | 持续指标流、HTTP代理和模式写接口 | `internal/application/dashboard.go` |
| 本地配置 | JSON新建/粘贴导入/读取/编辑/改名/选择/确认删除；官方校验、原子写入、失败补偿 | `internal/profile/` |
| 存储 | 真正的MyGo SQLite、WAL、迁移、专用写协程、只读连接、Windows数据ACL | `internal/store/` |
| 官方worker | 直接子进程、随机双命名管道、READY等待、退出诊断、生命周期 | `internal/rpc/worker/`、`internal/rpc/client/` |
| 服务预检 | 安装布局、SCM服务路径/身份、版本匹配及64位注册表读取 | `internal/daemon/` |
| 协议生成 | daemon/desktop及递归依赖生成与摘要检查 | `scripts/proto/`、`internal/rpc/gen/` |

数据默认 `%APPDATA%\ProxyGlassDev`；`--data-dir <绝对路径>` 可指定隔离目录。SQLite保存元数据/选中项，配置内容独立写JSON文件。

### 指标和控制接口的重要语义

- `Callbacks.WatchMetrics(ctx, emit)` 对接官方 `SubscribeStatus`，请求1秒间隔。首帧没有有效速率；第二帧起、流量可用且数值有效才显示速率和追加曲线。不把首帧0当成观测0。
- 图表保留最多60个真实样本；不足2个样本只显示网格。线条与浅填充随浅深色改变，不把fixture注入生产入口。
- UI只在已观测到 `Running` 后开启指标订阅，停止/刷新/退出取消旧流；过期事件不进入新图。错误清空速率和历史，用户可通过“检测服务”重建订阅。
- HTTP代理和模式只有**用户明确点击**才写入。RPC边界再次检查调用者所有权、运行状态、代理能力或实际模式列表。不认领服务，不顺便启动核心。
- 启动核心前检查选中配置、官方校验、所有权和空闲状态；停止只接受本用户所有的核心。运行中改配置/切配置不会自动重载，需停止后重新启动。
- 分组页的分组筛选、名称/协议搜索、数量和1至4列网格已实现；它们只影响显示，**节点选择与主动测速尚未接入**。

## 4. 验证事实与证据

最后一轮代码变更后，下列检查已通过：

- `go test ./...`、`go vet ./...`。
- 三个锁定子模块工作树检查保持干净；上游没有项目补丁。
- 普通GUI及e2e GUI重新构建、按同一开发证书签名并通过校验。
- 新增模拟gRPC测试：首帧/后续帧速率语义、订阅取消、所有权拒绝、未知模式拒绝、代理能力拒绝、实际提交参数。
- 新增UI测试：运行卡条件显隐、显式代理点击、60样本窗口、停止取消和清空历史。

最新**真实窗口**流程证据：`build/ui-run-e535096a/result.json`，`passed=true`，9项：

1. 新建，经官方CheckConfig校验，SQLite和JSON保存。
2. 非法出站类型被官方拒绝，不新增数据行。
3. 合法JSON通过UI导入并保存。
4. 编辑/改名保持配置ID和内容路径。
5. 取消删除保留数据，确认删除移除记录和内容文件。
6. 无服务/未启动核心时分组显示真实空态。
7. 设置页三个网络相关开关仍禁用。
8. 正常关闭重开后，内容/名称/选中项恢复。
9. 设置菜单切换真实窗口浅深色。

同目录 `overview-light.png`、`overview-dark.png` 是**未连接真实窗口**，因此只显示配置卡，符合官方显示条件。

`build/ui-reference/{zh-Hans,en}-{light,dark}-{dashboard,groups,profiles,settings,menu}.png` 是无窗口布局测试产物。**有曲线、有节点的图使用显式测试数据，不能宣传成真实流量/真实节点验证。**

最新几次失败的 `build/ui-run-87264f7c`、`8c50bf97`、`f5a0ab76` 是自动化脚本未适配新菜单可访问性角色导致，已修正；不要将它们当成最新通过证据，也不要覆盖失败记录。

### 验证范围的硬边界

- 本轮没有在宿主机开启系统代理、TUN或修改路由/DNS；没有真实流量测试。
- 新增持续指标和代理/模式写接口本轮只通过模拟接口验证，**没有最新真实已连接服务的指标/网络切换证据**。
- 旧的安装/直连生命周期证据为 `build/installed-diagnostic.json`、`build/installed-core-smoke.json`、`build/dev-evidence/`。那是早期独立验证，不替代新增面板接口验证。
- 旧验证仅覆盖本机Windows 11 x64。ARM64后端构建通过，不等于ARM64真实运行；Win10及完整M0、多平台验收未完成。

## 5. 当前未解决项与建议顺序

### 第一优先：中文字形与视觉校准

小字号中文仍偏宋体，用户不满意。已确认本机安装 Microsoft YaHei UI；Windows System.Drawing 对照能显示正常无衬线中文，MyGo截图仍不一致。实验产物：

- `build/font-system-check.png`：系统绘制对照。
- `build/font-probe.png`：MyGo字体/语言/字号对照。

已尝试字体名称、单一字体、语言标签、从本机 `C:\Windows\Fonts\msyh.ttc` 注册别名，尚无整页有效修复。**不要重复声称“Lang(zh-CN)已修复”**：此前对照同时改变了字号，结论被整页复测推翻。临时探针源码已删除。

目前中文字体基线为 Microsoft YaHei UI、英文 Segoe UI，并设置内容语言。下一步应锁定相同字号/字重做可比较的渲染实验，测清实际选中字体及小字号渲染路径，判断是本项目使用问题还是MyGo问题。仍不得直接修改锁定上游；若确为上游问题，提供最小复现并选择兼容适配方案。不要把Windows字体文件打进发布包。

### 第二优先：继续对齐官方面板

- 补齐卡片显隐/排序及保存；现在采用用户要求的固定主布局。
- 真实运行数据、失联/重连、核心状态切换的UI验证；需遵守下节网络限制，不自动恢复测试盘安装或启动服务。
- 配置操作补齐完整文件导入/导出、远程订阅、更新时间、分享菜单/二维码。当前只有本地JSON和复制JSON，不能宣称官方配置功能全齐。
- 分组的节点选择/测速、连接列表、日志订阅、工具页实际功能、设置页完整分区仍未完成。
- 窄屏导航切换、无障碍细节、锁定版本的全部视觉差异仍待验收。

### 后续发布工作

发布安装器、升级/失败恢复、正式签名和发布工作流尚未完成。开发安装脚本不是最终安装包。

## 6. 网络与服务操作限制

用户明确担心测试会导致当前聊天断网。常规UI开发/验证应保持现有系统代理、DNS、默认路由和TUN不变；不要自动进行网络专项测试。不要只因为“代码已接入按钮”就自动点击网络控制。

早期本机C盘根目录允许Users FullControl，D盘根目录允许Authenticated Users Modify，官方安全检查会拒绝这些卷上的安装祖先。**换C/D子目录不能解决；不要改系统ACL、不要开unsafe、不要绕过官方检查。**

此前用专用2GiB动态VHDX完成受保护安装及卸载。最后记录状态为服务已卸载、测试盘已解除挂载；本次没有重新核对当前系统服务/挂载状态，接手时需要时只读确认。保留文件：

- `build/m0-test.vhdx`、`build/m0-test-volume.json`
- 测试卷标识：`CE9E52BF-9208-4238-ADAA-B6D1F3775467`
- 历史盘符P，安装路径 `P:\ProxyGlassDev`，数据路径 `P:\ProxyGlassDevData`
- 清理日志 `build/m0-test-cleanup.log`

操作说明：`scripts/dev/USAGE.md`、`scripts/dev/MANUAL-TEST.md`。这些是历史载体和后续选项，不是本handoff要求执行的步骤。

## 7. 构建、签名与窗口测试

工作区是Windows/PowerShell。先加载项目环境，避免误用全局缓存或Go版本：

```powershell
. ./scripts/env.ps1
$env:MYGO_SQLITE_LIBRARY="$PWD\build\app\mygo-sqlite3.dll"
go test ./...
go vet ./...
./scripts/check-upstream.ps1
./scripts/build.ps1
```

当前普通程序：`build/app/sing-box.exe`。需要同目录的 `mygo-sqlite3.dll`、`resources/daemon/`，不要只移动EXE。每次重建都会清除EXE签名，最终产物必须重新签。

本机开发证书位于 `build/cert-m0-service/`。PFX密码用DPAPI密文保存，只可由原Windows用户/原机器恢复；不要打印或提交私钥、密码/密文。

```powershell
$signingPassword = Get-Content build/cert-m0-service/password.dpapi -Raw | ConvertTo-SecureString
./scripts/sign/sign.ps1 `
  -Pfx build/cert-m0-service/codesign.pfx `
  -Certificate build/cert-m0-service/codesign.cer `
  -Password $signingPassword `
  -Files @('build/app/sing-box.exe','build/e2e/sing-box.exe')
```

上例的e2e文件必须已构建；只重签普通GUI时只列普通EXE。证书SHA256：`0B334FDF24E4CB3A1B77C6C8E0DE662691E8978F59F5D082D54E5561AAD21F03`。GUI和daemon必须同签名者；签名脚本不添加受信任根、不重签驱动/DLL。

真实窗口测试专用构建：

```powershell
. ./scripts/env.ps1
go build -tags e2e -trimpath -o build/e2e/sing-box.exe .
go build -o build/ui-driver.exe ./scripts/ui-driver
# 确认 build/e2e/ 已暂存 SQLite DLL、resources/daemon，再按上面命令签名。
python scripts/ui-e2e.py
```

`automation_e2e.go` 只在e2e构建引入本地自动化插件，普通构建不含入口。测试使用独立 `build/ui-run-*/data`，不碰默认用户配置。它禁止启动/停止/安装等网络和服务操作，仅操作自己启动的窗口，退出使用对应PID的WM_CLOSE。

自动化易踩点：

- 工具栏“其他”和概览“选择配置”的角色是 `menuButton`，不是普通 `button`。
- “管理配置”是带勾选状态的 `menuItemCheckBox`，不是 `menuItem`。
- capture之前先等UI空闲并核对当前页节点；之前曾将上一页截图误命名成Overview。不能只看文件名。
- Python环境的AF_UNIX支持不足，使用 `scripts/ui-driver/main.go` 桥接MyGo本地测试socket。

## 8. 锁定版本与仓库状态

以 `upstream.lock` 为准：

| 组件 | 当前锁定 |
|---|---|
| sing-box核心 | v1.14.4，`c005f35300ce76427dc285481ec5695f5b179cd2` |
| Apple客户端 | `2dd154e7586188d5fff8de615e8c821ed25ec089` |
| MyGo | `1f039303abe1bf8b8f6a94ae15c4d6c5ff5d739f`，Go1.27.3 |
| 桌面行为参考 | `8aab8eab022366e1032e952d499c9252b833e6c3` |

`go.mod` 模块为 `singboxwin.local/client`，MyGo replace指向本地子模块。正式公开仓库地址确定后再统一调整模块名。

交接时没有创建提交：`.gitmodules`、三个子模块记录已暂存，大多数项目源码/文档仍是未跟踪文件。**不要reset/clean，不要把未跟踪源码当临时垃圾删除。** `build/`、`.cache/`、私钥和可执行文件在忽略规则内；换机器后本地截图、证据和开发证书不会自动随源码出现。

完整设计文档入口是 `singbox_gui_docs/README.md`。视觉实现以第23章，尤其23.5、23.6为准；早期章节中冲突的四卡/固定代理布局已被最新用户要求替代。

## 9. 接手后的最小起步

先读本文件、根README、第23章和相关代码，查看最新真实窗口与fixture截图的区别。确认当前工作树后，从“相同字号下的中文字体最小复现”或用户新指定的功能继续。不要重做已完成的服务安装/签名方案，也不要为了看到曲线而自动启用网络。
