# 本地配置服务

范围仅本地 JSON 配置：创建、导入原文、读写、名称唯一化、选择与删除。未实现远程配置、BPF、二维码、编辑器、连接后的重载或界面变更事件；调用方在成功后刷新界面，需要重载时自行调用官方服务。

```go
svc, err := profile.Open(ctx, validatedApplicationDataDirectory,
    profile.CheckFunc(func(ctx context.Context, content string) error {
        // 调用已认证 worker 的 ApplicationService.CheckConfig。
        return application.CheckConfig(ctx, content)
    }))
// 退出时 svc.Close()。
```

必须注入官方校验器；nil 会失败。此包不导入 sing-box 运行时代码，也不会猜测或写用户 APPDATA。父层提供绝对数据目录并保证应用单实例，同一目录只创建一个 Service。校验器不能重入同一 Service。校验需要服务可用时，其错误直接返回；不会自动降级成仅 JSON 语法检查。

`Create(ctx,name)` 写 `{}`；`ImportJSON(ctx,name,content)` 接收已读到的 UTF-8 字符串，去掉开头 BOM，不改内容格式。二者成功后自动选中。`Rename` 与导入均自动尝试 `名称 (1)` 等；`Selected` 无有效选择时返回 `nil,nil`；操作不存在的 ID 返回 `ErrNotFound`。`UpdateContent` 也先校验。Service 方法串行，允许多 goroutine 调用；数据库写由 store 的唯一 writer goroutine 执行。

SQLite 元数据位于 `settings.db`，配置位于 `configs/config_<id>.json`。ID 按 `max(id)+1` 分配。数据库使用 WAL、FULL 同步、一个写连接与两个只读连接；三个迁移按规定依赖顺序记录于 migrations。偏好使用 `store.Preference(ctx,db,key,default)` 和 `db.SetPreference`，字符串为 UTF-8 原文，其余 JSON，nil 删除，读取错误/损坏值返回默认。

新建数据目录与 configs 在 Windows 上设置受保护 ACL，仅当前用户、SYSTEM、Administrators 完全控制。已存在目录的 ACL 不擅自修改，固定共享目录应由安装器负责；路径中的符号链接/重解析点被拒绝。未实现对正在被外部进程替换路径的抵御，依赖单实例及可信数据目录。

文件写入先创建同目录临时文件、Flush，再通过 Windows MoveFileEx 的 REPLACE_EXISTING + WRITE_THROUGH 原子替换。创建和删除以 `configs/.pending.json` 记录文件/数据库双存储操作：SQLite 提交失败时补偿；中断后下次 Open 按已提交元数据恢复/清理。删除先将内容移到暂存名，元数据与选中回退在同一 SQLite 事务提交。补偿或清理失败会返回错误并保留 journal，后续操作拒绝继续，需关闭重开恢复；调用方不要把这类错误当成操作必定未生效。遇到歧义文件状态失败关闭，不扫描删除未知文件。

运行时必须提供对应架构的 MyGo `mygo-sqlite3.dll`（SQLite 3.53.4，版本/摘要来自锁定子模块 `plugins/sqlite/mygo-plugin.json`）。打包应将 DLL 放应用资源目录或可执行文件旁，开发/测试可显式设置 `MYGO_SQLITE_LIBRARY`。缺 DLL 将报错；不使用内存数据库替代。加载器在非打包应用找不到 DLL 时可能下载到用户缓存，因此本项目测试显式设置 DLL 路径：

```powershell
. ./scripts/env.ps1
$env:MYGO_SQLITE_LIBRARY = Join-Path $PWD 'build/app/mygo-sqlite3.dll'
go test ./internal/profile ./internal/store
```

测试仅写 `t.TempDir()`：真实 SQLite 文件持久化、唯一名并发、校验失败、SQL 触发器写失败与补偿、删除后选择回退、四类中断恢复，以及 Windows 文件共享冲突下原子替换失败不丢旧内容。受限沙箱令牌无法访问真实私有 ACL 时需以普通用户完整令牌执行测试，不应削弱 ACL 让测试通过。
