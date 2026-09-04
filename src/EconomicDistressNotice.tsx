import {AlertTriangle} from 'lucide-react';

export default function EconomicDistressNotice({status}:{status:any}){
 if(!status?.foodShortage&&!status?.upkeepDefault)return null;
 const causes:string[]=[];
 if(status.foodShortage)causes.push(`Food shortage (${Number(status.hourlyFoodAvailable||0).toFixed(2)} of ${Number(status.hourlyFoodRequired||0).toFixed(2)} t available this turn)`);
 if(status.upkeepDefault)causes.push(`Unpaid upkeep (¥${Math.floor(Number(status.cashAvailableForUpkeep||0)).toLocaleString()} of ¥${Math.ceil(Number(status.hourlyCashUpkeep||0)).toLocaleString()} available this turn)`);
 return <section className="notice economic-distress" role="status"><AlertTriangle/><div><b>National productivity reduced to 50%</b><span>{causes.join(' · ')}</span><small>The penalty clears automatically when both Foodstuffs and cash upkeep can be paid. Multiple causes do not stack.</small></div></section>;
}
