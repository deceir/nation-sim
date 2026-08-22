import {useEffect,useState} from 'react';
import {Activity,ChevronLeft,ChevronRight,Clock3,Crosshair,Route,Shield,Swords} from 'lucide-react';
import './conflicts.css';

const units=['soldiers','tanks','ships','jets','drones'];
const unitNames:Record<string,string>={soldiers:'Soldiers',tanks:'Tanks',ships:'Ships',jets:'Fighter Jets',drones:'Drones'};
const api=async(path:string)=>{const response=await fetch('/api'+path,{credentials:'include'}),text=await response.text();let data:any={};try{data=text?JSON.parse(text):{}}catch{throw Error(`Invalid conflict response (${response.status}).`)}if(!response.ok)throw Error(data.error||'Could not load conflicts.');return data};
const label=(value:string)=>String(value||'').replaceAll('_',' ').replace(/\b\w/g,letter=>letter.toUpperCase());
const number=(value:any,digits=0)=>Number(value||0).toLocaleString(undefined,{maximumFractionDigits:digits});
const dateTime=(value:string)=>new Date(value).toLocaleString();
const navigate=(path:string)=>{history.pushState(null,'',path);window.dispatchEvent(new PopStateEvent('popstate'))};

function NationIdentity({nation}:{nation:any}){return <div className="conflict-nation"><img src={`/api/nations/${encodeURIComponent(nation.id)}/flag`} alt=""/><div><a href={`/nation/${encodeURIComponent(nation.id)}`} onClick={event=>{event.preventDefault();navigate(`/nation/${encodeURIComponent(nation.id)}`)}}>{nation.name}</a><span>Led by {nation.leaderName}</span>{nation.allianceName?<a className="conflict-alliance" href={`/alliance/${encodeURIComponent(nation.allianceID)}`} onClick={event=>{event.preventDefault();navigate(`/alliance/${encodeURIComponent(nation.allianceID)}`)}}>{nation.allianceName}</a>:<small>Independent</small>}</div></div>}

export default function ConflictsScreen({conflictID}:{conflictID?:string}){
 return conflictID?<ConflictDetail id={conflictID}/>:<ConflictDirectory/>;
}

function ConflictDirectory(){
 const[data,setData]=useState<any>(),[page,setPage]=useState(1),[status,setStatus]=useState('all'),[error,setError]=useState('');
 useEffect(()=>{setError('');void api(`/conflicts?page=${page}&pageSize=10&status=${status}`).then(setData).catch(error=>setError(error.message))},[page,status]);
 return <div className="conflicts-page">
  <section className="conflicts-hero"><Swords/><div><span className="eyebrow">GLOBAL MILITARY RECORD</span><h2>World Conflicts</h2><p>Recent wars and active campaigns.</p></div></section>
  <nav className="conflict-filters"><button className={status==='all'?'active':''} onClick={()=>{setStatus('all');setPage(1)}}>All</button><button className={status==='active'?'active':''} onClick={()=>{setStatus('active');setPage(1)}}>Active</button><button className={status==='ended'?'active':''} onClick={()=>{setStatus('ended');setPage(1)}}>Concluded</button></nav>
  {error&&<p className="error notice">{error}</p>}
  {!data?<section className="panel">Loading world conflicts…</section>:<>
   <section className="conflict-directory-list">{data.items.length?data.items.map((war:any)=><article className="conflict-directory-card" key={war.id}>
    <time>{new Date(war.declaredAt).toLocaleDateString()}</time>
    <NationIdentity nation={war.attacker}/><div className="conflict-versus"><Swords/><span>versus</span></div><NationIdentity nation={war.defender}/>
    <div className="conflict-directory-status"><span className={`war-stage ${war.stage}`}>{war.stage==='ended'?label(war.outcome||'concluded'):label(war.stage)}</span><b>{war.objectiveName}</b><small>{number(war.attacker.score,1)} - {number(war.defender.score,1)} score · Round {war.roundsResolved}</small><button onClick={()=>navigate(`/conflict/${encodeURIComponent(war.id)}`)}>View conflict</button></div>
   </article>):<section className="panel conflict-empty"><Shield/><h3>No conflicts found</h3></section>}</section>
   <nav className="conflict-pagination"><button disabled={data.page<=1} onClick={()=>setPage(page-1)}><ChevronLeft/>Previous</button><span>Page {data.page} of {data.pages} · {data.total} wars</span><button disabled={data.page>=data.pages} onClick={()=>setPage(page+1)}>Next<ChevronRight/></button></nav>
  </>}
 </div>
}

function ConflictDetail({id}:{id:string}){
 const[data,setData]=useState<any>(),[error,setError]=useState('');
 useEffect(()=>{let active=true;const load=()=>api('/conflicts/'+encodeURIComponent(id)).then(value=>active&&setData(value)).catch(error=>active&&setError(error.message));void load();const timer=setInterval(load,30000);return()=>{active=false;clearInterval(timer)}},[id]);
 if(error)return <section className="panel"><p className="error">{error}</p><button onClick={()=>navigate('/conflicts')}>Return to conflicts</button></section>;
 if(!data)return <section className="panel">Loading conflict overview…</section>;
 const completed=Math.min(100,Number(data.roundsResolved)/Math.max(1,Number(data.maximumRounds))*100);
 return <div className="conflict-detail-page">
  <button className="back" onClick={()=>navigate('/conflicts')}>← World Conflicts</button>
  <section className="conflict-detail-hero"><div><span className={`war-stage ${data.stage}`}>{data.stage==='ended'?label(data.outcome||'concluded'):label(data.stage)}</span><h2>{data.attacker.name} <small>versus</small> {data.defender.name}</h2><p>{data.objectiveName} · Declared {dateTime(data.declaredAt)}</p></div><Crosshair/></section>
  <section className="conflict-side-by-side"><ConflictSide title="Attacker" nation={data.attacker}/><div className="conflict-center-mark"><Swords/><b>{number(data.attacker.score,1)} - {number(data.defender.score,1)}</b><span>Score</span></div><ConflictSide title="Defender" nation={data.defender}/></section>
  <section className="conflict-progress panel"><header><b>Campaign progress</b><span>Round {data.roundsResolved} / {data.maximumRounds}</span></header><i><em style={{width:`${completed}%`}}/></i></section>
  <section className="conflict-meta-strip"><div><Route/><span>Distance</span><b>{number(data.distanceKm)} km</b></div><div><Clock3/><span>Mobilization</span><b>{data.mobilizationRounds} rounds</b></div><div><Activity/><span>Route</span><b>{label(data.routeType)}</b></div><div><Shield/><span>Objective</span><b>{data.objectiveName}</b></div></section>
  <section className="conflict-detail-grid">
   <section className="panel conflict-forces"><span className="eyebrow">WAR STATISTICS</span><h3>Military commitments</h3><div className="conflict-force-table"><header><span>Unit</span><b>Attacker</b><b>Defender</b></header>{units.map(unit=><div key={unit}><span>{unitNames[unit]}</span><ForceValue force={data.forces.attacker?.[unit]}/><ForceValue force={data.forces.defender?.[unit]}/></div>)}</div></section>
   <section className="panel conflict-timeline"><div className="conflict-timeline-title"><div><span className="eyebrow">WAR TIMELINE</span><h3>Strategic reports</h3></div><Activity/></div><div className="conflict-timeline-scroll">{data.reports.length?data.reports.map((report:any)=><article key={report.round}><header><b>Round {report.round}</b><time>{dateTime(report.resolvedAt)}</time></header><p>{report.summary}</p><div><span>{label(report.attackerOperation)} · {number(report.attackerStrength)} strength</span><span>{label(report.defenderOperation)} · {number(report.defenderStrength)} strength</span></div><small>Losses: {lossText(report.attackerLosses)} / {lossText(report.defenderLosses)}</small></article>):<p className="conflict-no-reports">No strategic reports yet.</p>}</div></section>
  </section>
 </div>
}

function ConflictSide({title,nation}:{title:string;nation:any}){return <article className="conflict-side"><span className="eyebrow">{title}</span><NationIdentity nation={nation}/><div className="conflict-side-stats"><span>Resolve <b>{number(nation.resolve)}%</b></span><span>Readiness <b>{number(nation.readiness)}%</b></span><span>Organization <b>{number(nation.organization)}%</b></span></div></article>}
function ForceValue({force}:{force:any}){return <b>{number(force?.remaining)} <small>({number(force?.lost)} lost)</small></b>}
function lossText(losses:Record<string,number>){const parts=Object.entries(losses||{}).filter(([,amount])=>Number(amount)>0).map(([unit,amount])=>`${number(amount)} ${unitNames[unit]||unit}`);return parts.length?parts.join(', '):'none'}
