"""Build all 60 cards from the approved final MD into an isolated resource set."""
import argparse
from collections import Counter
import csv, hashlib, io, json, re, shutil
from pathlib import Path

SPEC_SHA256="921c0962c8a849eaad0a1f48085bb4cd9d8dab8d58c654144f82fc76a499728e"
ATTRIBUTES=("FIRE","ICE","WIND","LIGHT","DARK")
ELEMENTS=("火","冰","风","光","暗")
GUARDS=("WIND","FIRE","ICE","DARK","LIGHT")
ROUND_TABLE_TAG="20020006"
HOLY_KING_TAG="20020014"
FAMILIES=(("莫德雷德",10219001),("兰斯洛特",10219021),("高文",11001205),("崔斯坦",10219006),("欧文",10219026),("加拉哈德",11001210),("加雷斯",10219011),("贝德维尔",10219031),("卡拉德克",11001215),("帕罗米德",10219016),("珀西瓦尔",10219036),("佩里亚丝",11001220))
CARD_IDS={start+n for _,start in FAMILIES for n in range(5)}
TABLES=("card.csv","skill_player.csv","skill_role_player.csv","support_skill.csv","support_skill_role.csv")
EXPECTED_SLOTS=((4,5,4,5),(5,5),(4,5,5),(5,5),(5,5,5),(5,4),(4,5),(4,5,4,5),(4,5,4,5),(4,5,3,5),(5,),(3,5,5,4,5))
PASSIVE_SLOTS=((2,3),(4,5),(3,4),(3,4),(4,5),(1,5),(4,5),(4,5),(3,4),(2,3),(2,4),(3,5))

def require(ok,message):
    if not ok: raise ValueError(message)

def sha(raw): return hashlib.sha256(raw).hexdigest()
def read_rows(raw): return list(csv.reader(io.StringIO(raw.decode("utf-8-sig"))))
def encode_rows(rows):
    stream=io.StringIO(newline="")
    csv.writer(stream,lineterminator="\n").writerows(rows)
    return stream.getvalue().encode("utf-8")
def unique_row(rows,record_id):
    matches=[r for r in rows if r and r[0]==str(record_id)]
    require(len(matches)==1,f"Expected one record {record_id}")
    return matches[0]
def replace_cards(raw,replacements,originals):
    lines=raw.splitlines(keepends=True)
    for card_id,replacement in replacements.items():
        indices=[i for i,line in enumerate(lines) if line.lstrip(b"\xef\xbb\xbf").startswith(f"{card_id},".encode())]
        require(len(indices)==1,f"Expected single-line card {card_id}")
        i=indices[0];line=lines[i]
        require(read_rows(line)==[originals[card_id]],f"Unexpected baseline {card_id}")
        ending=b"\r\n" if line.endswith(b"\r\n") else b"\n" if line.endswith(b"\n") else b""
        bom=b"\xef\xbb\xbf" if line.startswith(b"\xef\xbb\xbf") else b""
        lines[i]=bom+encode_rows([replacement]).rstrip(b"\n")+ending
    return b"".join(lines)
def growth(target,multiplier=1):
    q=target//196
    return (target-99*q)*multiplier,q*multiplier
def level_value(target,level,multiplier=1):
    b,q=growth(target,multiplier)
    return b+q*level
def skill(base,skill_id,role_id,priority,description,conditions=()):
    row=base.copy()
    row[0],row[3],row[4],row[5]=str(skill_id),description,"1","1"
    row[28:49]=[""]*21
    require(len(conditions)<=2,"More than two conditions")
    for i,c in enumerate(conditions):
        require(len(c)<=6,"Too many condition parameters")
        row[36+i*6:36+i*6+len(c)]=list(map(str,c))
    row[48],row[49]=str(priority),str(role_id)
    return row
def role(template,function,target,*parameters,chain=0):
    require(len(parameters)<=10,"Too many role parameters")
    row=template.copy();row[1:8]=[""]*7
    row[8:20]=[function,target,""]+["1"]*9
    row[20:30]=list(map(str,parameters))+[""]*(10-len(parameters))
    row[30:]=[str(chain),"",""]
    return row
def validate_programs(skills,roles,known_conditions,known_functions):
    require(all(n<=5 for n in Counter(r[0] for r in skills).values()),"More than five branches")
    counts=Counter(r[0] for r in roles)
    require(all(n<=5 for n in counts.values()),"More than five effects")
    for row in skills:
        require(all(row[i] in known_conditions|{"","NONE"} for i in (36,42)),f"Unknown client condition {row[0]}")
        require(row[49] in counts and 1<=int(row[5])<=counts[row[49]],f"Invalid display role {row[0]}")
    require(all(r[8] in known_functions for r in roles),"Unknown client effect")
def descriptions(spec,family,element,guard):
    block=spec.split(f"## {family+1}. ",1)[1].split("\n## ",1)[0]
    active,passive=block.split("### 绝技",1)[1].split("### 独立被动",1)
    call=""
    for marker in ("**祝福：**","**衍生卡：3C"):
        if marker in active:
            active,call=active.split(marker,1)
            call=marker+call
            break
    def clean(text,active):
        lines=[];headers=[]
        for line in text.splitlines():
            line=line.strip()
            if line.startswith("|"):
                cells=[x.strip() for x in line.strip("|").split("|")]
                if all(re.fullmatch(r"[-: ]+",x) for x in cells): continue
                if not headers: headers=cells;continue
                useful=[f"{key}：{value}" for key,value in zip(headers,cells) if "槽" not in key or "圣剑" in key]
                if len(useful)>1: lines.append("；".join(useful))
                continue
            headers=[]
            if not line or line.startswith(("槽位","共2档","**本体","本体：","衍生卡：","**基础兜底")): continue
            if "槽" in line and ("分支" in line or "分别计槽" in line): continue
            line=re.sub(r"[*\x60]","",line).lstrip("- ")
            lines.append(line)
        text="<br>".join(["【点数按99级标示；实际按技能等级成长】" if active else "【独立被动：全等级固定】",*lines])
        return re.sub(r"[*\x60]","",text).replace("对应防护属性",guard+"属性").replace("本属性",element+"属性")
    return clean(active,True),clean(passive,False),clean(call,True) if call else ""

def build_programs(family,index,attr,active_base,passive_base,templates,spec,token_id):
    attribute,guard=ATTRIBUTES[attr],GUARDS[attr]
    active_id,passive_id,call_id=93000000+index*100,93000050+index*100,93000020+index*100
    active,active_roles,passive,passive_roles=[],[],[],[]
    a,p=active_base.copy(),passive_base.copy()
    a[14]=str((9,1,1,3,1,1,2,9,1,1,1,1)[family])
    p[9]="PASSIVE";a[11]=p[11]=attribute
    active_text,passive_text,call_text=descriptions(spec,family,ELEMENTS[attr],ELEMENTS[ATTRIBUTES.index(guard)])
    def r(fn,target,*args,chain=0): return role(templates[fn],fn,target,*args,chain=chain)
    def fixed(fn,target,param,amount,turns=99,growing=False,multiplier=1):
        b,q=growth(amount,multiplier) if growing else (amount*multiplier,0)
        return r(fn,target,turns,param,1,b*1000,q*1000,0)
    def pair(fn,target,params,amount,turns=99,growing=False,multiplier=1):
        return [fixed(fn,target,x,amount,turns,growing,multiplier) for x in params]
    def scaled(fn,target,params,source,rate,per_level,turns):
        return [r(fn,target,turns,x,source,rate*10,0,per_level,0) for x in params]
    def resist(fn,amount,multiplier=1):
        b,q=growth(amount,multiplier)
        return r(fn,"ENEMY_ALL" if fn=="ATTR_DEF_DOWN" else "FRIEND_ALL",3,0,0,b,q*1000,attribute if fn=="ATTR_DEF_DOWN" else guard)
    def enchant(amount,multiplier=1):
        b,q=growth(amount,multiplier)
        return r("ENCHANT","FRIEND_ALL",3,b,1000,0,q,attribute)
    def gauge(amount): return r("BURST_GAUGE_QUICK_UP","FRIEND_ALL",amount,0)
    def bless(): return r("BLESS","SELF",4,0,1)
    def heal(amount):
        b,q=growth(amount)
        return r("HEAL_FIXED","FRIEND_ALL",b,q*1000,0,0,"MND")
    def damage(target,rate,kind="ALL"): return r("DAMAGE_BOOST",target,99,rate*10,0,0,0,attribute,kind,0,0)
    def boost(fn,target,rate,param=None):
        return r(fn,target,99,0,0,rate*10,0,attribute,*([param,0,0] if param else [0,0]))
    def counted(target,uses=10):
        return [r("DAMAGE_BOOST_COUNT2",target,1,uses,200,0,attribute,x) for x in ("PHYSICS","MAGIC")]
    def attack(amount,coefficient,hits,source="ATK",chain=0,critical=150,native=False):
        b,q=growth(amount)
        return r("ATTACK_AA","ENEMY_ALL",50119 if native else b,563071 if native else q*1000,coefficient*10,0,hits,source,critical,attribute,"PHYSICS" if source=="ATK" else "MAGIC",chain=chain)
    attr_two=[("SELF_OTHER_PLAY_ATTR",attribute,1)]
    two=[("SELF_PLAY_COST_NUM",0,0,2,0)]
    chain4=[("DECK_COMBO_COUNT",4,0)]
    deck=[("SELF_MAIN_DECK_ATTR",attribute,"NULL",5)]
    round_two=[("SELF_MAIN_DECK_TAG",ROUND_TABLE_TAG,0,0,2)]
    variants=[]
    def add(effects,conditions=(),priority=None): variants.append((effects,conditions,len(variants) if priority is None else priority))
    if family in (0,7):
        source,opposite=("ATK","INT") if family==0 else ("INT","ATK")
        old=templates["own_attack"]
        for mask in range(4):
            cond=([("SELF_PLAY_COST_TOTAL",15,0)] if mask&1 else [])+([("FRIEND_PLAY_ATTR_NUM",attribute,10)] if mask&2 else [])
            effects=[r("ATK_OP_PIERCING","SELECT",100,0)]
            if mask&1: effects.append(r("ATK_OP_DAMAGE_INCREASE","SELF",0,0,10000,0,opposite,0))
            effects += [attack(329803,1000,7 if mask&2 else 4,source,int(old[30] or 0),int(old[26])),gauge(50),r("TRANCE_GAUGE_VALUE_DOWN","ENEMY_ALL","TRANCE",20,0)]
            add(effects,cond)
    elif family==1:
        for n in (1,5): add([resist("ATTR_DEF_DOWN",157560,n),enchant(157560,n),gauge(20 if n==1 else 30),r("BLESS_TURN_UP","SELF",1,"NULL"),bless()],attr_two if n==5 else [])
    elif family==2:
        for n,target,cond in [(1,"SELF",[]),(2,"SELF",attr_two),(3,"FRIEND_ALL",attr_two+chain4)]:
            effects=pair("ATK_UP_FIXED",target,("ATK","INT"),41775,3,True,n)+pair("PARAM_LIMIT_BREAK_FIXED",target,("ATK","INT"),41775,3,True,n)
            if n>1: effects.append(r("CARD_SEARCH","SELF",1,"NULL","NULL",ROUND_TABLE_TAG))
            add(effects,cond)
    elif family==3:
        b,q=growth(1000000)
        for rate in (15,18):
            effects=[r("COVERING","SELF",1,900,0,"NULL","ALL"),r("ATTACK_BARRIER_APPOINT_ATTR","FRIEND_ALL",1,b,q,2,"ALL",guard)]
            effects+=scaled("DEF_UP_BY_SELF_PARAM","FRIEND_ALL",("DEF","MDEF"),"INT",rate,100,2)+[bless()]
            add(effects,[("SELF_OTHER_PLAY_SKILL_KIND","DEFENSE",1)] if rate==18 else [])
    elif family==4:
        for n,rate,res in [(1,10,31007),(2,30,62015),(3,50,93023)]:
            add([r("ATTR_CUT_ATTACK","FRIEND_ALL",1,0,rate*10,0,guard,"ALL"),enchant(240000,n),resist("ATTR_DEF_UP",res),gauge(n*10),r("DEAL_BONUS","FRIEND_ALL",4)],[("SELF_PLAY_COST_TOTAL",3 if n==2 else 7,0)] if n>1 else [])
    elif family==5:
        for rich in (False,True):
            effects=[gauge(20)]+scaled("DEF_UP_BY_SELF_PARAM","FRIEND_ALL",("DEF","MDEF"),"ATK",15,82,3)
            effects += [r("CARD_TOKEN","SELECT",0,1,token_id)] if rich else counted("SELECT")
            add(effects,[("TARGET_JOB_TYPE","MILLIONAIRE")] if rich else [])
    elif family==6:
        for strong in (False,True):
            effects=[r("WEAKNESS","SELECT",99,1200,0)]+scaled("ATK_BREAK_BY_SELF_PARAM","ENEMY_ALL" if strong else "SELECT",("ATK","INT"),"INT",18 if strong else 15,70,99)+[gauge(35 if strong else 20)]
            if strong: effects.append(resist("ATTR_DEF_DOWN",930231))
            add(effects,two+chain4 if strong else [])
    elif family==8:
        for mask in range(4):
            n=1+bool(mask&1)+bool(mask&2)
            effects=[fixed("PARAM_LIMIT_BREAK_FIXED","FRIEND_ALL","INT",30843,3,True,n),fixed("ATK_UP_FIXED","FRIEND_ALL","INT",20052,3,True,n),resist("ATTR_DEF_DOWN",310077,n),gauge(20)]
            if mask&1: effects.append(r("CARD_SEARCH","SELF",1,"NULL","NULL",ROUND_TABLE_TAG))
            add(effects,(attr_two if mask&1 else [])+(chain4 if mask&2 else []))
    elif family==9:
        for early,multiple,priority in [(True,False,1),(True,True,3),(False,False,0),(False,True,2)]:
            n=2 if early else 1;b,q=growth(10010,n)
            effects=[r("REGENERATE_FIXED","FRIEND_ALL",99,b,q*1000,0,0,"MND"),fixed("ATK_UP_FIXED","FRIEND_ALL","MAX_HP",51419,99,True,n),r("CRITICAL_UP","FRIEND_ALL",99,1000*n,0)]
            if early or multiple: effects.append(gauge(20*(int(early)+int(multiple))))
            if multiple: effects.append(bless())
            add(effects,([("TURN",0,2)] if early else [])+(two if multiple else []),priority)
    elif family==10:
        add(scaled("ATK_UP_BY_SELF_PARAM","FRIEND_ALL",("ATK","INT"),"HP",10,206,4)+scaled("DEF_UP_BY_SELF_PARAM","FRIEND_ALL",("DEF","MDEF"),"HP",10,206,4)+[bless()])
    elif family==11:
        for job in (None,"MERCENARY","MILLIONAIRE","THIEF","SINGER"):
            effects=[heal(20854),gauge(20),r("CARD_SEARCH","SELF",1,"NULL","NULL",ROUND_TABLE_TAG)]
            if job=="MERCENARY": effects+=counted("FRIEND_ALL",5)
            elif job=="MILLIONAIRE": effects += [enchant(620114),fixed("ATK_UP_FIXED","FRIEND_ALL","MAX_HP",102838,3,True)]
            elif job=="THIEF": effects.append(resist("ATTR_DEF_DOWN",1240308))
            elif job=="SINGER": effects += [r("CRITICAL_DAMAGE_BOOST","FRIEND_ALL",1,150),r("CRITICAL_UP","FRIEND_ALL",1,1000,0)]
            add(effects,[("TARGET_JOB_TYPE",job)] if job else [])
    def emit(rows,role_rows,base,program_id,branch,effects,conditions,priority,description,visual):
        s=skill(base,program_id,program_id+branch,priority,description if branch==0 else "",conditions)
        copies=[x.copy() for x in effects]
        for i,e in enumerate(copies):
            e[0]=s[49]
            if i==0: e[1:8],e[31]=visual[1:8],"10000"
        for i,e in enumerate(copies):
            if e[8]=="ATTACK_AA": s[4],s[5]=str(i*10+1),str(i+1);break
        rows.append(s);role_rows.extend(copies)
        return s
    require(tuple(len(x[0]) for x in variants)==EXPECTED_SLOTS[family],f"Active slots {family}")
    for v,(effects,conditions,priority) in enumerate(variants):
        emit(active,active_roles,a,active_id,v,effects,conditions,priority,active_text,templates["active_visual"])
    call_effects=None
    if family==1: call_effects=[attack(10180,300,7)]+pair("ATK_UP_FIXED","SELF",("ATK","INT"),6080,3,True)+scaled("ATK_UP_BY_SELF_PARAM","SELF",("ATK","INT"),"HP",25,0,3)
    elif family==3: call_effects=[gauge(30),r("DEAL_BONUS","FRIEND_ALL",4)]
    elif family==5:
        old=templates["token_attack"]
        call_effects=[attack(105863,1500,4,chain=int(old[30] or 0),critical=int(old[26]),native=True)]+scaled("DEF_UP_BY_SELF_PARAM","FRIEND_ALL",("DEF","MDEF"),"ATK",18,82,3)+counted("FRIEND_ALL")
    elif family==9: call_effects=pair("ATK_UP_FIXED","FRIEND_ALL",("ATK","INT"),50000,3,True)+pair("PARAM_LIMIT_BREAK_FIXED","FRIEND_ALL",("ATK","INT"),25000,3,True)
    elif family==10: call_effects=[heal(27904)]+pair("PARAM_LIMIT_BREAK_FIXED","FRIEND_ALL",("ATK","INT"),100000,4,True)+[gauge(25),fixed("ATK_UP_FIXED","FRIEND_ALL","MAX_HP",102838,3,True)]
    if call_effects:
        c=a.copy();c[1]=FAMILIES[family][0]+("衍生卡" if family==5 else "祝福")
        c[10],c[13],c[14],c[19]=("ATTACK","PHYSICS","3","ENEMY_ALL") if family==5 else ("SUPPORT","MAGIC","0","SELF")
        if family==1: c[10],c[13],c[19]="ATTACK","PHYSICS","ENEMY_ALL"
        s=emit(active,active_roles,c,call_id,0,call_effects,[],0,call_text,templates["active_visual"])
        if family!=5: s[28:36]=["USER_ATTACK_END" if family==1 else "ENEMY_ACTION_END","4","SELF_OTHER_PLAY_ATTR",attribute,"2","","",""]
    for strong in (False,True):
        target="FRIEND_ALL" if strong else "SELF"
        conditions=round_two if family in (2,5,8,11) else deck
        if family==0:
            effects=[damage("SELF",18),r("NEED_COST_DOWN","SELF",99,1,"ALL","NULL","NULL",0,5,0,0)]
            if strong: effects.append(r("NEED_COST_DOWN","SELF",99,2,"ALL","NULL","NULL",ROUND_TABLE_TAG,5,0,0))
        elif family==1:
            effects=pair("PARAM_LIMIT_BREAK_FIXED",target,("ATK","INT"),80000)
            if strong: effects.append(damage("FRIEND_ALL",6))
            effects += [boost("ATK_UP_BOOST","SELF",20,x) for x in ("ATK","INT")]
        elif family==2:
            effects=[damage(target,12 if strong else 6)]
            if strong: effects.append(gauge(15))
        elif family==3:
            effects=[r("BEGINNING_DRAW","SELF")]+[boost("DEF_UP_BOOST","SELF",18 if strong else 9,x) for x in ("DEF","MDEF")]
            if strong: effects.append(r("DAMAGE_CUT2","SELF",99,0,0,100,0,guard,"ALL",0,0))
        elif family==4:
            effects=pair("DEF_UP_FIXED","FRIEND_ALL",("DEF","MDEF"),30000 if strong else 15000)
            if strong: effects.append(r("GUTS","FRIEND_ALL",99,1,30))
        elif family==5:
            effects=[damage("FRIEND_ALL",15 if strong else 10,"ALL" if strong else "PHYSICS")]
            if strong:
                effects += [fixed("PARAM_LIMIT_BREAK_FIXED","FRIEND_ALL","ATK",25000),fixed("ATK_UP_FIXED","FRIEND_ALL","ATK",50000)]+[r("GUARD_BREAK_CUT","FRIEND_ALL",99,0,0,150,0,x) for x in ("DEF","MDEF")]
        elif family==6:
            effects=[r("BEGINNING_DRAW","SELF"),boost("ATTR_DEF_DOWN_BOOST","SELF",50)]+[boost("ATK_BREAK_BOOST","SELF",12 if strong else 6,x) for x in ("ATK","INT")]
            if strong: effects.append(fixed("ATK_UP_FIXED","FRIEND_ALL","MAX_HP",12000))
        elif family==7:
            effects=[fixed("ATK_UP_FIXED","FRIEND_ALL","INT",50000),fixed("PARAM_LIMIT_BREAK_FIXED","FRIEND_ALL","INT",25000),damage("SELF",16),boost("ATK_UP_BOOST","SELF",20,"INT")]
            if strong: effects.append(r("NEED_COST_DOWN","SELF",99,2,"ALL","NULL","NULL",ROUND_TABLE_TAG,5,0,0))
        elif family==8:
            effects=[damage("FRIEND_ALL",15 if strong else 10,"ALL" if strong else "MAGIC")]
            if strong: effects.append(boost("ATTR_DEF_DOWN_BOOST","FRIEND_ALL",100))
        elif family==9:
            effects=pair("PARAM_LIMIT_BREAK_FIXED","FRIEND_ALL",("ATK","INT"),60000 if strong else 40000)
            if strong: effects.append(damage("FRIEND_ALL",10))
        elif family==10:
            effects=[fixed("ATK_UP_FIXED","FRIEND_ALL","MAX_HP",12000),damage("FRIEND_ALL",10)]
            if strong: effects += [boost("ATK_UP_BOOST","FRIEND_ALL",30,x) for x in ("ATK","INT")]
        elif family==11:
            effects=[gauge(15 if strong else 10)]
            if strong: effects += [r("ATK_BREAK_CUT","FRIEND_ALL",99,0,0,150,0,x) for x in ("ATK","INT")]
        if family in (2,4,8,11):
            effects += [r("NEED_COST_DOWN","SELF",99,1,"ALL","NULL","NULL",HOLY_KING_TAG,0,0,0),r("NEED_COST_DOWN","SELF",99,2,"ALL","NULL","NULL",0,5,0,0)]
        require(len(effects)==PASSIVE_SLOTS[family][int(strong)],f"Passive slots {family}")
        emit(passive,passive_roles,p,passive_id,int(strong),effects,conditions if strong else [],int(strong),passive_text,templates["passive_visual"])
    return active,active_roles,passive,passive_roles,call_id if call_effects else 0

def build(source,output,spec_path):
    spec_raw=spec_path.read_bytes()
    require(sha(spec_raw)==SPEC_SHA256,"Final MD changed; review and update generator")
    spec=spec_raw.decode("utf-8-sig");source,output=source.resolve(),output.resolve()
    require(not output.exists() and not output.is_relative_to(source) and not source.is_relative_to(output),"Output must be new and outside source")
    manifest_raw=(source/"resource-set.json").read_bytes();manifest=json.loads(manifest_raw)
    card_path=Path(manifest["entrypoints"]["cn-combat-card-master"]);battle_path=Path(manifest["entrypoints"]["cn-combat-master-root"])
    paths={name:card_path if name=="card.csv" else battle_path/name for name in TABLES}
    for path in paths.values(): require(not path.is_absolute() and ".." not in path.parts and (source/path).resolve().is_relative_to(source),"Unsafe resource path")
    original={name:(source/path).read_bytes() for name,path in paths.items()}
    tables={name:read_rows(raw) for name,raw in original.items()}
    all_skills=tables["skill_player.csv"]+tables["support_skill.csv"]
    all_roles=tables["skill_role_player.csv"]+tables["support_skill_role.csv"]
    additions={name:[] for name in TABLES[1:]};replacements,old_cards,cards={},{},[]
    for family,(_,start) in enumerate(FAMILIES):
        for attr in range(5):
            card_id=start+attr;index=family*5+attr;card=unique_row(tables["card.csv"],card_id)
            require(not card[39].startswith("93"),"Input already has final patch")
            active=next(r for r in tables["skill_player.csv"] if r[0]==card[39])
            passive=next(r for r in tables["support_skill.csv"] if r[0]==card[46])
            own_ids={r[49] for r in tables["skill_player.csv"] if r[0]==card[39]}
            own=[r for r in tables["skill_role_player.csv"] if r[0] in own_ids]
            templates={r[8]:r for r in reversed(all_roles) if len(r)>=33}
            templates["active_visual"]=own[0]
            templates["passive_visual"]=next(r for r in tables["support_skill_role.csv"] if r[0]==passive[49])
            if family in (0,7): templates["own_attack"]=next(r for r in own if r[8]=="ATTACK_AA")
            token_id=0
            if family==5:
                token_id=int(next(r for r in own if r[8]=="CARD_TOKEN")[22])
                call=next(r for r in tables["skill_player.csv"] if r[0]==card[44])
                templates["token_attack"]=next(r for r in tables["skill_role_player.csv"] if r[0]==call[49] and r[8]=="ATTACK_AA")
            programs=build_programs(family,index,attr,active,passive,templates,spec,token_id)
            for name,rows in zip(TABLES[1:],programs[:4]): additions[name].extend(rows)
            updated=card.copy();updated[39],updated[46]=programs[0][0][0],programs[2][0][0]
            if programs[4]: updated[44]=str(programs[4])
            if family in (1,2,4,5,7,8,10,11):
                require(updated[68] in ("20020001","20020010"),f"Unexpected restriction tag {card_id}");updated[68]="20020010"
            replacements[card_id],old_cards[card_id]=updated,card
            cards.append({"card_id":card_id,"family":family,"attribute":ATTRIBUTES[attr],"skill_id":int(updated[39]),"passive_id":int(updated[46]),"call_id":programs[4],"token_id":token_id,"active_slots":list(EXPECTED_SLOTS[family]),"passive_slots":list(PASSIVE_SLOTS[family])})
    new_ids={r[0] for rows in additions.values() for r in rows}
    require(not new_ids & {r[0] for r in all_skills+all_roles if r},"Generated ID collision")
    known_conditions={r[i] for r in all_skills if len(r)>=50 for i in (36,42)}
    validate_programs(additions["skill_player.csv"]+additions["support_skill.csv"],additions["skill_role_player.csv"]+additions["support_skill_role.csv"],known_conditions,{r[8] for r in all_roles if len(r)>=33})
    changed={"card.csv":replace_cards(original["card.csv"],replacements,old_cards)}
    for name,rows in additions.items():
        raw=original[name];changed[name]=raw+(b"\n" if raw and not raw.endswith(b"\n") else b"")+encode_rows(rows)
    inventory={f["path"]:f for f in manifest["files"]}
    for name,relative in paths.items():
        entry=inventory[relative.as_posix()]
        require(entry["sha256"]==sha(original[name]) and entry["bytes"]==len(original[name]),f"Baseline checksum mismatch {name}")
    receipt={"schema":2,"scope":"Final 60 cards; full-battle single-use Owen GUTS; no deferred cards","spec_sha256":SPEC_SHA256,"source_resource_set":str(source),"cards":cards,"preserved_card_ids":[],"deferred_card_ids":[],"new_ids":sorted(map(int,new_ids)),"files":{}}
    for folder in dict.fromkeys([card_path.parent,battle_path]): shutil.copytree(source/folder,output/folder,dirs_exist_ok=True)
    for name,relative in paths.items():
        data=changed[name];backup=output/"backup"/relative;backup.parent.mkdir(parents=True,exist_ok=True);backup.write_bytes(original[name]);(output/relative).write_bytes(data)
        require((output/relative).read_bytes()==data,"Read-back failed")
        inventory[relative.as_posix()].update(bytes=len(data),sha256=sha(data))
        receipt["files"][relative.as_posix()]={"before_sha256":sha(original[name]),"after_sha256":sha(data),"bytes":len(data)}
    (output/"backup/resource-set.json").write_bytes(manifest_raw)
    (output/"resource-set.json").write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    (output/"patch.json").write_text(json.dumps(receipt,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    return receipt

if __name__=="__main__":
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--resource-set",type=Path,required=True)
    parser.add_argument("--output",type=Path,required=True)
    parser.add_argument("--spec",type=Path,default=Path(__file__).resolve().parents[1]/"docs/CARD_MODDING_2.md")
    args=parser.parse_args();result=build(args.resource_set,args.output,args.spec)
    print(json.dumps({"output":str(args.output),"cards":len(result["cards"]),"spec_sha256":result["spec_sha256"]},ensure_ascii=False))
