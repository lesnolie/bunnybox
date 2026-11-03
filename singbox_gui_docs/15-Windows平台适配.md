# 15 Windows 平台适配

本章规定 macOS 平台能力在 Windows 上的映射、替代与省略。凡未在本章列出的功能，按前述各章原样实现。

## 15.1 映射总表

| macOS 能力 | Windows 实现 | 节 |
|---|---|---|
| Network Extension / 系统扩展 | 官方守护进程服务 `sing-box-daemon`（SYSTEM，内嵌核心）+ wintun TUN；GUI 经 worker 与之通信 | 2.3、15.2 |
| 系统扩展安装/更新/卸载 | 安装器注册服务；GUI 经 UAC 执行 `sing-box-daemon.exe service install/start`；卸载经卸载程序 | 15.2 |
| VPN 配置“连接状态”（NEVPNStatus） | `SubscribeServiceStatus` | 5 |
| NEVPNManager 按需启动（On Demand） | GUI 内置评估器 + 守护进程启动/停止 | 15.4 |
| NEPacketTunnel 隧道设置（includeAllNetworks 等） | 改写 tun 入站 JSON | 11.4、15.3 |
| 系统代理（NE proxySettings） | 守护进程以所有者用户身份设置 WinINet 代理 | 11.7、15.3 |
| iCloud Drive 配置 | 不支持；导入兼容为本地 | 15.5 |
| 菜单栏额外项（MenuBarExtra） | 通知区域托盘图标 + 自绘弹出菜单 | 12 |
| 登录项（SMAppService） | MyGo `SetOpenAtLogin`（HKCU Run） | 12.4 |
| Sparkle 式更新 / GitHub 发布 | 自有 GitHub 发布检查 + Ed25519 签名的 SHA256SUMS 校验 + 守护进程 `InstallUpdate`（回退 UAC 运行安装包） | 12.5、21、22 |
| 共享表单（ShareLink） | Windows 共享（DataTransferManager）+ 资源管理器回退 | 6、A-12 |
| 二维码扫描（相机） | Media Foundation 摄像头 + 解码 | 6、A-13 |
| 辅助服务（Root Helper） | 省略 | 15.6 |
| 完全磁盘访问权限 / 应用位置检查 | 省略（由安装器与守护进程的安装路径检查替代） | 15.6 |
| WiFi 状态访问（位置权限） | 守护进程经 WLAN API 获取；需 Windows 位置权限 | 15.7 |
| App Store 评分 | 省略 | 15.6 |
| 本地化 RTL（波斯语） | MyGo 布局方向 RTL | 15.8 |

## 15.2 服务安装、修复与卸载

与 sing-box-for-desktop 相同：服务由安装器注册；GUI 只提供“安装/启动/修复”（经 UAC）与“卸载”（打开卸载程序）入口。守护进程可执行文件始终是 `<安装目录>\resources\daemon\sing-box-daemon.exe`。

### 15.2.1 检测（GUI 启动、主窗口激活、每次操作失败后）
1. 读取安装布局（2.6）。无效 → E-DAEMON-09，停止检测。
2. 签名自检（14.3）：GUI 与守护进程文件签名者不同 → E-DAEMON-05；GUI 证书不在有效期内 → E-DAEMON-06。
3. 以普通权限运行 `sing-box-daemon.exe service status`（无窗口，超时 10 秒）：退出码 3 → “服务未安装”（E-DAEMON-01）；退出码 2 → “服务未运行”（E-DAEMON-07）；退出码 0 → 继续。普通用户只能查询服务状态，不能启动服务（服务 DACL 只给已验证用户查询权限）。
4. 拉起 worker（2.3.1）并调用 DesktopService.`GetDaemonInfo`：失败 → 按 14.2 映射；`version` 与随附守护进程的 `version` 输出不同 → “服务需要修复”（E-DAEMON-04）；成功 → 正常，同时读取所有权（2.4.1）。

### 15.2.2 安装器
安装器为 NSIS，按机器安装（需要管理员），界面与行为与 sing-box-for-desktop 的安装器一致：

| 步骤 / 页面 | 行为 |
|---|---|
| 启动检查 | Windows 10 1809 及以上；安装包架构与系统原生架构不同时提示“此安装程序包含 <架构> 版本，但 Windows 的原生架构为 <架构>。为了获得最佳兼容性和性能，请改用原生安装程序。仍要继续吗？”；检测到另一构建方的 sing-box 安装时按 15.10 处理 |
| 已有安装 | 页面“已有安装 / 选择安装方式。”：“更新”（保留目录设置）/“重新安装” |
| 安装方式 | “安装”（默认目录）/“自定义安装” |
| 安装位置（仅自定义） | 应用安装目录（默认 `%ProgramFiles%\sing-box`）；应用数据目录（复选“使用标准用户数据目录”，即每用户 `%APPDATA%\sing-box`；取消勾选时为所有用户共用的固定目录）；守护进程数据目录（默认 `C:\ProgramData\sing-box-daemon`）；“数据目录改变时迁移现有数据”复选框；各项带“浏览...” |
| 路径校验 | 目录必须在固定的 NTFS 本地磁盘上；三个目录互不重叠；安装目录本身不能是重解析点；上级目录不得包含重解析点 |
| 不安全上级目录 | 上级目录可被非管理员替换时显示“安装路径不安全”页：“修复权限”（只移除危险权限）/“不安全安装”。选择后者进入确认页，须勾选“我理解并接受此安全风险。”才能“继续不安全安装”，此时守护进程以 `--allow-unsafe-installation-directory-permissions` 注册（只跳过路径与权限检查，签名者比较仍执行） |
| 守护进程数据目录无效 | 提示“sing-box 守护进程数据目录无效……继续安装将删除此目录，以便守护进程用安全权限重新创建。” |
| 安装 | 关闭运行中的 `sing-box.exe` → 停止服务 → 复制文件 → 执行 `sing-box-daemon.exe service install --working-directory <守护进程数据目录>`（守护进程自行检查安装目录、设置 ACL、注册并启动服务）→ 写入注册表布局键（`LayoutVersion`=2、`InstallationID`、`ApplicationDataDirectory`（标准目录时删除该值）、`DaemonDataDirectory`）→ 注册 Taildrop 右键菜单“使用 Taildrop 发送”（`"sing-box.exe" --taildrop-send="%1"`）→ 完成数据迁移 |
| 注册失败 | 显示“无法注册 sing-box 守护进程（代码 n）。”及命令输出，自动卸载已复制的文件并回滚数据迁移 |
| 完成页 | “创建桌面快捷方式”复选框；“运行 sing-box”以**登录用户身份**启动（非提权，2.3.2） |
| `--updated` | 跳过所有页面，沿用已有布局；配合 `--force-run` 时安装后以登录用户身份启动 `sing-box.exe --updated` 并退出（12.5） |

安装器 UI 语言：English、فارسی、Русский、简体中文、繁體中文，与应用一致。

### 15.2.3 安装 / 启动 / 修复（GUI 发起）
均以 `ShellExecuteExW`（verb `runas`，`SEE_MASK_NOCLOSEPROCESS`，`SW_HIDE`）直接提升启动 `<安装目录>\resources\daemon\sing-box-daemon.exe`，在后台 goroutine 中等待退出（超时 120 秒），UAC 对话框显示的程序为 `sing-box-daemon.exe`：

| 操作 | 命令 | 入口 |
|---|---|---|
| 安装 / 修复 | `service install --working-directory <注册表中的守护进程数据目录>`（守护进程注册服务后自行启动并等待其运行） | 仪表盘“服务未安装”“服务需要修复”；设置“服务”分节“修复”（对应基线“更新”） |
| 启动 | `service start` | 仪表盘“服务未运行”；设置“服务”分节 |

- 退出码 0：成功，重新检测（15.2.1）并刷新仪表盘与设置。
- 用户取消 UAC（1223）：静默返回，不弹错误。无法启动提升进程（1224）：“错误 / 无法以管理员身份运行守护进程命令。”
- 其他：错误对话框，动作名 “install service” / “start service”，描述 `service install failed with exit code <n>` 及守护进程输出的末行；守护进程拒绝安装目录（不安全路径）时附加“请使用安装程序修复安装位置。”
- 修复不改变守护进程数据目录与其中数据。

### 15.2.4 卸载
- GUI 设置中的“卸载”打开系统“应用和功能”中的本应用卸载程序（与 sing-box-for-desktop 一致，不在 GUI 内单独移除服务）。
- 卸载程序：关闭 `sing-box.exe` → `sing-box-daemon.exe service uninstall`（停止核心、恢复系统代理、删除服务）→ 删除程序文件、Taildrop 菜单与快捷方式 → 卸载选项页“保留数据”（默认勾选）；取消勾选时删除守护进程数据目录、应用数据目录与注册表布局键。
- 移除服务失败时显示“无法移除 sing-box 守护进程（代码 n）。”及命令输出，并中止卸载。

## 15.3 TUN 与系统代理
- TUN 由核心在守护进程中通过 wintun 创建；GUI 不直接操作适配器、路由或防火墙。
- 隧道选项（11.4）与配置重载（11.6）在 `StartService` 前作用于配置 JSON；运行中修改隧道选项时停止后重新启动服务，修改配置重载项时调用 `ReloadService`（2.5）。
- `strict_route` 在 Windows 上通过 WFP 阻止非隧道 DNS 泄漏，与基线 includeAllNetworks/enforceRoutes 的语义对应（11.4）。
- 系统代理：配置中 tun 入站 `platform.http_proxy.enabled` 为真时，守护进程的平台接口以服务所有者的会话令牌（`WTSQueryUserToken`，模拟该用户）写入 WinINet 代理设置并广播 `INTERNET_OPTION_SETTINGS_CHANGED`；停止服务、所有者变更、服务卸载时恢复原值。`system_proxy_enabled` 为假时 GUI 在服务启动后调用 `SetSystemProxyEnabled(false)`。
- 进程路由规则（`process_name`、`process_path`）由守护进程直接支持（SYSTEM 可查询所有进程），无需额外组件。
- 会话变化（`SERVICE_CONTROL_SESSIONCHANGE`）：所有者注销时系统代理随会话失效，核心继续运行；重新登录后重新应用系统代理。

## 15.4 按需启动评估器
### 15.4.1 激活条件
- `always_on` 为真：使用默认规则集 [连接，接口类型任意，探测 URL `http://www.msftconnecttest.com/connecttest.txt`]（基线默认规则探测苹果的强制门户地址；Windows 使用系统的 NCSI 地址，A-22）。
- 否则 `on_demand_enabled` 为真：使用 `on_demand_rules`；为空时使用默认规则集。
- 用户手动断开（仪表盘/托盘开关）：评估器暂停，直到用户下次手动连接（对应基线“wasOnDemandEnabled”行为）。
- 评估器在 GUI 进程中运行；GUI 退出后不再评估（服务保持当前状态）。开机自启时（12.4）评估器随 GUI 启动。

### 15.4.2 网络状态采集
| 条件 | 数据来源 |
|---|---|
| 网络变化事件 | `NotifyNetworkConnectivityHintChange` + `NotifyIpInterfaceChange`，合并 2 秒抖动 |
| 接口类型 | `GetBestRoute2` 到 0.0.0.0 / :: 的出口接口（排除 wintun 及本服务 TUN），`IfType`：6 以太网，71 Wi-Fi |
| SSID | 出口接口为 Wi-Fi 时 `WlanQueryInterface(wlan_intf_opcode_current_connection)`（需位置权限，15.7） |
| DNS 搜索域 | `GetAdaptersAddresses` 的 `DnsSuffix` 与 `FirstDnsSuffix` 列表 |
| DNS 服务器 | 同上 `FirstDnsServerAddress` 列表 |
| 探测 URL | 对 URL 发 GET（5 秒超时，不跟随重定向，绕过系统代理，经物理接口），HTTP 200 视为匹配 |

### 15.4.3 规则匹配
- 自上而下，跳过不支持的“评估连接”规则，第一条所有条件都满足的受支持规则生效；条件为空视为满足。
- 接口类型：任意 / 以太网 / Wi-Fi；SSID：当前 SSID 在列表中；搜索域：网络默认搜索域在列表中（不区分大小写）；DNS 服务器：网络的每个 DNS 服务器都在列表中；探测 URL：返回 200。
- 无网络（无默认路由）时不评估，保持当前状态。
- 动作：
  - 连接：服务未运行时启动当前选中配置（与手动启动相同流程，含所有权检查；所有权为 Other 时不接管，记录日志）。
  - 断开：服务运行时停止。
  - 忽略：保持当前状态。
  - 评估连接：当前 Windows 实现不支持。评估器跳过该规则并继续检查下一条受支持规则；只有不支持规则且无匹配规则时保持当前状态。界面必须标记不支持，不得转换为全局连接（11.5、A-23）。
- 无规则匹配：保持当前状态。
- 失败：自动启动失败不弹对话框，发系统通知“自动连接失败”+ 原因；同一网络状态下不重复尝试。

## 15.5 iCloud 配置
- Windows 不提供 iCloud 配置类型。新建配置的“类型”只有“本地 / 远程”。
- 导入 `.bpf` 中 type 为 ICLOUD 的配置时转换为本地配置（内容随文件导入），不提示。
- 数据库中 type=1 的历史记录按本地处理，图标使用 `cloud_sync_20_filled` 仅作标识。

## 15.6 省略的功能
| 基线功能 | 原因 |
|---|---|
| 辅助服务（Root Helper）及“需要辅助服务”提示 | 守护进程以 SYSTEM 运行，已具备所需权限 |
| 完全磁盘访问权限检查 | Windows 无此权限模型 |
| 应用位置检查 / 移动到应用程序 | 由安装器与守护进程的安装路径检查保证（15.2.2） |
| App Store 评分 | 非商店分发 |
| excludeAPNs / excludeCellularServices / excludeDeviceCommunication | 无对应系统概念 |
| 小组件、控制中心控件、快捷指令 | macOS/iOS 专有；托盘与深度链接覆盖相同操作（可选：Windows 跳转列表，A-24） |
| tvOS / iOS 专属页面 | 非本产品范围 |

## 15.7 WiFi 状态与位置权限
- `wifi_ssid` / `wifi_bssid` 规则由守护进程读取 WLAN 信息。Windows 11 24H2 起读取 SSID 需要“位置”权限。
- 当配置包含这两类规则且读取失败时，仪表盘显示提示“需要位置权限才能读取 WiFi 状态”，按钮“打开设置”→ `ms-settings:privacy-location`（替代基线“WiFi 状态访问”提示，正文使用附录 B 对应条目的 Windows 改写）。
- 按需启动评估器读取 SSID 同样受此限制；无法读取时 SSID 条件视为不匹配。

## 15.8 本地化与 RTL
- 支持语言：English、فارسی、Русский、简体中文、繁體中文；默认跟随系统 `GetUserPreferredUILanguages`。
- 波斯语时整个窗口与托盘弹出菜单使用 RTL 布局；图标中带方向的（chevron、arrow）水平镜像；数字、URL、日志与 JSON 编辑器保持 LTR。
- 语言变更后调用 DesktopService `SetLocale` 使核心错误消息同步本地化。

## 15.9 高 DPI 与主题
- 每显示器 DPI 感知 v2；所有尺寸以 DIP 表示（第 3 章）。
- 跟随系统浅色/深色（`AppsUseLightTheme`），运行时切换即时生效；托盘图标按任务栏主题（`SystemUsesLightTheme`）选择黑/白版本。
- 高对比度模式下使用系统颜色。

## 15.10 与官方 Windows 客户端共存
本应用随附未经修改的官方守护进程，因此与 sing-box 官方 Windows 客户端（sing-box-for-desktop）使用相同的系统资源，**两者不能同时安装**：

| 资源 | 本应用 | 官方客户端 |
|---|---|---|
| Windows 服务 | `sing-box-daemon` | 相同 |
| 控制管道 | `ProtectedPrefix\Administrators\sing-box` | 相同 |
| 守护进程数据目录 | `C:\ProgramData\sing-box-daemon`（默认） | 相同 |
| 安装布局注册表键 | `HKLM\SOFTWARE\SagerNet\sing-box` | 相同 |
| GUI 可执行文件名 | `sing-box.exe` | 相同 |
| 签名证书 | 本项目自签名证书 | 官方证书 |

- 两者的守护进程都只接受与自身同签名者的 GUI（2.9），因此混装会导致 E-DAEMON-05 或 E-DAEMON-08。
- 安装器检测：服务 `sing-box-daemon` 已存在且其可执行文件的签名证书与本安装包不同（或布局键中的安装目录不是本应用），则显示“检测到另一个构建方的 sing-box（例如官方客户端或自行构建的版本）。请先在“应用和功能”中卸载它，然后再安装。”，按钮“打开应用和功能”/“退出”，不允许继续。数据目录可保留：配置与设置格式由 GUI 自行管理，与官方客户端不通用。
- 源码构建版本（21.5）与官方发布版本同理互斥。
- GUI 运行时若检测到服务可执行文件的签名者与自身不同，按 E-DAEMON-05 提示重新安装。
