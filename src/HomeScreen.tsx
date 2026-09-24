import {useEffect,useMemo,useRef,useState} from 'react';
import {Activity,Building2,ChevronRight,Globe2,PackageCheck,Plane,RadioTower,Shield,Ship,ShoppingCart,Swords,TimerOff,Users,UsersRound} from 'lucide-react';
import {calculateMaritimeRoute,type MapPoint} from './maritimeRouting';
import './world-traffic.css';
import './world-news.css';

type HomeNation={id:string;name:string;leaderName:string;continent:string;population:number;powerLevel:number;cityCount:number;allianceID?:string;allianceName?:string;locationLat?:number|null;locationLng?:number|null};
type CurrentNation={ID:string;Name:string;LeaderName:string;Continent:string;Population:number;PowerLevel:number;LocationLat?:number|null;LocationLng?:number|null};
type Camera={zoom:number;x:number;y:number};
type HomeNews={id:string;category:'war'|'market'|'world';headline:string;summary:string;link?:string;createdAt:string};
type ConflictNation={id:string;name:string;lat:number;lng:number};
type ActiveConflict={id:string;attacker:ConflictNation;defender:ConflictNation};
const continentCenters:Record<string,[number,number]>={Africa:[12,7],Asia:[95,38],Europe:[17,51],'North America':[-104,43],'South America':[-61,-17],Oceania:[135,-25],Antarctica:[15,-76]};
const navigate=(path:string)=>{history.pushState(null,'',path);window.dispatchEvent(new PopStateEvent('popstate'))};
const compact=(value:number)=>Intl.NumberFormat(undefined,{notation:'compact',maximumFractionDigits:1}).format(Number(value||0));

export default function HomeScreen(){
 const[data,setData]=useState<any>(),[nations,setNations]=useState<HomeNation[]>([]),[nation,setNation]=useState<CurrentNation>(),[news,setNews]=useState<HomeNews[]>([]),[conflicts,setConflicts]=useState<ActiveConflict[]>([]),[error,setError]=useState('');
 useEffect(()=>{let active=true;Promise.all([fetch('/api/world/stats',{credentials:'include'}),fetch('/api/nations?sort=powerLevel',{credentials:'include'}),fetch('/api/me',{credentials:'include'}),fetch('/api/world/news',{credentials:'include'}),fetch('/api/conflicts?status=active&pageSize=50',{credentials:'include'})]).then(async([statsResponse,nationsResponse,meResponse,newsResponse,conflictsResponse])=>{if(!statsResponse.ok||!nationsResponse.ok||!meResponse.ok)throw Error('World intelligence is temporarily unavailable.');const[stats,worldNations,me,worldNews,worldConflicts]=await Promise.all([statsResponse.json(),nationsResponse.json(),meResponse.json(),newsResponse.ok?newsResponse.json():[],conflictsResponse.ok?conflictsResponse.json():{items:[]}]);if(active){setData(stats);setNations(worldNations);setNation(me.nation);setNews(worldNews);setConflicts(worldConflicts.items||[])}}).catch(reason=>active&&setError(reason.message));return()=>{active=false}},[]);
 const leaders=useMemo(()=>[...nations].sort((a,b)=>Number(b.powerLevel)-Number(a.powerLevel)||a.name.localeCompare(b.name)).slice(0,10),[nations]);
 const mapNations=useMemo(()=>{if(!nation)return[];const ranked:(HomeNation&{rank?:number;isCurrent:boolean})[]=leaders.map((item,index)=>({...item,rank:index+1,isCurrent:item.id===nation.ID}));if(!ranked.some(item=>item.id===nation.ID))ranked.push({id:nation.ID,name:nation.Name,leaderName:nation.LeaderName,continent:nation.Continent,population:nation.Population,powerLevel:nation.PowerLevel,cityCount:0,locationLat:nation.LocationLat,locationLng:nation.LocationLng,isCurrent:true});return ranked},[leaders,nation]);
 if(error)return <section className="panel wide home-error"><Globe2/><h2>World Home unavailable</h2><p>{error}</p></section>;
 if(!data||!nation)return <section className="home-loading"><Globe2/><span>Loading world command…</span></section>;
 const primaryStats=[['Nations',data.nations,Globe2],['Population',data.population,Users],['Provinces',data.cities,Building2],['Alliances',data.alliances,UsersRound],['Open offers',data.openMarketOrders,ShoppingCart],['Active wars',data.activeWars||0,Swords]] as const;
 return <div className="world-command">
  <section className="world-command-heading"><div><span className="eyebrow">WORLD OVERVIEW</span><h2>Diplomatia at a glance</h2></div><div className="world-live"><i/><span>Live world</span><b>{data.activePlayers} active now</b></div></section>
  <section className="world-indicator-strip">{primaryStats.map(([label,value,Icon])=><article key={label}><Icon/><span>{label}</span><b title={Number(value).toLocaleString()}>{compact(value)}</b></article>)}</section>
  <WorldNews items={news}/>
  <div className="world-command-grid">
   <section className="world-map-panel"><header><div><span className="eyebrow">GEOPOLITICAL MAP</span><h3>Leading nations</h3></div><span>Power Level ranking · top 10{mapNations.length>10?' + your nation':''}</span></header><WorldCommandMap nations={mapNations} conflicts={conflicts}/></section>
   <section className="world-rankings"><header><div><span className="eyebrow">WORLD RANKING</span><h3>Leading nations</h3></div><button onClick={()=>navigate('/leaderboards')}>All rankings <ChevronRight/></button></header><div className="world-ranking-list">{leaders.map((item,index)=><button className={item.id===nation.ID?'current':''} key={item.id} onClick={()=>navigate(`/nation/${encodeURIComponent(item.id)}`)}><span className="rank-number">{String(index+1).padStart(2,'0')}</span><img src={`/api/nations/${encodeURIComponent(item.id)}/flag`} alt=""/><span><b>{item.name}</b><small>{item.continent}{item.allianceName?` · ${item.allianceName}`:''}</small></span><strong title={`${Number(item.powerLevel||0).toLocaleString()} Power Level`}>{compact(item.powerLevel)} PL</strong></button>)}</div></section>
  </div>
  <div className="world-lower-grid">
   <section className="world-activity-panel"><header><div><span className="eyebrow">PLAYER ACTIVITY</span><h3>World presence</h3></div><Activity/></header><div><article><b>{Number(data.activePlayers).toLocaleString()}</b><span>Last 5 minutes</span></article><article><b>{Number(data.activePlayers24Hours).toLocaleString()}</b><span>Last 24 hours</span></article><article><b>{Number(data.activePlayersTwoWeeks).toLocaleString()}</b><span>Last 2 weeks</span></article></div></section>
   <section className="world-logistics-panel"><header><div><span className="eyebrow">GLOBAL COMMERCE</span><h3>Market traffic</h3></div><PackageCheck/></header><div className="world-ledger"><WorldLedger label="Trades conducted" value={data.totalTrades} icon={<ShoppingCart/>}/><WorldLedger label="Shipments underway" value={data.activeShipments} icon={<Ship/>}/><WorldLedger label="Delayed shipments" value={data.delayedTrades} icon={<TimerOff/>}/></div></section>
   <section className="world-military-panel"><header><div><span className="eyebrow">GLOBAL ARSENALS</span><h3>Military strength</h3></div><Shield/></header><div className="world-ledger"><WorldLedger label="Soldiers" value={data.military?.soldiers||0} icon={<Users/>}/><WorldLedger label="Tanks" value={data.military?.tanks||0} icon={<Shield/>}/><WorldLedger label="Ships" value={data.military?.ships||0} icon={<Ship/>}/><WorldLedger label="Fighter Jets" value={data.military?.jets||0} icon={<Plane/>}/><WorldLedger label="Drones" value={data.military?.drones||0} icon={<RadioTower/>}/></div></section>
  </div>
 </div>
}

function WorldNews({items}:{items:HomeNews[]}){return <section className="world-news-wire"><header><div><span className="eyebrow">WORLD NEWS</span><h3>Latest dispatches</h3></div><span className="world-news-live"><i/>Live desk</span></header><div className="world-news-scroll">{items.length?items.map(item=><article className={`world-news-item ${item.category}`} key={item.id} onClick={()=>item.link&&navigate(item.link)} tabIndex={item.link?0:undefined} role={item.link?'link':undefined} onKeyDown={event=>{if(item.link&&(event.key==='Enter'||event.key===' '))navigate(item.link)}}><span>{item.category}</span><div><b>{item.headline}</b><p>{item.summary}</p></div><time dateTime={item.createdAt}>{newsAge(item.createdAt)}</time></article>):<div className="world-news-empty">No major developments are being reported.</div>}</div></section>}

function newsAge(raw:string){const seconds=Math.max(0,Math.floor((Date.now()-new Date(raw).getTime())/1000));if(seconds<60)return'just now';if(seconds<3600)return`${Math.floor(seconds/60)}m`;if(seconds<86400)return`${Math.floor(seconds/3600)}h`;return`${Math.floor(seconds/86400)}d`}

function WorldCommandMap({nations,conflicts}:{nations:(HomeNation&{rank?:number;isCurrent:boolean})[];conflicts:ActiveConflict[]}){
 const svg=useRef<SVGSVGElement>(null),drag=useRef<{clientX:number;clientY:number;x:number;y:number}|null>(null),[camera,setCamera]=useState<Camera>({zoom:1,x:0,y:0});
 const clamp=(next:Camera)=>({...next,x:Math.min(0,Math.max(1000-1000*next.zoom,next.x)),y:Math.min(0,Math.max(500-500*next.zoom,next.y))});
 const zoom=(factor:number)=>setCamera(previous=>clamp({zoom:Math.min(4,Math.max(1,previous.zoom*factor)),x:previous.x,y:previous.y}));
 useEffect(()=>{const map=svg.current;if(!map)return;const wheel=(event:WheelEvent)=>{event.preventDefault();event.stopPropagation();zoom(event.deltaY<0?1.2:.84)};map.addEventListener('wheel',wheel,{passive:false});return()=>map.removeEventListener('wheel',wheel)},[]);
 const positioned=nations.map(item=>{const fallback=continentCenters[item.continent]||[0,0],jitter=((hash(item.id)%9)-4)*1.4,longitude=item.locationLng==null?fallback[0]+jitter:Number(item.locationLng),latitude=item.locationLat==null?fallback[1]+jitter*.4:Number(item.locationLat);return{...item,x:(longitude+180)/360*1000,y:(90-latitude)/180*500,approximate:item.locationLat==null||item.locationLng==null}});
 return <div className="world-command-map"><svg ref={svg} viewBox="0 0 1000 500" role="img" aria-label="World map showing leading Diplomatia nations" onPointerDown={event=>{if(camera.zoom>1){drag.current={clientX:event.clientX,clientY:event.clientY,x:camera.x,y:camera.y};event.currentTarget.setPointerCapture(event.pointerId)}}} onPointerMove={event=>{if(!drag.current||!svg.current)return;const rect=svg.current.getBoundingClientRect();setCamera(clamp({...camera,x:drag.current.x+(event.clientX-drag.current.clientX)*1000/rect.width,y:drag.current.y+(event.clientY-drag.current.clientY)*500/rect.height}))}} onPointerUp={()=>{drag.current=null}} onPointerCancel={()=>{drag.current=null}}>
  <rect width="1000" height="500" className="command-map-ocean"/><g transform={`translate(${camera.x} ${camera.y}) scale(${camera.zoom})`}><image className="command-map-land" href="/world-map.svg" width="1000" height="500" preserveAspectRatio="none"/><WorldTraffic/><ConflictActivity conflicts={conflicts} zoom={camera.zoom}/>{positioned.map(item=><g className={`command-nation-marker${item.isCurrent?' current':''}`} transform={`translate(${item.x} ${item.y}) scale(${1/camera.zoom})`} role="link" tabIndex={0} aria-label={`${item.name}, ${Number(item.powerLevel).toLocaleString()} Power Level`} onClick={()=>navigate(`/nation/${encodeURIComponent(item.id)}`)} onKeyDown={event=>{if(event.key==='Enter'||event.key===' ')navigate(`/nation/${encodeURIComponent(item.id)}`)}} key={item.id}><circle className="marker-halo" r="15"/><circle className="marker-core" r="8"/><text y="3">{item.rank||'•'}</text><g className="marker-label" transform="translate(13 -13)"><rect width={Math.max(82,item.name.length*7+28)} height="31" rx="2"/><text x="9" y="13">{item.name}</text><text className="marker-population" x="9" y="25">{compact(item.powerLevel)} PL{item.approximate?' · approximate location':''}</text></g><title>{item.name} · {Number(item.powerLevel).toLocaleString()} Power Level</title></g>)}</g>
 </svg><div className="world-map-controls"><button onClick={()=>zoom(1.35)} aria-label="Zoom in">+</button><button onClick={()=>zoom(.74)} aria-label="Zoom out">−</button><button onClick={()=>setCamera({zoom:1,x:0,y:0})}>World</button></div><div className="world-map-key"><span><i/>Top nations</span><span><i className="current"/>Your nation</span>{conflicts.length>0&&<span><i className="conflict"/>Active conflict</span>}</div></div>
}

function ConflictActivity({conflicts,zoom}:{conflicts:ActiveConflict[];zoom:number}){
 const fronts=conflicts.flatMap(conflict=>[
  {key:`${conflict.id}-attacker`,conflict,nation:conflict.attacker},
  {key:`${conflict.id}-defender`,conflict,nation:conflict.defender},
 ]).filter(front=>Number.isFinite(Number(front.nation.lat))&&Number.isFinite(Number(front.nation.lng)));
 return <g className="command-conflict-layer" aria-hidden="true">{fronts.map(front=>{const seed=hash(front.key),cycle=7+(seed%9),delay=-((seed%997)/997*cycle),jitterX=((seed>>>5)%9)-4,jitterY=((seed>>>11)%7)-3,x=(Number(front.nation.lng)+180)/360*1000+jitterX,y=(90-Number(front.nation.lat))/180*500+jitterY;return <g className="conflict-hotspot" key={front.key} transform={`translate(${x} ${y}) scale(${1/zoom})`} style={{'--conflict-cycle':`${cycle}s`,'--conflict-delay':`${delay}s`} as React.CSSProperties}><circle className="conflict-glow" r="3"/><circle className="conflict-shockwave conflict-shockwave-one" r="4"/><circle className="conflict-shockwave conflict-shockwave-two" r="4"/><path className="conflict-sparks" d="M0-4V-10 M3-3L8-8 M4 0H11 M3 3L8 8 M0 4V10 M-3 3L-8 8 M-4 0H-11 M-3-3L-8-8"/><title>{front.conflict.attacker.name}–{front.conflict.defender.name} conflict front</title></g>})}</g>
}

const aircraftRoutes=[
 {kind:'plane',path:'M 164 171 Q 423 55 688 174',duration:82,delay:-57},
 {kind:'plane',path:'M 521 167 Q 688 91 856 181',duration:70,delay:-21},
 {kind:'plane',path:'M 126 221 Q 318 105 512 178',duration:94,delay:-74},
] as const;

const shippingLanes=[
 {origin:{lat:40.7,lng:-74},destination:{lat:38.7,lng:-9.1},duration:112,delay:-40},
 {origin:{lat:-23.9,lng:-46.3},destination:{lat:-33.9,lng:18.4},duration:138,delay:-98},
 {origin:{lat:-4,lng:39.7},destination:{lat:1.3,lng:103.8},duration:126,delay:-68},
 {origin:{lat:33.7,lng:-118.2},destination:{lat:8.9,lng:-79.6},duration:105,delay:-29},
] as const;

const pathLength=(points:MapPoint[])=>points.slice(1).reduce((total,point,index)=>total+Math.hypot(point.x-points[index].x,point.y-points[index].y),0);
const svgPath=(points:MapPoint[])=>points.length?`M ${points.map(point=>`${point.x.toFixed(1)} ${point.y.toFixed(1)}`).join(' L ')}`:'';

function WorldTraffic(){
 const[shipRoutes,setShipRoutes]=useState<Array<{kind:'ship';path:string;duration:number;delay:number}>>([]);
 useEffect(()=>{let active=true;void Promise.all(shippingLanes.map(async lane=>{const route=await calculateMaritimeRoute(lane.origin,lane.destination),segment=[...route.seaSegments].sort((a,b)=>pathLength(b)-pathLength(a))[0]||[];return{kind:'ship' as const,path:svgPath(segment),duration:lane.duration,delay:lane.delay}})).then(routes=>{if(active)setShipRoutes(routes.filter(route=>route.path))}).catch(()=>{if(active)setShipRoutes([])});return()=>{active=false}},[]);
 const routes=[...shipRoutes,...aircraftRoutes];
 return <g className="command-map-traffic" aria-hidden="true">{routes.map((route,index)=><g key={`${route.kind}-${index}`}><path className={`traffic-route ${route.kind}`} d={route.path}/>{[0,.5].map(offset=><g className={`traffic-vehicle ${route.kind}`} key={offset}><animateMotion dur={`${route.duration}s`} begin={`${route.delay-route.duration*offset}s`} repeatCount="indefinite" rotate="auto" path={route.path}/>{route.kind==='ship'?<path d="M-9 1h18l-3 5H-5z M-4 0v-5h7l3 5z"/>:<image className="traffic-plane-image" href="/ambient-plane.png" x="-12" y="-8" width="24" height="16" preserveAspectRatio="xMidYMid meet"/>}</g>)}</g>)}</g>
}

function WorldLedger({label,value,icon}:{label:string;value:number;icon:React.ReactNode}){return <article>{icon}<span>{label}</span><b title={Number(value).toLocaleString()}>{compact(value)}</b></article>}
function hash(value:string){let result=0;for(let index=0;index<value.length;index++)result=(result*31+value.charCodeAt(index))|0;return Math.abs(result)}
