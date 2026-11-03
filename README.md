# Proxy Glass 开发工程

目标：MyGo 原生前端复刻官方 macOS 客户端；后端使用未修改的官方 sing-box/boxdd。

接手开发先读 [HANDOFF.md](HANDOFF.md)，包含最新界面要求、代码入口、验证证据、未解决项与构建签名步骤。

项目自身代码采用 [GPL-3.0](LICENSE)。FlClash 作为补充 UI 参考，固定版本、逐页采用范围和后续开发边界见 [第23章](singbox_gui_docs/23-界面参考与实现范围.md#234-flclash-补充参考2026-11-02)；来源及第三方许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 当前状态

当前已完成本机 M0 服务接入闭环、本地配置管理，并按用户提供的真实 macOS 截图重做 MyGo 界面；多系统完整闸门仍未通过。2026-11-02 已接入真实 SQLite 与官方配置校验。最新视觉参考和版本边界见 [第23章](singbox_gui_docs/23-界面参考与实现范围.md)，不宣称已完成所有页面的一比一校准。

本轮以 UI 用户流程为主，不重复验证官方核心算法。未启动任何实际网络专项测试，也未修改系统代理、默认路由、DNS 或 TUN。用户显式点击启动/停止可以调用官方核心；不自动启动选中配置。

### 当前可用范围

- 本地 JSON 新建、粘贴导入、编辑、重命名、选择和确认删除；保存前由官方 worker CheckConfig 校验。SQLite 保存元数据、偏好及选中项，JSON 内容独立原子写入文件。
- 开发数据默认 `%APPDATA%\ProxyGlassDev`，与官方客户端数据目录分离；`--data-dir <绝对路径>` 可指定临时目录。GUI 使用单实例锁，退出时关闭数据库与 worker。
- 服务“检测”仅查询；显式启动前检查选中配置、所有权及空闲状态，不接管其他用户、不替换运行中的核心。停止仅允许本用户拥有的核心。没有自动重载：运行中修改或切换配置要先停止再启动才应用。
- 当前导入是粘贴 JSON，尚无文件选择器、BPF、二维码、远程订阅；日志和工具页尚无真实数据订阅。设置页中的系统代理、隧道、自动连接仍禁用；概览中的系统代理使用官方能力条件及显式点击接口。
- 最新概览按用户最新macOS截图排列为上传/下载大卡→状态/连接小卡→能力可用时的HTTP代理/模式→配置卡；侧栏顺序为概览、分组、连接、日志、工具、设置。配置管理移至“其他→管理配置”，概览使用整行配置选择器及编辑/复制JSON操作。默认窗口1000×760。中英文浅深色截图位于 `build/ui-reference/`；有数据的曲线/分组截图均为显式测试fixture。
- 运行时接入持续只读指标订阅及最多60个样本的曲线。首帧无有效速率，显示横线；未连接时按官方条件隐藏运行卡片。停止/刷新/退出取消旧订阅，旧事件不进入新图表。HTTP代理、Clash模式按实际能力显示，用户点击后才调用官方写接口，并再次核对所有权、运行状态和合法模式。本轮仅模拟接口验证，未在宿主机切换网络或执行真实流量测试。
- 参考 FlClash 增加分组筛选、名称/协议本地搜索、可见/总节点数、手动刷新及随正文宽度变化的1至4列网格。生产数据仍来自官方接口；本地筛选不切换节点或触发测速。FlClash 的 Dart 源码未直接移植，也不引入 Flutter 运行时。

本次继续调整侧栏、字号、字重、工具栏和浅深色卡片层次。中文小字号仍存在偏宋体的渲染问题，尚未通过最终视觉验收；诊断与未解决项见第23章23.5节。

### 本轮用户流程验证

- 全项目 `go test ./...` 和 `go vet ./...` 通过；使用真实 MyGo SQLite DLL，配置测试覆盖持久化、并发同名、失败补偿和Windows原子替换。UI无窗口交互测试覆盖新建/选择/编辑/改名/导入/删除，以及后台失败显示。
- 运行中的真实原生窗口验证通过：新建配置→官方配置检查→SQLite与内容文件保存→关闭重开后恢复；非法出站类型被官方拒绝且未落库；代理与隧道开关禁用。
- 最新实际窗口验证证据：`build/ui-run-e535096a/result.json`，9项检查通过，包括配置导入/编辑/改名/删除、重开恢复、分组空态、网络开关禁用、浅深色切换。真实概览截图为同目录 `overview-light.png`、`overview-dark.png`。测试只使用build下的临时数据目录，未写默认用户配置。新增分组筛选与搜索使用显式测试数据完成交互验证；没有将真实网络连通或节点切换列为通过。
- 前后只读网络快照 `build/network-before-ui.json`、`network-after-ui.json` 的系统代理、IPv4默认路由、DNS一致。`internal/networkcheck` 仅有待用的本地loopback代理检查代码，没有运行真实流量检查，不能算网络功能验收通过。

已验证事实：
- GUI 编译及 `go vet ./...` 通过；`go test ./...` 通过，真实命名管道测试需在允许本地管道的环境运行。
- 官方后端 amd64、arm64 构建通过，按各架构锁定模块暂存驱动和 libcronet.dll；amd64 实际运行版本为 1.14.4、core API 4。arm64 尚未在真实 ARM64 系统运行。
- 三个锁定上游工作树保持未修改；协议重复生成无差异。
- 最终开发 GUI 和两架构 daemon 通过同证书签名校验；篡改、无签名、不同证书的负向测试通过。没有安装受信任根证书。
- 已在专用 NTFS 虚拟盘完成受保护安装，普通用户 `--diagnose` 的 worker_ready、application_rpc、daemon_rpc、version_match 均为 true；官方服务以 LocalSystem 运行，版本1.14.4、布局2、服务路径均匹配。证据：`build/installed-diagnostic.json`。此前服务未安装的负向记录保留于 `build/validation-m0-service.json`。
- 新增服务路径、LocalSystem 身份、64位布局和版本匹配检查；缺服务、停止、旧安装路径和无效布局的回归测试通过。界面“检查服务连接”只执行上述只读诊断。
- `--smoke-core` 的9组 fake gRPC 安全测试通过；真实受保护安装中 core_started=true、core_stopped=true，退出码0，初始IDLE。证据：`build/installed-core-smoke.json`。本轮没有配置 TUN、监听入站或系统代理。
- 17项安装预检通过。C/D卷根权限不改，通过新建2GiB动态VHDX并仅加固其根权限解决测试安装位置；官方后端安全检查保持启用。真实 Install、Repair、Uninstall 均成功，证据在 `build/dev-evidence/`，清理日志 `build/m0-test-cleanup.log`。服务已卸载、测试盘已卸载挂载；VHDX、盘内数据、marker与开发布局键保留。本机仅覆盖 Windows 11 x64 10.0.26100，仍不是全量 M0 验收通过。

## 并行开发切片

本轮三个子代理均为 GPT-6.1 Sol / medium。共享工程，按目录分工；父代理负责整合和 README。后续仍按以下边界分工，公共接口先协调，避免并发修改依赖清单。

| 切片 | 文件所有权 | 本轮结果 | 后续接口 |
|---|---|---|---|
| 协议 | scripts/proto、internal/rpc/gen、go.mod/go.sum | 完整协议与递归依赖生成、检查和测试完成 | 生成 daemon/desktop 客户端桩；其他切片将依赖需求交此切片统一处理 |
| 通信 | internal/rpc/worker、internal/rpc/client | worker 生命周期、双管道、语言参数和测试完成 | Application/Daemon 返回 grpc.ClientConn；不导入界面 |
| 签名 | scripts/sign | 开发证书、签名、官方规则校验及负向验证完成 | 签最终产物；不改上游、驱动或系统服务 |
| 整合 | main.go、internal/diagnostics、构建脚本、README | 原生入口、只读探针、双架构构建与运行库暂存完成 | 服务安装工作在下一阶段单独执行，不能并发改本机服务 |

本轮延续并行分工：安装切片写 `scripts/dev`；诊断切片写 `internal/daemon`、`internal/diagnostics`；生命周期切片写 `internal/smoke`。父代理统一入口、证据收集和最终构建签名。

## 获取、生成与构建

```powershell
git submodule update --init --recursive
. ./scripts/env.ps1
go mod download
# protoc 必须为 3.20.3；脚本核验实际版本
./scripts/proto/install-tools.ps1
./scripts/proto/generate.ps1
./scripts/proto/generate.ps1 -Mode check
./scripts/build.ps1
./scripts/daemon/build.ps1 -Arch amd64
./scripts/daemon/build.ps1 -Arch arm64
go test ./...
go vet ./...
```

GUI 输出 `build/app/sing-box.exe`，后端输出 `build/<arch>/resources/daemon/`。每次重新构建会清除对应 EXE 的签名，必须对最终产物统一重新签名。构建脚本不安装服务、不修改系统代理。

Go 工具链及缓存位于 `.cache/`；首次构建需要下载。MyGo 通过锁定子模块引用；模块名暂用 `singboxwin.local/client`，公开仓库确定后再统一调整。协议工具只共享生成的类型，不导入核心运行时代码。

签名用法见 [scripts/sign/README.md](scripts/sign/README.md)。本轮最终开发证书在 `build/cert-m0-service/`，随机密码使用 Windows DPAPI 加密保存在同目录 `password.dpapi`，已验证原用户在原机器恢复后可用于重签；没有明文密码文件。后续重建复用此证书，按签名说明恢复密码；不要把 PFX、密码密文或明文提交仓库。旧 final/final2 证书不再对应当前产物。

## 只读连接诊断

诊断由 GUI 主程序自身启动 worker，避免辅助探针进程破坏官方父进程认证约束。它读取版本、启动 worker、调用 FormatConfig 和 GetDaemonInfo；不调用服务安装、所有权接管或核心启动/停止。

在最终布局中，`sing-box.exe` 同级必须存在 `resources/daemon/sing-box-daemon.exe` 及相应运行库，GUI 与 daemon 必须为同一签名者：

```powershell
./build/app/sing-box.exe --diagnose > build/diagnose.json
```

成功退出码为 0；失败为 1，JSON 保留已完成阶段和错误。worker 应用侧成功不代表守护服务认证或 TUN 可用。开发目录中的成功结果也不能替代受保护安装布局验证。建议使用以下命令保存带本机环境与文件摘要的证据；失败也保留 JSON，但命令报错，不能把报错捕获等同通过：

```powershell
./scripts/validate-m0.ps1 -Executable "$PWD\build\app\sing-box.exe" -Out build/validation-m0-service.json
```

## 实际窗口流程检查

普通构建不包含自动化入口。仅本地测试用 e2e 构建，安装源不可混用测试数据、私钥或自动化socket：

```powershell
. ./scripts/env.ps1
go build -tags e2e -trimpath -o build/e2e/sing-box.exe .
go build -o build/ui-driver.exe ./scripts/ui-driver
```

将锁定的 `mygo-sqlite3.dll` 和 `resources/daemon/` 暂存到 `build/e2e/`，GUI与daemon以同一开发证书签名后，运行 `python scripts/ui-e2e.py`。它通过MyGo的仅本地测试socket操作真实窗口，不开启核心、不操作系统网络；每次使用独立build数据目录，并关闭自己启动的进程。普通构建的界面入口为 `build/app/sing-box.exe`。

## 开发安装与实机冒烟

当前这台电脑的逐步操作见 [手动测试说明](scripts/dev/MANUAL-TEST.md)：使用专用 VHDX 测试盘，管理员安装、普通用户验证、管理员卸载。该链路现已实际跑通并清理，测试盘未挂载，服务已卸载。此前注册表预检误判 CREATOR OWNER 占位项已修复；无需修改系统注册表权限。磁盘根权限问题仍存在，因此仍使用测试盘，而非 C/D 盘子目录。

安装、修复、卸载和只读预检的用法见 [scripts/dev/USAGE.md](scripts/dev/USAGE.md)。脚本仅管理自身 marker 标识的安装，不接管已有官方客户端，默认保留数据；不是发布安装包。先运行 `./scripts/dev/test-preflight.ps1` 及安装 `-Plan`，检查通过后才在管理员 PowerShell 执行真实安装。

本机 C:\ 根目录的 BUILTIN\Users FullControl，以及 D:\ 根目录的 Authenticated Users Modify，包含非管理员删除/替换能力；官方检查包括卷根，因此换到这些盘的其他子目录不能解决。已验证专用VHD卷可通过检查，无需修复C/D权限或启用unsafe参数。虚拟磁盘是临时测试载体，不等于虚拟机隔离；宿主文件依然位于当前电脑。

服务安装通过后，先从**已安装的真实 GUI 路径**运行只读诊断；之后关闭其他客户端，仅在核心空闲时显式执行以下直连生命周期测试：

```powershell
./scripts/validate-m0.ps1 -Executable P:\ProxyGlassDev\sing-box.exe -Out build/installed-diagnostic.json
./scripts/validate-m0.ps1 -Executable P:\ProxyGlassDev\sing-box.exe -SmokeCore -Out build/installed-core-smoke.json
```

`--smoke-core` 会认领空闲服务并持久化最小测试配置；拒绝接管其他用户或替换已运行核心。官方没有原子的“仅空闲时启动/按实例停止”RPC，测试前仍需确保同用户其他客户端不并发操作。未通过实例/所有权复核时宁可报告清理失败，不停止未知核心。它只覆盖直连生命周期，不能代替 TUN、系统代理恢复、升级和全部目标系统验证。

## 后续可并行切片

1. 安装验证：已有本机虚拟盘安装/修复/卸载证据，继续补充失败恢复、升级和标准用户安装等未覆盖场景。
2. 验证切片：直连生命周期已通过，下一步补全 TUN、系统代理恢复、M0 各目标系统测试和证据；服务安装与测试不能由多个代理同时操作同一机器。
3. 视觉切片：已按用户截图及文章重做概览/分组/配置/设置布局，持续指标与曲线接口已接入并通过模拟流测试，继续补充真实已连接数据验证和锁定版本差异校准；下一功能重点为文件导入、日志订阅与配置运行中的行为。

完整规范见 [开发文档](singbox_gui_docs/README.md)。本地配置闭环与三个界面样板已接入；完整业务页面、发布安装包、发布工作流尚未完成。开发服务管理脚本已完成本机真实验证，但不代表发布安装器或多系统验收已完成。遇到官方后端兼容阻塞时报告原因，不自行给上游打补丁。
