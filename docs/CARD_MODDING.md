# 本机卡牌技能修改：圆桌型莫德雷德 9C

> 2026-11-23 状态说明：当前源码和已安装补丁已改为四分支，用户确认10219001正常。本页下文的八分支故障、待适配说明为历史记录，不代表当前状态。该卡第二批的最终技能目标见 [第二批最终技能版本](CARD_MODDING_2.md)，文档目标不代表已安装补丁内容。

> 2026-11-19 更新：当前 8 分支补丁不兼容发布版 Windows 客户端，不能作为可玩补丁安装。仅更新服务端 CSV 不会更新 `platform/pc` 下载包；将现有 8 分支直接打入下载包，又会导致客户端解析技能时数组越界。资源可以从服务端热更新，但必须先满足客户端的 5 分支结构及现有条件能力，不能把原生 DLL 加载成功当作客户端界面验证。

## 2026-11-19 Windows 错误与恢复

- 已确认新资源成功下载：客户端与新服务端主表 SHA-256 均为 `146a50a87702501e92ca64b6722da41ba5f863fe2d2912e8128a933c0eb347cd`。
- 客户端日志反复出现 `IndexOutOfRangeException`，栈顶为 `SkillDataContainer.onCsvParserWithId`，随后经过 `getSkillCsvData`、`CardUtility.GetArthurType`、`DeckUtility.CalcDeckRank`。原生技能结构只有 5 个 extend 槽，而新技能 `91021900` 有 8 行分支；原表每个技能最多 5 行。
- 新增条件 `SELF_PLAY_COST_TOTAL_AND_CHAIN` 也未在发布版客户端条件集合中确认支持；仅把分支截成 5 行会丢失要求的组合，不能作为修复。
- 已恢复 `E:\Game\ma-server\platform\pc` 的 `container.dat`、`catalog.dat`、`delivery.json`，以及 `E:\Game\KairiseiMA-Windows` 下载缓存中的同一主表。恢复后的主表 SHA-256 为 `f83ce0f6ff260595c9e6d01fbf27a11c71e485e90fa997ec2e2085237d8ddf06`。没有恢复玩家数据库或撤销玩家测试期间的操作。
- 恢复后已实际进入主页、卡牌菜单、卡组编辑和变身列表，未再出现标题错误；`_local/mordred-pc-resource-patch/client-recovery.log` 中没有 `IndexOutOfRangeException`、`SkillDataContainer` 异常或其他 `Exception`。日志仍有 Live2D 渲染警告，不能据此声称整个客户端没有任何错误。正常登录下载检查后，客户端主表仍与恢复的服务端主表哈希一致。
- 上述验证只证明登录和卡组页面已恢复。本次没有验证恢复后的战斗；服务端 CSV 仍是原来的完整补丁，客户端则恢复为旧技能资源，两端尚未统一，不能把当前状态当作已完成的 9C 改卡版本。
- `_local/mordred-pc-resource-patch` 保留失败产物、原始资源备份和日志用于诊断，**不要安装其中的 overlay**。`Build-MordredClientPatch.py` 已增加分支数量、效果数量与条件兼容检查，安装脚本也拒绝没有通过该检查的旧产物。
- 当前后端技能定义仍保留原来的完整效果，但完整的客户端适配尚未完成。下一版必须保留全部条件、通过客户端解析与卡组页面验证，再做单人战斗验证。

此修改只针对卡牌 `10219001` 的佣兵绝技与佣兵被动。普通技能、其他职业使用的普通被动和共用旧绝技的卡牌 `11001050` 保持原样。卡牌 ID 不变；新绝技 ID 为 `91021900`，效果组使用 `91021900`–`91021907`；新佣兵被动及效果组 ID 为 `91021908`。原佣兵被动 `63000760` 的定义保留，只有本卡改用新被动。

## 实际效果

- 9 费，火属性物理全体攻击；每个目标 4 段。
- 每段基础威力为 `329803 + 物理攻击力 × 10`，基础值不随卡牌等级变化。
- 每次释放自身圣剑槽增加 50 点，沿用原有槽上限和状态规则。
- 4chain 时无视 100% 正物理防御；其他减伤、属性耐性等仍按原规则计算。
- 自身本回合选用卡牌的原始费用合计至少 15 时，每段另加 `魔法攻击力 × 10`。
- 全队本回合选用的火属性卡牌至少 10 张时，4 段变为 7 段。双属性含火算一张；不统计未选用的卡组卡牌和 Sphere 技能。
- 两种统计都包含本卡；三个条件可以同时生效。原本第 3 回合的强化和当前 HP 叠加已移除。

上式是进入伤害结算前的基础威力，最终伤害仍经过原有连锁、暴击、属性、增减伤和上限规则。保留原绝技的连锁威力系数与暴击参数。

## 佣兵被动

- 移除原佣兵被动的初始抽牌保证。
- 自身火属性伤害提升 12%，同时覆盖物理与魔法伤害，不提升其他属性伤害。
- 自身原始费用至少 7C 的卡牌费用降低 2C，不限属性和技能种类；7C→5C、8C→6C、9C→7C。其他队员不受影响。
- 因此本卡绝技表中仍为 9C，佣兵被动生效后实际消耗 7C。绝技“自身使用卡牌费用合计至少 15”的条件仍统计原始费用，例如 9C+6C 满足条件，实际消耗合计为 13C。
- 使用已有 `DAMAGE_BOOST` 与 `NEED_COST_DOWN` 常驻效果，不新增战斗逻辑。其他职业使用本卡时仍采用原普通被动 `63000036`（初始抽牌）。

## 生成独立测试补丁

在仓库的 `server` 目录执行（需要本机 Go 环境）：

```powershell
go run ./cmd/kairi-mordred-patch `
  -resource-set 'E:\Game\ma-server\resource-set\resource-set.json' `
  -output '..\_local\mordred-9c\resource-set'
```

输出目录必须尚不存在，且不能位于原资源集内部。工具只读取原安装目录，复制战斗与卡牌 master 到输出目录，保存五个变更文件（`card.csv`、`skill_player.csv`、`skill_role_player.csv`、`support_skill.csv`、`support_skill_role.csv`）及原清单的备份，并更新输出清单的大小和哈希。`patch.json` 记录变更前后的 SHA-256。不会读取或改写玩家数据库。

输出是**后端测试输入和资源覆盖补丁**，不是包含媒体资源、平台资源和玩家存档的完整服务端目录。不要把它当作完整安装包直接启动。

## 验证与构建

在 `server` 目录执行：

```powershell
$env:CN602_MORDRED_PATCH_SET = (Resolve-Path '..\_local\mordred-9c\resource-set').Path
go test ./...
go vet ./...
go run ./cmd/kairi-battle-audit `
  -cn-combat-card-master '..\_local\mordred-9c\resource-set\_local\control\server\cn602-card-master\card.csv' `
  -cn-combat-master-root '..\_local\mordred-9c\resource-set\_local\control\server\cn602-battle-master'
$env:CGO_ENABLED = '0'
go build -buildvcs=false -trimpath '-ldflags=-s -w' `
  -o '..\_local\mordred-9c\kairi-server.exe' ./cmd/kairi-server
```

自动化验证包括新被动生效后的全部 8 种绝技条件组合、费用 14/15 和火卡 9/10 边界、每个敌人的 4/7 次实际伤害、单次圣剑槽增加、伤害预览、结果编码及旧卡隔离。被动检查包含初始抽牌移除、火属性物理/魔法增伤、其他属性不增伤、6/7/8/9/10C 减费边界、跨属性和技能种类、仅自身、重复查询不重复扣费、原始费用统计、常驻保留及其他职业普通被动保留。主表审计使用项目现有的静态契约和内存战斗模拟。

## 打包与安装

在仓库根目录运行 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\tools\powershell\Package-MordredPatch.ps1`，将已验证的产物打包为 `_local/mordred-9c/mordred-10219001-windows-amd64-时间戳.zip`。ZIP 只包含需要覆盖的 7 个文件、说明和校验记录，不包含玩家存档或其他资源。归档中的每个文件都会再次读取并校验哈希。停服、版本检查、备份、覆盖与回滚步骤见同目录的 `MORDRED_PATCH_INSTALL.md`，也会作为包内 `README.md` 提供。

## 服务端扩展与客户端限制

原技能格式每个分支只有两个条件。为表达三个条件同时生效，新增仅由后端识别的 `SELF_PLAY_COST_TOTAL_AND_CHAIN`：参数 0/1 为自身原始费用总和的上下界，参数 2/3 为连锁数上下界，参数 4 未使用；上界 0 仍表示不限。它复用原有费用与连锁判断。其余分支使用已有条件与效果。

### 单人模式现场排查（2026-11-18）

- 已核对本机服务端 exe、五个 CSV、清单全部与补丁哈希一致，启动记录也记录了补丁 exe 的哈希；不是启动器选择错误。
- 已只读核对测试卡组，实际携带的是 `10219001`，不是同名同图的 `11001050`。
- 测试请求使用 `/TeamBattleSoloStart`，服务端响应 `is_solo_online: 0`；`server/internal/httpapi/battle.go` 说明单人战斗的完整原生回放引擎未在该 HTTP 结算路径中重放。客户端日志包含 `BattleSoloFunction.SetupBattlePluginNative`。
- 发布版客户端的主表位置：`E:\Game\ma-client\client-data\persistent\patch\main_c\container.dat`。资源映射将本次涉及的五个 CSV 均标为该包内的 TextAsset。
- 发布版客户端的战斗库位置：`E:\Game\ma-client\ClientRebuild6_Data\Plugins\x86_64\kairisei_battlecore.dll`。其 Go 构建信息为 `kairisei.local/server/clientrebuildbridge/cmd/battlecore-library`，依赖 `kairisei.local/server`，使用 `c-shared`；桥接模块未包含在本仓库中。
- 客户端元数据包含原有费用、火卡数量及 Chain 条件，但没有本补丁新增的 `SELF_PLAY_COST_TOTAL_AND_CHAIN`。不能直接把当前后端 CSV 重打包到客户端并宣称可用。

目前只有发布版客户端，尚未取得 `clientrebuildbridge` 源码，也未完成原生模块与主表的配套修改。下一步需要取得该桥接模块及其构建方式，或重新验证一种完全使用客户端现有能力、且严格保留所有条件的实现；不得静默删减三个条件。修改发布内容时还需同步平台资源清单、校验和缓存更新策略，避免登录下载覆盖本地修改。

PC/Android 客户端使用独立的主表与资源包。本工具**不重建客户端主表、技能说明或特效资源**；新技能 ID、分支及该组合条件尚未完成客户端兼容性验证，不能仅凭后端测试通过就视为游戏内可用。需先配套处理客户端，再在独立完整安装副本中验证显示、选卡、消耗、伤害与动画。

测试安装时正常停服，备份测试副本原 exe 和整个 `_local/data/`，只覆盖补丁记录的五个 CSV、`resource-set.json` 和配套重编译 exe。回滚需恢复原 exe 与 `backup` 中的对应文件；若测试存档已变化，使用停服备份恢复。不要在运行中的正式安装目录覆盖文件。
