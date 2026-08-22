import {useEffect,useState} from 'react';
import {ChevronLeft,ChevronRight,Swords} from 'lucide-react';
import './war-history.css';

type WarNation={id:string;name:string;leaderName:string;allianceID?:string;allianceName?:string};
type WarHistoryItem={id:string;declaredAt:string;objective:string;objectiveName:string;stage:string;outcome:string;winnerNationID:string;attacker:WarNation;defender:WarNation};
type WarHistoryData={items:WarHistoryItem[];page:number;pageSize:number;pages:number;total:number;perspectiveNationID?:string};

const navigate=(path:string)=>{history.pushState(null,'',path);window.dispatchEvent(new PopStateEvent('popstate'))};
const label=(value:string)=>value.replaceAll('_',' ').replace(/\b\w/g,letter=>letter.toUpperCase());

function NationCell({nation}:{nation:WarNation}){return <div className="war-history-nation"><a href={`/nation/${encodeURIComponent(nation.id)}`} onClick={event=>{event.preventDefault();navigate(`/nation/${encodeURIComponent(nation.id)}`)}}>{nation.name}</a><span>{nation.leaderName}</span>{nation.allianceID&&nation.allianceName?<a className="war-history-alliance" href={`/alliance/${encodeURIComponent(nation.allianceID)}`} onClick={event=>{event.preventDefault();navigate(`/alliance/${encodeURIComponent(nation.allianceID!)}`)}}>{nation.allianceName}</a>:<small>Independent</small>}</div>}

function resultFor(item:WarHistoryItem,perspectiveNationID?:string){
 if(item.stage!=='ended')return{label:'Active war',tone:'active'};
 if(!item.winnerNationID||item.outcome==='stalemate')return{label:'Stalemate',tone:'neutral'};
 if(perspectiveNationID)return item.winnerNationID===perspectiveNationID?{label:'Victory',tone:'victory'}:{label:'Defeat',tone:'defeat'};
 return item.winnerNationID===item.attacker.id?{label:'Aggressor victory',tone:'victory'}:{label:'Defender victory',tone:'victory'};
}

export function WarHistoryView({data,perspectiveNationID,onPage}:{data:WarHistoryData;perspectiveNationID?:string;onPage?:(page:number)=>void}){return <section className="panel war-history-panel"><header className="war-history-heading"><div><span className="eyebrow">CONFLICT RECORD</span><h2>War history</h2></div><b>{data.total.toLocaleString()} {data.total===1?'war':'wars'}</b></header>{data.items.length?<div className="war-history-table"><div className="war-history-row war-history-head"><span>Date</span><span>Aggressor</span><span>Defender</span><span>Information</span></div>{data.items.map(item=>{const result=resultFor(item,perspectiveNationID||data.perspectiveNationID);return <article className="war-history-row" key={item.id}><div className="war-history-date"><time title={new Date(item.declaredAt).toLocaleString()}>{new Date(item.declaredAt).toLocaleString(undefined,{year:'numeric',month:'2-digit',day:'2-digit',hour:'numeric',minute:'2-digit'})}</time><span>{item.objectiveName||label(item.objective)}</span></div><NationCell nation={item.attacker}/><NationCell nation={item.defender}/><div className="war-history-result"><b className={result.tone}>{result.label}</b><a href={`/conflict/${encodeURIComponent(item.id)}`} onClick={event=>{event.preventDefault();navigate(`/conflict/${encodeURIComponent(item.id)}`)}}><Swords/>Timeline</a></div></article>})}</div>:<div className="war-history-empty"><Swords/><h3>No wars recorded</h3><p>No conflicts involving this {perspectiveNationID?'nation':'Alliance’s current nations'} have been declared.</p></div>}{data.pages>1&&<nav className="war-history-pagination" aria-label="War history pages"><button disabled={data.page===1} onClick={()=>onPage?.(data.page-1)}><ChevronLeft/>Previous</button><span>Page {data.page} of {data.pages}</span><button disabled={data.page===data.pages} onClick={()=>onPage?.(data.page+1)}>Next<ChevronRight/></button></nav>}</section>}

export default function WarHistoryTable({scope,id}:{scope:'nation'|'alliance';id:string}){
 const[page,setPage]=useState(1),[data,setData]=useState<WarHistoryData>(),[error,setError]=useState('');
 useEffect(()=>{setPage(1)},[scope,id]);
 useEffect(()=>{let cancelled=false;setError('');fetch(`/api/${scope==='nation'?'nations':'alliances'}/${encodeURIComponent(id)}/wars?page=${page}`,{credentials:'include'}).then(async response=>{const body=await response.json().catch(()=>({}));if(!response.ok)throw Error(body.error||'Could not load war history.');if(!cancelled)setData(body)}).catch(reason=>{if(!cancelled)setError((reason as Error).message)});return()=>{cancelled=true}},[scope,id,page]);
 if(error)return <section className="panel war-history-panel"><p className="error notice">{error}</p></section>;
 if(!data)return <section className="panel war-history-panel war-history-loading">Loading war history…</section>;
 return <WarHistoryView data={data} perspectiveNationID={scope==='nation'?id:undefined} onPage={setPage}/>;
}
