import React,{useEffect,useId,useRef,useState}from'react';
import {isLand}from'./landMask';

export type WorldLocation={latitude:number;longitude:number;continent:string};
type Point=[number,number];
type Shape={name:string;points:Point[]};
type Camera={zoom:number;x:number;y:number};

const shapes:Shape[]=[
  {name:'North America',points:[[-168,72],[-52,72],[-52,48],[-78,7],[-100,8],[-118,18],[-130,48]]},
  {name:'South America',points:[[-82,13],[-34,8],[-45,-25],[-53,-56],[-74,-50],[-80,-5]]},
  {name:'Europe',points:[[-12,72],[45,72],[50,36],[25,34],[-10,36]]},
  {name:'Africa',points:[[-18,36],[52,36],[50,10],[35,-35],[10,-35],[-18,5]]},
  {name:'Asia',points:[[40,78],[180,75],[180,7],[135,-10],[105,0],[72,8],[40,36]]},
  {name:'Oceania',points:[[105,-8],[180,-8],[180,-50],[108,-50]]},
  {name:'Antarctica',points:[[-180,-60],[180,-60],[180,-90],[-180,-90]]},
];

function inPolygon(point:Point,polygon:Point[]){let inside=false;for(let i=0,j=polygon.length-1;i<polygon.length;j=i++){const[aLng,aLat]=polygon[i],[bLng,bLat]=polygon[j];if((aLat>point[1])!==(bLat>point[1])&&point[0]<(bLng-aLng)*(point[1]-aLat)/(bLat-aLat)+aLng)inside=!inside}return inside}
function locate(latitude:number,longitude:number){return shapes.find(shape=>inPolygon([longitude,latitude],shape.points))?.name}
function clampCamera(camera:Camera):Camera{return{...camera,x:Math.min(0,Math.max(1000-1000*camera.zoom,camera.x)),y:Math.min(0,Math.max(500-500*camera.zoom,camera.y))}}

export default function WorldLocationPicker({value,onChange,disabled=false,readOnly=false,compact=false}:{value:WorldLocation|null;onChange:(value:WorldLocation)=>void;disabled?:boolean;readOnly?:boolean;compact?:boolean}){
  const svg=useRef<SVGSVGElement>(null),labelId=useId(),drag=useRef<{clientX:number;clientY:number;x:number;y:number;moved:boolean}|null>(null),ignoreClick=useRef(false),[hint,setHint]=useState(readOnly?'Approximate national position':'Select a point on land.'),[camera,setCamera]=useState<Camera>({zoom:1,x:0,y:0});
  const screenPoint=(clientX:number,clientY:number)=>{if(!svg.current)return null;const matrix=svg.current.getScreenCTM();if(!matrix)return null;const cursor=svg.current.createSVGPoint();cursor.x=clientX;cursor.y=clientY;return cursor.matrixTransform(matrix.inverse())};
  const setZoom=(nextZoom:number,focus={x:500,y:250})=>setCamera(old=>{const zoom=Math.min(6,Math.max(1,nextZoom)),ratio=zoom/old.zoom;return clampCamera({zoom,x:focus.x-(focus.x-old.x)*ratio,y:focus.y-(focus.y-old.y)*ratio})});
  useEffect(()=>{const map=svg.current;if(!map)return;const wheel=(event:WheelEvent)=>{event.preventDefault();event.stopPropagation();const point=screenPoint(event.clientX,event.clientY);if(point)setZoom(camera.zoom*(event.deltaY<0?1.25:.8),point)};map.addEventListener('wheel',wheel,{passive:false});return()=>map.removeEventListener('wheel',wheel)},[camera.zoom]);
  const choose=(event:React.MouseEvent<SVGSVGElement>)=>{
    if(ignoreClick.current){ignoreClick.current=false;return}
    if(disabled||readOnly)return;const point=screenPoint(event.clientX,event.clientY);if(!point)return;
    const mapX=(point.x-camera.x)/camera.zoom,mapY=(point.y-camera.y)/camera.zoom,longitude=mapX/1000*360-180,latitude=90-mapY/500*180;
    if(!isLand(latitude,longitude)){setHint('That point is ocean. Choose a position on land.');return}
    const continent=locate(latitude,longitude);
    if(!continent){setHint('That land position could not be assigned to a continent. Choose a nearby point.');return}
    const next={latitude:Number(latitude.toFixed(4)),longitude:Number(longitude.toFixed(4)),continent};onChange(next);setHint(`${continent} selected.`);
  };
  const pointerDown=(event:React.PointerEvent<SVGSVGElement>)=>{if(camera.zoom<=1||disabled)return;drag.current={clientX:event.clientX,clientY:event.clientY,x:camera.x,y:camera.y,moved:false};event.currentTarget.setPointerCapture(event.pointerId)};
  const pointerMove=(event:React.PointerEvent<SVGSVGElement>)=>{if(!drag.current||!svg.current)return;const rect=svg.current.getBoundingClientRect(),dx=(event.clientX-drag.current.clientX)*1000/rect.width,dy=(event.clientY-drag.current.clientY)*500/rect.height;if(Math.abs(dx)+Math.abs(dy)>3)drag.current.moved=true;setCamera(clampCamera({zoom:camera.zoom,x:drag.current.x+dx,y:drag.current.y+dy}))};
  const pointerUp=()=>{if(drag.current?.moved)ignoreClick.current=true;drag.current=null};
  const markerX=value?(value.longitude+180)/360*1000:0,markerY=value?(90-value.latitude)/180*500:0;
  return <div className={`world-location-picker${readOnly?' readonly':''}${compact?' compact':''}`}>
    <div className="location-heading"><div><span className="eyebrow">NATIONAL LOCATION</span><h3>{readOnly?'Position in the world':'Pick a position on the world map'}</h3></div>{value&&<strong>{value.continent}</strong>}</div>
    {!readOnly&&<p>Click the approximate location of your capital. Scroll to zoom and drag to pan. Diplomatia determines the continent automatically and stores the coordinates for future distance-based mechanics.</p>}
    <div className="map-viewport">
      <svg ref={svg} viewBox="0 0 1000 500" onClick={choose} onPointerDown={pointerDown} onPointerMove={pointerMove} onPointerUp={pointerUp} onPointerCancel={pointerUp} role="img" aria-labelledby={labelId} className={disabled?'disabled':''}>
        <title id={labelId}>Interactive zoomable world map location picker</title>
        <rect width="1000" height="500" className="map-ocean"/>
        <g transform={`translate(${camera.x} ${camera.y}) scale(${camera.zoom})`}>
          <image href="/world-map.svg" x="0" y="0" width="1000" height="500" preserveAspectRatio="none" className="detailed-world-map"/>
          {value&&<g className="map-marker" transform={`translate(${markerX} ${markerY}) scale(${1/camera.zoom})`}><circle r="14"/><circle r="5"/></g>}
        </g>
      </svg>
      <div className="map-controls" aria-label="Map zoom controls"><button type="button" onClick={()=>setZoom(camera.zoom*1.4)} aria-label="Zoom in">+</button><button type="button" onClick={()=>setZoom(camera.zoom/1.4)} aria-label="Zoom out">−</button><button type="button" onClick={()=>setCamera({zoom:1,x:0,y:0})} disabled={camera.zoom===1} aria-label="Reset map view">Reset</button></div>
      <span className="map-zoom">{Math.round(camera.zoom*100)}%</span>
    </div>
    <div className="location-readout"><span>{hint}</span>{value&&<code>{Math.abs(value.latitude).toFixed(4)}°{value.latitude>=0?'N':'S'} · {Math.abs(value.longitude).toFixed(4)}°{value.longitude>=0?'E':'W'}</code>}</div>
  </div>
}
