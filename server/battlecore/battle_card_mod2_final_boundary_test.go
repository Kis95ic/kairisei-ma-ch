package battlecore

import (
	"fmt"
	"testing"
)

func TestCardMod2FinalConjunctiveGates(t *testing.T) {
	catalog := finalCatalog(t)
	for _, family := range []int{2, 6, 8} {
		for element := 0; element < 5; element++ {
			for mask := 0; mask < 4; mask++ {
				t.Run(fmt.Sprintf("%d/%d/%d", family, element, mask), func(t *testing.T) {
					e, a := finalEngine(catalog, family, finalStarts[family]+element, 99)
					if mask&1 != 0 {
						e.turnActions = append(e.turnActions, battleAction{memberType: 1, cardType: 2, skill: CombatSkillDefinition{Attribute: a.skill.Attribute}})
					}
					chain := 1
					if mask&2 != 0 {
						chain = 4
					}
					_, got := e.selectCombatSkillBranchWithIndex(a, e.turnActions, map[string]int{a.skill.Attribute: chain})
					want := mask
					if family == 2 {
						want = 0
						if mask&1 != 0 {
							want = 1
						}
						if mask == 3 {
							want = 2
						}
					}
					if family == 6 {
						want = 0
						if mask == 3 {
							want = 1
						}
					}
					finalEqual(t, "AND gate", got, want)
				})
			}
		}
	}
}

func TestCardMod2FinalHeatIgnoresDisabledAndOtherStates(t *testing.T) {
	catalog := finalCatalog(t)
	for _, family := range []int{0, 7} {
		e, a := finalEngine(catalog, family, finalStarts[family], 99)
		for state := 0; state <= 3; state++ {
			e.enemies[0].Trance = battleEnemyTrance{State: state, Value: 800, NormalLimit: 1000, OverheatLimit: 1000, Reactive: &EnemyGaugeDefinition{}}
			role := catalog.SkillRoles[a.skill.FunctionID][len(catalog.SkillRoles[a.skill.FunctionID])-1]
			e.executePlayerTranceRole(a, role)
			want := 800
			if state == 2 {
				want = 600
			}
			finalEqual(t, "heat state filter", e.enemies[0].Trance.Value, want)
		}
	}
}
