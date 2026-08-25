import {BookOpen,ShieldAlert} from 'lucide-react';
import './war-information.css';

const unitKeys=['soldiers','tanks','ships','jets','drones'];
const unitNames:Record<string,string>={soldiers:'Soldiers',tanks:'Tanks',ships:'Ships',jets:'Fighter Jets',drones:'Drones'};
const number=(value:any,digits=0)=>Number(value||0).toLocaleString(undefined,{maximumFractionDigits:digits});

type DamageSide={name:string;forces:Record<string,any>;incomingPressure:number;infrastructureDamage:number;institutionsDestroyed:number};

export function WarDamageSummary({attacker,defender,ended}:{attacker:DamageSide;defender:DamageSide;ended:boolean}){
 return <section className="panel war-damage-ledger">
  <header><div><span className="eyebrow">DAMAGE RECEIVED</span><h3>Campaign damage</h3></div><ShieldAlert/></header>
  <div>{[attacker,defender].map(side=>{
   const losses=unitKeys.map(unit=>[unit,Number(side.forces?.[unit]?.lost||0)] as const),total=losses.reduce((sum,[,amount])=>sum+amount,0);
   return <article key={side.name}><h4>{side.name}</h4><dl><div><dt>Military units lost</dt><dd>{number(total)}</dd></div><div><dt>Incoming damage pressure</dt><dd>{number(side.incomingPressure,2)}</dd></div><div><dt>Infrastructure destroyed</dt><dd>{ended?number(side.infrastructureDamage,1):'Pending'}</dd></div><div><dt>Institutions destroyed</dt><dd>{ended?number(side.institutionsDestroyed):'Pending'}</dd></div></dl><p>{losses.map(([unit,amount])=>`${unitNames[unit]} ${number(amount)}`).join(' · ')}</p></article>
  })}</div>
 </section>
}

export function WarRulesPanel({rules}:{rules:any}){
 const r=rules||{};
 return <details className="panel war-rules-panel"><summary><span><BookOpen/><b>Victory conditions and combat modifiers</b></span><small>How campaigns end and what reduces combat strength</small></summary><div className="war-rules-content">
  <section><h4>How a war is won</h4><ul><li>Successful expeditionary operations in the opposing homeland build score, reduce enemy Resolve, and create strategic damage pressure. Defending that homeland denies those gains.</li><li>Reducing enemy Resolve to 0 ends the war immediately in a decisive victory.</li><li>Otherwise, the campaign ends after {r.maximumRounds||20} rounds. A score lead of more than {r.stalemateScoreMargin??3} wins; a closer result is a stalemate.</li><li>Winning margins below {r.majorVictoryMargin||15} are minor, {r.majorVictoryMargin||15}–{(r.decisiveVictoryMargin||35)-1} are major, and {r.decisiveVictoryMargin||35}+ are decisive.</li><li>Capitulation unlocks after round {r.minimumCapitulationRound||4}, or sooner when Resolve falls to {r.earlyCapitulationResolve||35}% or lower.</li></ul></section>
  <section><h4>Current negative modifiers</h4><dl><div><dt>Readiness</dt><dd>Directly scales a front's combat strength and declines during sustained action.</dd></div><div><dt>Organization</dt><dd>Directly scales combat strength; Resupply restores both Readiness and Organization.</dd></div><div><dt>Supply</dt><dd>Ranges from 35–100%. Shortfalls in cash, Foodstuffs, Energy, or Military Equipment reduce effective strength.</dd></div><div><dt>War exhaustion</dt><dd>Reduces expeditionary strength by up to 35%. Homeland resistance offsets exhaustion defensively.</dd></div><div><dt>Distance</dt><dd>Expeditionary forces can spend 1–{r.maximumMobilizationRounds||4} rounds in transit and consume more supplies over longer routes.</dd></div><div><dt>Operation mismatch</dt><dd>Units outside the selected operation's specialty fight at reduced effectiveness.</dd></div><div><dt>Aggressive posture</dt><dd>Provides +13% strength but causes 15% more casualties. Entrenched provides +8% strength; Balanced is neutral.</dd></div></dl></section>
  <section><h4>Strategic damage</h4><p>Winning control in enemy territory builds damage pressure. Infrastructure and institution losses are assessed when the war ends, capped at {r.maximumInfrastructureDamagePercent||6}% Infrastructure and a {r.maximumInstitutionRiskPercent||3}% destruction chance per institution.</p></section>
 </div></details>
}
