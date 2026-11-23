package battlecore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCardMod2FinalIndependentPassives(t *testing.T) {
	catalog := finalCatalog(t)
	for family, start := range finalStarts {
		for element := 0; element < 5; element++ {
			for _, count := range []int{1, 2, 4, 5, 6} {
				for _, level := range []int{1, 99} {
					t.Run(fmt.Sprintf("%d/count%d/level%d", start+element, count, level), func(t *testing.T) {
						e, a := finalEngine(catalog, family, start+element, level)
						owner := &e.players[0]
						for n := 1; n < 10; n++ {
							fakeID := 99070000 + n
							card := catalog.Cards[fakeID]
							card.ProfileTags = [8]int{}
							catalog.Skills[fakeID][0].Attribute = "NEUTRAL"
							if n < count {
								card.ProfileTags[0] = 20020006
								catalog.Skills[fakeID][0].Attribute = a.skill.Attribute
							}
							catalog.Cards[fakeID] = card
						}
						for n := range owner.SupportDeck {
							owner.SupportDeck[n] = owner.Deck[0]
						}
						rows, err := e.executeMainCardPassives(owner)
						if err != nil {
							t.Fatal(err)
						}
						if _, err = encodeOptionalBattleResults(rows); err != nil {
							t.Fatal(err)
						}
						gate := 5
						if family == 2 || family == 5 || family == 8 || family == 11 {
							gate = 2
						}
						strong := count >= gate
						for i := range e.players {
							p := &e.players[i]
							own := i == 0
							atk, mag, def, mdf, hp, al, ml, ga := 10000, 10000, 1000, 1000, 100000, 99999, 99999, 0
							ratePhys, rateMagic := 0, 0
							switch family {
							case 0:
								if own {
									ratePhys, rateMagic = 180, 180
								}
							case 1:
								if own || strong {
									al += 80000
									ml += 80000
								}
								if strong {
									ratePhys, rateMagic = 60, 60
								}
								if own {
									for _, param := range []string{"ATK", "INT"} {
										finalEqual(t, "ATK boost", sphereSupportBoostedValue(p.Effects, "ATK_UP_BOOST", a.skill.Attribute, param, 1000), 1200)
									}
								}
							case 2:
								if strong {
									ratePhys, rateMagic, ga = 120, 120, 15
								} else if own {
									ratePhys, rateMagic = 60, 60
								}
							case 3:
								if own {
									rate := 1090
									if strong {
										rate = 1180
									}
									for _, param := range []string{"DEF", "MDEF"} {
										finalEqual(t, "DEF boost", sphereSupportBoostedValue(p.Effects, "DEF_UP_BOOST", a.skill.Attribute, param, 1000), rate)
									}
									mod2Effect(t, p.Effects, "BEGINNING_DRAW", "")
									if strong {
										cut := mod2Effect(t, p.Effects, "DAMAGE_CUT2", "")
										finalEqual(t, "attribute cut", cut.Rate, 100)
										if cut.Attribute != finalGuards[element] {
											t.Fatal("wrong cut element")
										}
									}
								}
							case 4:
								value := 15000
								if strong {
									value = 30000
								}
								def += value
								mdf += value
								if strong {
									guts := mod2Effect(t, p.Effects, "GUTS", "")
									finalEqual(t, "guts uses", guts.Uses, 1)
									finalEqual(t, "guts HP", guts.Rate, 30)
								}
							case 5:
								ratePhys = 100
								if strong {
									ratePhys, rateMagic = 150, 150
									al += 25000
									atk += 50000
									for _, param := range []string{"DEF", "MDEF"} {
										finalEqual(t, "DEF debuff cut", parameterDebuffValue(p.Effects, "GUARD_BREAK_FIXED", param, 1000, 0, 0), 850)
									}
								}
							case 6:
								if strong {
									hp += 12000
								}
								if own {
									mod2Effect(t, p.Effects, "BEGINNING_DRAW", "")
									finalEqual(t, "resistance boost", mod2Effect(t, p.Effects, "ATTR_DEF_DOWN_BOOST", "").Rate, 500)
									rate := 1060
									if strong {
										rate = 1120
									}
									for _, param := range []string{"ATK", "INT"} {
										finalEqual(t, "ATK debuff boost", sphereSupportBoostedValue(p.Effects, "ATK_BREAK_BOOST", a.skill.Attribute, param, 1000), rate)
									}
								}
							case 7:
								mag += 50000
								ml += 25000
								if own {
									ratePhys, rateMagic = 160, 160
									finalEqual(t, "INT boost", sphereSupportBoostedValue(p.Effects, "ATK_UP_BOOST", a.skill.Attribute, "INT", 1000), 1200)
								}
							case 8:
								rateMagic = 100
								if strong {
									ratePhys, rateMagic = 150, 150
									finalEqual(t, "resistance boost", mod2Effect(t, p.Effects, "ATTR_DEF_DOWN_BOOST", "").Rate, 1000)
								}
							case 9:
								value := 40000
								if strong {
									value = 60000
									ratePhys, rateMagic = 100, 100
								}
								al += value
								ml += value
							case 10:
								hp += 12000
								ratePhys, rateMagic = 100, 100
								if strong {
									for _, param := range []string{"ATK", "INT"} {
										finalEqual(t, "team ATK boost", sphereSupportBoostedValue(p.Effects, "ATK_UP_BOOST", a.skill.Attribute, param, 1000), 1300)
									}
								}
							case 11:
								ga = 10
								if strong {
									ga = 15
									for _, param := range []string{"ATK", "INT"} {
										finalEqual(t, "ATK debuff cut", parameterDebuffValue(p.Effects, "ATK_BREAK_FIXED", param, 1000, 0, 0), 850)
									}
								}
							}
							finalEqual(t, "ATK", p.Attack, atk)
							finalEqual(t, "INT", p.Magic, mag)
							finalEqual(t, "DEF", p.Defense, def)
							finalEqual(t, "MDEF", p.MDefense, mdf)
							finalEqual(t, "HP", p.MaxHP, hp)
							finalEqual(t, "physical limit", p.LimitAttack, al)
							finalEqual(t, "magic limit", p.LimitMagic, ml)
							finalEqual(t, "opening gauge", p.Burst, ga)
							_, physical := damageBoostTerms(p.Effects, a.skill.Attribute, "PHYSICS", 9)
							_, magic := damageBoostTerms(p.Effects, a.skill.Attribute, "MAGIC", 9)
							finalEqual(t, "physical rate", physical, ratePhys)
							finalEqual(t, "magic rate", magic, rateMagic)
							_, unrelated := damageBoostTerms(p.Effects, "NEUTRAL", "PHYSICS", 9)
							finalEqual(t, "unrelated element", unrelated, 0)
							before := len(p.Effects)
							for turn := 0; turn < 12; turn++ {
								e.turn++
								e.tickMemberPersistentEffects(nil, p)
							}
							finalEqual(t, "persistent effect count", len(p.Effects), before)
							finalEqual(t, "opening gauge not repeated", p.Burst, ga)
							if family == 4 && strong {
								revived, _ := resolveGuts(p.MemberType, p.MaxHP, 0, &p.Effects)
								finalEqual(t, "revival after turn 9", revived, p.MaxHP*30/100)
								second, _ := resolveGuts(p.MemberType, p.MaxHP, 0, &p.Effects)
								finalEqual(t, "no second revival", second, 0)
							}
						}
						if family == 0 || family == 7 || family == 2 || family == 4 || family == 8 || family == 11 {
							for _, tag := range []int{0, 20020006, 20020014} {
								for _, cost := range []int{1, 4, 5, 7, 9} {
									fake := catalog.Cards[99070009]
									fake.ProfileTags = [8]int{tag}
									catalog.Cards[99070009] = fake
									catalog.Skills[99070009][0].Cost = cost
									want := cost
									if family == 0 || family == 7 {
										if cost >= 5 {
											if family == 0 {
												want--
											}
											if strong && tag == 20020006 {
												want -= 2
											}
										}
									} else {
										if tag == 20020014 {
											want--
										}
										if cost >= 5 {
											want -= 2
										}
									}
									finalEqual(t, "cost gate", e.effectiveCardCost(owner, 10, cost), max(0, want))
									e.players[1].Deck[9] = owner.Deck[9]
									finalEqual(t, "cost owner only", e.effectiveCardCost(&e.players[1], 10, cost), cost)
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestCardMod2FinalCallPrograms(t *testing.T) {
	catalog := finalCatalog(t)
	for _, family := range []int{1, 3, 5, 9, 10} {
		for element := 0; element < 5; element++ {
			for _, level := range []int{1, 50, 99} {
				t.Run(fmt.Sprintf("family%d/element%d/level%d", family, element, level), func(t *testing.T) {
					e, a := finalEngine(catalog, family, finalStarts[family]+element, level)
					other := battleAction{memberType: 1, cardType: 2, skill: CombatSkillDefinition{Attribute: a.skill.Attribute}}
					e.turnActions = append(e.turnActions, other)
					if family == 5 {
						a.target = 2
					}
					if _, err := e.executePlayerAction(a, map[string]int{a.skill.Attribute: 1}); err != nil {
						t.Fatal(err)
					}
					if family == 5 {
						player := &e.players[1]
						token := player.handSlotCard(23)
						skill, roles, err := e.handCardSkill(player, token)
						if err != nil {
							t.Fatal(err)
						}
						finalEqual(t, "token cost", skill.Cost, 3)
						finalEqual(t, "token slots", len(roles), 5)
						rows, err := e.executePlayerAction(battleAction{memberType: 2, cardType: 23, cardID: token.CardID, cardLevel: level, skill: skill, target: 5}, map[string]int{skill.Attribute: 1})
						if err != nil {
							t.Fatal(err)
						}
						if _, err = encodeOptionalBattleResults(rows); err != nil {
							t.Fatal(err)
						}
						hits := 0
						for _, r := range rows {
							if r.Command == 60 && r.Args[0] == 5 && r.Args[2] < 0 {
								hits++
							}
						}
						finalEqual(t, "token hits", hits, 4)
						finalEqual(t, "token power", e.cardDisplayPower(player, level, skill, roles), 50119+563071*level/1000+150000)
						for _, p := range e.players {
							finalEqual(t, "token defense", p.Defense, 1000+82*level+1500+82*level+1800)
							finalEqual(t, "token no extra gauge", p.Burst, 20)
							for _, kind := range []string{"PHYSICS", "MAGIC"} {
								boosts := countedDamageSnapshot(p.Effects, a.skill.Attribute, kind)
								if len(boosts) != 1 || boosts[0].Uses != 10 {
									t.Fatal("token counted boosts", boosts)
								}
							}
						}
						return
					}
					owner := &e.players[0]
					finalEqual(t, "registered blessing", len(owner.BlessHolds), 1)
					timing := "ENEMY_ACTION_END"
					if family == 1 {
						timing = "USER_ATTACK_END"
					}
					wrong := "USER_ATTACK_END"
					if timing == wrong {
						wrong = "ENEMY_ACTION_END"
					}
					if rows, err := e.executeBlessHolds(wrong, 22); err != nil || finalHoldCount(rows) != 0 {
						t.Fatal("wrong timing triggered")
					}
					e.turnActions = e.turnActions[:1]
					if rows, err := e.executeBlessHolds(timing, 22); err != nil || finalHoldCount(rows) != 0 {
						t.Fatal("one card triggered blessing")
					}
					e.turnActions = append(e.turnActions, other)
					// Test the blessing without earlier main-skill parameter buffs.
					for i := range e.players {
						p := &e.players[i]
						p.Effects = nil
						p.Attack = 10000
						p.Magic = 10000
						p.LimitAttack = 99999
						p.LimitMagic = 99999
						p.MaxHP = 100000
						p.HP = 10000
						p.Burst = 0
					}
					rows, err := e.executeBlessHolds(timing, 22)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = encodeOptionalBattleResults(rows); err != nil {
						t.Fatal(err)
					}
					executed, hits := 0, 0
					for _, r := range rows {
						if r.Command == CommandHoldSkill {
							executed++
						}
						if r.Command == 60 && r.Args[0] == 5 && r.Args[2] < 0 {
							hits++
						}
					}
					finalEqual(t, "one trigger per phase", executed, 1)
					switch family {
					case 1:
						finalEqual(t, "blessing hits", hits, 7)
						finalEqual(t, "blessing ATK", owner.Attack, 10000+finalG(6080, level)+2500)
						finalEqual(t, "blessing INT", owner.Magic, 10000+finalG(6080, level)+2500)
						// Existing holds extend first; the new hold starts with four turns.
						if _, err := e.executePlayerAction(a, map[string]int{a.skill.Attribute: 1}); err != nil {
							t.Fatal(err)
						}
						finalEqual(t, "old blessing extended", owner.BlessHolds[0].Remaining, 5)
						finalEqual(t, "new blessing duration", owner.BlessHolds[1].Remaining, 4)
						owner.BlessHolds = owner.BlessHolds[:1]
					case 3:
						for _, p := range e.players {
							finalEqual(t, "blessing gauge", p.Burst, 30)
							finalEqual(t, "blessing draw", mod2Effect(t, p.Effects, "DEAL_BONUS", "").Value, 4)
						}
					case 9:
						for _, p := range e.players {
							finalEqual(t, "blessing ATK", p.Attack, 10000+finalG(50000, level))
							finalEqual(t, "blessing INT", p.Magic, 10000+finalG(50000, level))
							finalEqual(t, "blessing limit", p.LimitAttack, 99999+finalG(25000, level))
						}
					case 10:
						for _, p := range e.players {
							finalEqual(t, "blessing HP", p.MaxHP, 100000+finalG(102838, level))
							finalEqual(t, "blessing healing", p.HP, 10000+finalG(27904, level)+finalG(102838, level))
							finalEqual(t, "blessing limit", p.LimitAttack, 99999+finalG(100000, level))
							finalEqual(t, "blessing gauge", p.Burst, 25)
						}
					}
					e.turn++
					e.tickBlessHolds()
					if rows, err := e.executeBlessHolds(timing, 22); err != nil || finalHoldCount(rows) != 1 {
						t.Fatal("repeat blessing failed")
					}
					for len(owner.BlessHolds) > 0 {
						e.turn++
						e.tickBlessHolds()
					}
					if rows, err := e.executeBlessHolds(timing, 22); err != nil || finalHoldCount(rows) != 0 {
						t.Fatal("expired blessing triggered")
					}
				})
			}
		}
	}
}

var finalStarts = []int{10219001, 10219021, 11001205, 10219006, 10219026, 11001210, 10219011, 10219031, 11001215, 10219016, 10219036, 11001220}

func finalHoldCount(rows []BattleResult) int {
	count := 0
	for _, row := range rows {
		if row.Command == CommandHoldSkill {
			count++
		}
	}
	return count
}

var finalSlots = [][]int{{4, 5, 4, 5}, {5, 5}, {4, 5, 5}, {5, 5}, {5, 5, 5}, {5, 4}, {4, 5}, {4, 5, 4, 5}, {4, 5, 4, 5}, {4, 5, 3, 5}, {5}, {3, 5, 5, 4, 5}}
var finalPassiveSlots = [][]int{{2, 3}, {4, 5}, {3, 4}, {3, 4}, {4, 5}, {1, 5}, {4, 5}, {4, 5}, {3, 4}, {2, 3}, {2, 4}, {3, 5}}
var finalGuards = []string{"WIND", "FIRE", "ICE", "DARK", "LIGHT"}

func finalCatalog(t *testing.T) *CombatCatalog {
	t.Helper()
	root := os.Getenv("CN602_CARD_MOD2_FINAL_SET")
	if root == "" {
		t.Skip("set CN602_CARD_MOD2_FINAL_SET to validate all 60 final cards")
	}
	base := filepath.Join(root, "_local/control/server")
	catalog, err := LoadCombatCatalog(filepath.Join(base, "cn602-card-master/card.csv"), filepath.Join(base, "cn602-battle-master"))
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.ValidatePlayerFunctionCoverage(); err != nil {
		t.Fatal(err)
	}
	return catalog
}
func finalEqual(t *testing.T, label string, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %d want %d", label, got, want)
	}
}
func finalG(target, level int) int { return target - target/196*(99-level) }
func finalEngine(catalog *CombatCatalog, family, id, level int) (*BattleEngine, battleAction) {
	e := newSphereContractEngine(catalog)
	owner := &e.players[0]
	owner.ArthurType = []int{1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4}[family]
	owner.Deck[0] = BattleCard{CardID: id, CardType: 1, Level: level}
	for n := range owner.DeckOrder {
		owner.DeckOrder[n] = -1
	}
	skill := catalog.Skills[catalog.Cards[id].ArthurSkillID][0]
	for n := 1; n < 10; n++ {
		fakeID := 99070000 + n
		fake := CombatCardDefinition{ID: fakeID, NormalSkillID: fakeID}
		if n == 2 {
			fake.ProfileTags[0] = 20020006
		}
		catalog.Cards[fakeID] = fake
		fakeSkill := skill
		fakeSkill.ID, fakeSkill.Attribute, fakeSkill.Cost = fakeID, "NEUTRAL", 0
		catalog.Skills[fakeID] = []CombatSkillDefinition{fakeSkill}
		owner.Deck[n] = BattleCard{CardID: fakeID, CardType: n + 1, Level: level}
	}
	owner.DeckOrder[0], owner.DeckOrder[1] = 1, 2
	for i := range e.players {
		e.players[i].BurstState = burstGaugeNormal
		e.players[i].HP = 10000
	}
	e.enemyCount = 2
	e.enemies[1] = e.enemies[0]
	e.enemies[1].MemberType = 6
	for i := 0; i < 2; i++ {
		e.enemies[i].Attack = 100000
		e.enemies[i].BaseAttack = 100000
		e.enemies[i].Magic = 100000
		e.enemies[i].BaseMagic = 100000
		e.enemies[i].Trance = battleEnemyTrance{State: 2, Value: 800, NormalLimit: 1000, OverheatLimit: 1000, Reactive: &EnemyGaugeDefinition{}}
	}
	a := battleAction{memberType: 1, cardID: id, cardType: 1, cardLevel: level, skill: skill, target: 5}
	if callID := catalog.Cards[id].CallSkillID; callID != 0 {
		a.callSkill = catalog.Skills[callID][0]
		a.callRoles = catalog.SkillRoles[a.callSkill.FunctionID]
	}
	e.turnActions = []battleAction{a}
	return e, a
}

func TestCardMod2FinalAllCards(t *testing.T) {
	catalog := finalCatalog(t)
	for family, start := range finalStarts {
		for element := 0; element < 5; element++ {
			id := start + element
			t.Run(fmt.Sprint(id), func(t *testing.T) {
				card := catalog.Cards[id]
				finalEqual(t, "active ID", card.ArthurSkillID, 93000000+(family*5+element)*100)
				if err := catalog.ValidateMainCardPassives(card); err != nil {
					t.Fatal(err)
				}
				for i, skillID := range []int{card.ArthurSkillID, card.ArthurPassiveSkillID} {
					want := finalSlots[family]
					if i == 1 {
						want = finalPassiveSlots[family]
					}
					branches := catalog.Skills[skillID]
					finalEqual(t, "branch count", len(branches), len(want))
					for n, s := range branches {
						finalEqual(t, "effect slots", len(catalog.SkillRoles[s.FunctionID]), want[n])
					}
				}
				for _, level := range []int{1, 50, 99} {
					for variant := range finalSlots[family] {
						t.Run(fmt.Sprintf("active/%d/%d", level, variant), func(t *testing.T) {
							e, a := finalEngine(catalog, family, id, level)
							chain := 1
							other := battleAction{memberType: 1, cardType: 2, skill: CombatSkillDefinition{Attribute: a.skill.Attribute, Kind: "DEFENSE"}}
							switch family {
							case 0, 7:
								count := 9
								if variant&2 != 0 {
									count = 10
								}
								for n := 1; n < count; n++ {
									x := other
									x.memberType = 2
									x.cardType = n + 1
									if n == 1 {
										x.memberType = 1
										x.skill.Cost = 5
										if variant&1 != 0 {
											x.skill.Cost = 6
										}
									}
									e.turnActions = append(e.turnActions, x)
								}
							case 1, 2, 3:
								if variant > 0 {
									e.turnActions = append(e.turnActions, other)
								}
								if family == 2 && variant == 2 {
									chain = 4
								}
							case 4:
								if variant > 0 {
									other.skill.Cost = 2
									if variant == 2 {
										other.skill.Cost = 6
									}
									e.turnActions = append(e.turnActions, other)
								}
							case 5:
								a.target = 1
								if variant == 1 {
									a.target = 2
								}
								e.players[0].ArthurType = 1
							case 6:
								if variant == 1 {
									e.turnActions = append(e.turnActions, other)
									chain = 4
								}
							case 8:
								if variant&1 != 0 {
									e.turnActions = append(e.turnActions, other)
								}
								if variant&2 != 0 {
									chain = 4
								}
							case 9:
								if variant >= 2 {
									e.turn = 3
								}
								if variant%2 != 0 {
									e.turnActions = append(e.turnActions, other)
								}
							case 11:
								a.target = 1
								e.players[0].ArthurType = variant
							}
							// A sphere never contributes to selected-card counts or cost.
							e.turnActions = append(e.turnActions, battleAction{memberType: 1, sphereSlot: 1, skill: CombatSkillDefinition{Attribute: a.skill.Attribute, Cost: 99}})
							chains := map[string]int{a.skill.Attribute: chain}
							selected, branch := e.selectCombatSkillBranchWithIndex(a, e.turnActions, chains)
							finalEqual(t, "selected branch", branch, variant)
							roles := catalog.SkillRoles[selected.FunctionID]
							rows, err := e.executePlayerAction(a, chains)
							if err != nil {
								t.Fatal(err)
							}
							if _, err = encodeOptionalBattleResults(rows); err != nil {
								t.Fatal(err)
							}
							gauge := 0
							for _, role := range roles {
								if role.Function == "BURST_GAUGE_QUICK_UP" {
									gauge = combatParameterInt(role.Parameters[0])
								}
							}
							for _, p := range e.players {
								finalEqual(t, "team gauge", p.Burst, gauge)
							}
							effect := func(p int, fn, param string) battleEffect { return mod2Effect(t, e.players[p].Effects, fn, param) }
							enemyEffect := func(fn, param string) battleEffect { return mod2Effect(t, e.enemies[0].Effects, fn, param) }
							switch family {
							case 0, 7:
								hits := [2]int{}
								heat := [2]int{}
								for _, r := range rows {
									if r.Command == 60 && len(r.Args) > 2 && r.Args[0] >= 5 && r.Args[0] <= 6 && r.Args[2] < 0 {
										hits[r.Args[0]-5]++
									}
									if r.Command == 205 {
										heat[r.Args[0]-5]++
									}
								}
								count := 4
								if variant&2 != 0 {
									count = 7
								}
								if hits != [2]int{count, count} || heat != [2]int{1, 1} {
									t.Fatalf("hits/heat %v/%v", hits, heat)
								}
								for _, enemy := range e.enemies[:2] {
									finalEqual(t, "remaining heat", enemy.Trance.Value, 600)
								}
								modifiers := collectAttackModifiers(roles, &e.players[0], level, chain, false)
								finalEqual(t, "pierce", modifiers.piercingRate, 100)
								extra := 0
								if variant&1 != 0 {
									extra = 100000
								}
								finalEqual(t, "extra attack source", modifiers.damageIncrease, extra)
								finalEqual(t, "attack display", e.cardDisplayPower(&e.players[0], level, selected, roles), finalG(329803, level)+100000+extra)
							case 1:
								n := 1
								if variant == 1 {
									n = 5
								}
								finalEqual(t, "resistance down", enemyEffect("ATTR_DEF_DOWN", "").Value, finalG(157560, level)*n)
								finalEqual(t, "enchant", effect(0, "ENCHANT", "").Value, finalG(157560, level)*n)
							case 2:
								n := variant + 1
								for i, p := range e.players {
									bonus := 0
									if i == 0 || variant == 2 {
										bonus = finalG(41775, level) * n
									}
									finalEqual(t, "ATK", p.Attack, 10000+bonus)
									finalEqual(t, "INT", p.Magic, 10000+bonus)
									finalEqual(t, "ATK limit", p.LimitAttack, 99999+bonus)
									finalEqual(t, "INT limit", p.LimitMagic, 99999+bonus)
								}
							case 3:
								rate := 15
								if variant == 1 {
									rate = 18
								}
								for i, p := range e.players {
									finalEqual(t, "DEF", p.Defense, 1000+100*level+100*rate)
									shield := effect(i, "ATTACK_BARRIER_APPOINT_ATTR", "")
									finalEqual(t, "shield threshold", shield.Value, finalG(1000000, level))
									finalEqual(t, "shield hits", shield.Uses, 2)
									if shield.Attribute != finalGuards[element] {
										t.Fatal("wrong shield element")
									}
								}
								finalEqual(t, "cover rate", effect(0, "COVERING", "").Rate, 900)
							case 4:
								n := variant + 1
								res := []int{31007, 62015, 93023}[variant]
								for i := range e.players {
									finalEqual(t, "enchant", effect(i, "ENCHANT", "").Value, finalG(240000, level)*n)
									guard := effect(i, "ATTR_DEF_UP", "")
									// Native resistance-up stores the fixed damage delta as negative.
									finalEqual(t, "resistance up", guard.Value, -finalG(res, level))
									if guard.Attribute != finalGuards[element] || effect(i, "ATTR_CUT_ATTACK", "").Attribute != finalGuards[element] {
										t.Fatal("wrong defensive element")
									}
								}
							case 5:
								for _, p := range e.players {
									finalEqual(t, "DEF", p.Defense, 1000+82*level+1500)
								}
								if variant == 0 {
									for _, kind := range []string{"PHYSICS", "MAGIC"} {
										boost := countedDamageSnapshot(e.players[0].Effects, a.skill.Attribute, kind)
										if len(boost) != 1 || boost[0].Uses != 10 || countedDamageHitRate(boost, 9) != 200 || countedDamageHitRate(boost, 10) != 0 {
											t.Fatal("selected counted boost", boost)
										}
										if len(countedDamageSnapshot(e.players[1].Effects, a.skill.Attribute, kind)) != 0 {
											t.Fatal("boost leaked")
										}
									}
								} else {
									finalEqual(t, "token CALL", e.players[1].Tokens[0].SkillID, card.CallSkillID)
									finalEqual(t, "token level", e.players[1].Tokens[0].Level, level)
								}
							case 6:
								finalEqual(t, "mark", weaknessAdjustedPower(1000, e.enemies[0].Effects), 2200)
								finalEqual(t, "mark target", weaknessAdjustedPower(1000, e.enemies[1].Effects), 1000)
								rate := 15
								if variant == 1 {
									rate = 18
								}
								for i, enemy := range e.enemies[:2] {
									delta := 0
									if i == 0 || variant == 1 {
										delta = 70*level + 100*rate
									}
									finalEqual(t, "enemy ATK", enemy.Attack, 100000-delta)
									finalEqual(t, "enemy INT", enemy.Magic, 100000-delta)
								}
								if variant == 1 {
									finalEqual(t, "resistance down", enemyEffect("ATTR_DEF_DOWN", "").Value, finalG(930231, level))
								}
							case 8:
								n := 1 + (variant & 1) + ((variant >> 1) & 1)
								for _, p := range e.players {
									finalEqual(t, "INT", p.Magic, 10000+finalG(20052, level)*n)
									finalEqual(t, "INT limit", p.LimitMagic, 99999+finalG(30843, level)*n)
								}
								finalEqual(t, "resistance down", enemyEffect("ATTR_DEF_DOWN", "").Value, finalG(310077, level)*n)
							case 9:
								n := 1
								if variant < 2 {
									n = 2
								}
								for i, p := range e.players {
									finalEqual(t, "HP cap", p.MaxHP, 100000+finalG(51419, level)*n)
									finalEqual(t, "regen", effect(i, "REGENERATE_FIXED", "").Value, finalG(10010, level)*n)
									finalEqual(t, "crit", effect(i, "CRITICAL_UP", "").Rate, 1000*n)
								}
							case 10:
								for _, p := range e.players {
									finalEqual(t, "ATK", p.Attack, 10000+206*level+1000)
									finalEqual(t, "INT", p.Magic, 10000+206*level+1000)
									finalEqual(t, "DEF", p.Defense, 1000+206*level+1000)
									finalEqual(t, "MDEF", p.MDefense, 1000+206*level+1000)
								}
							case 11:
								for _, p := range e.players {
									bonus := 0
									if variant == 2 {
										bonus = finalG(102838, level)
									}
									finalEqual(t, "heal", p.HP, 10000+finalG(20854, level)+bonus)
									finalEqual(t, "HP cap", p.MaxHP, 100000+bonus)
								}
								if variant == 2 {
									finalEqual(t, "enchant", effect(0, "ENCHANT", "").Value, finalG(620114, level))
								}
								if variant == 3 {
									finalEqual(t, "resistance down", enemyEffect("ATTR_DEF_DOWN", "").Value, finalG(1240308, level))
								}
								if variant == 4 {
									_, critical := e.nativeCriticalOutcome(e.players[0].Effects, 0)
									finalEqual(t, "critical multiplier", critical, 300)
								}
							}
							needsSearch := family == 2 && variant > 0 || family == 8 && variant&1 != 0 || family == 11
							if needsSearch {
								finalEqual(t, "round-table search", e.players[0].DeckOrder[0], 2)
							}
							blessing := family == 1 || family == 3 || family == 10 || family == 9 && variant%2 == 1
							if blessing {
								finalEqual(t, "blessing count", len(e.players[0].BlessHolds), 1)
								hold := e.players[0].BlessHolds[0]
								finalEqual(t, "blessing duration", hold.Remaining, 4)
								finalEqual(t, "CALL level", hold.CardLevel, level)
								if !hold.Repeat || hold.Skill.ID != card.CallSkillID {
									t.Fatal("blessing payload")
								}
							}
						})
					}
				}
			})
		}
	}
}
