# 受保护开发安装

此脚本用于 M0 开发验证，不是发布安装器。固定使用官方的 `sing-box.exe`、`resources\daemon\sing-box-daemon.exe`、`sing-box-daemon` 服务和 64 位 `HKLM\SOFTWARE\SagerNet\sing-box` 布局键。它们与官方客户端互斥；发现无本脚本 marker 的目录、数据、服务或注册表布局时直接拒绝。

需要 64 位 PowerShell 7.2 以上、项目 Go 工具链，以及与两个可执行文件签名完全一致的 DER 格式 `.cer`。脚本复用 `scripts/sign/verify.go`，拒绝未签名、篡改、过期和不同签名者；有效开发自签证书不要求导入系统信任库。

先准备完整安装源目录，例如 `build\dev-payload`：根目录放已签名 `sing-box.exe`；`resources\daemon` 放官方构建输出的全部文件，其中仅 `sing-box-daemon.exe` 由项目追加相同签名。不要混入 PFX、私钥或签名证书。脚本不重新签名 DLL 或驱动，不修改上游。

```powershell
# 只读系统检查；Go 验签工具可能在项目 .cache 中产生编译缓存。
./scripts/dev/manage.ps1 -Action Install -Source C:\work\SingBoxWin_dev_docs\build\dev-payload -Certificate C:\work\SingBoxWin_dev_docs\build\cert\codesign.cer -Plan
# -WhatIf 与 -Plan 同样执行检查，不创建目录、不写注册表、不调用服务命令。
./scripts/dev/manage.ps1 -Action Install -Source C:\work\SingBoxWin_dev_docs\build\dev-payload -Certificate C:\work\SingBoxWin_dev_docs\build\cert\codesign.cer -WhatIf

# 以下命令由操作者在提升的 PowerShell 中执行。
./scripts/dev/manage.ps1 -Action Install -Source C:\work\SingBoxWin_dev_docs\build\dev-payload -Certificate C:\work\SingBoxWin_dev_docs\build\cert\codesign.cer
./scripts/dev/manage.ps1 -Action Repair -Certificate C:\work\SingBoxWin_dev_docs\build\cert\codesign.cer
./scripts/dev/manage.ps1 -Action Uninstall -Certificate C:\work\SingBoxWin_dev_docs\build\cert\codesign.cer
```

默认程序目录 `C:\Program Files\ProxyGlassDev`，守护进程数据目录 `C:\ProgramData\ProxyGlassDev`。可以通过 `-InstallDirectory` 和 `-DataDirectory` 指定互不重叠的固定 NTFS 本地绝对路径，其父目录必须存在且受保护；拒绝重解析点、非管理员可替换的上级目录及已有无主目录。不会提供跳过安全检查的参数。请关闭已安装 GUI 后执行。

Install 仅创建新安装，或重装本脚本已经卸载并保留 marker 的同一签名者安装；不覆盖现有文件。Repair 不更新二进制，只在文件哈希、签名、布局、marker 和既有数据 ACL 匹配后，调用官方 `service install --working-directory <目录>`。官方代码负责完整安装树及守护进程数据 ACL 加固、服务注册和启动。成功后才写布局版本 2、InstallationID、DaemonDataDirectory，以及用于所有权检查的两个 ProxyGlassDev 专用值。ApplicationDataDirectory 不写入，GUI 继续使用每用户默认目录。

Uninstall 委派官方 `service uninstall`；服务确认消失后，仅逐个删除 marker 清单中哈希仍匹配的程序文件和随之变空的清单目录。**始终保留数据**、未知文件、安装 marker 及布局键。保留 marker 与布局键用于识别后续同一开发安装；它不是彻底清理功能，不会删除官方公共注册表父键。需要切换官方客户端时，应先人工审阅证据，再单独移除这些保留的开发布局资源。

每次真实操作在项目 `build\dev-evidence\<时间-随机标识>` 记录操作前快照、完整文件哈希、官方命令输出与结果，可用 `-EvidenceDirectory` 改变证据根目录。脚本不会自动回滚：官方服务命令可能已部分生效，尤其超时后进程可能仍在运行；失败时保留安装文件、marker 和证据，禁止直接重试覆盖。由操作者检查服务、注册表、目录和证据后决定恢复步骤。脚本不终止 GUI、不删除数据、不注册卸载入口/快捷方式/文件关联，不能替代第 20.7 章真实 Windows 验收。

## 验证与当前环境阻塞

运行 `./scripts/dev/test-preflight.ps1` 可执行语法解析及无系统写入的前置检查测试：覆盖路径穿越、网络盘、卷根、备用数据流、目录重叠、第三方服务、无主布局键和不安全服务参数。

2026-11-02 本机只读检查发现，`C:\` 对 `BUILTIN\Users` 有生效的 FullControl ACE，标记为 NoPropagateInherit，而不是 InheritOnly。官方 `security_windows.go` 的 `validateInstallationAncestors` 会检查卷根本身，这条权限包含 DELETE / WRITE_DAC / WRITE_OWNER / DELETE_CHILD，因此安全安装在调用服务命令前即被阻止。父代理同时确认 D 盘根对 Authenticated Users 的 Modify 包含 DELETE，亦不能安全替代。本脚本的 `-Plan` / `-WhatIf` 应在此明确失败；管理员提升不能解决这种路径权限问题。本轮没有修改整盘 ACL、启用 unsafe 参数或执行真实安装/修复/卸载；真实系统变更与回滚尚未验证。
