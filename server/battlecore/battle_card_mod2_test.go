package battlecore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Load the generated masters, so parameter binding and branch selection are
// exercised together with the actual battle consumers and result encoder.
func TestCardMod2GeneratedMasters(t *testing.T) {
	root := os.Getenv("CN602_CARD_MOD2_SET")
	if root == "" {
		t.Skip("set CN602_CARD_MOD2_SET to the generated second-batch patch")
	}
	base := filepath.Join(root, "_local/control/server")
	catalog, err := LoadCombatCatalog(filepath.Join(base, "cn602-card-master/card.csv"), filepath.Join(base, "cn602-battle-master"))
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.ValidatePlayerFunctionCoverage(); err != nil {
		t.Fatal(err)
	}
	ids := []int{10219002, 10219003, 10219004, 10219005, 10219031, 10219032, 10219033, 10219034, 10219035, 11001215, 11001216, 11001217, 11001218, 11001219}
	for index, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			card := catalog.Cards[id]
			if card.ArthurSkillID != 92000000+index*100 || card.ArthurPassiveSkillID != 92000050+index*100 {
				t.Fatal("wrong independent program references")
			}
			if err := catalog.ValidateMainCardPassives(card); err != nil {
				t.Fatal(err)
			}
			skill := catalog.Skills[card.ArthurSkillID][0]
			caradoc, mordred := id >= 11001215, id < 10219006
			for _, level := range []int{1, 99} {
				for mask := 0; mask < 4; mask++ {
					t.Run(fmt.Sprintf("active/level%d/mask%d", level, mask), func(t *testing.T) {
						engine := newSphereContractEngine(catalog)
						for i := range engine.players {
							engine.players[i].BurstState = burstGaugeNormal
						}
						owner := &engine.players[0]
						owner.ArthurType = 3
						if mordred {
							owner.ArthurType = 1
							if skill.Cost != 9 {
								t.Fatal("wrong original cost")
							}
						}
						owner.Deck[0] = BattleCard{CardID: id, CardType: 1, Level: level}
						for i := range owner.DeckOrder {
							owner.DeckOrder[i] = -1
						}
						engine.enemyCount = 2
						engine.enemies[1] = engine.enemies[0]
						engine.enemies[1].MemberType = 6
						current := battleAction{memberType: 1, cardID: id, cardType: 1, cardLevel: level, skill: skill, target: 5}
						engine.turnActions = []battleAction{current}
						chain := 1
						if caradoc {
							if mask&1 != 0 {
								engine.turnActions = append(engine.turnActions, battleAction{memberType: 1, cardType: 2, skill: skill})
							}
							if mask&2 != 0 {
								chain = 4
							}
						} else {
							count, cost := 9, 5
							if mask&1 != 0 {
								cost = 6
							}
							if mask&2 != 0 {
								count = 10
							}
							for i := 1; i < count; i++ {
								member, selectedCost := 2, 0
								if i == 1 {
									member, selectedCost = 1, cost
								}
								engine.turnActions = append(engine.turnActions, battleAction{memberType: member, cardType: i + 1,
									skill: CombatSkillDefinition{Attribute: skill.Attribute, Cost: selectedCost}})
							}
						}
						// Spheres must not satisfy card-count or original-cost gates.
						engine.turnActions = append(engine.turnActions, battleAction{memberType: 1, sphereSlot: 1,
							skill: CombatSkillDefinition{Attribute: skill.Attribute, Cost: 99}})
						chains := map[string]int{skill.Attribute: chain}
						selected, branch := engine.selectCombatSkillBranchWithIndex(current, engine.turnActions, chains)
						if branch != mask {
							t.Fatalf("branch=%d want %d", branch, mask)
						}
						roles := catalog.SkillRoles[selected.FunctionID]
						if !caradoc {
							modifiers := collectAttackModifiers(roles, owner, level, chain, false)
							wantExtra := 0
							if mask&1 != 0 {
								wantExtra = 100000
							}
							if modifiers.piercingRate != 100 || modifiers.damageIncrease != wantExtra ||
								engine.cardDisplayPower(owner, level, selected, roles) != 429803+wantExtra {
								t.Fatalf("wrong pierce, scaling or preview: %+v", modifiers)
							}
						}
						rows, err := engine.executePlayerAction(current, chains)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := encodeOptionalBattleResults(rows); err != nil {
							t.Fatal(err)
						}
						if caradoc {
							multiplier := 1 + mask&1 + (mask>>1)&1
							for _, player := range engine.players {
								if player.Burst != 20 || player.Magic != 10000+20052*multiplier || player.LimitMagic != 99999+30843*multiplier {
									t.Fatalf("wrong team benefit: gauge=%d magic=%d limit=%d", player.Burst, player.Magic, player.LimitMagic)
								}
							}
							for _, enemy := range engine.enemies[:2] {
								if len(enemy.Effects) != 1 || enemy.Effects[0].Value != 310077*multiplier || enemy.Effects[0].Attribute != skill.Attribute || enemy.Effects[0].Remaining != 3 {
									t.Fatalf("wrong resistance debuff: %+v", enemy.Effects)
								}
							}
						} else {
							var hits [2]int
							for _, row := range rows {
								if row.Command == 60 && len(row.Args) >= 3 && row.Args[0] >= 5 && row.Args[0] <= 6 && row.Args[2] < 0 {
									hits[row.Args[0]-5]++
								}
							}
							wantHits := 4
							if mask&2 != 0 {
								wantHits = 7
							}
							if hits != [2]int{wantHits, wantHits} || owner.Burst != 50 || engine.players[1].Burst != 0 {
								t.Fatalf("hits=%v, gauge=%d", hits, owner.Burst)
							}
						}
					})
				}
			}
			for _, deckCount := range []int{1, 2, 3, 4, 5, 6} {
				t.Run(fmt.Sprintf("passive/deck%d", deckCount), func(t *testing.T) {
					engine := newSphereContractEngine(catalog)
					owner := &engine.players[0]
					owner.ArthurType = 3
					if mordred {
						owner.ArthurType = 1
					}
					owner.Deck[0] = BattleCard{CardID: id, CardType: 1, Level: 99}
					for i := 1; i < 10; i++ {
						fakeID := 99030000 + i
						fakeCard := CombatCardDefinition{ID: fakeID, NormalSkillID: fakeID}
						fakeSkill := skill
						fakeSkill.ID, fakeSkill.Cost, fakeSkill.Attribute = fakeID, 5, "NEUTRAL"
						if i < deckCount {
							fakeSkill.Attribute = skill.Attribute
							fakeCard.ProfileTags[0] = 20020006
						}
						catalog.Cards[fakeID] = fakeCard
						catalog.Skills[fakeID] = []CombatSkillDefinition{fakeSkill}
						defer delete(catalog.Cards, fakeID)
						defer delete(catalog.Skills, fakeID)
						owner.Deck[i] = BattleCard{CardID: fakeID, CardType: i + 1, Level: 99}
					}
					rows, err := engine.executeMainCardPassives(owner)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := encodeOptionalBattleResults(rows); err != nil {
						t.Fatal(err)
					}
					if caradoc {
						wantRate, wantBoost := 100, 0
						if deckCount >= 2 {
							wantRate, wantBoost = 150, 500
						}
						if deckCount >= 3 {
							wantBoost = 1000
						}
						for _, player := range engine.players {
							_, rate := damageBoostTerms(player.Effects, skill.Attribute, "MAGIC", 1)
							_, physical := damageBoostTerms(player.Effects, skill.Attribute, "PHYSICS", 1)
							if rate != wantRate || deckCount == 1 && physical != 0 || deckCount >= 2 && physical != 150 {
								t.Fatalf("wrong team damage boost %d/%d", rate, physical)
							}
							boost := 0
							for _, effect := range player.Effects {
								if effect.Function == "ATTR_DEF_DOWN_BOOST" {
									boost += effect.Rate
								}
							}
							if boost != wantBoost {
								t.Fatalf("wrong resistance boost %d", boost)
							}
						}
					} else {
						wantRate, discount := 160, 0
						if mordred {
							wantRate, discount = 180, 1
						}
						for _, kind := range []string{"PHYSICS", "MAGIC"} {
							_, rate := damageBoostTerms(owner.Effects, skill.Attribute, kind, 9)
							_, unrelated := damageBoostTerms(owner.Effects, "NEUTRAL", kind, 9)
							if rate != wantRate || unrelated != 0 {
								t.Fatalf("wrong elemental boost %d/%d", rate, unrelated)
							}
						}
						if !mordred {
							for _, player := range engine.players {
								if player.Magic != 60000 || player.LimitMagic != 124999 {
									t.Fatalf("wrong Bedivere team magic/limit %d/%d", player.Magic, player.LimitMagic)
								}
							}
						}
						for _, cost := range []int{4, 5, 9} {
							for _, tag := range []int{0, 20020006} {
								fakeID := 99030009
								fake := catalog.Cards[fakeID]
								fake.ProfileTags = [8]int{tag}
								catalog.Cards[fakeID] = fake
								catalog.Skills[fakeID][0].Cost = cost
								want := cost
								if cost >= 5 {
									want -= discount
									if deckCount >= 5 && tag != 0 {
										want -= 2
									}
								}
								for range 2 {
									if got := engine.effectiveCardCost(owner, 10, cost); got != want {
										t.Fatalf("cost %d tag %d => %d want %d", cost, tag, got, want)
									}
								}
								engine.players[1].Deck[9] = owner.Deck[9]
								if got := engine.effectiveCardCost(&engine.players[1], 10, cost); got != cost {
									t.Fatal("discount leaked to teammate")
								}
							}
						}
					}
					before := len(owner.Effects)
					engine.tickMemberPersistentEffects(nil, owner)
					if len(owner.Effects) != before {
						t.Fatal("permanent passive expired")
					}
				})
			}
		})
	}
}
