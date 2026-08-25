import {useEffect,useState} from 'react';
import {Activity,CheckCircle2,Clock3,Crosshair,Flag,MapPinned,Shield,ShieldCheck,Swords,Truck} from 'lucide-react';
import WarTheaterMap from './WarTheaterMap';
import {WarDamageSummary,WarRulesPanel} from './WarInformation';
import './war-system.css';
import './war-reinforcements.css';

const api=async(path:string,options?:RequestInit)=>{const response=await fetch('/api'+path,{credentials:'include',headers:{'Content-Type':'application/json'},...options}),text=await response.text();let data:any={};try{data=text?JSON.parse(text):{}}catch{throw Error(`Invalid war response (${response.status}).`)}if(!response.ok)throw Error(data.error||'War action failed.');return data};
const units=['soldiers','tanks','ships','jets','drones'];
const names:Record<string,string>={soldiers:'Soldiers',tanks:'Tanks',ships:'Ships',jets:'Fighter Jets',drones:'Drones'};
const label=(value:string)=>value.replaceAll('_',' ').replace(/\b\w/g,letter=>letter.toUpperCase());
const when=(value:string)=>{const date=new Date(value),remaining=date.getTime()-Date.now(),stamp=date.toLocaleString(undefined,{dateStyle:'medium',timeStyle:'short'});if(remaining<=0)return stamp;const total=Math.max(0,Math.ceil(remaining/1000)),hours=Math.floor(total/3600),minutes=Math.floor(total%3600/60),seconds=total%60;return `${stamp} · ${hours?`${hours}h `:''}${minutes}m ${seconds}s remaining`};

export default function WarScreen({warID}:{warID?:string}){
 return warID?<WarDetail id={warID}/>:<WarOverview/>;
}

function WarOverview(){
 const initialTarget=new URLSearchParams(location.search).get('target')||'';
 const[data,setData]=useState<any>(),[military,setMilitary]=useState<any>(),[error,setError]=useState(''),[view,setView]=useState<'active'|'history'|'declare'>(initialTarget?'declare':'active');
 const[target,setTarget]=useState(initialTarget),[objective,setObjective]=useState('territorial_pressure'),[forces,setForces]=useState<Record<string,number>>({}),[review,setReview]=useState(false),[busy,setBusy]=useState(false);
 const load=()=>Promise.all([api('/wars'),api('/military')]).then(([wars,military])=>{setData(wars);setMilitary(military)}).catch(e=>setError(e.message));
 useEffect(()=>{void load()},[]);
 const submit=async()=>{setBusy(true);setError('');try{const result=await api('/wars',{method:'POST',body:JSON.stringify({defenderName:target.trim(),objective,forces})});location.href='/war/'+encodeURIComponent(result.warID)}catch(e){setError((e as Error).message);setReview(false)}finally{setBusy(false)}};
 if(!data||!military)return <section className="panel wide">Opening the strategic command room…</section>;
 const active=data.wars.filter((war:any)=>war.stage!=='ended'),history=data.wars.filter((war:any)=>war.stage==='ended'),list=view==='history'?history:active;
 return <div className="war-page">
<section className="war-hero">
<Crosshair/>
<div>
<span className="eyebrow">STRATEGIC COMMAND</span>
<h2>War Room</h2>
<p>Direct finite campaigns through three-hour strategic rounds. Distance delays deployment, supply affects combat power, and every war reaches a firm conclusion.</p>
</div>
</section>{error&&<p className="error notice">{error}</p>}<section className="war-status-strip">
<div>
<Activity/>
<span>War exhaustion</span>
<b>{Number(data.status.warExhaustion||0).toFixed(1)}%</b>
</div>
<div>
<Swords/>
<span>Offensive capacity</span>
<b>{active.filter((w:any)=>w.isAttacker).length} / {data.rules.offensiveSlots}</b>
</div>
<div>
<Shield/>
<span>Defensive capacity</span>
<b>{active.filter((w:any)=>!w.isAttacker).length} / {data.rules.defensiveSlots}</b>
</div>
<div>
<Clock3/>
<span>Campaign limit</span>
<b>{data.rules.maximumRounds} rounds</b>
</div>
</section>
<nav className="war-tabs">
<button className={view==='active'?'active':''} onClick={()=>setView('active')}>Active wars <span>{active.length}</span>
</button>
<button className={view==='history'?'active':''} onClick={()=>setView('history')}>War history</button>
<button className={view==='declare'?'active':''} onClick={()=>setView('declare')}>Declare war</button>
</nav>{view!=='declare'?<section className="war-list">{list.length?list.map((war:any)=>
<a className="war-card" href={`/war/${encodeURIComponent(war.id)}`} key={war.id}>
<div>
<span className={'war-stage '+war.stage}>{war.stage}</span>
<h3>{war.attackerName} <small>versus</small> {war.defenderName}</h3>
<p>{war.objectiveName}</p>
</div>
<div className="war-card-score">
<span>{Number(war.attackerScore).toFixed(1)} — {Number(war.defenderScore).toFixed(1)}</span>
<b>{Number(war.attackerResolve).toFixed(0)}% · {Number(war.defenderResolve).toFixed(0)}% resolve</b>
</div>
<div className="war-card-meta">
<span>Round {war.roundsResolved} / {data.rules.maximumRounds}</span>
<span>{Math.round(Number(war.distanceKm)).toLocaleString()} km · {label(war.routeType)}</span>
<span>{war.stage==='ended'?(war.outcome?label(war.outcome):'Concluded'):`Next round ${when(war.nextRoundAt)}`}</span>
</div>
</a>):<div className="panel war-empty">
<ShieldCheck/>
<h3>{view==='history'?'No concluded wars':'No active wars'}</h3>
<p>{view==='history'?'Completed campaigns will remain available here as a permanent record.':'Your armed forces currently have no active commitments.'}</p>
</div>}</section>:<section className="panel war-declaration">
<div className="war-section-heading">
<div>
<span className="eyebrow">FORMAL DECLARATION</span>
<h2>Open a campaign</h2>
</div>
<Flag/>
</div>
<label>Defending nation<input value={target} onChange={event=>{setTarget(event.target.value);setReview(false)}} placeholder="Enter the exact nation name"/>
</label>
<div className="war-objectives">{Object.entries(data.objectives).map(([key,value]:any)=>
<button type="button" className={objective===key?'selected':''} onClick={()=>{setObjective(key);setReview(false)}} key={key}>
<b>{value.name}</b>
<span>{value.description}</span><small>{value.effect}</small>
</button>)}</div>
<div className="deployment-editor">
<h3>Initial force commitment</h3>
<p>The initial force enters the opposing homeland immediately. Committed units cannot be sold, decommissioned, or used in another war.</p>{military.units.map((unit:any)=>
<label key={unit.key}>
<span>
<b>{unit.name}</b>
<small>{Number(unit.availableQuantity).toLocaleString()} available</small>
</span>
<input type="number" min="0" max={unit.availableQuantity} step="1" value={forces[unit.key]||''} onChange={event=>{setForces({...forces,[unit.key]:Math.max(0,Math.floor(Number(event.target.value))) });setReview(false)}}/>
</label>)}</div>{review?<div className="war-final-confirm">
<b>Confirm declaration against {target || 'this nation'}?</b>
<p>Guardian status is revoked when the declaration succeeds. This starts a real campaign and locks the selected units into it.</p>
<button className="primary danger-action" disabled={busy||!target.trim()} onClick={()=>void submit()}>{busy?'Declaring war…':'Confirm declaration'}</button>
<button disabled={busy} onClick={()=>setReview(false)}>Go back</button>
</div>:<button className="primary" disabled={!target.trim()||Object.values(forces).reduce((sum,value)=>sum+Number(value||0),0)<1} onClick={()=>setReview(true)}>Review declaration</button>}</section>}</div>
}

function WarDetail({id}:{id:string}){
 const[data,setData]=useState<any>(),[error,setError]=useState(''),[homeOperation,setHomeOperation]=useState('hold'),[homePosture,setHomePosture]=useState('entrenched'),[foreignOperation,setForeignOperation]=useState('hold'),[foreignPosture,setForeignPosture]=useState('entrenched'),[forces,setForces]=useState<Record<string,number>>({}),[originFOBID,setOriginFOBID]=useState(''),[deploymentTheater,setDeploymentTheater]=useState('foreign'),[busy,setBusy]=useState(''),[confirmCapitulation,setConfirmCapitulation]=useState(false),[ordersSaved,setOrdersSaved]=useState(false),[,setClock]=useState(0);
 const load=(syncOrders=false)=>api('/wars/'+encodeURIComponent(id)).then(result=>{setData(result);if(syncOrders){setHomeOperation(result.currentOrder?.homeOperation||'hold');setHomePosture(result.currentOrder?.homePosture||'entrenched');setForeignOperation(result.currentOrder?.foreignOperation||'hold');setForeignPosture(result.currentOrder?.foreignPosture||'entrenched')}}).catch(e=>setError(e.message));
 useEffect(()=>{void load(true);const refreshTimer=setInterval(()=>void load(false),30000),clockTimer=setInterval(()=>setClock(value=>value+1),1000);return()=>{clearInterval(refreshTimer);clearInterval(clockTimer)}},[id]);
 const saveOrders=async()=>{setBusy('orders');setError('');setOrdersSaved(false);try{await api(`/wars/${id}/orders`,{method:'PUT',body:JSON.stringify({homeOperation,homePosture,foreignOperation,foreignPosture})});await load(true);setOrdersSaved(true)}catch(e){setError((e as Error).message)}finally{setBusy('')}};
 const deploy=async()=>{setBusy('deploy');setError('');try{const homeTheater=data.isAttacker?data.theaters.attackerHomeland:data.theaters.defenderHomeland,foreignTheater=data.isAttacker?data.theaters.defenderHomeland:data.theaters.attackerHomeland;await api(`/wars/${id}/deploy`,{method:'POST',body:JSON.stringify({forces,originFOBID:deploymentTheater==='home'?'':originFOBID,theater:deploymentTheater==='home'?homeTheater:foreignTheater})});setForces({});await load()}catch(e){setError((e as Error).message)}finally{setBusy('')}};
 const capitulate=async()=>{setBusy('capitulate');setError('');try{await api(`/wars/${id}/capitulate`,{method:'POST'});await load();setConfirmCapitulation(false)}catch(e){setError((e as Error).message)}finally{setBusy('')}};
 if(!data)return <section className="panel wide">Loading the campaign record…{error&&<p className="error notice">{error}</p>}</section>;
 const mySide=data.isAttacker?'attacker':'defender',enemySide=data.isAttacker?'defender':'attacker',myForces=data.forces[mySide]||{},enemyForces=data.forces[enemySide]||{},myFobs=data.isAttacker?data.attackerFOBs||[]:data.defenderFOBs||[];
 return <div className="war-page war-detail">
<a className="back" href="/wars">← War Room</a>
<section className="war-detail-hero">
<div>
<span className={'war-stage '+data.stage}>{data.stage}</span>
<h2>{data.attackerName} <small>versus</small> {data.defenderName}</h2>
<p>{data.objectiveName} · {data.objectiveDescription}</p><small className="war-objective-effect">{data.objectiveEffect}</small>
</div>
<div>
	<b>Round {data.roundsResolved} / {data.rules?.maximumRounds||20}</b>
<span>{data.stage==='ended'?`${label(data.outcome||'concluded')} · ${label(data.endReason||'ended')}`:`Next resolution ${when(data.nextRoundAt)}`}</span>
</div>
</section>{error&&<p className="error notice">{error}</p>}<section className="war-balance">
<WarSide name={data.attackerName} score={data.attackerScore} resolve={data.attackerResolve} readiness={data.attackerReadiness} organization={data.attackerOrganization} damagePressure={data.attackerDamagePressure} active={data.isAttacker}/>
<div className="war-versus">
<Swords/>
<span>{Math.round(Number(data.distanceKm)).toLocaleString()} km</span>
<small>{label(data.routeType)} route · {data.mobilizationRounds} round mobilization</small>
</div>
<WarSide name={data.defenderName} score={data.defenderScore} resolve={data.defenderResolve} readiness={data.defenderReadiness} organization={data.defenderOrganization} damagePressure={data.defenderDamagePressure} active={!data.isAttacker}/>
	</section>
	<WarRulesPanel rules={data.rules}/>
	<WarTheaterMap attacker={{name:data.attackerName,lat:data.attackerLat,lng:data.attackerLng,fobs:data.attackerFOBs||[]}} defender={{name:data.defenderName,lat:data.defenderLat,lng:data.defenderLng,fobs:data.defenderFOBs||[]}} routeType={data.routeType} stage={data.stage} distanceKm={data.distanceKm} roundsResolved={data.roundsResolved} deployments={data.deployments||[]}/>
	<section className="war-force-comparison">
	<ForceTable title="Your forces" forces={myForces}/>
	<ForceTable title="Opposing observed forces" forces={enemyForces}/>
	</section><WarDamageSummary ended={data.stage==='ended'} attacker={{name:data.attackerName,forces:data.forces.attacker||{},incomingPressure:data.defenderDamagePressure,infrastructureDamage:data.attackerInfrastructureDamage,institutionsDestroyed:data.attackerInstitutionsDestroyed}} defender={{name:data.defenderName,forces:data.forces.defender||{},incomingPressure:data.attackerDamagePressure,infrastructureDamage:data.defenderInfrastructureDamage,institutionsDestroyed:data.defenderInstitutionsDestroyed}}/>{data.stage!=='ended'&&<>
<section className="panel war-orders">
<div className="war-section-heading">
<div>
<span className="eyebrow">NEXT STRATEGIC ROUND</span>
<h2>Operational orders</h2>
</div>
<Crosshair/>
</div>{data.currentOrder?<div className="war-locked-order">
<CheckCircle2/>
<div>
<span>LOCKED IN FOR ROUND {data.currentOrder.round}</span>
<b>Homeland: {data.operations[data.currentOrder.homeOperation]} · {data.postures[data.currentOrder.homePosture]}</b>
<b>Expeditionary: {data.operations[data.currentOrder.foreignOperation]} · {data.postures[data.currentOrder.foreignPosture]}</b>
<small>Saved {when(data.currentOrder.submittedAt)}</small>
</div>
</div>:<div className="war-locked-order empty">
<Shield/>
<div>
<span>NO ORDERS LOCKED IN</span>
<b>Default: Hold Position · Entrenched</b>
<small>Submit orders before the next resolution to replace the default.</small>
</div>
</div>}<div className="war-front-order-grid"><section><span className="eyebrow">HOMELAND FRONT</span><h3>Defensive command</h3><div className="war-order-grid">
<label>Operation<select value={homeOperation} onChange={e=>{setHomeOperation(e.target.value);setOrdersSaved(false)}}>{Object.entries(data.operations).map(([key,value]:any)=>
<option value={key} key={key}>{value}</option>)}</select>
</label>
<label>Posture<select value={homePosture} onChange={e=>{setHomePosture(e.target.value);setOrdersSaved(false)}}>{Object.entries(data.postures).map(([key,value]:any)=>
<option value={key} key={key}>{value}</option>)}</select>
</label></div></section><section><span className="eyebrow">EXPEDITIONARY FRONT</span><h3>Foreign command</h3><div className="war-order-grid">
<label>Operation<select value={foreignOperation} onChange={e=>{setForeignOperation(e.target.value);setOrdersSaved(false)}}>{Object.entries(data.operations).map(([key,value]:any)=><option value={key} key={key}>{value}</option>)}</select></label>
<label>Posture<select value={foreignPosture} onChange={e=>{setForeignPosture(e.target.value);setOrdersSaved(false)}}>{Object.entries(data.postures).map(([key,value]:any)=><option value={key} key={key}>{value}</option>)}</select></label>
</div></section>
</div>
<p>Each front resolves independently. Orders can be revised until the round resolves; an unordered front holds an entrenched posture.</p>{ordersSaved&&<p className="war-order-saved">
<CheckCircle2/>Your orders are confirmed for the next round.</p>}<button className="primary" disabled={busy!==''} onClick={()=>void saveOrders()}>{busy==='orders'?'Saving orders…':data.currentOrder?'Update locked orders':'Save round orders'}</button>
</section>
<section className="panel war-reinforcements">
<div className="war-section-heading">
<div>
<span className="eyebrow">FORCE COMMITMENT</span>
<h2>Deploy reinforcements</h2>
</div>
<Truck/>
</div>
<div className="reinforcement-route"><label>Destination<select value={deploymentTheater} onChange={event=>{setDeploymentTheater(event.target.value);if(event.target.value==='home')setOriginFOBID('')}}><option value="home">Homeland defense</option><option value="foreign">Expedition into {data.isAttacker?data.defenderName:data.attackerName}</option></select></label><label className="reinforcement-origin">Deployment origin<select disabled={deploymentTheater==='home'} value={originFOBID} onChange={event=>setOriginFOBID(event.target.value)}><option value="">{data.isAttacker?data.attackerName:data.defenderName} Homeland</option>{myFobs.map((base:any)=><option value={base.id} key={base.id}>{base.name} · {base.continent}</option>)}</select></label></div>
<div className="reinforcement-inputs">{units.map(unit=>{const maximum=Number(data.availableForDeployment?.[unit]||0);return <label key={unit}>
<span>{names[unit]}</span>
<input type="number" min="0" max={maximum} step="1" inputMode="numeric" value={forces[unit]||''} onChange={e=>setForces({...forces,[unit]:Math.min(maximum,Math.max(0,Math.floor(Number(e.target.value))))})}/>
<small>Maximum deployable <b>{maximum.toLocaleString()}</b>
</small>
</label>})}</div>
<p>{deploymentTheater==='home'?'Homeland reinforcements are available for the next resolution.':'Expeditionary arrival time is calculated from the selected origin to the opposing homeland.'}</p>
<button disabled={busy!==''||Object.values(forces).reduce((sum,value)=>sum+Number(value||0),0)<1} onClick={()=>void deploy()}>{busy==='deploy'?'Deploying…':'Commit reinforcements'}</button>
</section>
</>}<section className="war-reports">
<div className="war-section-heading">
<div>
<span className="eyebrow">AFTER-ACTION RECORD</span>
<h2>Strategic reports</h2>
</div>
<Activity/>
</div>{data.reports.length?data.reports.map((report:any)=>
<article className="war-report" key={report.round}>
<header>
<b>Round {report.round}</b>
<time>{when(report.resolvedAt)}</time>
</header>
<p>{report.summary}</p>
<div>
<span>{data.attackerName}: H {label(report.attackerHomeOperation)} / {label(report.attackerHomePosture)} · E {label(report.attackerOperation)} / {label(report.attackerForeignPosture)} · {Number(report.attackerSupply*100).toFixed(0)}% supplied</span>
<span>{data.defenderName}: H {label(report.defenderHomeOperation)} / {label(report.defenderHomePosture)} · E {label(report.defenderOperation)} / {label(report.defenderForeignPosture)} · {Number(report.defenderSupply*100).toFixed(0)}% supplied</span>
</div>
<small>Losses: {lossText(report.attackerLosses)} / {lossText(report.defenderLosses)}</small>
</article>):<div className="war-empty compact">No strategic rounds have resolved yet.</div>}</section>{data.stage!=='ended'&&<section className="war-capitulation">{confirmCapitulation?<>
<div>
<b>Confirm capitulation?</b>
<p>This immediately concedes the campaign and begins post-war reconstruction.</p>
</div>
<button className="danger" disabled={busy!==''} onClick={()=>void capitulate()}>{busy==='capitulate'?'Concluding war…':'Confirm capitulation'}</button>
<button disabled={busy!==''} onClick={()=>setConfirmCapitulation(false)}>Cancel</button>
</>:<button className="danger" onClick={()=>setConfirmCapitulation(true)}>Capitulate</button>}</section>}</div>
}

function WarSide({name,score,resolve,readiness,organization,damagePressure,active}:{name:string;score:number;resolve:number;readiness:number;organization:number;damagePressure:number;active:boolean}){return <article className={active?'my-side':''}>
<span>{active?'YOUR COMMAND':'BELLIGERENT'}</span>
<h3>{name}</h3>
<b>{Number(score).toFixed(1)} score</b>
<Meter label="Resolve" value={resolve}/>
<Meter label="Readiness" value={readiness}/>
<Meter label="Organization" value={organization}/>
	<span className="war-damage-pressure">Damage pressure inflicted <b>{Number(damagePressure||0).toFixed(2)}</b></span>
</article>}
function Meter({label:meterLabel,value}:{label:string;value:number}){return <div className="war-meter">
<span>{meterLabel}<b>{Number(value).toFixed(0)}%</b>
</span>
<i>
<em style={{width:`${Math.max(0,Math.min(100,Number(value)))}%`}}/>
</i>
</div>}
function ForceTable({title,forces}:{title:string;forces:Record<string,any>}){return <article className="two-front-force-table">
<h3>{title}</h3><div className="war-force-heading"><span>Unit</span><b>Homeland</b><b>Expeditionary</b><b>En Route</b><b>Strategic Reserve</b></div>{units.map(unit=>{const force=forces[unit];return <div className="war-force-row" key={unit}>
<span>{names[unit]}</span>
<b>{Number(force?.homeTheater||0).toLocaleString()}</b>
<b>{Number(force?.foreignTheater||0).toLocaleString()}</b>
<b>{Number(force?.enRoute||0).toLocaleString()}</b>
<b>{Number(force?.reserve||0).toLocaleString()}</b>
</div>})}</article>}
function lossText(losses:Record<string,number>){const parts=Object.entries(losses||{}).filter(([,value])=>Number(value)>0).map(([unit,value])=>`${Number(value).toLocaleString()} ${names[unit]}`);return parts.length?parts.join(', '):'none'}
