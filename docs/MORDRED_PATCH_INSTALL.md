# 莫德雷德 10219001 后端测试补丁安装说明

> **单人模式尚未实现。** 2026-11-18 已确认，现有客户端的单人模式使用客户端主表和独立的原生战斗 DLL。本包只更新服务端，安装成功后单人卡牌的说明、费用及实际技能效果仍可能完全不变。当前不能把本包当作单人可用的改卡补丁；已安装用户不需要通过更换启动方式或反复覆盖来解决。需要回滚时见下文。

适用环境：Windows x64，与本次补丁源文件哈希匹配的完整服务端安装。此包已包含重编译的 exe，无需在服务端安装 Go。

补丁包含 9C 绝技及佣兵被动：取消初始抽牌、自身火属性物理/魔法伤害提升 12%、自身原始费用 ≥7C 的卡牌减 2C。本卡绝技在该被动生效后实际消耗 7C。

## 目录内容

- `overlay/`：需要覆盖到服务端根目录的 7 个文件：exe、5 个 CSV、资源清单。
- `baseline-sha256.json`：安装前服务端的 6 个资源文件必须匹配的哈希。
- `patch.json`、`verification.json`：改动和后端验证记录。
- `TECHNICAL.md`：技能细节、重新生成资源及构建的方法。
- `SHA256SUMS.txt`：包内文件哈希；打包工具已逐个读取 ZIP 条目验证内容。

只合并 `overlay` 中的文件。不要删除原 `resource-set` 目录，也不要把说明文件、备份或整个补丁目录放进服务端的 `resource-set`；启动器会检查该目录的文件清单。

## 安装

1. 将 ZIP 解压到服务端目录外，例如 `E:\Game\mordred-patch-test`。建议使用完整服务端的测试副本。
2. 退出游戏，运行目标服务端的 `Stop-Server.cmd`，确认提示 `STOPPED` 或已经停止；若停服报错，不要继续覆盖。
3. 在解压目录中打开 PowerShell，执行下面的命令。`$server` 改成实际目标目录；当前示例是本机 `E:\Game\ma-server`。

```powershell
$ErrorActionPreference = 'Stop'
$server = 'E:\Game\ma-server'
$patch = (Get-Location).Path
$overlay = Join-Path $patch 'overlay'

# 安装前核对完整资源清单与五个原始 CSV，避免覆盖其他版本或已有改卡。
$baseline = Get-Content -LiteralPath (Join-Path $patch 'baseline-sha256.json') -Raw -Encoding UTF8 | ConvertFrom-Json
foreach ($entry in $baseline.PSObject.Properties) {
    $actual = (Get-FileHash -LiteralPath (Join-Path $server $entry.Name) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $entry.Value) { throw "资源版本不匹配，未安装：$($entry.Name)" }
}
if (@(Get-Process -Name kairi-server -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq (Join-Path $server 'kairi-server.exe') }).Count) {
    throw '目标服务端仍在运行，请先停服。'
}

# 将旧程序、旧资源和停服后的存档备份到服务端目录外。
$backup = Join-Path (Split-Path -Parent $server) ('ma-server-backup-' + (Get-Date -Format 'yyyyMMdd-HHmmssfff'))
$payload = @(Get-ChildItem -LiteralPath $overlay -Recurse -File)
if ($payload.Count -ne 7) { throw '补丁覆盖文件数量不正确，未安装。' }
foreach ($file in $payload) {
    $relative = $file.FullName.Substring($overlay.Length + 1)
    $destination = Join-Path $backup "files/$relative"
    New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $server $relative) -Destination $destination
    if ((Get-FileHash -LiteralPath $destination).Hash -ne (Get-FileHash -LiteralPath (Join-Path $server $relative)).Hash) { throw '备份校验失败，未安装。' }
}
Copy-Item -LiteralPath (Join-Path $server '_local/data') -Destination (Join-Path $backup 'data') -Recurse

# 只覆盖这七个文件，保留原有媒体资源、配置与玩家数据库。
Copy-Item -Path (Join-Path $overlay '*') -Destination $server -Recurse -Force
foreach ($file in $payload) {
    $relative = $file.FullName.Substring($overlay.Length + 1)
    if ((Get-FileHash -LiteralPath (Join-Path $server $relative)).Hash -ne (Get-FileHash -LiteralPath $file.FullName).Hash) { throw "覆盖校验失败，请回滚：$relative" }
}
Write-Host "安装完成。备份目录：$backup"
```

4. 使用原来的 `Start-Server.cmd` 或启动器启动服务端；启动器原有地址、端口、平台配置保持不变。如果有启动报错，保留错误内容并回滚，不要只恢复其中一个 CSV。
5. 当前发布版客户端尚未适配，不能用单人模式验证本包效果。完成客户端主表与战斗库适配后，才可进行游戏内的费用、被动、4/7 段攻击、圣剑槽和条件分支验收。此限制不限于卡面文字。

## 回滚

先退出游戏并停服。将备份目录中的 `files` 下全部内容合并覆盖回服务端根目录，再按原方式启动：

```powershell
$ErrorActionPreference = 'Stop'
$server = 'E:\Game\ma-server'
$backup = 'E:\Game\ma-server-backup-实际生成的时间'
Copy-Item -Path (Join-Path $backup 'files/*') -Destination $server -Recurse -Force
```

这会恢复旧 exe、五个 CSV 和资源清单。若还要恢复测试前的存档，保持停服，先将当前 `_local/data` 改名留存，再将备份中的 `data` 整目录复制回 `_local/data`，避免混用新旧 SQLite 文件。

## 已知限制

此包仅修改后端，未更新 PC/Android 客户端主表、说明及资源。单人模式在客户端的 `kairisei_battlecore.dll` 中计算，不调用此包的 Go 服务端战斗引擎。本次发布版 DLL 的 Go 模块为 `kairisei.local/server/clientrebuildbridge/cmd/battlecore-library`，其桥接源码不在当前仓库；新增组合条件也不在当前客户端元数据中。需要配套处理主表和原生模块，不能把后端测试通过视为整套游戏已经适配。不要通过同步 CDN 来替代客户端主表适配。

## 再次打包

在本仓库根目录运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\powershell\Package-MordredPatch.ps1
```

此命令打包 `_local/mordred-9c` 下已生成、已验证的资源和 exe，生成带时间戳的 ZIP，并核对归档内每个文件；不会改动或启停已安装的服务端。资源或代码修改后，先按 `TECHNICAL.md` 重新生成、测试和构建，再打包。
