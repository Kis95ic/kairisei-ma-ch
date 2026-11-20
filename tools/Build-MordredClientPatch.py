"""Stage the existing Mordred CSV patch in Windows downloadable resources.

Requires UnityPy (the local investigation environment is also supported).
Never writes to the installed server or client. Does not change skill semantics.
"""

import argparse
from collections import Counter
import copy
import csv
import gzip
import hashlib
import io
import json
from pathlib import Path
import sys
import zlib

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "_local/modding-python"))
import UnityPy


def sha(data):
    return hashlib.sha256(data).hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def content_id(profile, movie_profile):
    fields = ("name", "bytes", "sha256")
    identity = {
        "platform": profile["platform"],
        "unity_version": profile["unity_version"],
        "catalog_sha256": profile["catalog"]["sha256"],
        "bundles": [{k: row[k] for k in fields} for row in profile["bundles"]],
        "movies": {
            "index_bytes": movie_profile["index"]["bytes"],
            "index_sha256": movie_profile["index"]["sha256"],
            "files": [{k: row[k] for k in fields}
                      for row in sorted(movie_profile["files"], key=lambda r: r["name"])],
        },
    }
    return sha(json.dumps(identity, sort_keys=True, separators=(",", ":")).encode())


def encode_rows(rows, encoding):
    stream = io.StringIO(newline="")
    csv.writer(stream, lineterminator="\n").writerows(rows)
    return stream.getvalue().encode(encoding)


def validate_skill_variants(rows, known_conditions):
    for skill_id, count in Counter(row[0] for row in rows).items():
        require(count <= 5,
                f"Skill {skill_id} has {count} variants; the Windows client supports at most 5")
    for row in rows:
        require(len(row) >= 50, f"Incomplete skill row: {row[0]}")
        for index in (36, 42):
            require(row[index] in known_conditions | {"", "NONE"},
                    f"Skill {row[0]} uses a condition absent from client masters: {row[index]}")


def validate_role_counts(rows):
    for role_id, count in Counter(row[0] for row in rows).items():
        require(count <= 5, f"Role set {role_id} exceeds the client's 5 role slots")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server-dir", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    server, output = args.server_dir.resolve(), args.output.resolve()
    require(not output.exists(), "Output must be a new directory")
    require(not output.is_relative_to(server), "Output must be outside the installation")
    platform = server / "platform/pc"
    profile = json.loads((platform / "delivery.json").read_text(encoding="utf-8-sig"))
    require(profile["platform"] == "StandaloneWindows64", "Expected Windows profile")
    require(profile["artifact_root"] == ".", "Expected the portable package layout")
    movie = json.loads((platform / profile["movies"]["file"]).read_text(encoding="utf-8-sig"))
    require(content_id(profile, movie) == profile["content_id"], "Original content ID mismatch")
    bundle_rows = [r for r in profile["bundles"] if r["name"] == "main_c/container.dat"]
    require(len(bundle_rows) == 1, "Expected one master bundle")
    bundle = bundle_rows[0]
    require(bundle.get("file") == "bundles/main_c/container.dat", "Unexpected bundle transport")
    original = (platform / bundle["file"]).read_bytes()
    require(sha(original) == bundle["sha256"] and len(original) == bundle["bytes"],
            "Original master bundle differs from its manifest")
    require(f"{zlib.crc32(original):08X}" == bundle["crc32"], "Original bundle CRC mismatch")
    catalog_bytes = (platform / profile["catalog"]["file"]).read_bytes()
    require(sha(catalog_bytes) == profile["catalog"]["sha256"], "Original catalog hash mismatch")
    catalog = gzip.decompress(catalog_bytes)

    source = server / "resource-set/_local/control/server"
    table_folders = {
        "card.csv": "cn602-card-master",
        "skill_player.csv": "cn602-battle-master",
        "skill_role_player.csv": "cn602-battle-master",
        "support_skill.csv": "cn602-battle-master",
        "support_skill_role.csv": "cn602-battle-master",
    }
    source_rows = {}
    source_pins = {}
    for name, folder in table_folders.items():
        path = source / folder / name
        raw = path.read_bytes()
        source_pins[str(path.relative_to(server)).replace("\\", "/")] = sha(raw)
        source_rows[name] = list(csv.reader(io.StringIO(raw.decode("utf-8-sig"))))
    card = [r for r in source_rows["card.csv"] if r and r[0] == "10219001"]
    require(len(card) == 1 and card[0][39] == "91021900" and card[0][46] == "91021908",
            "Install the matching server CSV patch first")

    env = UnityPy.load(original)
    before = {o.path_id: sha(o.get_raw_data()) for o in env.objects}
    changed, expected = {}, {}
    new_ids = {str(i) for i in range(91021900, 91021909)}
    for obj in env.objects:
        if obj.type.name != "TextAsset":
            continue
        asset = obj.read()
        name = asset.m_Name
        if name not in table_folders:
            continue
        require(name not in expected, f"Duplicate table {name}")
        raw = asset.m_Script.encode("utf-8", "surrogateescape")
        # Preserve UTF-8 and every unrelated record rather than replacing whole tables.
        encoding = "utf-8"
        decoded = raw.decode(encoding)
        require(decoded.encode(encoding) == raw, f"Encoding mismatch in {name}")
        old_rows = list(csv.reader(io.StringIO(decoded)))
        require(not any(r and r[0] in new_ids for r in old_rows), f"Already patched: {name}")
        if name == "card.csv":
            matches = [r for r in old_rows if r and r[0] == "10219001"]
            require(len(matches) == 1 and matches[0][39] == "11106962"
                    and matches[0][46] == "63000760", "Unexpected original card")
            lines = raw.splitlines(keepends=True)
            indices = [i for i, line in enumerate(lines) if line.startswith(b"10219001,")]
            require(len(indices) == 1, "Card must occupy a single CSV record")
            replacement = matches[0].copy()
            replacement[39], replacement[46] = "91021900", "91021908"
            index = indices[0]
            ending = b"\r\n" if lines[index].endswith(b"\r\n") else b"\n"
            lines[index] = encode_rows([replacement], encoding).rstrip(b"\n") + ending
            updated = b"".join(lines)
        else:
            additions = [r for r in source_rows[name] if r and r[0] in new_ids]
            require(additions, f"No patch rows in {name}")
            if name in {"skill_player.csv", "support_skill.csv"}:
                conditions = {r[i] for r in old_rows if len(r) >= 50 for i in (36, 42)}
                validate_skill_variants(additions, conditions)
            else:
                validate_role_counts(additions)
            updated = raw + (b"\n" if not raw.endswith(b"\n") else b"") + encode_rows(additions, encoding)
        expected[name] = updated
        asset.m_Script = updated.decode("utf-8", "surrogateescape")
        asset.save()
        changed[obj.path_id] = name
    require(len(expected) == 5, "Five target tables must be updated")
    rebuilt = next(iter(env.files.values())).save(packer="original")
    check = UnityPy.load(rebuilt)
    after = {o.path_id: sha(o.get_raw_data()) for o in check.objects}
    require(before.keys() == after.keys(), "Bundle object inventory changed")
    require({i for i in before if before[i] != after[i]} == set(changed),
            "Unrelated bundle objects changed")
    verified = set()
    for obj in check.objects:
        if obj.path_id in changed:
            asset = obj.read()
            require(asset.m_Script.encode("utf-8", "surrogateescape") == expected[asset.m_Name],
                    f"Round-trip failed: {asset.m_Name}")
            verified.add(asset.m_Name)
    require(verified == expected.keys(), "Missing round-trip tables")

    old_profile = copy.deepcopy(profile)
    bundle["bytes"], bundle["sha256"], bundle["crc32"] = len(rebuilt), sha(rebuilt), f"{zlib.crc32(rebuilt):08X}"
    catalog_lines = catalog.splitlines(keepends=True)
    indices = [i for i, line in enumerate(catalog_lines) if line.startswith(b"<b>,main_c/container.dat,")]
    require(len(indices) == 1, "Expected one master catalog entry")
    index = indices[0]
    fields = catalog_lines[index].rstrip(b"\r\n").split(b",")
    fields[2], fields[4] = bundle["crc32"].encode(), str(bundle["bytes"]).encode()
    catalog_lines[index] = b",".join(fields) + b"\n"
    rebuilt_catalog = gzip.compress(b"".join(catalog_lines), mtime=0)
    profile["catalog"]["bytes"], profile["catalog"]["sha256"] = len(rebuilt_catalog), sha(rebuilt_catalog)
    profile["content_id"] = content_id(profile, movie)
    require(profile["content_id"] != old_profile["content_id"], "Content version did not change")

    files = {
        "platform/pc/bundles/main_c/container.dat": rebuilt,
        "platform/pc/catalog.dat": rebuilt_catalog,
        "platform/pc/delivery.json": (json.dumps(profile, ensure_ascii=False, indent=2) + "\n").encode(),
    }
    receipt = {
        "card_id": 10219001,
        "scope": "Windows download resources; preserves the existing server skill definitions",
        "old_content_id": old_profile["content_id"],
        "content_id": profile["content_id"],
        "source_master_sha256": source_pins,
        "changed_text_assets": sorted(expected),
        "unchanged_bundle_objects": len(before) - len(changed),
        "client_schema_compatible": True,
        "known_limit": "Schema and resource checks do not replace in-game display and battle verification.",
        "files": {},
    }
    for relative, data in files.items():
        old = (server / relative).read_bytes()
        target = output / "overlay" / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
        backup = output / "backup" / relative
        backup.parent.mkdir(parents=True, exist_ok=True)
        backup.write_bytes(old)
        require(target.read_bytes() == data and backup.read_bytes() == old, "Written-file verification failed")
        receipt["files"][relative] = {"before_sha256": sha(old), "after_sha256": sha(data), "bytes": len(data)}
    (output / "receipt.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(receipt, indent=2))


if __name__ == "__main__":
    main()
