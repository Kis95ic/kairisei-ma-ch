// kairi-mordred-patch prepares an isolated server-data patch for card 10219001.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"kairisei.local/server/battlecontent"
)

const (
	cardID             = "10219001"
	sourceSkillID      = "11106962"
	sourcePassiveID    = "63000760"
	patchSkillID       = 91021900
	patchPassiveID     = 91021908
	description        = "敌全体/物理剑/329803点火属性伤害<br>以物理攻击力1000%的4次攻击<br>无视物理防御<br>自身/圣剑解放增加50%<br>【自身/使用卡牌原始费用合计15以上】<br>魔法攻击力1000%物理攻击威力叠加<br>【全队使用火属性卡牌合计10以上】<br>攻击次数+3"
	passiveDescription = "《常驻发动》追加EX技能:<br>自身/提升12%火属性伤害<br>自身/原始费用7C及以上卡牌费用降低2C"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("kairi-mordred-patch", flag.ContinueOnError)
	manifestPath := flags.String("resource-set", "", "original resource-set.json (read only)")
	output := flags.String("output", "", "new patch directory; must not exist")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *manifestPath == "" || *output == "" {
		return errors.New("-resource-set and -output are required")
	}
	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	var manifest map[string]json.RawMessage
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	var entries map[string]string
	if err = json.Unmarshal(manifest["entrypoints"], &entries); err != nil {
		return err
	}
	cardName, battleName := entries["cn-combat-card-master"], entries["cn-combat-master-root"]
	if !filepath.IsLocal(cardName) || !filepath.IsLocal(battleName) {
		return errors.New("combat entrypoints must be local relative paths")
	}
	sourceRoot, err := filepath.Abs(filepath.Dir(*manifestPath))
	if err != nil {
		return err
	}
	destination, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if relative, err := filepath.Rel(sourceRoot, destination); err == nil && (relative == "." || filepath.IsLocal(relative)) {
		return errors.New("output must be outside the original resource set")
	}
	if _, err = os.Stat(destination); !os.IsNotExist(err) {
		return errors.New("output directory already exists or cannot be inspected")
	}
	cardPath := filepath.Join(sourceRoot, cardName)
	battleRoot := filepath.Join(sourceRoot, battleName)
	defs, err := battlecontent.LoadDefinitions(cardPath, battleRoot)
	if err != nil {
		return err
	}
	for id := patchSkillID; id <= patchPassiveID; id++ {
		if len(defs.Skills[id]) != 0 || len(defs.SkillRoles[id]) != 0 {
			return fmt.Errorf("patch skill/role ID %d is already used", id)
		}
	}
	skillsPath := filepath.Join(battleRoot, "skill_player.csv")
	rolesPath := filepath.Join(battleRoot, "skill_role_player.csv")
	cards, err := os.ReadFile(cardPath)
	if err != nil {
		return err
	}
	skills, err := os.ReadFile(skillsPath)
	if err != nil {
		return err
	}
	roles, err := os.ReadFile(rolesPath)
	if err != nil {
		return err
	}
	supportSkills, err := os.ReadFile(filepath.Join(battleRoot, "support_skill.csv"))
	if err != nil {
		return err
	}
	supportRoles, err := os.ReadFile(filepath.Join(battleRoot, "support_skill_role.csv"))
	if err != nil {
		return err
	}
	updatedCard, err := patchCard(cards)
	if err != nil {
		return err
	}
	base, err := findRow(skills, sourceSkillID, -1, "")
	if err != nil {
		return err
	}
	attack, err := findRow(roles, sourceSkillID, 8, "ATTACK_AA")
	if err != nil {
		return err
	}
	newSkills, newRoles := skillRows(base, attack)
	visual, err := findRow(roles, sourceSkillID, -1, "")
	if err != nil {
		return err
	}
	for i, role := range newRoles {
		if i == 0 || newRoles[i-1][0] != role[0] {
			copy(role[1:8], visual[1:8])
		}
	}
	passive, err := findRow(supportSkills, sourcePassiveID, -1, "")
	if err != nil {
		return err
	}
	boost, err := findRow(supportRoles, sourcePassiveID, 8, "DAMAGE_BOOST")
	if err != nil {
		return err
	}
	newPassive, newPassiveRoles := passiveRows(passive, boost)
	changes := map[string][]byte{
		cardName: updatedCard,
		filepath.ToSlash(filepath.Join(battleName, "skill_player.csv")):       appendRows(skills, newSkills),
		filepath.ToSlash(filepath.Join(battleName, "skill_role_player.csv")):  appendRows(roles, newRoles),
		filepath.ToSlash(filepath.Join(battleName, "support_skill.csv")):      appendRows(supportSkills, [][]string{newPassive}),
		filepath.ToSlash(filepath.Join(battleName, "support_skill_role.csv")): appendRows(supportRoles, newPassiveRoles),
	}
	// Copy only the executable masters, not media, account databases or live files.
	for _, relative := range []string{filepath.Dir(cardName), battleName} {
		if relative == "." {
			return errors.New("card master requires its own directory")
		}
		if err = os.CopyFS(filepath.Join(destination, relative), os.DirFS(filepath.Join(sourceRoot, relative))); err != nil {
			return err
		}
	}
	var files []struct {
		Path   string `json:"path"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	}
	if err = json.Unmarshal(manifest["files"], &files); err != nil {
		return err
	}
	receipt := map[string]any{"card_id": cardID, "original_skill_id": sourceSkillID, "skill_id": patchSkillID,
		"original_passive_id": sourcePassiveID, "passive_id": patchPassiveID,
		"source_resource_set": *manifestPath, "scope": "server masters only; requires rebuilt server; client bundles are not included"}
	checksums := map[string]any{}
	for name, content := range changes {
		original, err := os.ReadFile(filepath.Join(sourceRoot, name))
		if err != nil {
			return err
		}
		backup := filepath.Join(destination, "backup", name)
		if err = os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(backup, original, 0644); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(destination, name), content, 0644); err != nil {
			return err
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(content))
		checksums[name] = map[string]any{"before_sha256": fmt.Sprintf("%x", sha256.Sum256(original)), "after_sha256": digest}
		found := false
		for i := range files {
			if files[i].Path == name {
				files[i].Bytes, files[i].SHA256, found = len(content), digest, true
			}
		}
		if !found {
			return fmt.Errorf("changed file is absent from inventory: %s", name)
		}
	}
	manifest["files"], err = json.Marshal(files)
	if err != nil {
		return err
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(destination, "resource-set.json"), append(manifestBytes, '\n'), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(destination, "backup", "resource-set.json"), raw, 0644); err != nil {
		return err
	}
	receipt["files"] = checksums
	report, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(destination, "patch.json"), append(report, '\n'), 0644); err != nil {
		return err
	}
	fmt.Println("Prepared server-only patch:", destination)
	return nil
}

func findRow(raw []byte, id string, column int, value string) ([]string, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	for {
		row, err := r.Read()
		if err == io.EOF {
			return nil, fmt.Errorf("CSV row %s/%s is missing", id, value)
		}
		if err != nil {
			return nil, err
		}
		if len(row) > 0 && row[0] == id && (column < 0 || len(row) > column && row[column] == value) {
			return row, nil
		}
	}
}

func patchCard(raw []byte) ([]byte, error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	var start, end int64
	var replacement []string
	for {
		before := r.InputOffset()
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) == 0 || row[0] != cardID {
			continue
		}
		if replacement != nil {
			return nil, errors.New("duplicate target card")
		}
		if len(row) <= 46 || row[39] != sourceSkillID || row[46] != sourcePassiveID {
			return nil, errors.New("target Arthur skill or passive changed; refusing to overwrite")
		}
		row[39] = strconv.Itoa(patchSkillID)
		row[46] = strconv.Itoa(patchPassiveID)
		start, end, replacement = before, r.InputOffset(), row
	}
	if replacement == nil {
		return nil, errors.New("target card is missing")
	}
	encoded := encodeRows([][]string{replacement})
	if end > 1 && bytes.Equal(raw[end-2:end], []byte("\r\n")) {
		encoded = append(encoded[:len(encoded)-1], '\r', '\n')
	}
	result := append([]byte(nil), raw[:start]...)
	result = append(result, encoded...)
	return append(result, raw[end:]...), nil
}

func skillRows(base, attack []string) ([][]string, [][]string) {
	var skills, roles [][]string
	for mask := 0; mask < 4; mask++ {
		id := strconv.Itoa(patchSkillID + mask)
		skill := append([]string(nil), base...)
		skill[0], skill[3], skill[14], skill[49] = strconv.Itoa(patchSkillID), description, "9", id
		for i := 28; i < 49; i++ {
			skill[i] = ""
		}
		skill[48] = strconv.Itoa(mask)
		conditions := [][]string{}
		if mask&1 != 0 {
			conditions = append(conditions, []string{"SELF_PLAY_COST_TOTAL", "15", "0"})
		}
		if mask&2 != 0 {
			conditions = append(conditions, []string{"FRIEND_PLAY_ATTR_NUM", "FIRE", "10"})
		}
		for i, condition := range conditions {
			copy(skill[36+i*6:42+i*6], condition)
		}
		makeRole := func(function, target string, parameters ...string) []string {
			row := append([]string(nil), attack...)
			row[0], row[8], row[9], row[30] = id, function, target, ""
			for i := 20; i < 30; i++ {
				row[i] = ""
			}
			copy(row[20:30], parameters)
			return row
		}
		roles = append(roles, makeRole("ATK_OP_PIERCING", "SELECT", "100", "0"))
		attackIndex := 1
		if mask&1 != 0 {
			roles = append(roles, makeRole("ATK_OP_DAMAGE_INCREASE", "SELF", "0", "0", "10000", "0", "INT", "0"))
			attackIndex++
		}
		hits := "4"
		if mask&2 != 0 {
			hits = "7"
		}
		row := makeRole("ATTACK_AA", "SELECT", "329803", "0", "10000", "0", hits, "ATK", "150", "FIRE", "PHYSICS")
		row[30] = attack[30]
		roles = append(roles, row, makeRole("BURST_GAUGE_QUICK_UP", "SELF", "50", "0"))
		skill[4], skill[5] = strconv.Itoa(attackIndex*10+1), strconv.Itoa(attackIndex+1)
		skills = append(skills, skill)
	}
	return skills, roles
}

func encodeRows(rows [][]string) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.WriteAll(rows)
	return b.Bytes()
}

func passiveRows(base, boost []string) ([]string, [][]string) {
	id := strconv.Itoa(patchPassiveID)
	skill := append([]string(nil), base...)
	skill[0], skill[3], skill[49] = id, passiveDescription, id
	damage := append([]string(nil), boost...)
	damage[0], damage[9] = id, "SELF"
	for i := 20; i < 30; i++ {
		damage[i] = ""
	}
	copy(damage[20:30], []string{"99", "120", "0", "0", "0", "FIRE", "ALL", "0", "0", "0"})
	cost := append([]string(nil), damage...)
	cost[8] = "NEED_COST_DOWN"
	copy(cost[20:30], []string{"99", "2", "ALL", "NULL", "0", "0", "7", "0", "0", "0"})
	return skill, [][]string{damage, cost}
}

func appendRows(raw []byte, rows [][]string) []byte {
	result := append([]byte(nil), raw...)
	if len(result) > 0 && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}
	return append(result, encodeRows(rows)...)
}
