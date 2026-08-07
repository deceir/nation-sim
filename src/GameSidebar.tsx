import {Building2,Coins,Globe2,Landmark,LogOut,Microscope,Settings,Shield,ShoppingCart,Swords,UserRound,UsersRound} from 'lucide-react';

type Props={tab:string;setTab:(tab:string)=>void;logout:()=>void};
const world=[['home','World Home',Globe2],['nations','Nations',UsersRound],['economy','Economy',Coins],['technology','Technology',Microscope],['market','World Market',ShoppingCart],['coalition','Coalitions',Shield],['military','Military',Swords]] as const;
const personal=[['overview','My Nation',Landmark],['profile','Edit Nation',UserRound],['settings','Settings',Settings]] as const;

export default function GameSidebar({tab,setTab,logout}:Props){return <aside className="game-sidebar"><div className="brand"><div className="crest">D</div><b>Diplomatia</b></div><div className="nav-groups"><nav className="world-nav"><span className="nav-heading">THE WORLD</span>{world.map(([key,label,Icon])=><button className={tab===key?'active':''} onClick={()=>setTab(key)} key={key}><Icon size={16}/><span>{label}</span></button>)}</nav><nav className="personal-nav"><span className="nav-heading">YOUR ACCOUNT</span>{personal.map(([key,label,Icon])=><button className={tab===key?'active':''} onClick={()=>setTab(key)} key={key}><Icon size={16}/><span>{label}</span></button>)}<button className="logout" onClick={logout}><LogOut size={16}/><span>Sign out</span></button></nav></div></aside>}
