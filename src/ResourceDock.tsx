import {useEffect,useState} from 'react';
import type {ComponentProps} from 'react';
import {Boxes} from 'lucide-react';
import ResourceIcon from './ResourceIcon';

const items=[['foodstuffs','Foodstuffs','Food'],['timber','Timber','Bauxite'],['fibers','Fibers','Food'],['basic_metals','Basic Metals','Iron'],['energy','Energy','Oil'],['strategic_minerals','Strategic Minerals','Uranium'],['textiles','Textiles','Aluminum'],['processed_foods','Processed Foods','Food'],['construction_materials','Construction Materials','Steel'],['basic_goods','Basic Goods','Steel'],['consumer_goods','Consumer Goods','Aluminum'],['military_equipment','Military Equipment','Munitions'],['luxury_goods','Luxury Goods','Uranium']] as const;

const fullNumber=(value:number)=>value.toLocaleString(undefined,{maximumFractionDigits:6});
const compactNumber=(value:number)=>{
 const absolute=Math.abs(value);
 const units:[[number,string],[number,string],[number,string],[number,string]]=[[1e12,'T'],[1e9,'B'],[1e6,'M'],[1e3,'k']];
 for(const[threshold,suffix]of units)if(absolute>=threshold){const scaled=value/threshold;return `${Number(scaled.toFixed(1))}${suffix}`}
 return fullNumber(value);
};

function DockItem({label,icon,value,cash=false}:{label:string;icon:ComponentProps<typeof ResourceIcon>['type'];value:number;cash?:boolean}){
 const full=(cash?'¥':'')+fullNumber(value);
 return <div className={'dock-item '+(cash?'cash':'')} title={`${label}: ${full}`} aria-label={`${label}: ${full}`}><ResourceIcon type={icon}/><div><span>{label}</span><b>{cash?'¥':''}{compactNumber(value)}</b></div></div>;
}

export default function ResourceDock(){
 const[data,setData]=useState<any>();
 useEffect(()=>{const load=()=>Promise.all([fetch('/api/me',{credentials:'include'}).then(r=>r.json()),fetch('/api/strategy',{credentials:'include'}).then(r=>r.json())]).then(([me,strategy])=>setData({nation:me.nation,stockpiles:strategy.stockpiles||{}}));void load();const refresh=()=>{void load()};window.addEventListener('diplomatia:resources',refresh);const timer=setInterval(load,15000);return()=>{clearInterval(timer);window.removeEventListener('diplomatia:resources',refresh)}},[]);
 if(!data)return <div className="resource-dock loading">Loading national stockpiles…</div>;
 return <section className="resource-dock" aria-label="Resources on hand"><div className="dock-title"><Boxes size={15}/><span>ON HAND</span></div><DockItem label="Treasury" icon="Treasury" value={Number(data.nation.Treasury||0)} cash/>{items.map(([key,label,icon])=><DockItem key={key} label={label} icon={icon} value={Number(data.stockpiles[key]||0)}/>)}</section>;
}
