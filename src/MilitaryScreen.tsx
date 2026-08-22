import {useEffect,useState} from 'react';
import {Anchor,Box,Coins,Factory,Shield,Trash2,Users,Zap} from 'lucide-react';
import './military-system.css';
import './military-defense-settings.css';

const yen=(value:number)=>`¥${Number(value||0).toLocaleString(undefined,{maximumFractionDigits:1})}`;
const names:Record<string,string>={basic_metals:'Basic Metals',construction_materials:'Construction Materials',energy:'Energy',timber:'Timber',strategic_minerals:'Strategic Minerals',basic_goods:'Basic Goods',military_equipment:'Military Equipment'};
const projects:Record<string,string>={armored_vehicle_program:'Armored Vehicle Program',naval_shipyard:'Naval Shipyard Authority',aviation_industry:'Aviation Industry Act',advanced_ordnance:'Advanced Ordnance Act'};
const api=async(path:string,options?:RequestInit)=>{const response=await fetch('/api'+path,{credentials:'include',headers:{'Content-Type':'application/json'},...options}),text=await response.text();let data:any={};try{data=text?JSON.parse(text):{}}catch{throw Error(`Invalid military response (${response.status}).`)}if(!response.ok)throw Error(data.error||'Military action failed.');return data};

export default function MilitaryScreen(){
 const[data,setData]=useState<any>();
 const[amounts,setAmounts]=useState<Record<string,number>>({});
 const[defense,setDefense]=useState<Record<string,number>>({});
 const[defenseStatus,setDefenseStatus]=useState('');
 const[error,setError]=useState('');
 const[busy,setBusy]=useState('');
 const load=()=>api('/military').then(next=>{setData(next);setDefense(Object.fromEntries((next.units||[]).map((unit:any)=>[unit.key,Number(unit.automaticDefensePercent??60)])))}).catch(e=>setError(e.message));
 useEffect(()=>{void load()},[]);
 const act=async(unit:string,action:'produce'|'decommission')=>{const quantity=Math.floor(Number(amounts[unit]||0));if(quantity<1){setError('Enter a whole-unit quantity greater than zero.');return}setBusy(unit+action);setError('');try{await api(`/military/${action}`,{method:'POST',body:JSON.stringify({unitType:unit,quantity})});setAmounts({...amounts,[unit]:0});window.dispatchEvent(new Event('diplomatia:resources'));await load()}catch(e){setError((e as Error).message)}finally{setBusy('')}};
 const saveDefense=async()=>{setBusy('defense');setError('');setDefenseStatus('');try{const percentages=Object.fromEntries(data.units.map((unit:any)=>[unit.key,Math.max(0,Math.min(100,Math.round(Number(defense[unit.key]??60))))]));await api('/military/defense-settings',{method:'PUT',body:JSON.stringify({percentages})});setDefense(percentages);setDefenseStatus('Automatic defense settings saved.')}catch(e){setError((e as Error).message)}finally{setBusy('')}};
 if(!data)return <section className="panel wide">Reviewing military readiness…</section>;
 const dailyCash=data.units.reduce((sum:number,u:any)=>sum+Number(u.dailyCashUpkeep||0),0),dailyEnergy=data.units.reduce((sum:number,u:any)=>sum+Number(u.dailyEnergyUpkeep||0),0);
 return <div className="military-page">
  <section className="military-hero"><Shield/><div><span className="eyebrow">NATIONAL DEFENSE INVENTORY</span><h2>Military Command</h2><p>Produce forces, manage mobilization and capacity, and maintain the units committed to active campaigns.</p></div></section>
  {!data.projectRequirementsEnabled&&<p className="notice military-testing-notice"><b>Testing configuration:</b> National Project requirements for domestic military production are temporarily disabled.</p>}
  {error&&<p className="error notice">{error}</p>}
  <div className="military-summary"><div><Users/><span>Population</span><b>{Number(data.population).toLocaleString()}</b></div><div><Anchor/><span>Provinces</span><b>{Number(data.provinces).toLocaleString()}</b></div><div><Coins/><span>Daily cash upkeep</span><b>{yen(dailyCash)}</b></div><div><Zap/><span>Daily Energy upkeep</span><b>{dailyEnergy.toLocaleString(undefined,{maximumFractionDigits:2})} t</b></div></div>
  <section className="panel automatic-defense">
   <header><div><span className="eyebrow">DEFENSIVE MOBILIZATION</span><h3>Automatic Defense</h3></div><button className="primary" disabled={busy!==''} onClick={()=>void saveDefense()}>{busy==='defense'?'Saving…':'Save'}</button></header>
   <div className="automatic-defense-grid">{data.units.map((unit:any)=><label key={unit.key}><span>{unit.name}</span><div><input type="number" min="0" max="100" step="1" value={defense[unit.key]??60} onChange={e=>setDefense({...defense,[unit.key]:Number(e.target.value)})}/><b>%</b></div></label>)}</div>
   {defenseStatus&&<small className="defense-saved">{defenseStatus}</small>}
  </section>
  <section className="military-grid">{data.units.map((unit:any)=><article className="panel military-unit" key={unit.key}>
   <header><div><span className="eyebrow">{unit.tradable?'TRADEABLE EQUIPMENT':'DOMESTIC PERSONNEL'}</span><h3>{unit.name}</h3></div><b>{Number(unit.quantity).toLocaleString()} <small>/ {Number(unit.capacity).toLocaleString()}</small></b></header>
   <div className="military-capacity"><i style={{width:Math.min(100,Number(unit.quantity)/Math.max(1,Number(unit.capacity))*100)+'%'}}/></div>
   <small>{Number(unit.availableQuantity).toLocaleString()} available · {Number(unit.committedQuantity||0).toLocaleString()} deployed · {Number(unit.escrowedQuantity||0).toLocaleString()} in market escrow</small>
   <div className="unit-facts"><span><Coins/> {yen(unit.dailyCashUpkeep)} daily upkeep</span>{Number(unit.dailyEnergyUpkeep)>0&&<span><Zap/> {Number(unit.dailyEnergyUpkeep).toLocaleString(undefined,{maximumFractionDigits:2})} t Energy daily</span>}<span><Factory/> {yen(unit.cashCost)} each</span></div>
   <div className="unit-costs">{Object.entries(unit.resourceCosts||{}).map(([resource,cost])=><span key={resource}><Box/>{cost as number} t {names[resource]||resource} each</span>)}</div>
   <p className="military-mobilization">Daily mobilization: <b>{Number(unit.producedToday).toLocaleString()} / {Number(unit.dailyProductionLimit).toLocaleString()}</b> · {Number(unit.dailyProductionRemaining).toLocaleString()} remaining today</p>
   {!unit.canProduce&&<p className="project-gate">Domestic production requires the <b>{projects[unit.requiredProject]||unit.requiredProject}</b>. Market and private purchases remain available.</p>}
   <label>Whole units<input type="number" min="1" max={unit.dailyProductionRemaining} step="1" value={amounts[unit.key]||''} onChange={e=>setAmounts({...amounts,[unit.key]:Number(e.target.value)})}/></label>
   <div className="military-actions"><button className="primary" disabled={!unit.canProduce||Number(unit.dailyProductionRemaining)<=0||busy!==''} onClick={()=>act(unit.key,'produce')}>{busy===unit.key+'produce'?'Producing…':'Produce domestically'}</button><button className="danger" disabled={unit.decommissionLocked||Number(unit.availableQuantity)<=0||busy!==''} onClick={()=>act(unit.key,'decommission')}><Trash2/>{busy===unit.key+'decommission'?'Decommissioning…':'Decommission'}</button></div>
   {unit.decommissionLocked&&<small className="decommission-lock">Produced today: decommissioning unlocks next server day.</small>}
  </article>)}</section>
  <p className="military-note">Domestic production can mobilize about 10% of each unit cap per UTC server day, creating an approximately ten-day path from an empty force to full capacity. Equipment purchases still use market delivery and overall capacity rules. Soldiers cannot be traded.</p>
 </div>
}
