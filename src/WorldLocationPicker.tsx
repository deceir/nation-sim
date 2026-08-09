import React,{useId,useRef,useState}from'react';

export type WorldLocation={latitude:number;longitude:number;continent:string};
type Point=[number,number];
type Shape={name:string;points:Point[]};

const shapes:Shape[]=[
  {name:'North America',points:[[-168,72],[-52,72],[-52,48],[-78,7],[-100,8],[-118,18],[-130,48]]},
  {name:'South America',points:[[-82,13],[-34,8],[-45,-25],[-53,-56],[-74,-50],[-80,-5]]},
  {name:'Europe',points:[[-12,72],[45,72],[50,36],[25,34],[-10,36]]},
  {name:'Africa',points:[[-18,36],[52,36],[50,10],[35,-35],[10,-35],[-18,5]]},
  {name:'Asia',points:[[40,78],[180,75],[180,7],[135,-10],[105,0],[72,8],[40,36]]},
  {name:'Oceania',points:[[105,-8],[180,-8],[180,-50],[108,-50]]},
  {name:'Antarctica',points:[[-180,-60],[180,-60],[180,-90],[-180,-90]]},
];

const projected=([lng,lat]:Point)=>`${(lng+180)/360*1000},${(90-lat)/180*500}`;
function inPolygon(point:Point,polygon:Point[]){let inside=false;for(let i=0,j=polygon.length-1;i<polygon.length;j=i++){const[aLng,aLat]=polygon[i],[bLng,bLat]=polygon[j];if((aLat>point[1])!==(bLat>point[1])&&point[0]<(bLng-aLng)*(point[1]-aLat)/(bLat-aLat)+aLng)inside=!inside}return inside}
function locate(latitude:number,longitude:number){return shapes.find(shape=>inPolygon([longitude,latitude],shape.points))?.name}

export default function WorldLocationPicker({value,onChange,disabled=false,readOnly=false,compact=false}:{value:WorldLocation|null;onChange:(value:WorldLocation)=>void;disabled?:boolean;readOnly?:boolean;compact?:boolean}){
  const svg=useRef<SVGSVGElement>(null),labelId=useId(),[hint,setHint]=useState(readOnly?'Approximate national position':'Select a point on land.');
  const choose=(event:React.MouseEvent<SVGSVGElement>)=>{
    if(disabled||readOnly||!svg.current)return;
    const matrix=svg.current.getScreenCTM();if(!matrix)return;
    const cursor=svg.current.createSVGPoint();cursor.x=event.clientX;cursor.y=event.clientY;
    const point=cursor.matrixTransform(matrix.inverse()),longitude=point.x/1000*360-180,latitude=90-point.y/500*180,continent=locate(latitude,longitude);
    if(!continent){setHint('That point is ocean. Choose a position on land.');return}
    const next={latitude:Number(latitude.toFixed(4)),longitude:Number(longitude.toFixed(4)),continent};onChange(next);setHint(`${continent} selected.`);
  };
  return <div className={`world-location-picker${readOnly?' readonly':''}${compact?' compact':''}`}>
    <div className="location-heading"><div><span className="eyebrow">NATIONAL LOCATION</span><h3>{readOnly?'Position in the world':'Pick a position on the world map'}</h3></div>{value&&<strong>{value.continent}</strong>}</div>
    {!readOnly&&<p>Click the approximate location of your capital. Diplomatia determines the continent automatically and stores the coordinates for future distance-based mechanics.</p>}
    <svg ref={svg} viewBox="0 0 1000 500" onClick={choose} role="img" aria-labelledby={labelId} className={disabled?'disabled':''}>
      <title id={labelId}>Interactive world map location picker</title>
      <defs><radialGradient id="ocean"><stop stopColor="#1f2a36"/><stop offset="1" stopColor="#10161e"/></radialGradient></defs>
      <rect width="1000" height="500" fill="url(#ocean)"/>
      {[125,250,375].map(y=><line key={'y'+y} x1="0" x2="1000" y1={y} y2={y} className="map-grid"/>)}
      {[250,500,750].map(x=><line key={'x'+x} y1="0" y2="500" x1={x} x2={x} className="map-grid"/>)}
      {shapes.map(shape=><polygon key={shape.name} points={shape.points.map(projected).join(' ')} className={value?.continent===shape.name?'selected':''}><title>{shape.name}</title></polygon>)}
      {value&&<g className="map-marker" transform={`translate(${(value.longitude+180)/360*1000} ${(90-value.latitude)/180*500})`}><circle r="14"/><circle r="5"/></g>}
    </svg>
    <div className="location-readout"><span>{hint}</span>{value&&<code>{Math.abs(value.latitude).toFixed(4)}°{value.latitude>=0?'N':'S'} · {Math.abs(value.longitude).toFixed(4)}°{value.longitude>=0?'E':'W'}</code>}</div>
  </div>
}
