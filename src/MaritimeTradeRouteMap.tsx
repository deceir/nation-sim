import {useEffect,useMemo,useState} from 'react';
import {Ship} from 'lucide-react';

type GeoPoint={lat:number;lng:number};
type Port=GeoPoint&{id:string;links:string[]};

const ports:Port[]=[
 {id:'tokyo',lat:35,lng:140,links:['singapore']},
 {id:'singapore',lat:1,lng:104,links:['tokyo','mumbai','sydney']},
 {id:'mumbai',lat:19,lng:73,links:['singapore','suez','east-africa']},
 {id:'suez',lat:30,lng:32,links:['mumbai','east-africa','mediterranean']},
 {id:'mediterranean',lat:36,lng:14,links:['suez','rotterdam','west-africa']},
 {id:'rotterdam',lat:52,lng:4,links:['mediterranean','new-york']},
 {id:'east-africa',lat:-4,lng:40,links:['mumbai','suez','cape']},
 {id:'cape',lat:-34,lng:18,links:['east-africa','west-africa','sydney','rio']},
 {id:'west-africa',lat:6,lng:-5,links:['cape','mediterranean','new-york','rio']},
 {id:'new-york',lat:40,lng:-74,links:['rotterdam','west-africa','panama']},
 {id:'panama',lat:9,lng:-80,links:['new-york','los-angeles','rio','peru']},
 {id:'los-angeles',lat:34,lng:-118,links:['panama','peru']},
 {id:'rio',lat:-23,lng:-43,links:['panama','west-africa','cape','buenos-aires']},
 {id:'buenos-aires',lat:-35,lng:-58,links:['rio','peru']},
 {id:'peru',lat:-12,lng:-77,links:['panama','los-angeles','buenos-aires']},
 {id:'sydney',lat:-34,lng:151,links:['singapore','cape','west-australia']},
 {id:'west-australia',lat:-32,lng:115,links:['sydney','singapore','cape']},
];
const byID=new Map(ports.map(port=>[port.id,port]));
const radians=(n:number)=>n*Math.PI/180;
const geoDistance=(a:GeoPoint,b:GeoPoint)=>{const dLat=radians(b.lat-a.lat),dLng=radians(b.lng-a.lng),x=Math.sin(dLat/2)**2+Math.cos(radians(a.lat))*Math.cos(radians(b.lat))*Math.sin(dLng/2)**2;return 6371*2*Math.atan2(Math.sqrt(x),Math.sqrt(1-x))};
const nearestPort=(point:GeoPoint)=>ports.reduce((best,port)=>geoDistance(point,port)<geoDistance(point,best)?port:best,ports[0]);

function seaRoute(start:Port,end:Port){
 const distances=new Map(ports.map(port=>[port.id,Infinity])),previous=new Map<string,string>(),open=new Set(ports.map(port=>port.id));distances.set(start.id,0);
 while(open.size){let current='';for(const id of open)if(!current||(distances.get(id)??Infinity)<(distances.get(current)??Infinity))current=id;if(!current||current===end.id)break;open.delete(current);const port=byID.get(current)!;for(const nextID of port.links){if(!open.has(nextID))continue;const next=byID.get(nextID)!;const candidate=(distances.get(current)??Infinity)+geoDistance(port,next);if(candidate<(distances.get(nextID)??Infinity)){distances.set(nextID,candidate);previous.set(nextID,current)}}}
 const route:Port[]=[];let id:string|undefined=end.id;while(id){const port=byID.get(id);if(port)route.unshift(port);if(id===start.id)break;id=previous.get(id)}return route[0]?.id===start.id?route:[start,end];
}
const screenPoint=(point:GeoPoint)=>({x:(point.lng+180)/360*1000,y:(90-point.lat)/180*500});
const pointList=(points:GeoPoint[])=>points.map(point=>{const p=screenPoint(point);return `${p.x},${p.y}`}).join(' ');
const interpolate=(points:GeoPoint[],progress:number)=>{const projected=points.map(screenPoint),lengths=projected.slice(1).map((point,index)=>Math.hypot(point.x-projected[index].x,point.y-projected[index].y)),total=lengths.reduce((sum,n)=>sum+n,0);let target=total*progress;for(let i=0;i<lengths.length;i++){if(target<=lengths[i]){const ratio=lengths[i]?target/lengths[i]:0;return{x:projected[i].x+(projected[i+1].x-projected[i].x)*ratio,y:projected[i].y+(projected[i+1].y-projected[i].y)*ratio}}target-=lengths[i]}return projected.at(-1)!};

export default function MaritimeTradeRouteMap({shipment}:{shipment:any}){
 const[,tick]=useState(0);useEffect(()=>{const timer=setInterval(()=>tick(value=>value+1),1000);return()=>clearInterval(timer)},[]);
 const origin={lat:Number(shipment.originLat),lng:Number(shipment.originLng)},destination={lat:Number(shipment.destinationLat),lng:Number(shipment.destinationLng)};
 const originPort=nearestPort(origin),destinationPort=nearestPort(destination),ocean=useMemo(()=>seaRoute(originPort,destinationPort),[originPort.id,destinationPort.id]),route=[origin,...ocean,destination];
 const progress=shipment.status==='delivered'?1:(()=>{const start=new Date(shipment.departedAt).getTime(),end=new Date(shipment.estimatedArrivalAt).getTime();return Math.max(Number(shipment.progress)||0,Math.min(.99,(Date.now()-start)/(end-start)))})();
 const marker=interpolate(route,progress),originScreen=screenPoint(origin),destinationScreen=screenPoint(destination);
 return <div className="route-map maritime-route"><svg viewBox="0 0 1000 500" role="img" aria-label={`Maritime shipment route from ${shipment.seller} to ${shipment.buyer}`}><image href="/world-map.svg" width="1000" height="500" preserveAspectRatio="none"/><polyline className="land-leg" points={pointList([origin,originPort])}/><polyline className="sea-leg" points={pointList(ocean)}/><polyline className="land-leg" points={pointList([destinationPort,destination])}/><circle className="route-end origin" cx={originScreen.x} cy={originScreen.y} r="7"/><circle className="route-end" cx={destinationScreen.x} cy={destinationScreen.y} r="7"/><g className="trade-marker" transform={`translate(${marker.x} ${marker.y})`}><circle r="15"/><Ship size={18} x={-9} y={-9}/></g></svg><div className="route-label start">{shipment.seller}</div><div className="route-label end">{shipment.buyer}</div><div className="route-live"><i/>{shipment.status==='delivered'?'Delivered':`${Math.round(progress*100)}% en route`}</div><div className="route-legend"><span><i className="sea"/>Sea route</span><span><i className="land"/>Port connection</span></div></div>;
}
