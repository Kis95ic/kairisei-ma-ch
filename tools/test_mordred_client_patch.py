import importlib
import unittest

patch = importlib.import_module("Build-MordredClientPatch")


class ClientSchemaTests(unittest.TestCase):
    def rows(self, count, condition="SELF_PLAY_COST_TOTAL"):
        rows = []
        for _ in range(count):
            row = [""] * 51
            row[0], row[36] = "91021900", condition
            rows.append(row)
        return rows

    def test_existing_five_slots(self):
        patch.validate_skill_variants(self.rows(5), {"SELF_PLAY_COST_TOTAL"})
        patch.validate_role_counts(self.rows(5))

    def test_sixth_and_eighth_variants_are_rejected(self):
        for count in (6, 8):
            with self.subTest(count=count), self.assertRaisesRegex(ValueError, "at most 5"):
                patch.validate_skill_variants(self.rows(count), {"SELF_PLAY_COST_TOTAL"})

    def test_custom_condition_is_rejected_even_within_slot_limit(self):
        with self.assertRaisesRegex(ValueError, "absent from client"):
            patch.validate_skill_variants(self.rows(1, "SELF_PLAY_COST_TOTAL_AND_CHAIN"), {"SELF_PLAY_COST_TOTAL"})

    def test_sixth_role_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "5 role slots"):
            patch.validate_role_counts(self.rows(6))


if __name__ == "__main__":
    unittest.main()
