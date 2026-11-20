package battlecore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedCostAndChainCondition(t *testing.T) {
	engine, _ := coreBattleFixture(t)
	current := battleAction{memberType: 1, cardType: 1, skill: CombatSkillDefinition{Cost: 9, Attribute: "FIRE"}}
	for _, cost := range []int{5, 6, 7} {
		for _, chain := range []int{0, 3, 4} {
			others := []battleAction{current, {memberType: 1, cardType: 2, skill: CombatSkillDefinition{Cost: cost}},
				{memberType: 2, cardType: 1, skill: CombatSkillDefinition{Cost: 99}}, {memberType: 1, sphereSlot: 1, skill: CombatSkillDefinition{Cost: 99}}}
			got := engine.branchConditionSatisfied("SELF_PLAY_COST_TOTAL_AND_CHAIN", [5]string{"15", "0", "4", "0"}, current, others, map[string]int{"FIRE": chain})
			if got != (cost >= 6 && chain >= 4) {
				t.Fatalf("cost=%d chain=%d: %t", cost, chain, got)
			}
		}
	}
	if !SupportsPlayerBranchCondition("SELF_PLAY_COST_TOTAL_AND_CHAIN") {
		t.Fatal("startup validation rejects the condition")
	}
	if engine.branchConditionSatisfied("SELF_PLAY_COST_TOTAL_AND_CHAIN", [5]string{"9", "0", "4", "0"}, battleAction{memberType: 1, sphereSlot: 1, skill: current.skill}, []battleAction{current}, map[string]int{"FIRE": 4}) {
		t.Fatal("sphere acquired a card chain")
	}
}

// This optional gate executes the actual generated CSVs, never the live server or player database.
func TestMordred9CPatchExecution(t *testing.T) {
	root := os.Getenv("CN602_MORDRED_PATCH_SET")
	if root == "" {
		t.Skip("set CN602_MORDRED_PATCH_SET to generated patch directory")
	}
	base := filepath.Join(root, "_local/control/server")
	catalog, err := LoadCombatCatalog(filepath.Join(base, "cn602-card-master/card.csv"), filepath.Join(base, "cn602-battle-master"))
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Cards[10219001].ArthurSkillID != 91021900 || catalog.Cards[10219001].NormalSkillID != 11106961 || catalog.Cards[10219001].ArthurPassiveSkillID != 91021908 || catalog.Cards[10219001].PassiveSkillID != 63000036 || catalog.Cards[11001050].ArthurSkillID != 11106962 || catalog.Skills[11106962][0].Cost != 3 {
		t.Fatal("patch identity or isolation failed")
	}
	if len(catalog.Skills[91021900]) != 4 {
		t.Fatal("client requires the four-variant patch")
	}
	for mask := 0; mask < 8; mask++ {
		t.Run(fmt.Sprint(mask), func(t *testing.T) {
			engine := newSphereContractEngine(catalog)
			engine.players[0].BurstState = burstGaugeNormal
			engine.enemyCount = 2
			engine.enemies[1] = engine.enemies[0]
			engine.enemies[1].MemberType = 6
			skill := catalog.Skills[91021900][0]
			current := battleAction{memberType: 1, cardID: 10219001, cardType: 1, cardLevel: 99, skill: skill, target: 5}
			engine.players[0].Deck[0] = BattleCard{CardID: 10219001, CardType: 1, Level: 99}
			if _, err := engine.executeMainCardPassives(&engine.players[0]); err != nil {
				t.Fatal(err)
			}
			if cost := engine.effectiveCardCost(&engine.players[0], 1, skill.Cost); cost != 7 {
				t.Fatalf("Mordred effective cost=%d, want 7", cost)
			}
			engine.turnActions = []battleAction{current}
			cost := 5
			if mask&2 != 0 {
				cost = 6
			}
			count := 9
			if mask&4 != 0 {
				count = 10
			}
			for i := 1; i < count; i++ {
				member := 2
				if i == 1 {
					member = 1
				} else if mask&1 != 0 && i == count-1 {
					member = 4
				} else if i > 5 {
					member = 3
				}
				other := battleAction{memberType: member, cardType: i + 1, skill: CombatSkillDefinition{Attribute: "FIRE_ICE", Cost: 0}}
				if member == 1 {
					other.skill.Cost = cost
				}
				engine.turnActions = append(engine.turnActions, other)
			}
			chains := battleActionChainCounts(engine.turnActions)
			selected, branch := engine.selectCombatSkillBranchWithIndex(current, engine.turnActions, chains)
			wantBranch := mask >> 1
			if branch != wantBranch || selected.FunctionID != 91021900+wantBranch {
				t.Fatalf("selected branch=%d role=%d for mask=%d", branch, selected.FunctionID, mask)
			}
			roles := catalog.SkillRoles[selected.FunctionID]
			modifiers := collectAttackModifiers(roles, &engine.players[0], 99, maxCombatChain("FIRE", chains), false)
			wantPierce, wantMagic, wantHits := 100, 0, 4
			if mask&2 != 0 {
				wantMagic = 100000
			}
			if mask&4 != 0 {
				wantHits = 7
			}
			if modifiers.piercingRate != wantPierce || modifiers.damageIncrease != wantMagic {
				t.Fatalf("wrong modifiers: %+v", modifiers)
			}
			if power := engine.cardDisplayPower(&engine.players[0], 99, selected, roles); power != 429803+wantMagic {
				t.Fatalf("preview power %d", power)
			}
			rows, err := engine.executePlayerAction(current, chains)
			if err != nil {
				t.Fatal(err)
			}
			var hits [2]int
			for _, row := range rows {
				if row.Command == 60 && len(row.Args) >= 3 && row.Args[0] >= 5 && row.Args[0] <= 6 && row.Args[2] < 0 {
					hits[row.Args[0]-5]++
				}
			}
			if hits != [2]int{wantHits, wantHits} || engine.players[0].Burst != 50 {
				t.Fatalf("hits=%v gauge=%d; want %d hits and gauge50", hits, engine.players[0].Burst, wantHits)
			}
			if _, err = encodeOptionalBattleResults(rows); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("passive", func(t *testing.T) {
		engine := newSphereContractEngine(catalog)
		owner := &engine.players[0]
		owner.Deck[0] = BattleCard{CardID: 10219001, CardType: 1, Level: 99}
		rows, err := engine.executeMainCardPassives(owner)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := encodeOptionalBattleResults(rows); err != nil {
			t.Fatal(err)
		}
		if len(owner.Effects) != 2 || owner.Effects[0].Function != "DAMAGE_BOOST" || owner.Effects[1].Function != "NEED_COST_DOWN" || owner.Effects[1].CostCardType != 0 {
			t.Fatalf("wrong startup effects: %+v", owner.Effects)
		}
		for _, sample := range []struct {
			attribute, physics string
			want               int
		}{{"FIRE", "PHYSICS", 120}, {"FIRE", "MAGIC", 120}, {"FIRE_ICE", "MAGIC", 120}, {"ICE", "PHYSICS", 0}, {"DARK", "MAGIC", 0}} {
			fixed, rate := damageBoostTerms(owner.Effects, sample.attribute, sample.physics, 9)
			if fixed != 0 || rate != sample.want {
				t.Fatalf("boost for %+v = %d/%d", sample, fixed, rate)
			}
		}
		for i, sample := range []struct {
			cost, want      int
			attribute, kind string
		}{{6, 6, "FIRE", "ATTACK"}, {7, 5, "ICE", "SORCERY"}, {8, 6, "DARK", "SUPPORT"}, {9, 7, "FIRE", "ATTACK"}, {10, 8, "WIND", "DEFENSE"}} {
			id, slot := 99021900+sample.cost, i+2
			if _, exists := catalog.Cards[id]; exists || len(catalog.Skills[id]) != 0 {
				t.Fatalf("test fixture ID %d is already used", id)
			}
			catalog.Cards[id] = CombatCardDefinition{ID: id, NormalSkillID: id}
			skill := catalog.Skills[91021900][0]
			skill.ID, skill.Cost, skill.Attribute, skill.Kind = id, sample.cost, sample.attribute, sample.kind
			catalog.Skills[id] = []CombatSkillDefinition{skill}
			owner.Deck[slot-1] = BattleCard{CardID: id, CardType: slot, Level: 99}
			engine.players[1].Deck[slot-1] = owner.Deck[slot-1]
			for attempt := 0; attempt < 2; attempt++ {
				if got := engine.effectiveCardCost(owner, slot, sample.cost); got != sample.want {
					t.Fatalf("base cost %d -> %d, want %d", sample.cost, got, sample.want)
				}
			}
			if got := engine.effectiveCardCost(&engine.players[1], slot, sample.cost); got != sample.cost {
				t.Fatalf("discount leaked to another player: %d", got)
			}
		}
		current := battleAction{memberType: 1, cardType: 1, skill: catalog.Skills[91021900][0]}
		other := battleAction{memberType: 1, cardType: 2, skill: catalog.Skills[99021906][0]}
		if engine.effectiveCardCost(owner, 1, 9)+engine.effectiveCardCost(owner, 2, 6) != 13 ||
			!engine.branchConditionSatisfied("SELF_PLAY_COST_TOTAL", [5]string{"15", "0"}, current, []battleAction{current, other}, nil) {
			t.Fatal("cost-total condition must use original 9+6, not discounted 7+6")
		}
		engine.tickMemberPersistentEffects(nil, owner)
		if len(owner.Effects) != 2 || engine.effectiveCardCost(owner, 1, 9) != 7 {
			t.Fatal("permanent passive expired at turn end")
		}
		for _, player := range engine.players[1:] {
			if len(player.Effects) != 0 {
				t.Fatal("self passive leaked to another player")
			}
		}
		normal, normalRoles, err := catalog.CardMainPassive(catalog.Cards[10219001], 2)
		if err != nil || normal.ID != 63000036 || len(normalRoles) != 1 || normalRoles[0].Function != "BEGINNING_DRAW" || catalog.SkillRoles[63000760][0].Function != "BEGINNING_DRAW" {
			t.Fatal("ordinary or original passive changed")
		}
	})
}
