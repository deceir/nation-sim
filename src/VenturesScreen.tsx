import {useEffect,useMemo,useState} from 'react';
import {ArrowDownToLine,ArrowUpFromLine,BriefcaseBusiness,Clock3,Coins,RefreshCw,TrendingUp,TriangleAlert} from 'lucide-react';
import './ventures.css';

type Account={personalCapital:number;treasury:number;personalCapitalCap:number;dailyTransferLimit:number;transferUsedToday:number;transferRemaining:number;activeCapital:number;activeCapitalCap:number;activeCount:number;activeLimit:number;nextRefreshAt:string};
type Opportunity={id:string;title:string;description:string;risk:'low'|'medium'|'high';minInvestment:number;maxInvestment:number;durationHours:number;minReturnBps:number;maxReturnBps:number;expiresAt:string};
type Venture={id:string;title:string;description:string;risk:'low'|'medium'|'high';status:'active'|'claimable'|'collected'|'cancelled';amountInvested:number;outcomeBps?:number;payout?:number;maturesAt:string;resolvedAt?:string;collectedAt?:string;createdAt:string};
type Dashboard={account:Account;opportunities:Opportunity[];ventures:Venture[]};

const cash=(value:number)=>`¥${Math.round(value).toLocaleString()}`;
const percent=(bps:number)=>`${bps>0?'+':''}${(bps/100).toFixed(bps%100===0?0:1)}%`;
const remaining=(date:string,now:number)=>{const ms=new Date(date).getTime()-now;if(ms<=0)return'Resolving…';const hours=Math.floor(ms/3600000),minutes=Math.max(1,Math.ceil((ms%3600000)/60000));return hours>0?`${hours}h ${minutes}m`:`${minutes}m`};
async function request(path:string,options?:RequestInit){const response=await fetch('/api'+path,{credentials:'include',headers:{'Content-Type':'application/json'},...options}),body=await response.text();let data:any={};if(body){try{data=JSON.parse(body)}catch{throw Error(`The server returned an invalid response (HTTP ${response.status}).`)}}if(!response.ok)throw Error(data.error||`Request failed (HTTP ${response.status}).`);return data}

export default function VenturesScreen(){
 const[data,setData]=useState<Dashboard|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(''),[direction,setDirection]=useState<'to_personal'|'to_treasury'>('to_personal'),[transfer,setTransfer]=useState(''),[amounts,setAmounts]=useState<Record<string,string>>({}),[now,setNow]=useState(Date.now());
 const load=async()=>{try{setData(await request('/ventures'));setError('')}catch(e){setError((e as Error).message)}};
 useEffect(()=>{void load();const poll=setInterval(load,60000),clock=setInterval(()=>setNow(Date.now()),30000);return()=>{clearInterval(poll);clearInterval(clock)}},[]);
 const active=useMemo(()=>data?.ventures.filter(v=>v.status==='active'||v.status==='claimable')||[],[data]),history=useMemo(()=>data?.ventures.filter(v=>v.status==='collected'||v.status==='cancelled').slice(0,10)||[],[data]);
 const act=async(key:string,path:string,body?:unknown)=>{setBusy(key);setError('');try{await request(path,{method:'POST',body:body===undefined?undefined:JSON.stringify(body)});window.dispatchEvent(new Event('diplomatia:resources'));await load()}catch(e){setError((e as Error).message)}finally{setBusy('')}};
 const submitTransfer=(e:React.FormEvent)=>{e.preventDefault();const value=Math.floor(Number(transfer));if(value>0)void act('transfer','/ventures/transfer',{direction,amount:value}).then(()=>setTransfer(''))};
 if(!data)return <section className="panel ventures-loading"><RefreshCw className="spin"/><p>{error||'Loading private opportunities…'}</p></section>;
 const a=data.account;
 return <div className="ventures-page">
  <section className="ventures-hero"><div><span className="eyebrow">PERSONAL PORTFOLIO</span><h2>Private Ventures</h2><p>Commit limited personal capital to private opportunities while your nation’s larger economy continues independently.</p></div><BriefcaseBusiness/></section>
  {error&&<p className="error notice">{error}</p>}
  <section className="venture-summary">
   <article><span>Personal Capital</span><b>{cash(a.personalCapital)}</b><small>Deposit capacity {cash(Math.max(0,a.personalCapitalCap-a.personalCapital))}</small></article>
   <article><span>Daily transfer allowance</span><b>{cash(a.transferRemaining)}</b><small>{cash(a.transferUsedToday)} of {cash(a.dailyTransferLimit)} used</small></article>
   <article><span>Capital at work</span><b>{cash(a.activeCapital)}</b><small>{cash(a.activeCapitalCap)} maximum</small></article>
   <article><span>Active ventures</span><b>{a.activeCount} / {a.activeLimit}</b><small>Claimable results do not occupy a slot</small></article>
  </section>
  <section className="venture-transfer panel">
   <div><span className="eyebrow">CAPITAL TRANSFER</span><h2>Fund your portfolio</h2><p>Transfers in both directions share the same daily allowance. Venture proceeds return to Personal Capital before they can be moved into the national Treasury.</p></div>
   <form onSubmit={submitTransfer}>
    <div className="transfer-direction"><button type="button" className={direction==='to_personal'?'active':''} onClick={()=>setDirection('to_personal')}><ArrowDownToLine/>Treasury → Personal</button><button type="button" className={direction==='to_treasury'?'active':''} onClick={()=>setDirection('to_treasury')}><ArrowUpFromLine/>Personal → Treasury</button></div>
    <label>Cash amount<input type="number" min="1" step="1" max={a.transferRemaining} value={transfer} onChange={e=>setTransfer(e.target.value)} placeholder="0"/></label>
    <div className="transfer-balances"><span>National Treasury <b>{cash(a.treasury)}</b></span><span>Personal Capital <b>{cash(a.personalCapital)}</b></span></div>
    <button className="primary" disabled={busy!==''||!Number(transfer)}>{busy==='transfer'?'Transferring…':'Transfer funds'}</button>
   </form>
  </section>
  <section className="venture-section-heading"><div><span className="eyebrow">CURRENT POSITIONS</span><h2>Active ventures</h2></div><span>{active.length} position{active.length===1?'':'s'}</span></section>
  {active.length===0?<section className="panel venture-empty"><BriefcaseBusiness/><h3>No capital currently committed</h3><p>Select an opportunity below when you are ready.</p></section>:<div className="active-venture-grid">{active.map(v=>{const result=(v.payout||0)-v.amountInvested;return <article className={'panel active-venture '+v.status} key={v.id}><header><span className={'risk '+v.risk}>{v.risk} risk</span><span>{v.status==='claimable'?'Matured':remaining(v.maturesAt,now)}</span></header><h3>{v.title}</h3><p>{v.description}</p><div className="venture-position-value"><span>Invested<b>{cash(v.amountInvested)}</b></span>{v.status==='claimable'&&<><span>Result<b className={result>=0?'positive':'negative'}>{result>=0?'+':''}{cash(result)}</b></span><span>Collect<b>{cash(v.payout||0)}</b></span></>}</div>{v.status==='claimable'?<button className="primary" disabled={busy!==''} onClick={()=>act(v.id,'/ventures/'+v.id+'/collect')}>{busy===v.id?'Collecting…':'Collect result'}</button>:<button className="venture-cancel" disabled={busy!==''} onClick={()=>confirm(`Cancel ${v.title}? You will receive ${cash(Math.floor(v.amountInvested*.75))}, equal to 75% of the invested capital.`)&&act(v.id,'/ventures/'+v.id+'/cancel')}>{busy===v.id?'Cancelling…':'Cancel early — return 75%'}</button>}</article>})}</div>}
  <section className="venture-section-heading opportunities-heading"><div><span className="eyebrow">PRIVATE OPPORTUNITIES</span><h2>Available opportunities</h2></div><span><Clock3/>Refreshes in {remaining(a.nextRefreshAt,now)}</span></section>
  <div className="venture-opportunity-grid">{data.opportunities.map(o=><article className="panel venture-opportunity" key={o.id}><header><span className={'risk '+o.risk}>{o.risk} risk</span><span>{o.durationHours} hours</span></header><h3>{o.title}</h3><p>{o.description}</p><dl><div><dt>Investment range</dt><dd>{cash(o.minInvestment)} – {cash(o.maxInvestment)}</dd></div><div><dt>Possible return</dt><dd>{percent(o.minReturnBps)} to {percent(o.maxReturnBps)}</dd></div></dl><label>Commit Personal Capital<input type="number" min={o.minInvestment} max={o.maxInvestment} step="1" value={amounts[o.id]||''} onChange={e=>setAmounts({...amounts,[o.id]:e.target.value})} placeholder={cash(o.minInvestment)}/></label><button className="primary" disabled={busy!==''||!Number(amounts[o.id])} onClick={()=>act(o.id,'/ventures/invest',{opportunityId:o.id,amount:Math.floor(Number(amounts[o.id]))})}>{busy===o.id?'Committing…':'Invest'}</button></article>)}</div>
  <aside className="venture-risk-note"><TriangleAlert/><p><b>Returns are uncertain.</b> Displayed ranges describe the designed profile, not a guaranteed result. Cancelling before maturity permanently forfeits 25% of the invested capital.</p></aside>
  {history.length>0&&<section className="panel venture-history"><div><span className="eyebrow">RECENT ACTIVITY</span><h2>Portfolio history</h2></div><div>{history.map(v=><article key={v.id}><span><b>{v.title}</b><small>{v.status==='cancelled'?'Cancelled early':'Collected'} · {new Date(v.collectedAt||v.createdAt).toLocaleString()}</small></span><span>{cash(v.amountInvested)} → <b className={(v.payout||0)>=v.amountInvested?'positive':'negative'}>{cash(v.payout||0)}</b></span></article>)}</div></section>}
 </div>
}
