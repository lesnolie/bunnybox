# M0 开发签名

在 Windows、工程根目录运行，需要 Go 和 Windows SDK 的 `signtool.exe`：

```powershell
$password = Read-Host 'PFX password' -AsSecureString
./scripts/sign/New-SelfSignedCert.ps1 -Out build/cert -Password $password
./scripts/sign/sign.ps1 -Password $password -Files @(
    'build/app/sing-box.exe',
    'build/amd64/resources/daemon/sing-box-daemon.exe'
)
go run ./scripts/sign/verify.go -cert build/cert/codesign.cer `
    build/app/sing-box.exe build/amd64/resources/daemon/sing-box-daemon.exe
```

也可通过 `CODESIGN_PFX_PASSWORD` 环境变量传入密码；脚本不输出密码，也不把密码放入 `signtool` 命令行。证书生成/导入会临时写入 `CurrentUser/My`，结束时删除该证书及其存储中的私钥；不会安装到受信任根。运行环境必须允许访问当前用户密钥存储。

本机 M0 联调证书位于 `build/cert-m0-service`，密码通过 Windows 数据保护 API（DPAPI）加密保存为同目录 `password.dpapi`。在原 Windows 用户、原机器恢复密码后重签，无需生成新证书：

```powershell
$password = Get-Content build/cert-m0-service/password.dpapi -Raw | ConvertTo-SecureString
./scripts/sign/sign.ps1 -Pfx build/cert-m0-service/codesign.pfx `
    -Certificate build/cert-m0-service/codesign.cer -Password $password `
    -Files @('build/app/sing-box.exe', 'build/app/resources/daemon/sing-box-daemon.exe')
```

DPAPI 密文不是可跨机器的密码备份；`build` 已忽略，不提交 PFX 或该密文文件。

生成脚本只允许在工程 `build` 下输出，已有证书不会被覆盖。保存并复用同一份加密 PFX 和 DER 公钥证书；重新生成的证书即使主题和密钥相同，也不是守护进程认可的同一签名者。重建 EXE 后需要重新签名。签名失败可能留下部分已签名产物，应在重新签名并校验全部成功后使用。

`check-cert.go` 校验开发证书的自签名、密钥及签名算法、密钥用途、非 CA、序列号和至少 18 个月剩余有效期。生成时使用随机 128 位序列号、RSA 3072、SHA-256、仅代码签名用途和约 10 年有效期。

`verify.go` 只支持 Windows，按锁定的官方 `experimental/boxdd/authenticode_windows.go` 策略调用 `WinVerifyTrust`：允许根不受信任/证书链错误，但必须满足官方自签名回退规则，最后逐字节比较预期叶证书 DER。任何文件失败均返回非零退出码。默认预期证书为 `build/cert/codesign.cer`。

此目录实现本地开发闭环；不包含发布机密注入、安装包流水线或发布完整性签名。只签明确指定的 EXE，不重签上游驱动和 DLL，不加时间戳。
