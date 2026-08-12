export type MapPoint={x:number;y:number};
type Cell={x:number;y:number};
type WaterGrid={width:number;height:number;water:Uint8Array;bounds:Array<[number,number]>;portals:Map<number,Cell[]>};
export type MaritimeRoute={originPort:MapPoint;destinationPort:MapPoint;seaSegments:MapPoint[][];fullPath:MapPoint[];usedFallback:boolean};

const GRID_WIDTH=480,GRID_HEIGHT=240;
const routeCache=new Map<string,MaritimeRoute>();
let gridPromise:Promise<WaterGrid>|undefined;

const rowBounds=(y:number,width:number,height:number):[number,number]=>{
 const normalized=Math.abs((y+.5)/height*2-1),half=.18+.32*Math.sqrt(Math.max(0,1-normalized*normalized));
 return[Math.ceil(width*(.5-half))+1,Math.floor(width*(.5+half))-1];
};

function loadWaterGrid(){
 if(gridPromise)return gridPromise;
 gridPromise=new Promise((resolve,reject)=>{
  const image=new Image();
  image.onload=()=>{try{
   const canvas=document.createElement('canvas');canvas.width=GRID_WIDTH;canvas.height=GRID_HEIGHT;
   const context=canvas.getContext('2d',{willReadFrequently:true});if(!context)throw Error('Canvas unavailable');
   context.clearRect(0,0,GRID_WIDTH,GRID_HEIGHT);context.drawImage(image,0,0,GRID_WIDTH,GRID_HEIGHT);
   const pixels=context.getImageData(0,0,GRID_WIDTH,GRID_HEIGHT).data,land=new Uint8Array(GRID_WIDTH*GRID_HEIGHT),water=new Uint8Array(GRID_WIDTH*GRID_HEIGHT),bounds:Array<[number,number]>=[];
   for(let y=0;y<GRID_HEIGHT;y++){bounds[y]=rowBounds(y,GRID_WIDTH,GRID_HEIGHT);for(let x=0;x<GRID_WIDTH;x++){const pixel=(y*GRID_WIDTH+x)*4,alpha=pixels[pixel+3],luminance=(pixels[pixel]+pixels[pixel+1]+pixels[pixel+2])/3;if(alpha>32&&luminance<235)land[y*GRID_WIDTH+x]=1}}
   // One-cell clearance keeps smoothed routes visibly off coastlines and also
   // absorbs antialiasing around the SVG's country boundaries.
   for(let y=0;y<GRID_HEIGHT;y++){const[minX,maxX]=bounds[y];for(let x=minX;x<=maxX;x++){let blocked=false;for(let oy=-1;oy<=1&&!blocked;oy++)for(let ox=-1;ox<=1;ox++){const ny=y+oy,nx=x+ox;if(ny>=0&&ny<GRID_HEIGHT&&nx>=0&&nx<GRID_WIDTH&&land[ny*GRID_WIDTH+nx]){blocked=true;break}}if(!blocked)water[y*GRID_WIDTH+x]=1}}
   const grid:WaterGrid={width:GRID_WIDTH,height:GRID_HEIGHT,water,bounds,portals:new Map()};installCanalPortals(grid);retainLargestOcean(grid);resolve(grid);
  }catch(error){reject(error)}};
  image.onerror=()=>reject(Error('Unable to load world map coastline data.'));
  image.src='/world-map.svg';
 });
 return gridPromise;
}

const geoToMap=(lat:number,lng:number):MapPoint=>({x:(lng+180)/360*1000,y:(90-lat)/180*500});
const toCell=(point:MapPoint,grid:WaterGrid):Cell=>({x:Math.max(0,Math.min(grid.width-1,Math.round(point.x/1000*(grid.width-1)))),y:Math.max(0,Math.min(grid.height-1,Math.round(point.y/500*(grid.height-1))))});
const toMap=(cell:Cell,grid:WaterGrid):MapPoint=>({x:cell.x/(grid.width-1)*1000,y:cell.y/(grid.height-1)*500});
const cellKey=(cell:Cell)=>`${cell.x}:${cell.y}`;
const isWater=(grid:WaterGrid,x:number,y:number)=>y>=0&&y<grid.height&&x>=0&&x<grid.width&&grid.water[y*grid.width+x]===1;

function nearestWater(grid:WaterGrid,point:Cell){
 if(isWater(grid,point.x,point.y))return point;
 for(let radius=1;radius<Math.max(grid.width,grid.height);radius++){
  let best:Cell|undefined,bestDistance=Infinity;
  for(let oy=-radius;oy<=radius;oy++)for(let ox=-radius;ox<=radius;ox++){
   if(Math.max(Math.abs(ox),Math.abs(oy))!==radius)continue;const y=point.y+oy;if(y<0||y>=grid.height)continue;
   let x=point.x+ox;if(x<0)x+=grid.width;if(x>=grid.width)x-=grid.width;
   if(isWater(grid,x,y)){const distance=ox*ox+oy*oy;if(distance<bestDistance){best={x,y};bestDistance=distance}}
  }
  if(best)return best;
 }
 return point;
}

class MinHeap{
 private values:Array<{id:number;score:number}>=[];
 push(value:{id:number;score:number}){this.values.push(value);let i=this.values.length-1;while(i){const p=(i-1)>>1;if(this.values[p].score<=value.score)break;this.values[i]=this.values[p];i=p;this.values[i]=value}}
 pop(){if(!this.values.length)return undefined;const first=this.values[0],last=this.values.pop()!;if(this.values.length){this.values[0]=last;let i=0;for(;;){const left=i*2+1,right=left+1;let next=i;if(left<this.values.length&&this.values[left].score<this.values[next].score)next=left;if(right<this.values.length&&this.values[right].score<this.values[next].score)next=right;if(next===i)break;[this.values[i],this.values[next]]=[this.values[next],this.values[i]];i=next}}return first}
 get size(){return this.values.length}
}

function waterNeighbors(grid:WaterGrid,cell:Cell){
 const result:Array<{cell:Cell;cost:number}>=[];
 for(let oy=-1;oy<=1;oy++)for(let ox=-1;ox<=1;ox++){
  if(!ox&&!oy)continue;const y=cell.y+oy;if(y<1||y>=grid.height-1)continue;const[minX,maxX]=grid.bounds[y];let x=cell.x+ox;if(x<minX)x=maxX;if(x>maxX)x=minX;
  if(isWater(grid,x,y))result.push({cell:{x,y},cost:ox&&oy?Math.SQRT2:1});
 }
 const portalID=cell.y*grid.width+cell.x;for(const destination of grid.portals.get(portalID)||[])result.push({cell:destination,cost:1});return result;
}

function installCanalPortals(grid:WaterGrid){
 const portalPairs=[
  // Mediterranean / Red Sea approaches to the Suez Canal.
  [{lat:31.8,lng:31.5},{lat:28.5,lng:33.5}],
  // Caribbean / Pacific approaches to the Panama Canal.
  [{lat:10.5,lng:-79.5},{lat:7.5,lng:-80.5}],
 ];
 for(const[p1,p2]of portalPairs){const a=nearestWater(grid,toCell(geoToMap(p1.lat,p1.lng),grid)),b=nearestWater(grid,toCell(geoToMap(p2.lat,p2.lng),grid)),aID=a.y*grid.width+a.x,bID=b.y*grid.width+b.x;grid.portals.set(aID,[...(grid.portals.get(aID)||[]),b]);grid.portals.set(bID,[...(grid.portals.get(bID)||[]),a])}
}

function retainLargestOcean(grid:WaterGrid){
 const visited=new Uint8Array(grid.water.length);let largest:number[]=[];
 for(let id=0;id<grid.water.length;id++){
  if(!grid.water[id]||visited[id])continue;const component:number[]=[],queue=[id];visited[id]=1;
  for(let cursor=0;cursor<queue.length;cursor++){const current=queue[cursor],cell={x:current%grid.width,y:Math.floor(current/grid.width)};component.push(current);for(const neighbor of waterNeighbors(grid,cell)){const next=neighbor.cell.y*grid.width+neighbor.cell.x;if(!visited[next]){visited[next]=1;queue.push(next)}}}
  if(component.length>largest.length)largest=component;
 }
 grid.water.fill(0);for(const id of largest)grid.water[id]=1;
}

const heuristic=(a:Cell,b:Cell,width:number)=>Math.hypot(Math.min(Math.abs(a.x-b.x),width-Math.abs(a.x-b.x)),a.y-b.y);
function aStar(grid:WaterGrid,start:Cell,end:Cell){
 const size=grid.width*grid.height,gScore=new Float64Array(size),cameFrom=new Int32Array(size),closed=new Uint8Array(size);gScore.fill(Infinity);cameFrom.fill(-1);
 const startID=start.y*grid.width+start.x,endID=end.y*grid.width+end.x,open=new MinHeap();gScore[startID]=0;open.push({id:startID,score:heuristic(start,end,grid.width)});
 let iterations=0;
 while(open.size&&iterations++<size*3){const current=open.pop()!,id=current.id;if(closed[id])continue;closed[id]=1;if(id===endID){const path:Cell[]=[];for(let at=id;at>=0;at=cameFrom[at]){path.unshift({x:at%grid.width,y:Math.floor(at/grid.width)});if(at===startID)break}return path}const cell={x:id%grid.width,y:Math.floor(id/grid.width)};for(const neighbor of waterNeighbors(grid,cell)){const nextID=neighbor.cell.y*grid.width+neighbor.cell.x;if(closed[nextID])continue;const candidate=gScore[id]+neighbor.cost;if(candidate<gScore[nextID]){cameFrom[nextID]=id;gScore[nextID]=candidate;open.push({id:nextID,score:candidate+heuristic(neighbor.cell,end,grid.width)})}}}
 return[start,end];
}

function unwrap(path:Cell[],width:number){
 if(!path.length)return[];const output=[{...path[0]}];for(let i=1;i<path.length;i++){let x=path[i].x,previous=output[i-1].x;while(x-previous>width/2)x-=width;while(previous-x>width/2)x+=width;output.push({x,y:path[i].y})}return output;
}
function wrappedWater(grid:WaterGrid,x:number,y:number){let normalized=Math.round(x)%grid.width;if(normalized<0)normalized+=grid.width;return isWater(grid,normalized,Math.round(y))}
function hasWaterLine(grid:WaterGrid,a:Cell,b:Cell){const steps=Math.max(1,Math.ceil(Math.hypot(b.x-a.x,b.y-a.y)*2));for(let i=0;i<=steps;i++){const t=i/steps;if(!wrappedWater(grid,a.x+(b.x-a.x)*t,a.y+(b.y-a.y)*t))return false}return true}
function smooth(grid:WaterGrid,path:Cell[]){if(path.length<3)return path;const output=[path[0]];let index=0;while(index<path.length-1){let next=path.length-1;while(next>index+1&&!hasWaterLine(grid,path[index],path[next]))next--;output.push(path[next]);index=next}return output}

function splitWrapped(points:MapPoint[]){
 const segments:MapPoint[][]=[];let current:MapPoint[]=[];
 for(let i=0;i<points.length;i++){
  const point=points[i];if(!i){current=[{x:(point.x%1000+1000)%1000,y:point.y}];continue}const previous=points[i-1],previousBand=Math.floor(previous.x/1000),band=Math.floor(point.x/1000);
  if(previousBand!==band){const boundary=point.x>previous.x?(previousBand+1)*1000:previousBand*1000,t=(boundary-previous.x)/(point.x-previous.x),y=previous.y+(point.y-previous.y)*t,right=point.x>previous.x?1000:0,left=point.x>previous.x?0:1000;current.push({x:right,y});segments.push(current);current=[{x:left,y},{x:(point.x%1000+1000)%1000,y:point.y}]}else current.push({x:(point.x%1000+1000)%1000,y:point.y});
 }
 if(current.length)segments.push(current);return segments;
}

function alignToRoute(point:MapPoint,reference:MapPoint){let x=point.x;while(x-reference.x>500)x-=1000;while(reference.x-x>500)x+=1000;return{x,y:point.y}}

export async function calculateMaritimeRoute(origin:{lat:number;lng:number},destination:{lat:number;lng:number}){
 const grid=await loadWaterGrid(),originMap=geoToMap(origin.lat,origin.lng),destinationMap=geoToMap(destination.lat,destination.lng),start=nearestWater(grid,toCell(originMap,grid)),end=nearestWater(grid,toCell(destinationMap,grid)),key=`${cellKey(start)}>${cellKey(end)}:${originMap.x.toFixed(2)},${originMap.y.toFixed(2)}:${destinationMap.x.toFixed(2)},${destinationMap.y.toFixed(2)}`;
 const cached=routeCache.get(key);if(cached)return cached;
 const raw=aStar(grid,start,end),unwrapped=smooth(grid,unwrap(raw,grid.width)),sea=unwrapped.map(cell=>({x:cell.x/(grid.width-1)*1000,y:cell.y/(grid.height-1)*500})),originPort=toMap(start,grid),destinationPort=toMap(end,grid),fullPath=[alignToRoute(originMap,sea[0]),...sea,alignToRoute(destinationMap,sea.at(-1)!)],route={originPort,destinationPort,seaSegments:splitWrapped(sea),fullPath,usedFallback:raw.length===2&&!hasWaterLine(grid,unwrap(raw,grid.width)[0],unwrap(raw,grid.width)[1])};routeCache.set(key,route);return route;
}

export const projectNationLocation=geoToMap;
