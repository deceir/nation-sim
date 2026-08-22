import {useEffect,useMemo,useState} from 'react';
import {Crosshair,Shield,Truck} from 'lucide-react';
import {calculateMaritimeRoute,projectNationLocation,type MapPoint,type MaritimeRoute} from './maritimeRouting';
import './war-theater-map.css';

type Belligerent={name:string;lat:number;lng:number};
type DeploymentUnit={deployed:number;remaining:number;lost:number};
type Deployment={id:string;side:'attacker'|'defender';status:'in_transit'|'preparing'|'deployed'|'concluded';arrivesRound:number;departedAt:string;arrivalAt?:string|null;units:Record<string,DeploymentUnit>};
type Props={attacker:Belligerent;defender:Belligerent;routeType:string;stage:string;distanceKm:number;roundsResolved:number;deployments:Deployment[]};

const unitNames:Record<string,string>={soldiers:'soldiers',tanks:'tanks',ships:'ships',jets:'fighter jets',drones:'drones'};
const normalizeX=(x:number)=>(x%1000+1000)%1000;
const pointList=(points:MapPoint[])=>points.map(point=>`${point.x},${point.y}`).join(' ');

function directPath(origin:MapPoint,destination:MapPoint){
 let x=destination.x;
 while(x-origin.x>500)x-=1000;
 while(origin.x-x>500)x+=1000;
 return[origin,{x,y:destination.y}];
}

function splitWrapped(points:MapPoint[]){
 const segments:MapPoint[][]=[];let current:MapPoint[]=[];
 for(let index=0;index<points.length;index++){
  const point=points[index];
  if(index===0){current=[{x:normalizeX(point.x),y:point.y}];continue}
  const previous=points[index-1],previousBand=Math.floor(previous.x/1000),band=Math.floor(point.x/1000);
  if(previousBand!==band){const boundary=point.x>previous.x?(previousBand+1)*1000:previousBand*1000,ratio=(boundary-previous.x)/(point.x-previous.x),y=previous.y+(point.y-previous.y)*ratio;current.push({x:point.x>previous.x?1000:0,y});segments.push(current);current=[{x:point.x>previous.x?0:1000,y},{x:normalizeX(point.x),y:point.y}]}else current.push({x:normalizeX(point.x),y:point.y});
 }
 if(current.length)segments.push(current);
 return segments;
}

function interpolate(points:MapPoint[],progress:number){
 const lengths=points.slice(1).map((point,index)=>Math.hypot(point.x-points[index].x,point.y-points[index].y)),total=lengths.reduce((sum,value)=>sum+value,0);
 let target=total*progress;
 for(let index=0;index<lengths.length;index++){
  if(target<=lengths[index]){const ratio=lengths[index]?target/lengths[index]:0;return{x:normalizeX(points[index].x+(points[index+1].x-points[index].x)*ratio),y:points[index].y+(points[index+1].y-points[index].y)*ratio}}
  target-=lengths[index];
 }
 const last=points.at(-1)!;return{x:normalizeX(last.x),y:last.y};
}

function deploymentProgress(deployment:Deployment,now:number){
 const departed=new Date(deployment.departedAt).getTime(),arrival=new Date(deployment.arrivalAt||'').getTime();
 if(!Number.isFinite(departed)||!Number.isFinite(arrival)||arrival<=departed)return .94;
 return Math.max(.025,Math.min(.975,(now-departed)/(arrival-departed)));
}

function forceTotal(deployment:Deployment){return Object.values(deployment.units||{}).reduce((sum,unit)=>sum+Number(unit.remaining||0),0)}
function forceDescription(deployment:Deployment){const parts=Object.entries(deployment.units||{}).filter(([,unit])=>Number(unit.remaining)>0).map(([unit,value])=>`${Number(value.remaining).toLocaleString()} ${unitNames[unit]||unit}`);return parts.join(', ')||'No surviving forces'}
function totalFor(deployments:Deployment[],side:'attacker'|'defender',statuses:string[]){return deployments.filter(item=>item.side===side&&statuses.includes(item.status)).reduce((sum,item)=>sum+forceTotal(item),0)}
function routeLabel(value:string){return value?value[0].toUpperCase()+value.slice(1):'Strategic'}

export default function WarTheaterMap({attacker,defender,routeType,stage,distanceKm,roundsResolved,deployments=[]}:Props){
 const[route,setRoute]=useState<MaritimeRoute>(),[routeError,setRouteError]=useState(false),[now,setNow]=useState(Date.now());
 const origin=useMemo(()=>({lat:Number(attacker.lat),lng:Number(attacker.lng)}),[attacker.lat,attacker.lng]);
 const destination=useMemo(()=>({lat:Number(defender.lat),lng:Number(defender.lng)}),[defender.lat,defender.lng]);
 const originPoint=projectNationLocation(origin.lat,origin.lng),destinationPoint=projectNationLocation(destination.lat,destination.lng),isLand=routeType==='land';
 useEffect(()=>{const timer=setInterval(()=>setNow(Date.now()),1000);return()=>clearInterval(timer)},[]);
 useEffect(()=>{let active=true;setRoute(undefined);setRouteError(false);if(isLand)return()=>{active=false};void calculateMaritimeRoute(origin,destination).then(value=>{if(active)setRoute(value)}).catch(()=>{if(active)setRouteError(true)});return()=>{active=false}},[origin,destination,isLand]);
 const landPath=useMemo(()=>directPath(originPoint,destinationPoint),[originPoint.x,originPoint.y,destinationPoint.x,destinationPoint.y]);
 const fullPath=isLand?landPath:(route?.fullPath||landPath),transit=stage==='ended'?[]:deployments.filter(item=>item.side==='attacker'&&item.status==='in_transit'),attackerAtFront=totalFor(deployments,'attacker',['deployed']),defenderAtFront=totalFor(deployments,'defender',['deployed']),defenderPreparing=totalFor(deployments,'defender',['preparing']);
 return <section className="panel war-theater">
  <header className="war-theater-heading"><div><span className="eyebrow">CAMPAIGN THEATER</span><h3>Deployment map</h3></div><div className="war-theater-summary"><span><b>{routeLabel(routeType)}</b> route</span><span><b>{Math.round(Number(distanceKm)).toLocaleString()} km</b> campaign distance</span><span><b>{transit.length}</b> force group{transit.length===1?'':'s'} in transit</span></div></header>
  <div className="war-theater-map">
   <svg viewBox="0 0 1000 500" role="img" aria-label={`War theater from ${attacker.name} to the campaign front in ${defender.name}`}>
    <image href="/world-map.svg" width="1000" height="500" preserveAspectRatio="none"/>
    {isLand?splitWrapped(landPath).map((segment,index)=><polyline className="war-land-route" points={pointList(segment)} key={index}/>):route?<><polyline className="war-coastal-leg" points={pointList([originPoint,route.originPort])}/>{route.seaSegments.map((segment,index)=><polyline className="war-sea-route" points={pointList(segment)} key={index}/>)}<polyline className="war-coastal-leg" points={pointList([route.destinationPort,destinationPoint])}/></>:splitWrapped(landPath).map((segment,index)=><polyline className="war-route-loading" points={pointList(segment)} key={index}/>)}
    <g className="war-origin" transform={`translate(${originPoint.x} ${originPoint.y})`}><circle r="10"/><circle r="4"/><title>{attacker.name} - attacking nation</title></g>
    <g className="war-front" transform={`translate(${destinationPoint.x} ${destinationPoint.y})`}><circle className="war-front-pulse" r="25"/><circle r="17"/><Crosshair x={-10} y={-10} size={20}/><title>{defender.name} - campaign front</title></g>
    {transit.map((deployment,index)=>{const marker=interpolate(fullPath,deploymentProgress(deployment,now)),offset=(index%3-1)*8,labelLeft=marker.x>920;return <g className="war-convoy" transform={`translate(${marker.x} ${marker.y+offset})`} key={deployment.id}><circle r="14"/><Truck x={-8} y={-8} size={16}/><text x={labelLeft?-18:18} y="4" textAnchor={labelLeft?'end':'start'}>R{deployment.arrivesRound}</text><title>{`Arriving round ${deployment.arrivesRound}: ${forceDescription(deployment)}`}</title></g>})}
    {defenderPreparing>0&&<g className="war-local-mobilization" transform={`translate(${destinationPoint.x-28} ${destinationPoint.y+25})`}><circle r="11"/><Shield x={-7} y={-7} size={14}/><title>{`${defenderPreparing.toLocaleString()} defending units preparing for round ${roundsResolved+1}`}</title></g>}
   </svg>
   <div className="war-map-status">{routeError?'Detailed route unavailable; showing the direct strategic corridor.':stage==='ended'?'Campaign concluded.':!isLand&&!route?'Charting campaign route…':transit.length?`${transit.length} attacking force group${transit.length===1?' is':'s are'} still mobilizing.`:'All committed attacking forces are in theater.'}</div>
  </div>
  <div className="war-theater-ledger"><div><span className="war-ledger-origin"/><p><b>{attacker.name}</b><small>Attacking homeland</small></p></div><div><span className="war-ledger-front"/><p><b>{defender.name}</b><small>Campaign front · {attackerAtFront.toLocaleString()} attacking / {defenderAtFront.toLocaleString()} defending units present</small></p></div><div><span className="war-ledger-transit"/><p><b>Mobilizing forces</b><small>Markers advance toward their scheduled arrival round</small></p></div></div>
 </section>;
}
