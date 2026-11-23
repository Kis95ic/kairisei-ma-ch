import importlib
import json
import os
from pathlib import Path
import unittest
from unittest.mock import Mock

patch = importlib.import_module("Build-CardMod2Patch")


class CardMod2SafetyTests(unittest.TestCase):
    def test_reviewed_scope_includes_all_sixty_cards(self):
        self.assertEqual(len(patch.CARD_IDS), 60)
        self.assertIn(10219001, patch.CARD_IDS)

    def test_replace_preserves_other_bytes_and_line_endings(self):
        raw = b"1,a,keep\r\n2,b,old\r\n3,c,keep"
        self.assertEqual(patch.replace_cards(raw, {2: ["2", "b", "new"]}, {2: ["2", "b", "old"]}),
                         b"1,a,keep\r\n2,b,new\r\n3,c,keep")

    def test_changed_or_duplicate_card_fails(self):
        for raw in [b"2,b,changed\n", b"2,b,old\n2,b,old\n", b"3,b,old\n"]:
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                patch.replace_cards(raw, {2: ["2", "b", "new"]}, {2: ["2", "b", "old"]})

    def test_growth_all_levels_and_multipliers(self):
        for target in (329803,157560,10180,6080,41775,1000000,240000,31007,62015,93023,930231,30843,20052,310077,10010,51419,50000,25000,27904,100000,102838,20854,620114,1240308):
            for multiplier in (1,2,3,5):
                values=[patch.level_value(target,level,multiplier) for level in range(1,100)]
                self.assertEqual(values[-1],target*multiplier)
                self.assertEqual(values,sorted(values))
                self.assertGreaterEqual(values[0],target*multiplier//2)
                for level,value in enumerate(values,1):
                    self.assertEqual(value,(target-target//196*(99-level))*multiplier)

    def test_complete_table_and_call_descriptions(self):
        path=Path(__file__).resolve().parents[1]/"docs/CARD_MODDING_2.md"
        self.assertEqual(patch.sha(path.read_bytes()),patch.SPEC_SHA256)
        spec=path.read_text(encoding="utf-8-sig")
        for family, needles in [(1,("157560","787800")),(2,("41775","83550","125325")),(4,("31007","62015","93023")),(11,("620114","1240308","102838"))]:
            active,_,_=patch.descriptions(spec,family,"火","风")
            for needle in needles:self.assertIn(needle,active)
        for family in (1,3,5,9,10):
            _,_,call=patch.descriptions(spec,family,"火","风")
            self.assertTrue(call)
            self.assertNotIn("效果槽",call)

    def test_native_limits_and_unknown_conditions(self):
        skill = [""] * 51
        skill[0], skill[5], skill[49] = "92000000", "1", "92000000"
        role = [""] * 33
        role[0], role[8] = "92000000", "ATTACK_AA"
        patch.validate_programs([skill] * 5, [role] * 5, set(), {"ATTACK_AA"})
        for skills, roles in [([skill] * 6, [role]), ([skill], [role] * 6)]:
            with self.assertRaises(ValueError):
                patch.validate_programs(skills, roles, set(), {"ATTACK_AA"})
        skill[36] = "CUSTOM_CONDITION"
        with self.assertRaisesRegex(ValueError, "Unknown client condition"):
            patch.validate_programs([skill], [role], set(), {"ATTACK_AA"})


@unittest.skipUnless(os.environ.get("CN602_CARD_MOD2_FINAL_SET"), "set CN602_CARD_MOD2_FINAL_SET for actual generated masters")
class GeneratedPatchTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(os.environ["CN602_CARD_MOD2_FINAL_SET"])
        self.report = json.loads((self.root / "patch.json").read_text(encoding="utf-8"))

    def test_all_old_skill_bytes_and_inventory_hashes_preserved(self):
        manifest = json.loads((self.root / "resource-set.json").read_text(encoding="utf-8"))
        inventory = {r["path"]: r for r in manifest["files"]}
        for relative, entry in self.report["files"].items():
            before = (self.root / "backup" / relative).read_bytes()
            after = (self.root / relative).read_bytes()
            self.assertEqual(patch.sha(before), entry["before_sha256"])
            self.assertEqual(patch.sha(after), entry["after_sha256"])
            self.assertEqual(inventory[relative]["sha256"], patch.sha(after))
            self.assertEqual(inventory[relative]["bytes"], len(after))
            if Path(relative).name != "card.csv":
                self.assertTrue(after.startswith(before), relative)

    def test_only_approved_cards_and_fields_changed(self):
        relative = "_local/control/server/cn602-card-master/card.csv"
        before = (self.root / "backup" / relative).read_bytes()
        after = (self.root / relative).read_bytes()
        old = {int(r[0]): r for r in patch.read_rows(before) if r and r[0].isdigit()}
        new = {int(r[0]): r for r in patch.read_rows(after) if r and r[0].isdigit()}
        self.assertEqual(old.keys(), new.keys())
        changed = {key for key in old if old[key] != new[key]}
        self.assertEqual(changed, set(patch.CARD_IDS))
        for card in self.report["cards"]:
            card_id=card["card_id"];family=card["family"]
            expected={39,46}
            if card["call_id"]:expected.add(44)
            if family in (1,2,4,5,7,8,10,11):
                self.assertEqual(new[card_id][68],"20020010")
                if old[card_id][68] != new[card_id][68]:expected.add(68)
            self.assertEqual({i for i in range(len(old[card_id])) if old[card_id][i]!=new[card_id][i]},expected)
        self.assertEqual(patch.replace_cards(after, {key: old[key] for key in changed},
                                           {key: new[key] for key in changed}), before)
        self.assertEqual(len(self.report["deferred_card_ids"]), 0)
        self.assertNotEqual(old[10219001], new[10219001])

    def test_original_attack_chain_coefficients_preserved(self):
        base = self.root / "_local/control/server/cn602-battle-master"
        old_skills = patch.read_rows((self.root / "backup/_local/control/server/cn602-battle-master/skill_player.csv").read_bytes())
        roles = patch.read_rows((base / "skill_role_player.csv").read_bytes())
        old_cards=patch.read_rows((self.root / "backup/_local/control/server/cn602-card-master/card.csv").read_bytes())
        for item in self.report["cards"]:
            if not (10219001 <= item["card_id"] <= 10219005 or 10219031 <= item["card_id"] <= 10219035):
                continue
            old_id = patch.unique_row(old_cards,item["card_id"])[39]
            old_role_id = next(r[49] for r in old_skills if r[0] == old_id)
            old_attack = next(r for r in roles if r[0] == old_role_id and r[8] == "ATTACK_AA")
            for variant in range(4):
                attack = next(r for r in roles if r[0] == str(item["skill_id"] + variant) and r[8] == "ATTACK_AA")
                self.assertEqual(attack[30], old_attack[30])
                self.assertEqual(attack[26], old_attack[26])


if __name__ == "__main__":
    unittest.main()
