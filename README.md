# sing-box for Windows

**sing-box for Windows** 是 Windows 上的 sing-box 图形客户端，按官方 macOS 客户端的功能和布局实现。

后端是**未经修改的官方 sing-box 核心**（官方 Windows 服务 `sing-box-daemon`），所以代理功能、配置格式、协议、路由、DNS、TUN 都和官方一致。同一份配置在 Mac 和这里效果相同。

本项目代码采用 [GPL-3.0](LICENSE)，第三方来源与许可见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 安装

1. 在 [Releases](https://github.com/lesnolie/sing-box-for-windows/releases/latest) 下载 `sing-box-for-windows-<版本>-windows-amd64-setup.exe`。
2. 双击运行。
   - 如果出现"Windows 已保护你的电脑"，点"更多信息"→"仍要运行"。原因是安装包用的是本项目自己的签名证书，不是商业证书。
   - 管理员权限提示点"是"。
3. 安装目录、数据目录用默认值即可，然后从开始菜单打开 **sing-box for Windows**。

**为什么必须安装，没有免安装版：** TUN、改路由需要管理员权限。所以和官方客户端一样，核心以 Windows 服务的形式在后台运行，界面程序通过它控制核心。服务必须注册到系统，官方核心还会检查安装位置和签名，单个 exe 直接运行的方式无法通过检查。

**为什么只有 exe，没有 MSI：** 官方核心的自动更新只接受 NSIS 打包的 exe 安装包，MSI 无法自动更新。官方也只提供 exe。

### 安装后的文件

| 位置 | 内容 |
|---|---|
| `C:\Program Files\sing-box for Windows` | 程序，约 116 MB（见下文） |
| `C:\ProgramData\sing-box` | 核心的数据和配置；只有管理员和系统能修改，防止其他程序篡改 |

卸载：在"设置 → 应用"中卸载，或运行安装目录下的 `uninstall.exe`。卸载会删除服务和程序文件，**数据目录默认保留**。

## 使用

1. 在仪表盘"配置"卡片点 **+**，导入本地 JSON 文件，或填写远程订阅链接。
2. 选中配置，点左上角 ▶ 启动。

配置必须是 **sing-box 自己的 JSON 格式**。不支持 Clash YAML，也不支持 `vmess://`、`ss://` 等分享链接；官方客户端同样不支持，需要先转换。

### 会不会改动我的网络

程序本身没有默认模式，**网络行为完全由配置决定**：

- **配置有 `tun` 入站（带 `auto_route`）**：启动后会创建虚拟网卡，添加更高优先级的路由来接管流量，DNS 也按配置走 sing-box。原有网关设置不会被改写。停止后虚拟网卡和路由会自动撤除。
- **只有 `mixed` / `http` 入站**：系统网络完全不动，只开放一个本地代理端口。只有打开仪表盘上的"系统 HTTP 代理"开关，才会修改系统代理设置。
- **本机同时作为其他设备的网关**（网络共享、旁路由等）时，TUN 可能连转发流量一起接管，具体取决于路由规则。建议先用只有 `mixed` 入站的配置试运行。

### 工具

- **网络质量**：测试当前出站的延迟和上下行速度。
- **STUN**：向公网 STUN 服务器发送 UDP 包，检测当前出站的 NAT 类型和 UDP 连通性，主要关系到游戏联机、语音视频通话、P2P。默认服务器是官方核心的默认值 `stun.voipgate.com:3478`，可以修改。

## 自动更新

- GitHub Actions 每天检查一次官方 sing-box。官方发布**新的正式版**后，会自动编译、签名、打包并发布到本仓库的 Releases。
- **只跟正式版**，不跟 alpha / beta / rc 等预发布版本。
- 程序版本号就是核心版本号：v1.14.4 内含官方核心 1.14.4。
- 程序每天检查一次本仓库的新版本（可在"设置 → 应用 → 更新"中关闭）。有新版本时，侧栏底部会出现提示，点击后在"更新"栏选择"下载并安装"。
- 安装过程由官方核心完成。它会核对安装包的签名必须与当前安装一致，版本必须比当前新。核心会短暂重启，安装完成后程序自动重新打开。
- 发布前会检查新核心是否修改了界面与核心之间的通信协议。如果修改了，就**不发布**，而是自动在仓库开一个"自动发布失败"的 Issue。界面需要相应适配后才能发布。
- 界面的改动不会单独发布：官方核心只接受版本号更高的安装包，所以界面改动会随官方下一个正式版一起发布。

## 磁盘和内存

**磁盘约 116 MB**，大部分是官方核心：

| 文件 | 大小 | 说明 |
|---|---|---|
| `resources\daemon\sing-box-daemon.exe` | 75 MB | 官方核心，Windows 版带全部功能（Tailscale、各类协议、USB/IP 等），与官方客户端相同 |
| `sing-box.exe` | 30 MB | 界面程序 |
| `resources\daemon\libcronet.dll` | 9 MB | 官方核心的网络组件 |
| 其他 | 2 MB | 数据库组件、驱动 |

**内存：** 任务管理器里看到的是几个进程的合计，包括后台核心服务、核心工作进程、界面程序。界面在窗口打开时绘制图表和统计，本身就占几十 MB。

手机上看到的十几 MB 通常只是核心（iOS 的网络扩展），App 界面关掉后不占内存，统计口径不同。同一份配置下，核心本身的内存与手机接近，主要取决于规则集和节点数量。

想只保留核心运行：在设置中关闭"关闭后保留托盘"，关闭窗口后界面程序会完全退出，核心继续在后台工作。

## 与官方 macOS 客户端的差异

核心功能相同，使用同一个官方核心。界面上目前缺少以下部分：

- **Tailscale 管理**：状态、登录链接、设备列表、出口节点、Ping、SSH。核心照常运行 Tailscale 端点，但目前需要在配置里填写 `auth_key`，无法通过网页登录。欢迎贡献。
- 其他端点（OpenConnect、OpenVPN）的管理界面，以及 USB/IP。
- 远程控制。
- 调试页（macOS 版用于读取苹果系统的崩溃报告，Windows 上没有对应内容）。

## 开发

接手开发请先读 [HANDOFF.md](HANDOFF.md)，完整设计文档见 [singbox_gui_docs](singbox_gui_docs/README.md)。

```powershell
git submodule update --init third_party/sing-box third_party/mygo
. ./scripts/env.ps1
./scripts/proto/install-tools.ps1          # 需要 protoc 3.20.3
./scripts/proto/generate.ps1 -Mode check   # 核对与核心的通信协议
./scripts/package.ps1 -Pfx <证书.pfx> -Certificate <证书.cer> -Password <SecureString>
go test ./...
```

- `package.ps1` 会编译官方核心和界面，签名后用 NSIS 打出安装包，输出到 `build/release/`。
- 发布流程见 [.github/workflows/release.yml](.github/workflows/release.yml)。签名证书以加密密钥（Secrets）的形式保存在仓库中，不进入代码。
- `third_party/sing-box-for-apple` 只是界面参考，编译不需要。
- 上游子模块（`third_party/`）保持原样，不打补丁。
