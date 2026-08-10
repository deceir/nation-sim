import {Scale} from 'lucide-react';
import ResourceIcon from './ResourceIcon';
import './revenue-ledger.css';

const names:Record<string,string>={foodstuffs:'Foodstuffs',timber:'Timber',fibers:'Fibers',basic_metals:'Basic Metals',energy:'Energy',strategic_minerals:'Strategic Minerals',textiles:'Textiles',processed_foods:'Processed Foods',construction_materials:'Construction Materials',basic_goods:'Basic Goods',consumer_goods:'Consumer Goods',military_equipment:'Military Equipment',luxury_goods:'Luxury Goods'};
const icons:Record<string,any>={foodstuffs:'foodstuffs',timber:'timber',fibers:'fibers',basic_metals:'basic_metals',energy:'energy',strategic_minerals:'strategic_minerals',textiles:'textiles',processed_foods:'processed_foods',construction_materials:'construction_materials',basic_goods:'basic_goods',consumer_goods:'consumer_goods',military_equipment:'military_equipment',luxury_goods:'luxury_goods'};
const yen=(value:number)=>`¥${Math.round(value||0).toLocaleString()}`;
const amount=(value:number)=>Number(value||0).toLocaleString(undefined,{minimumFractionDigits:2,maximumFractionDigits:2});

function CashRow({label,daily,tone}:{label:string;daily:number;tone?:string}){return <div className={'ledger-row '+(tone||'')}><span>{label}</span><b>{yen(daily/24)}</b><b>{yen(daily)}</b></div>}
function ResourceRow({resource,daily,tone,detail}:{resource:string;daily:number;tone?:string;detail?:string}){return <div className={'ledger-row resource '+(tone||'')} title={detail}><span><ResourceIcon type={icons[resource]||'cash'}/>{names[resource]||resource}{detail&&<small>{detail}</small>}</span><b>{amount(daily/24)} t</b><b>{amount(daily)} t</b></div>}

export default function RevenueLedger({economy,strategy}:{economy:any;strategy:any}){
 const result=economy.result,model=strategy.result,production:Record<string,number>={},usage:Record<string,number>={},resources=Object.keys(names);
 for(const resource of resources)production[resource]=Number(model.production?.[resource]||0);
 usage.foodstuffs=Number(result.dailyFoodConsumption||0);
 usage.energy=Number(economy.military?.dailyEnergyUpkeep||0);
 usage.luxury_goods=Number(economy.luxuryConsumption?.projectedConsumption||0);
 for(const[output,recipe]of Object.entries(strategy.recipes||{}) as [string,Record<string,number>][])for(const[input,ratio]of Object.entries(recipe))usage[input]=(usage[input]||0)+Number(production[output]||0)*Number(ratio);
 const netResources=Object.fromEntries(resources.map(resource=>[resource,production[resource]-(usage[resource]||0)]));
 const allianceTax=Number(economy.alliance?.projectedDailyTax||0),citizenIncome=Number(result.dailyTax||0),luxuryIncome=Number(economy.luxuryConsumption?.projectedIncome||0),infrastructureUpkeep=Number((result.dailyInfrastructureUpkeep??result.dailyUpkeep)||0),civicUpkeep=Number(result.dailyCivicUpkeep||0),militaryUpkeep=Number(economy.military?.dailyCashUpkeep||0),netCash=citizenIncome+luxuryIncome-infrastructureUpkeep-civicUpkeep-militaryUpkeep-allianceTax;
 const usedResources=resources.filter(resource=>(usage[resource]||0)>0),producedResources=resources.filter(resource=>production[resource]>0);
 return <section className="panel national-ledger">
  <div className="ledger-title"><Scale/><div><span className="eyebrow">TOTALISTIC NATIONAL ACCOUNTS</span><h2>Complete revenue and resource summary</h2><p>Projected flows using the current Gear, policies, Projects, Province configuration, quotas, population, military, and Alliance tax.</p></div></div>
  <div className="ledger-head"><span>Account</span><b>Per turn</b><b>Per day</b></div>
  <details className="ledger-section" open><summary>Cash income</summary><CashRow label="Gross citizen tax revenue" daily={citizenIncome} tone="positive"/><CashRow label="Luxury Consumption Income" daily={luxuryIncome} tone="positive"/><CashRow label="Total cash income" daily={citizenIncome+luxuryIncome} tone="subtotal"/></details>
  <details className="ledger-section" open><summary>Cash expenses</summary><CashRow label="Infrastructure upkeep" daily={-infrastructureUpkeep} tone="negative"/><CashRow label="Civic Institution upkeep" daily={-civicUpkeep} tone="negative"/><CashRow label="Military upkeep" daily={-militaryUpkeep} tone="negative"/><CashRow label={economy.alliance?.name?`${economy.alliance.name} Alliance tax`:'Alliance tax'} daily={-allianceTax} tone="negative"/><CashRow label="Total cash expenses" daily={-(infrastructureUpkeep+civicUpkeep+militaryUpkeep+allianceTax)} tone="subtotal"/></details>
  <details className="ledger-section" open><summary>Resource usage</summary>{usedResources.length?usedResources.map(resource=><ResourceRow key={resource} resource={resource} daily={-(usage[resource]||0)} tone="negative" detail={resource==='foodstuffs'?`${amount(result.dailyFoodConsumption)} population upkeep plus ${amount((usage.foodstuffs||0)-Number(result.dailyFoodConsumption||0))} industrial input`:resource==='energy'?`${amount(economy.military?.dailyEnergyUpkeep||0)} military upkeep plus industrial inputs`:resource==='luxury_goods'?'Optional national Luxury Consumption':'Projected manufacturing input'}/>):<p className="ledger-empty">No resource consumption projected.</p>}</details>
  <details className="ledger-section" open><summary>Resource production</summary>{producedResources.length?producedResources.map(resource=><ResourceRow key={resource} resource={resource} daily={production[resource]} tone="positive"/>):<p className="ledger-empty">No commodity production projected.</p>}</details>
  <details className="ledger-section net" open><summary>Net national flows</summary><CashRow label="Treasury" daily={netCash} tone={netCash>=0?'positive':'negative'}/>{resources.map(resource=><ResourceRow key={resource} resource={resource} daily={Number(netResources[resource])} tone={Number(netResources[resource])>=0?'positive':'negative'}/>)}</details>
  <div className="ledger-overall"><span>Projected Treasury net income</span><b>{yen(netCash/24)} <small>per turn</small></b><b>{yen(netCash)} <small>per day</small></b></div><p className="ledger-footnote">All commodity figures are metric tonnes (t), not converted to Yen. Manufacturing output is conditional on sufficient input stockpiles at turn resolution; market prices remain player-defined.</p>
 </section>;
}
