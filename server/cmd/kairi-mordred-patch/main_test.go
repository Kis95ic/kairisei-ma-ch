package main

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"testing"

	"kairisei.local/server/battlecontent"
)

func TestPatchCardPreservesOtherRowsAndRejectsDrift(t *testing.T) {
	row := make([]string, 120)
	row[0], row[38], row[39], row[45], row[46] = cardID, "11106961", sourceSkillID, "63000036", sourcePassiveID
	target := encodeRows([][]string{row})
	raw := append([]byte("1,unchanged\r\n"), target...)
	raw = append(raw, []byte("2,unchanged\r\n")...)
	before := append([]byte(nil), raw...)
	patched, err := patchCard(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(patched, []byte("1,unchanged\r\n")) || !bytes.HasSuffix(patched, []byte("2,unchanged\r\n")) || !bytes.Equal(raw, before) {
		t.Fatal("patch changed surrounding rows or input bytes")
	}
	card, err := findRow(patched, cardID, -1, "")
	if err != nil || card[38] != row[38] || card[39] != strconv.Itoa(patchSkillID) || card[45] != row[45] || card[46] != strconv.Itoa(patchPassiveID) {
		t.Fatal("wrong card skill references")
	}
	if _, err := patchCard(patched); err == nil {
		t.Fatal("accepted a changed skill reference")
	}
	if _, err := patchCard(append(before, target...)); err == nil {
		t.Fatal("accepted duplicate cards")
	}
	if _, err := patchCard([]byte("1,other\n")); err == nil {
		t.Fatal("accepted missing card")
	}
	row[46] = "other-passive"
	if _, err := patchCard(encodeRows([][]string{row})); err == nil {
		t.Fatal("accepted a changed passive reference")
	}
}

func TestPassiveRowsRemoveDrawAndFilterOriginalCosts(t *testing.T) {
	base, boost := make([]string, 51), make([]string, 33)
	base[0], base[9], base[12], base[19], base[49] = sourcePassiveID, "PASSIVE", "MERCENARY", "SELF", sourcePassiveID
	boost[0], boost[8], boost[9], boost[20], boost[21], boost[25], boost[26] = sourcePassiveID, "DAMAGE_BOOST", "SELF", "99", "120", "FIRE", "PHYSICS"
	skillRow, roleRows := passiveRows(base, boost)
	skill, err := battlecontent.ParseSkill(skillRow)
	if err != nil {
		t.Fatal(err)
	}
	if skill.ID != patchPassiveID || skill.FunctionID != patchPassiveID || skill.Type != "PASSIVE" || skill.ArthurJob != "MERCENARY" || skill.Target != "SELF" || skillRow[3] != passiveDescription || strings.Contains(skillRow[3], "初始抽牌") {
		t.Fatalf("wrong passive: %+v", skill)
	}
	if len(roleRows) != 2 || base[0] != sourcePassiveID || boost[26] != "PHYSICS" {
		t.Fatal("wrong roles or changed source")
	}
	for i, row := range roleRows {
		role, err := battlecontent.ParseSkillRole(row)
		if err != nil {
			t.Fatal(err)
		}
		wantFunction := []string{"DAMAGE_BOOST", "NEED_COST_DOWN"}[i]
		wantParameters := [][10]string{{"99", "120", "0", "0", "0", "FIRE", "ALL", "0", "0", "0"}, {"99", "2", "ALL", "NULL", "0", "0", "7", "0", "0", "0"}}[i]
		if role.SkillID != patchPassiveID || role.Target != "SELF" || role.Function != wantFunction || role.Parameters != wantParameters {
			t.Fatalf("wrong passive role: %+v", role)
		}
	}
}

func TestSkillRowsCoverEveryCombination(t *testing.T) {
	base, attack := make([]string, 51), make([]string, 33)
	base[0], base[1], base[10], base[11], base[12], base[13], base[14], base[19], base[49] = sourceSkillID, "绝技／魔剑终刻", "ATTACK", "FIRE", "MERCENARY", "PHYSICS", "3", "ENEMY_ALL", sourceSkillID
	base[36], base[37] = "TURN", "3"
	attack[30] = "20"
	skills, roles := skillRows(base, attack)
	if len(skills) != 4 || base[14] != "3" || base[36] != "TURN" || attack[20] != "" {
		t.Fatal("variant count or source preservation")
	}
	for mask, row := range skills {
		skill, err := battlecontent.ParseSkill(row)
		if err != nil {
			t.Fatal(err)
		}
		if skill.ID != patchSkillID || skill.Cost != 9 || skill.Target != "ENEMY_ALL" || skill.BranchPriority != mask || skill.FunctionID != patchSkillID+mask {
			t.Fatalf("bad header: %+v", skill)
		}
		var selected [][]string
		for _, role := range roles {
			if role[0] == strconv.Itoa(skill.FunctionID) {
				selected = append(selected, role)
			}
		}
		hits, piercing, magic, gauge, attacks := "4", 0, 0, 0, 0
		if mask&2 != 0 {
			hits = "7"
		}
		for i, role := range selected {
			if _, err := battlecontent.ParseSkillRole(role); err != nil {
				t.Fatal(err)
			}
			switch role[8] {
			case "ATK_OP_PIERCING":
				piercing++
				if role[20] != "100" {
					t.Fatal("wrong piercing")
				}
			case "ATK_OP_DAMAGE_INCREASE":
				magic++
				if role[22] != "10000" || role[24] != "INT" {
					t.Fatal("wrong magic contribution")
				}
			case "BURST_GAUGE_QUICK_UP":
				gauge++
				if role[20] != "50" || role[9] != "SELF" {
					t.Fatal("wrong gauge")
				}
			case "ATTACK_AA":
				attacks++
				if role[20] != "329803" || role[21] != "0" || role[22] != "10000" || role[24] != hits || skill.DisplayRole != i+1 {
					t.Fatal("wrong attack or preview role")
				}
			}
		}
		if len(selected) > 5 || piercing != 1 || magic != mask&1 || gauge != 1 || attacks != 1 {
			t.Fatalf("mask %d has wrong effects", mask)
		}
	}
	if skills[3][36] != "SELF_PLAY_COST_TOTAL" || skills[3][42] != "FRIEND_PLAY_ATTR_NUM" || strings.Contains(description, "4chain") {
		t.Fatal("dual condition or unconditional piercing is incorrect")
	}
	encoded := appendRows([]byte("# existing\n"), skills)
	reader := csv.NewReader(strings.NewReader(string(encoded)))
	reader.FieldsPerRecord = -1
	parsed, err := reader.ReadAll()
	if err != nil || len(parsed) != 5 {
		t.Fatalf("CSV output: %v", err)
	}
}
