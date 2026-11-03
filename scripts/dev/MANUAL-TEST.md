# 当前电脑的手动 M0 测试

使用 PowerShell 7。项目已在 `build/m0-test.vhdx` 创建专用 NTFS 测试盘并卸载挂载；以下 Install 操作只挂载已记录身份的测试盘，不重新格式化它。不修改 C/D 盘权限，不启用 unsafe 参数。

之前的 `HKLM\SOFTWARE` 报错是开发预检误判 CREATOR OWNER 继承占位项，现已修正；17项前置测试通过。2026-11-02 已在这台 Windows 11 x64 上跑通安装、普通用户诊断、直连核心启动/停止、服务修复、卸载与测试盘卸载挂载。当前服务已卸载，测试盘未挂载；可用以下步骤重测。报错时保留完整输出，不要跳过检查。

## 1. 管理员终端：挂载并安装

开始菜单搜索 PowerShell 7，右键“以管理员身份运行”，粘贴：

```powershell
Set-Location C:\work\SingBoxWin_dev_docs
./scripts/dev/test-volume.ps1 -Action Install
```

该命令校验 VHD 身份、盘符、磁盘卷标，执行完整安装预检后，将签名程序安装到 `P:\ProxyGlassDev`，服务数据写入 `P:\ProxyGlassDevData`。成功时会输出 Installed the official test daemon。若 P 盘已被其他磁盘占用，命令会停止。

## 2. 普通终端：测试通信与启动/停止

打开另一个**普通权限** PowerShell 7，关闭其他 sing-box 客户端，依次执行：

```powershell
Set-Location C:\work\SingBoxWin_dev_docs
./scripts/validate-m0.ps1 -Executable P:\ProxyGlassDev\sing-box.exe -Out build/manual-diagnostic.json
```

仅在此命令成功后继续：

```powershell
./scripts/validate-m0.ps1 -Executable P:\ProxyGlassDev\sing-box.exe -SmokeCore -Out build/manual-core-smoke.json
```

首项成功要求 daemon_rpc=true、version_match=true。第二项成功要求 core_started=true、core_stopped=true、error为空。任何错误都保留 JSON 并停止测试；把终端完整报错或 JSON 内容发给开发者。

这一步只测试无入站监听、无 TUN、无系统代理的直连配置；会认领空闲服务并保存最小测试配置，不代表日常代理、驱动、升级或多系统验收已完成。

## 3. 管理员终端：卸载服务、卸载测试盘挂载

测试结束后回到第1步的管理员终端，依次执行：

```powershell
./scripts/dev/manage.ps1 -Action Uninstall -InstallDirectory P:\ProxyGlassDev -DataDirectory P:\ProxyGlassDevData -Certificate "$PWD\build\cert-m0-service\codesign.cer"
```

仅在卸载成功后继续：

```powershell
./scripts/dev/test-volume.ps1 -Action Detach
```

服务存在时脚本拒绝卸载测试盘挂载。若安装失败且没有服务，可直接执行 Detach；若安装只部分完成，请保留日志交给开发者处理，不手动删除文件。

VHDX 文件和证据保留在 build，卸载脚本保留盘内数据、marker 和开发布局注册表项，方便恢复审查；它不是彻底清理。不要删除其他服务、共享注册表父键或手动格式化任何现有磁盘。
