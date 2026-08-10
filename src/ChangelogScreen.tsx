import React,{useEffect,useRef,useState}from'react';
import{Bold,Braces,Edit3,Eye,Heading2,Italic,Link2,List,MessageSquareQuote,Send,Underline}from'lucide-react';

type Post={ID:string;Title:string;Body:string;AuthorName:string;AuthorNationID?:string;CreatedAt:string;UpdatedAt:string;CanEdit:boolean};
type Feed={posts:Post[];canPost:boolean;page:number;pages:number;total:number};

const escapeHTML=(value:string)=>value.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
function richText(value:string){
 let text=escapeHTML(value).replace(/\r\n?/g,'\n');
 text=text.replace(/```([\s\S]*?)```/g,'<pre><code>$1</code></pre>').replace(/\[code\]([\s\S]*?)\[\/code\]/gi,'<pre><code>$1</code></pre>');
 text=text.replace(/\[b\]([\s\S]*?)\[\/b\]/gi,'<strong>$1</strong>').replace(/\[i\]([\s\S]*?)\[\/i\]/gi,'<em>$1</em>').replace(/\[u\]([\s\S]*?)\[\/u\]/gi,'<u>$1</u>').replace(/\[s\]([\s\S]*?)\[\/s\]/gi,'<s>$1</s>');
 text=text.replace(/\[url=(https?:\/\/[^\]\s]+)\]([\s\S]*?)\[\/url\]/gi,'<a href="$1" target="_blank" rel="noopener noreferrer">$2</a>').replace(/\[color=(#[0-9a-f]{3,6})\]([\s\S]*?)\[\/color\]/gi,'<span style="color:$1">$2</span>');
 text=text.replace(/\[quote\]([\s\S]*?)\[\/quote\]/gi,'<blockquote>$1</blockquote>').replace(/\[list\]([\s\S]*?)\[\/list\]/gi,(_,items:string)=>`<ul>${items.split(/\[\*\]/i).filter(Boolean).map(x=>`<li>${x.trim()}</li>`).join('')}</ul>`);
 text=text.replace(/^### (.+)$/gm,'<h4>$1</h4>').replace(/^## (.+)$/gm,'<h3>$1</h3>').replace(/^# (.+)$/gm,'<h2>$1</h2>');
 text=text.replace(/\*\*([^\n*]+)\*\*/g,'<strong>$1</strong>').replace(/__([^\n_]+)__/g,'<strong>$1</strong>').replace(/(?<!\*)\*([^\n*]+)\*(?!\*)/g,'<em>$1</em>');
 text=text.replace(/\[([^\]\n]+)\]\((https?:\/\/[^)\s]+)\)/g,'<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
 text=text.replace(/^&gt; (.+)$/gm,'<blockquote>$1</blockquote>').replace(/^- (.+)$/gm,'<div class="rich-list-item">• <span>$1</span></div>');
 return text.replace(/\n/g,'<br/>').replace(/<\/pre><br\/>/g,'</pre>').replace(/<\/blockquote><br\/>/g,'</blockquote>');
}
async function request(path:string,options?:RequestInit){const response=await fetch('/api'+path,{credentials:'include',headers:{'Content-Type':'application/json'},...options});const raw=await response.text();let data:any={};try{data=raw?JSON.parse(raw):{}}catch{throw Error(`The server returned an invalid response (HTTP ${response.status}).`)}if(!response.ok)throw Error(data.error||'The changelog request failed.');return data}

export default function ChangelogScreen(){
 const[data,setData]=useState<Feed>(),[page,setPage]=useState(1),[loading,setLoading]=useState(true),[error,setError]=useState('');
 const[title,setTitle]=useState(''),[body,setBody]=useState(''),[editing,setEditing]=useState<string>(),[mode,setMode]=useState<'write'|'preview'>('write'),[saving,setSaving]=useState(false);
 const textarea=useRef<HTMLTextAreaElement>(null),editor=useRef<HTMLElement>(null);
 const load=async(next=page)=>{setLoading(true);setError('');try{const value=await request(`/changelog?page=${next}`);setData(value);setPage(value.page);window.dispatchEvent(new Event('diplomatia:changelog'))}catch(e){setError((e as Error).message)}finally{setLoading(false)}};
 useEffect(()=>{void load(page)},[page]);
 const insert=(before:string,after=before,placeholder='text')=>{const el=textarea.current;if(!el)return;const start=el.selectionStart,end=el.selectionEnd,selected=body.slice(start,end)||placeholder,next=body.slice(0,start)+before+selected+after+body.slice(end);setBody(next);requestAnimationFrame(()=>{el.focus();el.setSelectionRange(start+before.length,start+before.length+selected.length)})};
 const beginEdit=(post:Post)=>{setEditing(post.ID);setTitle(post.Title);setBody(post.Body);setMode('write');requestAnimationFrame(()=>editor.current?.scrollIntoView({behavior:'smooth',block:'start'}))};
 const reset=()=>{setEditing(undefined);setTitle('');setBody('');setMode('write')};
 const save=async()=>{setSaving(true);setError('');try{await request(editing?`/changelog/${editing}`:'/changelog',{method:editing?'PATCH':'POST',body:JSON.stringify({title,body})});reset();if(page!==1)setPage(1);else await load(1)}catch(e){setError((e as Error).message)}finally{setSaving(false)}};
 return <div className="changelog-page">
  <section className="changelog-hero"><span className="eyebrow">DIPLOMATIA DEVELOPMENT RECORD</span><h2>Changelog</h2><p>Updates to game systems, balance, interface, and development.</p></section>
  {error&&<p className="error notice">{error}</p>}
  <section className="changelog-feed" aria-busy={loading}>
   {loading&&!data?<div className="panel changelog-empty">Loading changelog…</div>:data?.posts.length?data.posts.map(post=><article className="changelog-entry" key={post.ID}>
    <header><div><time>{new Date(post.CreatedAt).toLocaleDateString(undefined,{year:'numeric',month:'long',day:'numeric'})}</time><h2>{post.Title}</h2></div>{post.CanEdit&&<button type="button" className="changelog-edit" onClick={()=>beginEdit(post)}><Edit3/> Edit post</button>}</header>
    <div className="changelog-content" dangerouslySetInnerHTML={{__html:richText(post.Body)}}/>
    <footer><span>Posted by <b>{post.AuthorName}</b></span><time>{new Date(post.CreatedAt).toLocaleString()}</time>{post.UpdatedAt!==post.CreatedAt&&<em>Edited {new Date(post.UpdatedAt).toLocaleString()}</em>}</footer>
   </article>):<div className="panel changelog-empty">No changelog entries have been published yet.</div>}
  </section>
  {!!data&&data.pages>1&&<nav className="changelog-pagination" aria-label="Changelog pages"><button disabled={page<=1} onClick={()=>setPage(page-1)}>Previous</button><span>Page {page} of {data.pages}</span><button disabled={page>=data.pages} onClick={()=>setPage(page+1)}>Next</button></nav>}
  {data?.canPost&&<section className="panel changelog-editor" ref={editor}>
   <header><div><span className="eyebrow">DEV PUBLISHING</span><h2>{editing?'Edit changelog entry':'Post to the changelog'}</h2></div>{editing&&<button type="button" onClick={reset}>Cancel edit</button>}</header>
   <label>Title<input value={title} onChange={e=>setTitle(e.target.value)} maxLength={180} placeholder="A clear summary of the update"/></label>
   <div className="editor-tabs"><button className={mode==='write'?'active':''} onClick={()=>setMode('write')}><Edit3/> Write</button><button className={mode==='preview'?'active':''} onClick={()=>setMode('preview')}><Eye/> Preview</button></div>
   {mode==='write'?<><div className="format-toolbar" role="toolbar" aria-label="Post formatting">
    <button title="Bold (BBCode)" onClick={()=>insert('[b]','[/b]')}><Bold/></button><button title="Italic (BBCode)" onClick={()=>insert('[i]','[/i]')}><Italic/></button><button title="Underline (BBCode)" onClick={()=>insert('[u]','[/u]')}><Underline/></button><button title="Heading (Markdown)" onClick={()=>insert('## ','','Heading')}><Heading2/></button><button title="Link (BBCode)" onClick={()=>insert('[url=https://example.com]', '[/url]','link text')}><Link2/></button><button title="Bulleted list (BBCode)" onClick={()=>insert('[list]\n[*]','\n[/list]','List item')}><List/></button><button title="Quote (BBCode)" onClick={()=>insert('[quote]','[/quote]','Quoted text')}><MessageSquareQuote/></button><button title="Code (BBCode)" onClick={()=>insert('[code]','[/code]','code')}><Braces/></button>
   </div><textarea ref={textarea} value={body} onChange={e=>setBody(e.target.value)} maxLength={50000} placeholder="Write the update using Markdown or BBCode…"/></>:<div className="changelog-preview">{body?<div className="changelog-content" dangerouslySetInnerHTML={{__html:richText(body)}}/>:<p>Your formatted preview will appear here.</p>}</div>}
   <div className="editor-footer"><small>Supports headings, bold, italic, links, lists, quotes, code, underline, strikethrough, and safe colors. Raw HTML is not rendered.</small><button className="primary" disabled={saving||title.trim().length<3||!body.trim()} onClick={()=>void save()}><Send/>{saving?'Saving…':editing?'Save changes':'Publish entry'}</button></div>
  </section>}
 </div>
}
