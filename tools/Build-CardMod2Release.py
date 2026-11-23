"""Package the reviewed 60-card release without touching installed games or saves."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import subprocess
import zipfile

def require(ok,message):
    if not ok: raise ValueError(message)

def sha(raw): return hashlib.sha256(raw).hexdigest()
def file_sha(path):
    h=hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda:stream.read(1024*1024),b""): h.update(block)
    return h.hexdigest()
def encoded(obj):return (json.dumps(obj,ensure_ascii=False,indent=2)+"\n").encode("utf-8")
def safe_name(name):
    p=PurePosixPath(name)
    require(not p.is_absolute() and ".." not in p.parts and "\\" not in name and ":" not in name,f"Unsafe ZIP path {name}")
def read_json(path):return json.loads(path.read_text(encoding="utf-8-sig"))

def build(args):
    repo=Path(__file__).resolve().parents[1]
    output=args.output.resolve()
    require(not output.exists(),"Output already exists; choose a fresh directory")
    require(output.is_relative_to(repo),"Release output must stay inside this workspace")
    git=lambda *a:subprocess.check_output(["git",*a],cwd=repo)
    require(git("branch","--show-current").strip()==b"codex/modded","Expected codex/modded")
    base=git("rev-parse","main").decode().strip()
    generated=args.generated.resolve()
    master=read_json(generated/"resource-set/patch.json")
    require(master["schema"]==2 and len(master["cards"])==60 and not master["deferred_card_ids"],"Incomplete master patch")
    spec=repo/"docs/CARD_MODDING_2.md"
    require(file_sha(spec)==master["spec_sha256"],"Final spec differs from generated patch")
    overrides={}
    for relative,entry in master["files"].items():
        path=generated/"resource-set"/relative
        require(file_sha(path)==entry["after_sha256"],f"Master checksum {relative}")
        overrides["resource-set/"+relative]=path
    overrides["resource-set/resource-set.json"]=generated/"resource-set/resource-set.json"
    platform_receipts={}
    for platform in ("pc","android"):
        receipt=read_json(generated/platform/"receipt.json")
        require(receipt["spec_sha256"]==master["spec_sha256"] and len(receipt["card_ids"])==60,"Platform spec mismatch")
        platform_receipts[platform]=receipt
        for relative,entry in receipt["files"].items():
            path=generated/platform/"overlay"/relative
            require(file_sha(path)==entry["after_sha256"],f"Platform checksum {relative}")
            overrides[relative]=path
    for name in ("kairi-server.exe","kairi-server-linux-amd64","kairi-server-linux-arm64"):
        path=repo/"_local/bin"/name
        require(path.is_file(),f"Missing built server {name}")
        overrides[name]=path

    with zipfile.ZipFile(args.server_update) as original:
        names=original.namelist()
        require(len(names)==len(set(n.casefold() for n in names)),"Duplicate original ZIP entries")
        for name in names:
            safe_name(name)
            require(name not in ("deployment.json","cdn.json") and not name.startswith(("_local/data/","client-data/")),"Author update contains mutable state")
        release= json.loads(original.read("release-manifest.json"))
        require(release["server_version"]=="2.0.4" and release["client_version"]=="2.0.4","Expected author 2.0.4 baseline")
        inventory={r["path"]:r for r in release["files"]}
        baseline=read_json(generated/"resource-set/backup/resource-set.json")
        require(all(inventory.get("resource-set/"+r["path"],{}).get("sha256")==r["sha256"] for r in baseline["files"]),"Author release and master baseline differ")
        output.mkdir(parents=True)
        report={"schema":1,"name":"CardMod2 final 60-card release","base_main":base,
                "source_head":git("rev-parse","HEAD").decode().strip(),"spec_sha256":master["spec_sha256"],
                "server_version":"2.0.4","client_version":"2.0.4","card_ids":sorted(r["card_id"] for r in master["cards"]),
                "client_binaries":"Unmodified author 2.0.4 Windows and signed Android; card changes are delivered by the paired server",
                "platform_content_ids":{p:r["content_id"] for p,r in platform_receipts.items()},
                "device_battle_tested":False,"artifacts":{}}
        for relative,path in overrides.items():
            inventory[relative]={"path":relative,"bytes":path.stat().st_size,"sha256":file_sha(path)}
        release["files"]=sorted(inventory.values(),key=lambda r:r["path"])
        release["resource_set_sha256"]=file_sha(overrides["resource-set/resource-set.json"])
        payload={name:original.read(name) for name in names if not name.endswith("/") and name not in overrides and name!="release-manifest.json"}
        for name,data in payload.items():
            require(name in inventory and sha(data)==inventory[name]["sha256"],f"Original update file not pinned: {name}")
        payload.update({name:path.read_bytes() for name,path in overrides.items()})
        payload["release-manifest.json"]=encoded(release)
        server_zip=output/"KairiseiMA-Server-Update.zip"
        with zipfile.ZipFile(server_zip,"w",compression=zipfile.ZIP_DEFLATED,compresslevel=6) as out:
            for name,data in sorted(payload.items()):out.writestr(name,data)
        with zipfile.ZipFile(server_zip) as check:
            require(set(check.namelist())==set(payload),"ZIP inventory changed")
            for name in check.namelist():
                require(sha(check.read(name))==sha(payload[name]),f"ZIP verification {name}")
        report["server_zip_entries"]=len(payload)
    del payload
    # Preserve the signed originals. No private installation or player data is read.
    for source,name in ((args.windows,"KairiseiMA-Windows.zip"),(args.android,"KairiseiMA-Android.apk")):
        with zipfile.ZipFile(source) as archive:
            require(archive.testzip() is None,f"Invalid original archive {source}")
            for entry in archive.namelist():safe_name(entry)
            if name.endswith(".zip"):
                require(json.loads(archive.read("client-data/client-storage.json"))=={"format":1,"application_id":"local.kairisei.clientrebuild.unity6"},"Client contains non-template storage")
                require(archive.read("client-data/data/local_server.txt").strip()==b"127.0.0.1:26040","Client contains non-template address")
        shutil.copyfile(source,output/name)
        require(file_sha(source)==file_sha(output/name),"Original client copy changed")

    # Include tracked changes and newly added source files without touching Git staging.
    diff=git("diff","--binary","main","--",".")
    untracked=git("ls-files","--others","--exclude-standard","-z").decode().split("\0")
    for name in filter(None,untracked):
        require(name.startswith(("tools/","server/","docs/")),"Unexpected untracked file; review before packaging: "+name)
        result=subprocess.run(["git","diff","--no-index","--binary","--","/dev/null",name],cwd=repo,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        require(result.returncode==1,"Unable to export new source: "+name)
        diff+=result.stdout
    patch=output/"CardMod2-from-main.patch"
    patch.write_bytes(diff)
    index=generated/"release-check.index"
    require(not index.exists(),"Patch validation index already exists")
    env=dict(os.environ,GIT_INDEX_FILE=str(index))
    subprocess.run(["git","read-tree","main"],cwd=repo,env=env,check=True)
    subprocess.run(["git","apply","--cached","--check",str(patch)],cwd=repo,env=env,check=True)
    shutil.copyfile(spec,output/spec.name)
    evidence=output/"verification"
    evidence.mkdir()
    for name in ("python-tests.log","go-test.log","go-card-final.log","go-vet.log","go-full-input.log","build-server.log","build-linux.log","apk-signature.log"):
        if (generated/name).is_file():shutil.copyfile(generated/name,evidence/name)
    for platform in ("pc","android"):
        (evidence/(platform+"-receipt.json")).write_bytes(encoded(platform_receipts[platform]))
    (evidence/"master-receipt.json").write_bytes(encoded(master))
    note=f"""# 卡牌魔改2最终配套包

本包实装最终MD的12组60张卡，火属性莫德雷德也包含在内。基于本地main提交 {base}（作者2.0.4），源码差异含codex/modded相对main的既有修改及本次新增文件。

## 文件
- KairiseiMA-Server-Update.zip：作者2.0.4累计更新内容，加上本次60张卡的服务端主表、PC与Android各自的热更新资源、重新编译的Windows/Linux amd64/Linux arm64服务端。
- KairiseiMA-Windows.zip：作者2.0.4完整Windows客户端，程序与原包一致。
- KairiseiMA-Android.apk：作者2.0.4原签名完整APK，文件与原包一致。
- CARD_MODDING_2.md：唯一最终技能说明，含等级公式和槽数。
- CardMod2-from-main.patch：相对上述main的源码补丁，已通过git apply --cached --check。
- SHA256SUMS.txt、release.json、verification：文件校验与构建验证记录。

## 更新
1. 先备份服务端存档和部署配置，停止服务端。
2. 在已有作者2.0.0完整服务端（或其后2.0.x版本）根目录解压覆盖服务端更新包。本包合入作者2.0.4累计更新；不能当作首次安装的完整资源包。包内不含deployment.json、cdn.json、玩家存档或数据库。
3. 启动服务端，确认客户端连接的是这个服务端。两平台delivery的content_id均已改变，客户端重新登录时应重新取得对应目录与主数据资源。远程CDN部署需要同步本次platform资源及目录，避免仍命中旧CDN数据。
4. Windows更新：保留原client-data整个文件夹，再替换其余程序文件。ZIP内client-data仅为作者空白初始模板，已有用户不要用模板覆盖自己的数据。
5. Android更新：APK保留作者签名，可覆盖同签名安装；已经安装作者2.0.4时无须重装。手机应连接可访问的服务端IP，不能使用电脑的127.0.0.1。
6. 本次魔改由配套服务端下发；只替换Windows程序或APK、仍连接未更新服务端，不会获得新技能。

## 已确认的规则
- 独立被动HP上限增益为全等级固定12000。
- 具有HP上限增益的绝技／祝福，99级最高档基础增益为102838。
- 欧文奋起：满足主卡组条件时开局赋予全队，整场战斗各可发动1次，恢复各自30%HP；没有9回合到期。
- 主体、祝福、衍生卡分别遵守5分支／每分支5槽／最多2直接条件；独立被动最多2档。
- 攻击与降抗使用卡牌属性；防护属性按最终MD的火→风、冰→火、风→冰、光→暗、暗→光对应。

## 验证范围
完成原生CSV结构与保留性检查、1–99级成长公式检查、60张卡绝技分支和独立被动战斗执行、祝福触发与到期、衍生卡、各目标一次削热、服务端测试和静态检查、平台资源重新读取、APK签名及发布ZIP逐文件校验。具体构建日志见verification。
尚未完成Windows客户端界面和Android真机战斗验收。保留原客户端程序不代表本次60张卡已经过真机实战验证。

## 回退
停服后恢复更新前备份的服务端主表、resource-set.json、两平台资源与delivery/catalog、release-manifest.json及服务端程序。客户端重新登录取得旧版资源。不要删除玩家存档或client-data。
"""
    (output/"安装说明.md").write_text(note,encoding="utf-8")
    for path in sorted(output.rglob("*")):
        if path.is_file():
            report["artifacts"][path.relative_to(output).as_posix()]={"bytes":path.stat().st_size,"sha256":file_sha(path)}
    (output/"release.json").write_bytes(encoded(report))
    entries=sorted(p for p in output.rglob("*") if p.is_file())
    (output/"SHA256SUMS.txt").write_text("".join(file_sha(p)+"  "+p.relative_to(output).as_posix()+"\n" for p in entries),encoding="utf-8")
    print(json.dumps({"output":str(output),"base_main":base,"server_entries":report["server_zip_entries"],"packages":[{"name":p.name,"bytes":p.stat().st_size,"sha256":file_sha(p)} for p in entries if p.suffix in (".zip",".apk",".patch")]},ensure_ascii=False,indent=2))

if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__)
    for name in ("server-update","windows","android","generated","output"):p.add_argument("--"+name,type=Path,required=True)
    build(p.parse_args())
