export type ResourceIconType='cash'|'foodstuffs'|'timber'|'fibers'|'basic_metals'|'energy'|'strategic_minerals'|'textiles'|'processed_foods'|'construction_materials'|'basic_goods'|'consumer_goods'|'military_equipment'|'luxury_goods';

export default function ResourceIcon({type}:{type:ResourceIconType}){
 if(type==='cash')return <span className="yen-icon" aria-hidden="true">¥</span>;
 const paths:Record<Exclude<ResourceIconType,'cash'>,React.ReactNode>={
  foodstuffs:<path d="M8 14V4M8 7C5 7 4 5 4 3c3 0 4 2 4 4Zm0 3c3 0 4-2 4-4-3 0-4 2-4 4ZM8 5c2-1 2-3 1-4-2 1-2 3-1 4Z"/>,
  timber:<><path d="M3 3h10v3H3zm1 5h9v3H4z"/><path d="M6 3v3m4 2v3"/></>,
  fibers:<><path d="M3 12c3-8 7-8 10-9-1 4-3 8-10 9Z"/><path d="m4 11 7-6"/></>,
  basic_metals:<><path d="M2 5h12l-2 6H4Z"/><path d="M5 7h6"/></>,
  energy:<path d="m9 1-5 8h4l-1 6 5-8H8Z"/>,
  strategic_minerals:<><path d="m8 1 5 5-5 9-5-9Z"/><path d="M3 6h10M8 1v14"/></>,
  textiles:<><path d="M3 3h10v10H3Z"/><path d="m4 11 7-7m-4 9 6-6"/></>,
  processed_foods:<><path d="M3 4h10v9H3Z"/><path d="M5 4V2h6v2M5 7h6M5 10h4"/></>,
  construction_materials:<path d="M3 2h10v3H9.5v6H13v3H3v-3h3.5V5H3Z"/>,
  basic_goods:<><path d="M2 5h12v8H2Z"/><path d="m2 5 3-3h6l3 3M6 8h4"/></>,
  consumer_goods:<><path d="M3 5h10l-1 9H4Z"/><path d="M6 5a2 2 0 0 1 4 0"/></>,
  military_equipment:<><path d="M3 13V6l2-3 2 3v7Zm6 0V7l1.5-2L12 7v6Z"/><path d="M2 13h12"/></>,
  luxury_goods:<><path d="m8 1 5 4-2 8H5L3 5Z"/><path d="M3 5h10M6 5l2 8 2-8"/></>
 };
 return <svg className="resource-icon" viewBox="0 0 16 16" role="img" aria-label={`${type.replaceAll('_',' ')} icon`} fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round">{paths[type]}</svg>
}
