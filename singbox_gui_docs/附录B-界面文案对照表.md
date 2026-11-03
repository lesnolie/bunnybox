# 附录B 界面文案对照表

本表列出界面全部文案的键（英文原文）及简体中文、繁体中文译文，共 495 条。实现时不得手工改写译文，必须由构建脚本 `tools/xcstrings2arb` 从锁定版本的 `Localizable.xcstrings` 生成 MyGo i18n 目录（`locales/en.json`、`locales/zh-Hans.json`、`locales/zh-Hant.json`、`locales/ru.json`、`locales/fa.json`），本表仅供查阅与验收比对。

- 占位符 `%@`、`%lld`、`%1$@` 等在转换时改写为 ARB 命名占位符（`{arg1}`、`{arg2}` …），顺序保持一致。
- 文案中 `**粗体**` 与 `` `代码` `` 为 Markdown 内联样式，界面按富文本渲染（MyGo `ui/markdown` 或富文本 Text）。
- 繁体、俄语、波斯语译文同样随构建生成；波斯语需启用 RTL 布局（见第 3 章）。
- 某键无中文译文时界面显示英文原文（与上游一致）。

| 键（英文原文） | 简体中文 | 繁体中文 |
|---|---|---|
| %lld Profiles | %lld 个配置 | %lld 個配置 |
| **If I’ve defended your modern life, please consider sponsoring me.** | **支持我的工作** | **支持我的工作** |
| A report is saved for each service run | 每次服务运行都会保存一份报告 | 每次服務運行都會儲存一份報告 |
| About | 关于 | 關於 |
| Action | 动作 | 動作 |
| Active | 活动 | 活動 |
| Add DNS Server | 添加 DNS 服务器 | 添加 DNS 服務器 |
| Add Domain | 添加域名 | 添加域名 |
| Add Domain Rule | 添加域名规则 | 添加域名規則 |
| Add Rule | 添加规则 | 添加規則 |
| Add SSID | 添加 SSID | 添加 SSID |
| Add Search Domain | 添加搜索域 | 添加搜索域 |
| All | 所有 | 所有 |
| All specified conditions must match for the rule to apply. Leave empty to match any network. | 所有指定条件必须匹配规则才会生效。留空则匹配任何网络。 | 所有指定條件必須匹配規則才會生效。留空則匹配任何網絡。 |
| Always On | 始终启动 | 始終啟動 |
| Always Show Symbol Bar | 始终显示符号栏 | 始終顯示符號列 |
| Any | 任意 | 任意 |
| Any network | 任意网络 | 任意網絡 |
| App | 应用 | 應用 |
| App Settings | 应用设置 | 應用設置 |
| Append `0.0.0.0/31` and `::/127` to `route_exclude_address` if not exists. | 将会附加 `0.0.0.0/31` 和 `::/127` 到 `route_exclude_address`。 | 將會附加 `0.0.0.0/31` 和 `::/127` 到 `route_exclude_address`。 |
| Append `push.apple.com` to `bypass_domain`, and `17.0.0.0/8` to `route_exclude_address`. | 将会附加 `push.apple.com` 到 `bypass_domain`， 附加 `17.0.0.0/8` 到 `route_exclude_address`。 | 將會附加 `push.apple.com` 到 `bypass_domain`， 附加 `17.0.0.0/8` 到 `route_exclude_address`。 |
| Apple TV | Apple TV | Apple TV |
| Are you sure to import profile %@? | 您确定要导入 %@ 吗？ | 您確定要導入 %@ 嗎？ |
| Are you sure to import remote profile %@? You will connect to %@ to download the configuration. | 您确定要导入 %1$@ 吗？将会连接到 %2$@ 下载配置。 | 您確定要導入 %1$@ 嗎？將會連接到 %2$@ 下載配置。 |
| Attached | 已占用 | 已佔用 |
| Auth URL | 认证链接 | 認證連結 |
| Authentication | 认证 | 認證 |
| Authorize | 授权 | 授權 |
| Auto Update | 自动更新 | 自動更新 |
| Auto Update Interval | 自动更新间隔 | 自動更新間隔 |
| Automatic Update Check | 自动检查更新 | 自動檢查更新 |
| Automatically connect VPN on any network. | 在任何网络上自动连接 VPN。 | 在任何網絡上自動連接 VPN。 |
| Automatically connect or disconnect VPN based on rules. | 根据规则自动连接或断开 VPN 连接。 | 根據規則自動連接或斷開 VPN 連接。 |
| Available | 可用 | 可用 |
| Backend | 后端 | 後端 |
| Beta | Beta 版 | Beta 版 |
| Beta Settings | Beta 版设置 | Beta 版設置 |
| Browse | 浏览 | 瀏覽 |
| Browser authentication is not supported on tvOS. | tvOS 不支持浏览器认证。 | tvOS 不支援瀏覽器認證。 |
| Bus / Device | 总线 / 设备 | 匯流排 / 裝置 |
| Bus ID | 总线 ID | 匯流排 ID |
| By default, segment routing is used in `auto_route` instead of global routing.<br>If `<route_address/route_exclude_address>` exists in the configuration, this item will not take effect on the corresponding network (commonly used to resolve HomeKit compatibility issues).<br>On macOS, enabling this option will cause Internet Sharing to not work properly. | 在默认情况下，`auto_route` 中使用分段路由，而非全局路由。<br>若配置中存在 `<route_address/route_exclude_address>`，则此项不会在对应网络上生效（常用于解决 HomeKit 兼容性问题）。<br>在 macOS 上，启用此选项将导致互联网共享无法正常工作。 | 在默認情況下，`auto_route` 中使用分段路由，而非全局路由。<br>若配置中存在 `<route_address/route_exclude_address>`，則此項不會在對應網絡上生效（常用於解決 HomeKit 兼容性問題）。<br>在 macOS 上，啟用此選項將導致網際網路共享無法正常工作。 |
| Cache Size | 缓存大小 | 快取大小 |
| Camera | 相机 | 相機 |
| Camera Access Denied | 相机访问被拒绝 | 相機訪問被拒絕 |
| Camera access denied | 相机访问被拒绝 | 相機訪問被拒絕 |
| Camera is not available | 相机不可用 | 相機不可用 |
| Cancel | 取消 | 取消 |
| Cancel Test | 取消测试 | 取消測試 |
| Cellular | 蜂窝网络 | 蜂窩網絡 |
| Chain | 链 | 鏈 |
| Changelog | 更新日志 | 更新日誌 |
| Check Update | 检查更新 | 檢查更新 |
| Checking... | 检查中... | 檢查中... |
| Choose | 选取 | 選取 |
| Cipher | 加密算法 | 加密演算法 |
| Clash Mode | Clash 模式 | Clash 模式 |
| Class & Interfaces | 类别与接口 | 類別與介面 |
| Clear Cache | 清除缓存 | 清除快取 |
| Clear Logs | 清除日志 | 清除日誌 |
| Close | 关闭 | 關閉 |
| Close All Connections | 关闭所有连接 | 關閉所有連接 |
| Closed | 已关闭 | 已關閉 |
| Closed At | 关闭时间 | 關閉時間 |
| Conditions | 条件 | 條件 |
| Confidence High | 置信度高 | 置信度高 |
| Confidence Low | 置信度低 | 置信度低 |
| Confidence Medium | 置信度中 | 置信度中 |
| Configuration | 配置 | 配置 |
| Configuration files may contain private content and should not be made public. | 配置文件可能包含隐私内容，不应该被公开发布。 | 配置檔案可能包含隱私內容，不應該被公開發佈。 |
| Configurations | 配置 | 組態 |
| Connect | 连接 | 連接 |
| Connect If Needed | 按需连接 | 按需連接 |
| Connect VPN if the destination is not directly accessible. | 如果目标无法直接访问，则连接 VPN。 | 如果目標無法直接訪問，則連接 VPN。 |
| Connect via SSH | 通过 SSH 连接 | 透過 SSH 連接 |
| Connected | 已连接 | 已連線 |
| Connecting... | 连接中... | 連接中... |
| Connection | 连接 | 連接 |
| Connections | 连接 | 連接 |
| Content | 内容 | 內容 |
| Context destroyed | 上下文已销毁 | 上下文已銷燬 |
| Continue | 继续 | 繼續 |
| Copy | 复制 | 複製 |
| Core | 核心 | 核心 |
| Crash Report | 崩溃报告 | 當機報告 |
| Create | 创建 | 創建 |
| Create Manually | 手动创建 | 手動創建 |
| Create New | 新建 | 新建 |
| Created At | 创建时间 | 創建時間 |
| Custom Configuration | 自定义配置 | 自定義配置 |
| DERP region | DERP 区域 | DERP 區域 |
| DERP-relayed connection | DERP 中继连接 | DERP 中繼連接 |
| DNS Search Domains | DNS 搜索域 | DNS 搜索域 |
| DNS Server | DNS 服务器 | DNS 服務器 |
| DNS Servers | DNS 服务器 | DNS 伺服器 |
| DNS server: %@ | DNS 服务器：%@ | DNS 服務器：%@ |
| DNS servers to use for resolving the destination. If resolution fails, VPN is started. | 用于解析目标的 DNS 服务器。如果解析失败，则启动 VPN。 | 用於解析目標的 DNS 伺服器。如果解析失敗，則啟動 VPN。 |
| Dark Configuration | 深色配置 | 深色配置 |
| Dark Custom Configuration | 深色自定义配置 | 深色自定義配置 |
| Dark Theme | 深色主题 | 深色主題 |
| Dashboard | 仪表 | 儀表 |
| Dashboard Items | 仪表项 | 儀表項 |
| Data Size | 数据大小 | 數據大小 |
| Date | 日期 | 日期 |
| Debug | 调试 | 調試 |
| Decide for each connection by its destination domain, using the domain rules below. | 根据每个连接的目标域名，按下方的域名规则决定是否连接。 | 根據每個連接的目標域名，按下方的域名規則決定是否連接。 |
| Default | 默认 | 默認 |
| Defined at interface level | 在接口层级定义 | 於介面層級定義 |
| Delete | 删除 | 刪除 |
| Delete All | 全部删除 | 全部刪除 |
| Delete Rule | 删除规则 | 刪除規則 |
| Deprecated Warning | 弃用警告 | 棄用警告 |
| Destination | 目标地址 | 目標地址 |
| Destroy | 清理 | 清理 |
| Details | 详细信息 | 詳細資訊 |
| Device class | 设备类别 | 裝置類別 |
| Devices | 设备 | 裝置 |
| Direct connection | 直接连接 | 直接連接 |
| Disable Deprecated Warnings | 禁用弃用警告 | 禁用棄用警告 |
| Disabled | 已禁用 | 已禁用 |
| Disconnect | 断开连接 | 斷開連接 |
| Do not show warnings about usages of deprecated features. | 不要显示关于弃用功能使用的警告。 | 不要顯示關於棄用功能使用的警告。 |
| Do you want to save the changes you made? | 您要保存所做的更改吗？ | 您要保存所做的更改嗎？ |
| Documentation | 文档 | 文檔 |
| Domain | 域 | 域 |
| Domain Rule | 域名规则 | 域名規則 |
| Domain Rules | 域名规则 | 域名規則 |
| Domains | 域名 | 域名 |
| Domains: %@ | 域名：%@ | 域名：%@ |
| Don't Save | 不保存 | 不保存 |
| Done | 完成 | 完成 |
| Downlink | 下行 | 下行 |
| Download | 下载 | 下載 |
| Download RPM | 下载 RPM | 下載 RPM |
| Drop files here to send, or click to select | 拖放文件到此处发送，或点击选择 | 拖放檔案到此處傳送，或點擊選擇 |
| Drop files here to send, or tap to select | 拖放文件到此处发送，或轻点选择 | 拖放檔案到此處傳送，或輕點選擇 |
| Each new connection is checked against these rules by its destination domain, from top to bottom. | 每个新连接会按目标域名从上到下依次匹配这些规则。 | 每個新連接會按目標域名從上到下依次匹配這些規則。 |
| Edit | 编辑 | 編輯 |
| Edit Content | 编辑内容 | 編輯內容 |
| Edit Custom Configuration | 编辑自定义配置 | 編輯自定義配置 |
| Edit Profile | 编辑配置 | 編輯配置 |
| Edit Rule | 编辑规则 | 編輯規則 |
| Edit Server | 编辑服务器 | 編輯伺服器 |
| Empty | 空 | 空 |
| Empty logs | 无日志 | 無日誌 |
| Empty profiles | 无配置 | 無配置 |
| Enable | 启用 | 啟用 |
| Enable Memory Limit | 启用内存限制 | 啟用記憶體限制 |
| Enable Power Report | 启用电源报告 | 啟用電源報告 |
| Enabled | 已启用 | 已啟用 |
| Encrypt with age for Project S | 为 Project S 使用 age 加密 | 為 Project S 使用 age 加密 |
| Endpoint | 端点 | 端點 |
| Endpoints | 端点 | 端點 |
| Error | 错误 | 錯誤 |
| Ethernet | 以太网 | 以太網 |
| Evaluate Connection | 评估连接 | 評估連接 |
| Exclude APNs Route | 排除 APNs 路由 | 排除 APNs 路由 |
| Exit Node | 出口节点 | 出口節點 |
| Expired | 已过期 | 已過期 |
| Expires | 过期时间 | 過期時間 |
| Expires %@ | %@过期 | %@過期 |
| Export Complete | 导出完成 | 匯出完成 |
| Export Report | 导出报告 | 匯出報告 |
| External Address | 外部地址 | 外部地址 |
| Failed to decode QRS data | 解码 QRS 数据失败 | 解碼 QRS 數據失敗 |
| Failed to read dropped file | 读取拖放文件失败 | 讀取拖放檔案失敗 |
| Failed to save rules | 保存规则失败 | 保存規則失敗 |
| Fetch Memory Report | 获取内存报告 | 取得記憶體報告 |
| File | 文件 | 文件 |
| File Path | 文件路径 | 文件路徑 |
| File is not valid UTF-8 text | 文件不是有效的 UTF-8 文本 | 檔案不是有效的 UTF-8 文字 |
| File will be located at iCloud Drive/sing-box/%@ | 此文件将位于 iCloud 云盘/sing-box/%@ | 此檔案將位於 iCloud 雲碟/sing-box/%@ |
| FileName | 文件名 | 檔案名稱 |
| Files | 文件 | 檔案 |
| Flavor | 协议类型 | 協議類型 |
| Follow Theme | 跟随主题 | 跟隨主題 |
| Font | 字体 | 字體 |
| Font Configuration | 字体配置 | 字體配置 |
| Fonts | 字体 | 字體 |
| Force Show Latest Version as Update | 强制显示最新版本为更新 | 強制顯示最新版本為更新 |
| Format | 格式化 | 格式化 |
| Forward Agent | 转发代理 | 轉發代理 |
| From Outbound | 来自出站 | 來自出站 |
| Full Disk Access permission is required | 需要完全磁盘访问权限 | 需要完全磁盤訪問權限 |
| Get higher GitHub API rate limits | 获取更高的 GitHub API 速率限制 | 取得更高的 GitHub API 速率限制 |
| Ghostty Configuration | Ghostty 配置 | Ghostty 配置 |
| GitHub Sponsors (recommended) | 通过 Github Sponsors (推荐) | 通過 Github Sponsors (推薦) |
| GitHub Token | GitHub Token | GitHub Token |
| Group | 分组 | 分組 |
| Groups | 组 | 組 |
| Handled unknown URL %@ | 无法处理的URL: %@ | 無法處理的URL: %@ |
| Hang | 卡顿 | 卡頓 |
| Helper Service | 辅助服务 | 輔助服務 |
| Helper Service Required | 需要辅助服务 | 需要輔助服務 |
| Hide VPN Icon | 隐藏 VPN 图标 | 隱藏 VPN 圖標 |
| Hostname | 主机名 | 主機名稱 |
| IP Version | IP 版本 | IP 版本 |
| IP address | IP 地址 | IP 地址 |
| Identity | 标识 | 標識 |
| Idle | 空闲 | 閒置 |
| Idle Latency | 空闲延迟 | 閒置延遲 |
| If enabled, you can quickly connect to this peer via the **context menu** (long press) on the Tailscale entry in Tools and on peer entries in the peer list.<br><br>This peer will also appear in the **New Session** menu when connected to other peers via SSH. | 启用后，你可以通过工具中 Tailscale 条目和节点列表中节点条目的**上下文菜单**（长按）快速连接到此节点。<br><br>当通过 SSH 连接到其他节点时，此节点也会出现在**新建会话**菜单中。 | 啟用後，你可以透過工具中 Tailscale 條目和節點列表中節點條目的**上下文選單**（長按）快速連接到此節點。<br><br>當透過 SSH 連接到其他節點時，此節點也會出現在**新建工作階段**選單中。 |
| If enabled, you can quickly connect to this peer via the **context menu** (long press) on the Tailscale entry in Tools and on peer entries in the peer list.<br><br>This peer will also appear in the **New Window** menu when connected to other peers via SSH. | 启用后，你可以通过工具中 Tailscale 条目和节点列表中节点条目的**上下文菜单**（长按）快速连接到此节点。<br><br>当通过 SSH 连接到其他节点时，此节点也会出现在**新建窗口**菜单中。 | 啟用後，你可以透過工具中 Tailscale 條目和節點列表中節點條目的**上下文選單**（長按）快速連接到此節點。<br><br>當透過 SSH 連接到其他節點時，此節點也會出現在**新建視窗**選單中。 |
| If set, a request is sent to this URL. If it doesn't return HTTP 200, VPN is started. | 如果设置，将向此 URL 发送请求。如果未返回 HTTP 200，则启动 VPN。 | 如果設置，將向此 URL 發送請求。如果未返回 HTTP 200，則啟動 VPN。 |
| Ignore | 忽略 | 忽略 |
| Import | 导入 | 導入 |
| Import Profile | 导入配置 | 導入配置 |
| Import Remote Profile | 导入远程配置 | 導入遠程配置 |
| Import from File | 从文件导入 | 從文件導入 |
| Import from iPhone or iPad | 从 iPhone 或 iPad 导入 | 從 iPhone 或 iPad 導入 |
| Imported | 已导入 | 已匯入 |
| In Minutes | 分钟 | 分鐘 |
| In the standalone version of SFM, reading WiFi state requires this app to be running. After you quit the SFM app, the sing-box service cannot continue to provide `wifi_ssid` and `wifi_bssid` routing rules. | 在 SFM 独立版本中，读取 WiFi 状态需要应用保持运行。退出 SFM 应用后，sing-box 服务将无法继续提供 `wifi_ssid` 和 `wifi_bssid` 路由规则。 | 在 SFM 獨立版本中，讀取 WiFi 狀態需要應用保持運行。退出 SFM 應用後，sing-box 服務將無法繼續提供 `wifi_ssid` 和 `wifi_bssid` 路由規則。 |
| Inbound | 入站 | 入站 |
| Inbound Type | 入站类型 | 入站類型 |
| Install | 安装 | 安裝 |
| Install Helper Service | 安装辅助服务 | 安裝輔助服務 |
| Install Network Extension | 安装网络扩展 | 安裝網絡擴展 |
| Install System Extension | 安装系统扩展 | 安裝系統擴展 |
| Interface %lld | 接口 %lld | 介面 %lld |
| Interface Type | 接口类型 | 接口類型 |
| Invalid IP address | 无效的 IP 地址 | 無效的 IP 地址 |
| Invalid QR code | 无效的二维码 | 無效的二維碼 |
| Keep Menu Bar in Background | 将菜单栏保留在后台 | 將菜單欄保留在後臺 |
| Key Expiry | 密钥过期 | 金鑰到期 |
| Key expiry disabled | 密钥过期已禁用 | 金鑰到期已停用 |
| Keyboard | 键盘 | 鍵盤 |
| Kill Connections | 终止连接 | 終止連線 |
| Kill all connections to free memory when the service memory exceeds the limit. | 当服务内存超出限制时，终止所有连接以释放内存。 | 當服務記憶體超出限制時，終止所有連線以釋放記憶體。 |
| Language | 语言 | 語言 |
| Language will be changed after restarting the app. | 重启应用后语言将生效。 | 重啟應用後語言將生效。 |
| Last Seen | 最后在线 | 最後上線 |
| Last Updated | 最后更新 | 最後更新 |
| Latency | 延迟 | 延遲 |
| Launch the application when the system is logged in. If enabled at the same time as `Show in Menu Bar` and `Keep Menu Bar in Background`, the application interface will not be opened automatically. | 在系统登录时启动应用，若同时开启 `在菜单栏中显示` 和 `将菜单栏保留在后台`，则不会自动打开应用界面。 | 在系統登錄時啟動應用，若同時開啟 `在菜單欄中顯示` 和 `將菜單欄保留在後臺`，則不會自動打開應用界面。 |
| Leave the VPN connection in its current state. | 保持 VPN 连接的当前状态。 | 保持 VPN 連接的當前狀態。 |
| Light Configuration | 浅色配置 | 淺色配置 |
| Light Custom Configuration | 浅色自定义配置 | 淺色自定義配置 |
| Light Theme | 浅色主题 | 淺色主題 |
| Loading... | 加载中 . . . | 加載中 . . . |
| Local | 本地 | 本地 |
| Local Device | 本机 | 本機 |
| Log Level | 日志级别 | 日誌級別 |
| Log out | 登出 | 登出 |
| Logs | 日志 | 日誌 |
| Logs and configuration files may contain private content and should not be made public. | 日志和配置文件可能包含隐私内容，不应该被公开发布。 | 日誌和配置檔案可能包含隱私內容，不應該被公開發佈。 |
| MagicDNS | MagicDNS | MagicDNS |
| Manage Imported Fonts | 管理已导入的字体 | 管理已匯入的字型 |
| Manage Servers... | 管理服务器... | 管理伺服器... |
| Managing working directory requires Helper Service. | 管理工作目录需要辅助服务。 | 管理工作目錄需要輔助服務。 |
| Matches only when a request to this URL returns HTTP 200. | 仅当请求此 URL 返回 HTTP 200 时匹配。 | 僅當請求此 URL 返回 HTTP 200 時匹配。 |
| Matches when every DNS server of the network is in this list. | 当网络的所有 DNS 服务器都在此列表中时匹配。 | 當網絡的所有 DNS 服務器都在此列表中時匹配。 |
| Matches when the default search domain of the network is one of these. | 当网络的默认搜索域是其中之一时匹配。 | 當網絡的默認搜索域是其中之一時匹配。 |
| Matches when the destination host name ends with one of these domains. | 当目标主机名以其中任一域名结尾时匹配。 | 當目標主機名以其中任一域名結尾時匹配。 |
| Max Runtime | 最大运行时间 | 最大執行時間 |
| Memory | 内存 | 內存 |
| Memory Limit | 内存限制 | 記憶體限制 |
| Metadata | 元数据 | 元數據 |
| Metered Connection | 按流量计费连接 | 按流量計費連線 |
| Missing URL | 缺少 URL | 缺少 URL |
| Missing access to selected file | 无法访问所选文件 | 無法訪問所選文件 |
| Missing file | 缺少文件 | 缺少文件 |
| Missing path | 缺少路径 | 缺少路徑 |
| Missing profile name | 缺少配置文件名称 | 缺少配置文件名稱 |
| Mode | 模式 | 模式 |
| Move Down | 下移 | 下移 |
| Move Up | 上移 | 上移 |
| Move to Applications | 移动到 Applications | 移動到 Applications |
| NAT Filtering | NAT 过滤 | NAT 過濾 |
| NAT Mapping | NAT 映射 | NAT 映射 |
| NAT Type Detection | NAT 类型检测 | NAT 類型偵測 |
| Name | 名称 | 名稱 |
| Need Reboot | 需要重启系统 | 需要重啟系統 |
| Network | 网络 | 網絡 |
| Network Quality | 网络质量 | 網路品質 |
| Never Connect | 从不连接 | 從不連接 |
| Never connect VPN for matching domains. | 对匹配的域名不连接 VPN。 | 對匹配的域名不連接 VPN。 |
| New Profile | 新建配置 | 新建配置 |
| New Rule | 新建规则 | 新建規則 |
| New Server | 新建服务器 | 新建伺服器 |
| New Session | 新建会话 | 新建工作階段 |
| New Window | 新建窗口 | 新建視窗 |
| New version available: %@ | 新版本可用：%@ | 新版本可用：%@ |
| No Default Route | 不使用默认路由 | 不使用默認路由 |
| No Tailscale endpoint is running | 没有正在运行的 Tailscale 端点 | 沒有正在運行的 Tailscale 端點 |
| No active connections | 暂无活动连接 | 暫無活動連接 |
| No closed connections | 暂无已关闭连接 | 暫無已關閉連接 |
| No connections | 暂无连接 | 暫無連接 |
| No data | 无数据 | 無資料 |
| No devices shared yet. | 尚未共享任何设备。 | 尚未共享任何裝置。 |
| No domains | 无域名 | 無域名 |
| No local devices available | 没有可用的本地设备 | 沒有可用的本機裝置 |
| No matching connections | 没有匹配的连接 | 沒有匹配的連接 |
| No servers | 无服务器 | 無伺服器 |
| No updates available | 没有可用的更新 | 沒有可用的更新 |
| No, thanks | 不，谢谢 | 不，謝謝 |
| Not Connected | 未连接 | 未連線 |
| Not supported by server | 服务器不支持 | 伺服器不支援 |
| OOM Report | 内存不足报告 | 記憶體不足報告 |
| OS | 操作系统 | 作業系統 |
| Ok | 好 | 好 |
| On Demand Rules | 按需启动 | 按需啟動 |
| Only HTTP and HTTPS URLs are allowed | 只允许 HTTP 和 HTTPS URL | 只允許 HTTP 和 HTTPS URL |
| Only available in the macOS standalone version | 仅在 macOS 独立版本中可用 | 僅在 macOS 獨立版本中可用 |
| Open | 打开 | 打開 |
| Open Auth URL | 打开认证链接 | 開啟認證連結 |
| Open Auth URL as QR Code | 以二维码打开认证链接 | 以二維碼開啟認證連結 |
| Open Settings | 打开设置 | 打開設置 |
| Open sing-box | 打开 sing-box | 開啟 sing-box |
| Optional | 可选 | 可選 |
| Other methods | 其他方式 | 其他方式 |
| Others | 其他 | 其他 |
| Outbound | 出站 | 出站 |
| Outbound Type | 出站类型 | 出站類型 |
| Overview | 概述 | 概述 |
| Packet Tunnel | 隧道 | 隧道 |
| Password | 密码 | 密碼 |
| Path | 路径 | 路徑 |
| Pause | 暂停 | 暫停 |
| Peer relay | 节点中继 | 節點中繼 |
| Peer relay connection | 节点中继连接 | 節點中繼連接 |
| Ping | Ping | Ping |
| Please enable camera access in Settings to scan QR codes. | 请在设置中启用相机访问权限以扫描二维码。 | 請在設置中啟用相機訪問權限以掃描二維碼。 |
| Please grant the permission for **SFMExtension**, then we can continue. | 请为 **SFMExtension** 授予权限，然后我们可以继续。 | 請爲 **SFMExtension** 授予權限，然後我們可以繼續。 |
| Power Report | 电源报告 | 電源報告 |
| Preparing | 正在准备 | 正在準備 |
| Probe URL | 探测 URL | 探測 URL |
| Probe: %@ | 探测：%@ | 探測：%@ |
| Product | 产品 | 產品 |
| Profile | 配置 | 配置 |
| Profile Override | 配置重载 | 配置重載 |
| Profile missing | 配置丢失 | 配置丟失 |
| Profile not found | 未找到配置 | 未找到配置 |
| Profiles | 配置 | 配置 |
| Protocol | 协议 | 協議 |
| Provide a soft memory limit for the service. The service will perform multiple processes to try to stay within this memory limit. | 为服务提供软内存限制。服务将执行多个进程以尝试保持在此内存限制范围内。 | 為服務提供軟記憶體限制。服務將執行多個程序以嘗試保持在此記憶體限制範圍內。 |
| Quick Connect | 快速连接 | 快速連接 |
| Quit | 退出 | 退出 |
| Quit sing-box | 退出 sing-box | 退出 sing-box |
| Rate on the App Store | 在 App Store 上评分 | 在 App Store 上評分 |
| Read from connection: %@ | 读取连接：%@ | 讀取連線：%@ |
| Real-time Speed | 实时速率 | 實時速率 |
| Reasserting | 重新连接中 | 重新連接中 |
| Reboot required. | 需要重启系统。 | 需要重啟系統。 |
| Receive | 接收 | 接收 |
| Redo | 重做 | 重做 |
| Register task failed | 注册任务失败 | 註冊任務失敗 |
| Remember SSH Options | 记住 SSH 选项 | 記住 SSH 選項 |
| Remote | 远程 | 遠程 |
| Remote Control | 远程控制 | 遠端控制 |
| Remote error: %@ | 远程错误：%@ | 遠端錯誤：%@ |
| Reports | 报告 | 報告 |
| Required | 必填 | 必填 |
| Reset | 重置 | 重置 |
| Response | 响应 | 回應 |
| Restart Required | 需要重启 | 需要重啟 |
| Results | 结果 | 結果 |
| Resume | 恢复 | 恢復 |
| Rules | 规则 | 規則 |
| Rules are evaluated in order from top to bottom. The first matching rule determines the action. | 规则从上到下依次评估。第一个匹配的规则决定操作。 | 規則從上到下依次評估。第一個匹配的規則決定操作。 |
| SSH | SSH | SSH |
| SSH Options | SSH 选项 | SSH 選項 |
| SSID: %@ | SSID：%@ | SSID：%@ |
| STUN Test | STUN 测试 | STUN 測試 |
| Save | 保存 | 保存 |
| Save Content JSON | 保存内容 JSON | 保存內容 JSON |
| Save File | 保存文件 | 保存文件 |
| Scan QR Code | 扫描二维码 | 掃描二維碼 |
| Search | 搜索 | 搜索 |
| Search Domain | 搜索域 | 搜索域 |
| Search domain: %@ | 搜索域：%@ | 搜索域：%@ |
| Secret | 密钥 | 密鑰 |
| Select Device | 选择设备 | 選擇設備 |
| Select Profile | 选择配置 | 選擇配置 |
| Send | 发送 | 發送 |
| Sending... | 发送中... | 傳送中... |
| Serial | 串行 | 序列 |
| Serial number | 序列号 | 序號 |
| Server | 服务器 | 伺服器 |
| Servers | 服务器 | 伺服器 |
| Service is Running | 服务正在运行 | 服務正在運行 |
| Service not started | 服务未启动 | 服務未啟動 |
| Services | 服务 | 服務 |
| Settings | 设置 | 設置 |
| Share | 分享 | 分享 |
| Share Content JSON File | 分享内容 JSON 文件 | 分享內容 JSON 文件 |
| Share File | 分享文件 | 分享文件 |
| Share URL as QR Code | 通过二维码分享 URL | 通過二維碼分享 URL |
| Share USB Device | 共享 USB 设备 | 共享 USB 裝置 |
| Share as QRS Code | 通过 QRS 码分享 | 通過 QRS 碼分享 |
| Share profile | 分享配置 | 分享配置 |
| Shared in | 共享至此 | 共享至此 |
| Show in Menu Bar | 在菜单栏中显示 | 在菜單欄中顯示 |
| Size | 大小 | 大小 |
| Slice Size | 分片大小 | 分片大小 |
| Sort By | 排序 | 排序 |
| Source | 源 | 源 |
| Source Code | 源代码 | 源代碼 |
| Speed | 速度 | 速度 |
| Sponsors | 赞助 | 贊助 |
| Stable | 稳定版 | 穩定版 |
| Start | 启动 | 啟動 |
| Start At Login | 开机启动 | 開機啟動 |
| Start Test | 开始测试 | 開始測試 |
| Start the VPN connection when conditions match. | 当条件匹配时启动 VPN 连接。 | 當條件匹配時啟動 VPN 連接。 |
| Started | 已启动 | 已啟動 |
| Starting | 启动中 | 啟動中 |
| State | 状态 | 狀態 |
| Status | 状态 | 狀態 |
| Stop | 停止 | 停止 |
| Stop Service and Continue | 停止服务并继续 | 停止服務並繼續 |
| Stop Sharing | 停止共享 | 停止共享 |
| Stop the VPN connection when conditions match. | 当条件匹配时停止 VPN 连接。 | 當條件匹配時停止 VPN 連接。 |
| Stopped | 已停止 | 已停止 |
| Stopping | 停止中 | 停止中 |
| Submit | 提交 | 提交 |
| System Default | 跟随系统 | 跟隨系統 |
| System Extension | 系统扩展 | 系統擴展 |
| System Extension removed. | 系统扩展已移除。 | 系統擴展已移除。 |
| System Extension updated. | 系统扩展已更新。 | 系統擴展已更新。 |
| System HTTP Proxy | 系统 HTTP 代理 | 系統 HTTP 代理 |
| Taildrop | Taildrop | Taildrop |
| Tailscale | Tailscale | Tailscale |
| Tailscale Addresses | Tailscale 地址 | Tailscale 位址 |
| Tailscale: %@ | Tailscale: %@ | Tailscale: %@ |
| Taiwan Flag Available | 特殊旗帜可用 | 特殊旗幟可用 |
| Terminal Type | 终端类型 | 終端類型 |
| The Helper Service provides process lookup, the Bridge outbound, and many other basic features. | 辅助服务提供进程查找、Bridge 出站等许多基础功能。 | 輔助服務提供進程查找、Bridge 出站等許多基礎功能。 |
| The QR code does not contain a valid profile import link. | 该二维码不包含有效的配置导入链接。 | 該二維碼不包含有效的配置導入鏈接。 |
| The service must be stopped before destroying the working directory. | 必须先停止服务才能清理工作目录。 | 必須先停止服務才能清理工作目錄。 |
| The sing-box service requires Helper Service to provide process lookup functionality, which supports `process_name` and `process_path` routing rules. | sing-box 服务需要辅助服务来提供进程查找功能，以支持 `process_name` 和 `process_path` 路由规则。 | sing-box 服務需要輔助服務來提供進程查找功能，以支持 `process_name` 和 `process_path` 路由規則。 |
| The symbol bar is hidden while a hardware keyboard is connected unless this is enabled. | 连接硬件键盘时隐藏符号栏，除非开启此选项。 | 連接硬體鍵盤時隱藏符號列，除非開啟此選項。 |
| Theme | 主题 | 主題 |
| Themes | 主题 | 主題 |
| This Device | 此设备 | 此裝置 |
| This app needs to be placed under the Applications folder to work. | 此应用程序需要放置在 Applications 文件夹下才能运行。 | 此應用程序需要放置在 Applications 文件夾下才能運行。 |
| This helper service provides process lookup for `process_name` and `process_path` routing rules, and manages the working directory. | 此辅助服务为 `process_name` 和 `process_path` 路由规则提供进程查找功能，并管理工作目录。 | 此輔助服務爲 `process_name` 和 `process_path` 路由規則提供進程查找功能，並管理工作目錄。 |
| To Clipboard | 到剪贴板 | 到剪貼板 |
| To File | 到文件 | 到文件 |
| To export this report to your iPhone or iPad, make sure sing-box is the **same version** on both devices and **VPN is disabled**. | 要将此报告导出到 iPhone 或 iPad，请确保两台设备上的 sing-box 为**相同版本**且**已关闭 VPN**。 | 要將此報告匯出至 iPhone 或 iPad，請確認兩部裝置上的 sing-box 為**相同版本**且**已關閉 VPN**。 |
| To import configurations from your iPhone or iPad, make sure sing-box is the **same version** on both devices and **VPN is disabled**. | 要从您的 iPhone 或 iPad 导入配置，请确保两台设备上的 sing-box 是**相同版本**且 **VPN 已禁用**。 | 要從您的 iPhone 或 iPad 導入配置，請確保兩臺設備上的 sing-box 是**相同版本**且 **VPN 已禁用**。 |
| To provide devices, use a Chromium-based browser with the sing-box dashboard, or the sing-box graphical client on macOS or Android. | 要提供设备，您可以使用基于 Chromium 的浏览器配合 sing-box dashboard，或使用 macOS 或 Android 上的 sing-box 图形客户端。 | 要提供裝置，您可以使用基於 Chromium 的瀏覽器配合 sing-box dashboard，或使用 macOS 或 Android 上的 sing-box 圖形客戶端。 |
| Tools | 工具 | 工具 |
| Traffic | 速率 | 速率 |
| Traffic Total | 流量 | 流量 |
| Transport | 传输 | 傳輸 |
| Type | 类型 | 類型 |
| USB/IP | USB/IP | USB/IP |
| USB/IP: %@ | USB/IP：%@ | USB/IP：%@ |
| Unavailable | 不可用 | 不可用 |
| Undo | 撤销 | 撤銷 |
| Unexpected message type %lld | 意外的消息类型 %lld | 意外的消息類型 %lld |
| Unified | 统一 | 統一 |
| Uninstall | 卸载 | 卸載 |
| Unknown | 未知 | 未知 |
| Unknown message type %u | 未知消息类型 %u | 未知訊息類型 %u |
| Unsaved Changes | 未保存的更改 | 未保存的更改 |
| Update | 更新 | 更新 |
| Update Settings | 更新设置 | 更新設定 |
| Update Track | 更新通道 | 更新通道 |
| Uplink | 上传 | 上傳 |
| Upload | 上传 | 上傳 |
| Upload RPM | 上传 RPM | 上傳 RPM |
| Use the + button to share a local USB device from this Mac. | 使用 + 按钮共享此 Mac 上的本地 USB 设备。 | 使用 + 按鈕共享此 Mac 上的本機 USB 裝置。 |
| User | 用户 | 用戶 |
| Username | 用户名 | 使用者名稱 |
| VPN will not connect automatically. | VPN 不会自动连接。 | VPN 不會自動連接。 |
| Verifying | 正在验证 | 正在驗證 |
| Version | 版本 | 版本 |
| View Content | 查看内容 | 查看內容 |
| View Release | 查看发布 | 查看發佈 |
| What is QRS | 什么是 QRS | 什麼是 QRS |
| When memory limit is enabled, you will receive a report if the service memory exceeds the limit. You can also manually trigger report collection. | 启用内存限制后，当服务内存超出限制时，您将会收到报告。您也可以手动触发收集报告。 | 啟用記憶體限制後，當服務記憶體超出限制時，您將會收到報告。您也可以手動觸發收集報告。 |
| Wi-Fi | Wi-Fi | Wi-Fi |
| WiFi State Access | WiFi 状态访问 | WiFi 狀態訪問 |
| With Configuration | 附带配置 | 附帶配置 |
| With Log | 附带日志 | 附帶日誌 |
| Without rules, the VPN connects on any network. | 没有规则时，VPN 会在任意网络上连接。 | 沒有規則時，VPN 會在任意網絡上連接。 |
| Working Directory | 工作目录 | 工作目錄 |
| Would you like to enable automatic update checking from **GitHub**? | 是否启用从 **GitHub** 自动检查更新？ | 是否啟用從 **GitHub** 自動檢查更新？ |
| Wrong application location | 应用程序位置错误 | 應用程序位置錯誤 |
| Yes | 是 | 是 |
| You can customize the terminal appearance by editing the Ghostty Configuration in App Settings. | 你可以通过在应用设置中编辑 Ghostty 配置来自定义终端外观。 | 你可以透過在應用程式設定中編輯 Ghostty 配置來自定義終端外觀。 |
| You will receive a report when a crash occurs. | 当遇到崩溃时，您将会收到报告。 | 當遇到崩潰時，您將會收到報告。 |
| You will receive a report when the service runs out of memory. You can also manually trigger report collection. | 当服务内存不足时，您将会收到报告。您也可以手动触发收集报告。 | 當服務記憶體不足時，您將會收到報告。您也可以手動觸發收集報告。 |
| You're on a metered connection. This test will use a significant amount of data. | 你正在使用按流量计费的连接。此测试将使用大量数据。 | 你正在使用按流量計費的連線。此測試將使用大量數據。 |
| [age](https://github.com/filosottile/age) is a modern and secure asymmetric encryption tool. When enabled, the zip file is encrypted with this project's public key so it can be posted publicly, e.g. in GitHub issues. | [age](https://github.com/filosottile/age) 是一个现代且安全的非对称加密工具。启用后，zip 文件将使用本项目的公钥加密，因此可以公开发布，例如发布在 GitHub issue 中。 | [age](https://github.com/filosottile/age) 是一個現代且安全的非對稱加密工具。啟用後，zip 檔案將使用本專案的公鑰加密，因此可以公開發佈，例如發佈在 GitHub issue 中。 |
| false | 否 | 否 |
| from %@ | 来自 %@ | 來自 %@ |
| https://sing-box.sagernet.org/ | https://sing-box.sagernet.org/zh/ | https://sing-box.sagernet.org/zh/ |
| https://sing-box.sagernet.org/changelog/ | https://sing-box.sagernet.org/zh/changelog/ | https://sing-box.sagernet.org/zh/changelog/ |
| https://sing-box.sagernet.org/configuration/ | https://sing-box.sagernet.org/zh/configuration/ | https://sing-box.sagernet.org/zh/configuration/ |
| to %@ | 发送至 %@ | 傳送至 %@ |
| true | 是 | 是 |


## Windows 按需启动差异文案（11.5、A-23）

以下文案覆盖 Apple 基线中关于逐连接域名评估的说明；不能再从上游直接覆盖回不适用的承诺。俄语与波斯语须在 M5 前补齐并校对，缺失时不得通过五语言验收。

| 英文 | 简体中文 | 繁體中文 |
|---|---|---|
| Windows does not currently support per-connection triggers based on the destination domain. | Windows 暂不支持按目标域名逐连接触发。 | Windows 暫不支援按目標網域逐連線觸發。 |
| This rule is not executed on Windows. | 此规则在 Windows 上不执行。 | 此規則在 Windows 上不執行。 |
| Rules are evaluated from top to bottom. Unsupported rules are skipped; the first matching rule determines the action. | 规则自上而下评估，跳过不支持的规则，第一条匹配的规则决定动作。 | 規則由上而下評估，略過不支援的規則，由第一條符合的規則決定動作。 |
