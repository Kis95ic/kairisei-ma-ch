# 本仓库的分支约定

- 日常开发、魔改、文档和工作流修改统一在 `codex/modded` 分支进行。修改前先检查当前分支。
- `origin` 指向自己的 fork：`https://github.com/Kis95ic/kairisei-ma-ch.git`。
- `upstream` 指向原作者：`https://github.com/kuuhaku1314/kairisei-ma-ch.git`，仅用于拉取，禁止向其推送。
- `main` 专用于同步原作者的 `main`。不要在 `main` 提交魔改，也不要将 `codex/modded` 合并进 `main`。
- GitHub 默认分支使用 `codex/modded`，使该分支中的 `.github/workflows/sync-upstream.yml` 能按计划执行，同时保持 `main` 不包含本仓库的配置提交。
- 自动同步计划为每小时第 17 分钟，也支持在 GitHub Actions 中手动运行。它只更新 GitHub 上的 `main`，不会自动修改本机文件或开发分支。
- GitHub 定时任务可能延迟；公开仓库长期无活动（60 天）时可能被停用。可在 Actions 页面重新启用；同步冲突或权限错误需要人工处理。

## 日常开发

```powershell
git switch codex/modded
git pull --ff-only
# 完成修改并检查后，选择需要提交的文件。
git add <文件路径>
git commit -m "说明本次修改"
git push
```

需要将原作者的新代码纳入魔改分支时，先提交或妥善保存当前工作，再执行：

```powershell
git fetch upstream
git switch codex/modded
git merge upstream/main
# 解决冲突并完成测试后推送。
git push
```

不要自动将上游更新合并到魔改分支。保留现有魔改，按需合并并验证兼容性。
