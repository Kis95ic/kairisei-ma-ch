package battlecore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func mod2Effect(t *testing.T, effects []battleEffect, function, parameter string) battleEffect {
	t.Helper()
	for _, effect := range effects {
		if effect.Function == function && (parameter == "" || effect.Parameter == parameter) {
			return effect
		}
	}
	t.Fatalf("missing %s/%s in %+v", function, parameter, effects)
	return battleEffect{}
}

func TestCardMod2Stage2GeneratedMasters(t *testing.T) {
	root := os.Getenv("CN602_CARD_MOD2_SET")
	if root == "" {
		t.Skip("set CN602_CARD_MOD2_SET to the cumulative stage-two patch")
	}
	base := filepath.Join(root, "_local/control/server")
	catalog, err := LoadCombatCatalog(filepath.Join(base, "cn602-card-master/card.csv"), filepath.Join(base, "cn602-battle-master"))
	if err != nil {
		t.Fatal(err)
	}
	ids := []int{10219021, 10219022, 10219023, 10219024, 10219025, 10219011, 10219012, 10219013, 10219014, 10219015, 11001220, 11001221, 11001222, 11001223, 11001224}
	for index, id := range ids {
		t.Run(fmt.Sprint(id), func(t *testing.T) {
			card := catalog.Cards[id]
			if card.ArthurSkillID != 92001400+index*100 || card.ArthurPassiveSkillID != 92001450+index*100 {
				t.Fatal("wrong independent references")
			}
			if err := catalog.ValidateMainCardPassives(card); err != nil {
				t.Fatal(err)
			}
			lancelot, gareth := index < 5, index >= 5 && index < 10
			job, branches := 4, 5
			if lancelot {
				job, branches = 1, 2
			} else if gareth {
				job, branches = 3, 2
			}
			if len(catalog.Skills[card.ArthurSkillID]) != branches {
				t.Fatal("wrong branch count")
			}
			for _, skillID := range []int{card.ArthurSkillID, card.ArthurPassiveSkillID} {
				for _, variant := range catalog.Skills[skillID] {
					if len(catalog.SkillRoles[variant.FunctionID]) > 5 {
						t.Fatal("native role slots exceeded")
					}
				}
			}
			if !combatCardHasProfileTag(card, 20020006) || combatCardHasProfileTag(card, RoundTableCardGroup) != gareth {
				t.Fatal("wrong deck restriction/search tags")
			}
			compositionErr := ValidateMainDeckComposition(job, []int{id, 10219001}, (*Definitions)(catalog))
			if gareth && compositionErr != ErrMultipleRoundTableCards || !gareth && compositionErr != nil {
				t.Fatalf("deck restriction: %v", compositionErr)
			}
			skill := catalog.Skills[card.ArthurSkillID][0]
			// A nonmatching card before one matching card proves search filters
			// and draw-pool reordering; a generated skill has no extra passives.
			for n := 1; n <= 9; n++ {
				fakeID := 99050000 + n
				catalog.Cards[fakeID] = CombatCardDefinition{ID: fakeID, NormalSkillID: fakeID}
				fakeSkill := skill
				fakeSkill.ID, fakeSkill.Attribute, fakeSkill.Cost = fakeID, "NEUTRAL", 0
				catalog.Skills[fakeID] = []CombatSkillDefinition{fakeSkill}
			}
			fake := catalog.Cards[99050002]
			fake.ProfileTags[0] = 20020006
			catalog.Cards[99050002] = fake
			newEngine := func(level int) (*BattleEngine, battleAction) {
				engine := newSphereContractEngine(catalog)
				owner := &engine.players[0]
				owner.ArthurType = job
				owner.Deck[0] = BattleCard{CardID: id, CardType: 1, Level: level}
				for i := range owner.DeckOrder {
					owner.DeckOrder[i] = -1
				}
				owner.Deck[1], owner.Deck[2] = BattleCard{CardID: 99050001, CardType: 2}, BattleCard{CardID: 99050002, CardType: 3}
				owner.DeckOrder[0], owner.DeckOrder[1] = 1, 2
				for i := range engine.players {
					engine.players[i].BurstState = burstGaugeNormal
					engine.players[i].HP = 10000
				}
				engine.enemyCount = 2
				engine.enemies[0].Attack, engine.enemies[0].BaseAttack = 100000, 100000
				engine.enemies[0].Magic, engine.enemies[0].BaseMagic = 100000, 100000
				engine.enemies[1] = engine.enemies[0]
				engine.enemies[1].MemberType = 6
				return engine, battleAction{memberType: 1, cardID: id, cardType: 1, cardLevel: level, skill: skill, target: 5}
			}
			for _, level := range []int{1, 99} {
				for _, count := range []int{1, 2, 5} {
					for _, chain := range []int{1, 2, 3, 4} {
						variants := []int{0, 1} // extra card matches/does not match the element
						if !lancelot && !gareth {
							variants = []int{0, 1, 2, 3, 4} // unmatched job, then four ordinary jobs
						}
						for _, variant := range variants {
							t.Run(fmt.Sprintf("active/lv%d/cards%d/chain%d/case%d", level, count, chain, variant), func(t *testing.T) {
								engine, action := newEngine(level)
								wantBranch, gauge, magnitude := 0, 20, 157560
								extraAttribute := skill.Attribute
								if variant == 1 {
									extraAttribute = "NEUTRAL"
								}
								if lancelot && count >= 2 && variant == 0 {
									wantBranch, gauge, magnitude = 1, 30, 787800
								} else if gareth && count >= 2 && chain == 4 {
									wantBranch, gauge = 1, 35
								} else if !lancelot && !gareth {
									action.target, wantBranch = 2, variant
									engine.players[1].ArthurType = variant
									if variant == 0 {
										engine.players[1].ArthurType = 5
									}
								}
								engine.turnActions = []battleAction{action}
								for n := 1; n < count; n++ {
									engine.turnActions = append(engine.turnActions, battleAction{memberType: 1, cardType: n + 1, skill: CombatSkillDefinition{Attribute: extraAttribute, Cost: 0}})
								}
								engine.turnActions = append(engine.turnActions,
									battleAction{memberType: 1, sphereSlot: 1, skill: skill},
									battleAction{memberType: 2, cardType: 1, skill: skill})
								chains := map[string]int{skill.Attribute: chain}
								_, branch := engine.selectCombatSkillBranchWithIndex(action, engine.turnActions, chains)
								if branch != wantBranch {
									t.Fatalf("branch %d, want %d", branch, wantBranch)
								}
								rows, err := engine.executePlayerAction(action, chains)
								if err != nil {
									t.Fatal(err)
								}
								if _, err := encodeOptionalBattleResults(rows); err != nil {
									t.Fatal(err)
								}
								for _, player := range engine.players {
									if player.Burst != gauge {
										t.Fatalf("team gauge %d, want %d", player.Burst, gauge)
									}
									if lancelot || !gareth && variant == 2 {
										want := magnitude
										if !lancelot {
											want = 620114
										}
										effect := mod2Effect(t, player.Effects, "ENCHANT", "")
										if effect.Value != want || effect.Remaining != 3 || effect.Attribute != skill.Attribute {
											t.Fatalf("enchant %+v, want %d", effect, want)
										}
									}
									if !lancelot && !gareth {
										hpBonus := 0
										if variant == 2 {
											hpBonus = 40322
											effect := mod2Effect(t, player.Effects, "ATK_UP_FIXED", "MAX_HP")
											if effect.Delta != hpBonus || effect.Remaining != 3 {
												t.Fatal("wrong HP benefit", effect)
											}
										}
										if player.HP != 30854+hpBonus || player.MaxHP != 100000+hpBonus {
											t.Fatalf("heal/HP %d/%d", player.HP, player.MaxHP)
										}
										if variant == 1 {
											for _, kind := range []string{"PHYSICS", "MAGIC"} {
												snapshot := countedDamageSnapshot(player.Effects, skill.Attribute, kind)
												if len(snapshot) != 1 || snapshot[0].Uses != 5 || snapshot[0].Remaining != 1 || countedDamageHitRate(snapshot, 4) != 200 || countedDamageHitRate(snapshot, 5) != 0 || len(countedDamageSnapshot(player.Effects, "NEUTRAL", kind)) != 0 {
													t.Fatalf("wrong five-use damage boost %+v", snapshot)
												}
											}
										} else if variant == 4 {
											flag, multiplier := engine.nativeCriticalOutcome(player.Effects, 0)
											if flag != 1 || multiplier != 300 || mod2Effect(t, player.Effects, "CRITICAL_UP", "").Remaining != 1 || mod2Effect(t, player.Effects, "CRITICAL_DAMAGE_BOOST", "").Remaining != 1 {
												t.Fatalf("critical %d/%d", flag, multiplier)
											}
										}
									}
								}
								for n, enemy := range engine.enemies[:2] {
									if lancelot || !gareth && variant == 3 {
										want := magnitude
										if !lancelot {
											want = 1240308
										}
										effect := mod2Effect(t, enemy.Effects, "ATTR_DEF_DOWN", "")
										if effect.Value != want || effect.Attribute != skill.Attribute || effect.Remaining != 3 {
											t.Fatalf("resistance %+v, want %d", effect, want)
										}
									} else if gareth {
										if n == 0 && (mod2Effect(t, enemy.Effects, "WEAKNESS", "").Remaining != 99 || weaknessAdjustedPower(1000, enemy.Effects) != 2200) || n == 1 && weaknessAdjustedPower(1000, enemy.Effects) != 1000 {
											t.Fatal("mark target/rate changed", enemy.Effects)
										}
										want := 0
										if n == 0 || wantBranch == 1 {
											want = 70*level + 1500
											if wantBranch == 1 {
												want += 300
											}
											for _, parameter := range []string{"ATK", "INT"} {
												effect := mod2Effect(t, enemy.Effects, "ATK_BREAK_BY_SELF_PARAM", parameter)
												if effect.Delta != -want || effect.Remaining != 99 {
													t.Fatalf("debuff %+v, want %d", effect, want)
												}
											}
										}
										if enemy.Attack != 100000-want || enemy.Magic != 100000-want {
											t.Fatal("debuff did not reach enemy parameters", enemy.Attack, enemy.Magic)
										}
									}
								}
								if !lancelot && !gareth && engine.players[0].DeckOrder[0] != 2 {
									t.Fatal("round-table search failed", engine.players[0].DeckOrder)
								}
							})
						}
					}
				}
			}
			for _, count := range []int{1, 2, 3, 4, 5, 6} {
				t.Run(fmt.Sprintf("passive/deck%d", count), func(t *testing.T) {
					engine, _ := newEngine(99)
					owner := &engine.players[0]
					for n := 1; n < 10; n++ {
						fakeID := 99050000 + n
						fake := catalog.Cards[fakeID]
						fake.ProfileTags = [8]int{}
						catalog.Skills[fakeID][0].Attribute = "NEUTRAL"
						if n < count {
							fake.ProfileTags[0] = 20020006
							catalog.Skills[fakeID][0].Attribute = skill.Attribute
						}
						catalog.Cards[fakeID] = fake
						owner.Deck[n] = BattleCard{CardID: fakeID, CardType: n + 1, Level: 99}
					}
					// Support-deck copies must not satisfy main-deck gates.
					for n := range owner.SupportDeck {
						owner.SupportDeck[n] = owner.Deck[0]
					}
					rows, err := engine.executeMainCardPassives(owner)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := encodeOptionalBattleResults(rows); err != nil {
						t.Fatal(err)
					}
					for n := range engine.players {
						player := &engine.players[n]
						if lancelot {
							limit, rate := 99999, 0
							if n == 0 || count >= 5 {
								limit += 80000
							}
							if count >= 5 {
								rate = 60
							}
							if player.LimitAttack != limit || player.LimitMagic != limit {
								t.Fatal("wrong limit recipient/value", n, player.LimitAttack, player.LimitMagic)
							}
							for _, kind := range []string{"PHYSICS", "MAGIC"} {
								_, got := damageBoostTerms(player.Effects, skill.Attribute, kind, 1)
								if got != rate {
									t.Fatal("wrong elemental damage rate", got)
								}
							}
						} else if gareth {
							boost, hp := 1060, 100000
							if count >= 5 {
								boost, hp = 1120, 108000
							}
							if player.MaxHP != hp {
								t.Fatal("wrong team HP", player.MaxHP)
							}
							if n != 0 {
								boost = 1000
							} else {
								mod2Effect(t, player.Effects, "BEGINNING_DRAW", "")
							}
							for _, parameter := range []string{"ATK", "INT"} {
								if got := sphereSupportBoostedValue(player.Effects, "ATK_BREAK_BOOST", skill.Attribute, parameter, 1000); got != boost {
									t.Fatal("wrong debuff boost", got, boost)
								}
								if got := sphereSupportBoostedValue(player.Effects, "ATK_BREAK_BOOST", "NEUTRAL", parameter, 1000); got != 1000 {
									t.Fatal("boost leaked to another element")
								}
							}
						} else {
							gauge, debuff := 10, 1000
							if count >= 2 {
								gauge = 15
							}
							if count >= 3 {
								debuff = 850
							}
							if player.Burst != gauge {
								t.Fatal("opening gauge", player.Burst)
							}
							for _, parameter := range []string{"ATK", "INT"} {
								if got := parameterDebuffValue(player.Effects, "ATK_BREAK_FIXED", parameter, 1000, 0, 0); got != debuff {
									t.Fatal("wrong received debuff cut", got)
								}
							}
						}
						before, gauge := len(player.Effects), player.Burst
						engine.tickMemberPersistentEffects(nil, player)
						if len(player.Effects) != before || player.Burst != gauge {
							t.Fatal("passive expired or opening gauge repeated")
						}
					}
				})
			}
		})
	}
}
