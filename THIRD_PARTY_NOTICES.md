# 第三方来源与许可

本项目自己的代码按 GNU GPL version 3 发布，完整文本见 LICENSE。第三方组件仍按各自许可证发布；本文件不重新授权第三方代码、字体或图像。

## FlClash 界面参考

- 项目：https://github.com/chen08209/FlClash
- 作者：chen08209 与 FlClash contributors。
- 本轮核对提交：`7fd417fb9a9a277c6a9339a8ac87f907df7c25f7`（v0.9.2）。
- 许可证：https://github.com/chen08209/FlClash/blob/7fd417fb9a9a277c6a9339a8ac87f907df7c25f7/LICENSE （GNU GPL version 3）。
- 当前用途：阅读导航、仪表盘密度、分组标签和节点卡片实现，参考交互结构，以 MyGo 独立实现。当前没有复制或翻译 Dart 源码，没有随应用分发 FlClash 的图标、字体、品牌或截图。
- 后续直接复制或翻译源码时，须在对应文件保留原版权及许可声明，注明原路径、提交和本项目修改；将条目更新为实际复用清单。发布二进制时提供与产物相对应的完整源码及构建说明。

## 名称与图标

- BunnyBox 是 sing-box 的非官方 Windows 客户端，不代表 SagerNet 官方发布。sing-box 的名称与品牌归原作者。sing-box 的许可证不允许衍生作品未经同意使用它的名字，所以本项目不以 sing-box 命名，也不使用官方图标。
- 应用图标 "Glossy Bunny in a Blue Box"（`assets/icon-source.png`）由本项目作者提供，随本项目以 GPL-3.0 发布。`assets/icon-256.png`、`icon-64.png`、`tray-32.png`、`installer.ico` 由它去底、缩放而成。

## MyGo 资源生成代码

- `scripts/winres/main.go` 改编自 third_party/mygo `cmd/mygo/winres.go`、`icon.go`（MIT 许可，Copyright (c) the MyGo authors），文件头注明了来源提交和修改。

## Windows 安装包

installer/sing-box.nsi 参照官方 sing-box 桌面客户端的 NSIS 安装流程（停止服务、更换文件、`service install` 注册服务、失败回滚）重新编写，用 NSIS（zlib/libpng 许可）打包；官方实现以 GPL-3.0 发布。安装包内部的产品名（ProductName）保留为 `sing-box`、界面程序文件名保留为 `sing-box.exe`，因为官方核心只认这两个名字：前者决定能否自动更新，后者决定核心是否接受界面的连接。它们不对用户显示。

## 当前运行依赖

锁定版本见 upstream.lock，具体许可与版权保留在各组件原目录：

- MyGo：third_party/mygo。
- 官方 sing-box / boxdd：third_party/sing-box；构建原版后端，不应用项目补丁。
- 官方 Apple 客户端：third_party/sing-box-for-apple，仅作行为及视觉核对参考。
- Go 模块依赖：go.mod、go.sum；发布前随安装包汇总实际分发依赖的许可证。

公开文章和用户提供的截图仅作开发参考，位于忽略提交的 build/references，不是本项目作品或运行资源。Windows 系统字体由本机提供，不复制字体文件。
