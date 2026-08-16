import React, { useEffect, useMemo, useState } from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import { Artwork, Callout, EmptyState, Icon, Pagination, Pill, SectionHeading, errorMessage } from './ui';

type Item = {
    id:string;
    kind:string;
    provider:string;
    provider_project_id?:string;
    provider_version_id?:string;
    display_name:string;
    target_path:string;
    filename:string;
    side:string;
    sha256?:string;
    enabled:boolean;
    metadata?:{ icon_url?:string; author?:string };
};

type ItemResponse = { items:Item[]; total:number; page:number; page_size:number };
type Mode = 'mods'|'files'|'custom'|'all';

const modeCopy: Record<Mode, { title:string; description:string; empty:string }> = {
    mods: { title:'Installed mods', description:'Provider-managed mods in this pack. Search, filter and change client/server side without leaving the table.', empty:'No provider mods match these filters.' },
    files: { title:'Managed files', description:'Configuration, datapacks, resource packs and other files tracked by Packwiz Manager.', empty:'No managed files match these filters.' },
    custom: { title:'Custom JARs', description:'Manually uploaded JARs are tracked with their checksum and destination path.', empty:'No custom JARs have been uploaded.' },
    all: { title:'Pack contents', description:'Every managed item in this Packwiz project.', empty:'This project has no managed items yet.' },
};

export default ({ base, project, mode = 'all', refreshKey = 0 }: { base:string; project:string; mode?:Mode; refreshKey?:number }) => {
    const [items,setItems]=useState<Item[]>([]),[error,setError]=useState(''),[loading,setLoading]=useState(false);
    const [page,setPage]=useState(1),[pageSize,setPageSize]=useState(25),[total,setTotal]=useState(0);
    const [query,setQuery]=useState(''),[provider,setProvider]=useState(''),[sideFilter,setSideFilter]=useState('');
    const [sort,setSort]=useState('name'),[direction,setDirection]=useState<'asc'|'desc'>('asc');

    const params = useMemo(() => ({ page, page_size:pageSize, q:query || undefined, provider:provider || undefined, side:sideFilter || undefined, group:mode, sort, direction }), [page,pageSize,query,provider,sideFilter,mode,sort,direction]);
    const load = async () => {
        setLoading(true); setError('');
        try {
            const r = await http.get(`${base}/projects/${project}/items`, { params });
            const data:ItemResponse = Array.isArray(r.data) ? { items:r.data,total:r.data.length,page:1,page_size:r.data.length || pageSize } : r.data;
            setItems(data.items || []); setTotal(data.total || 0);
            if (data.page && data.page !== page) setPage(data.page);
        } catch(e:any) { setError(errorMessage(e)); }
        finally { setLoading(false); }
    };

    useEffect(() => { const t=setTimeout(load, query ? 250 : 0); return()=>clearTimeout(t); }, [project,refreshKey,params]);
    useEffect(() => { setPage(1); }, [mode,provider,sideFilter,pageSize,sort,direction]);

    const setSide=async(id:string,value:string)=>{ try{ await http.patch(`${base}/projects/${project}/items/${id}/side`,{side:value}); await load(); }catch(e:any){setError(errorMessage(e));} };
    const remove=async(item:Item)=>{ if(!confirm(`Remove “${item.display_name}” from this Packwiz project?`))return; try{await http.delete(`${base}/projects/${project}/items/${item.id}`); if(items.length===1&&page>1)setPage(page-1); else await load();}catch(e:any){setError(errorMessage(e));} };
    const toggleSort=(next:string)=>{ if(sort===next)setDirection(direction==='asc'?'desc':'asc');else{setSort(next);setDirection('asc');} };
    const copyHash=async(hash?:string)=>{ if(!hash)return; try{await navigator.clipboard.writeText(hash);}catch{} };
    const copy = modeCopy[mode];

    return <div>
        <SectionHeading icon={mode==='files'?'files':mode==='custom'?'upload':'mods'} title={copy.title} description={copy.description}/>
        {error&&<div style={{marginBottom:12}}><Callout tone="error" icon="warning" title="Could not load pack contents">{error}</Callout></div>}
        <div className="pwm-toolbar">
            <div className="pwm-field grow"><label className="pwm-label" htmlFor={`pwm-item-search-${mode}`}>Search</label><div style={{position:'relative'}}><span style={{position:'absolute',left:9,top:9,color:'#64748b'}}><Icon name="search" size={16}/></span><input id={`pwm-item-search-${mode}`} className="pwm-select" style={{paddingLeft:32}} value={query} onChange={e=>{setQuery(e.target.value);setPage(1)}} placeholder="Name or target path…"/></div></div>
            <div className="pwm-field"><label className="pwm-label">Provider</label><select className="pwm-select" value={provider} onChange={e=>setProvider(e.target.value)}><option value="">All providers</option><option value="modrinth">Modrinth</option><option value="curseforge">CurseForge</option><option value="custom">Custom</option><option value="local">Local upload</option><option value="url">Remote URL</option></select></div>
            <div className="pwm-field"><label className="pwm-label">Side</label><select className="pwm-select" value={sideFilter} onChange={e=>setSideFilter(e.target.value)}><option value="">All sides</option><option value="both">Both</option><option value="server">Server</option><option value="client">Client</option></select></div>
            <div className="pwm-field"><label className="pwm-label">Rows</label><select className="pwm-select" value={pageSize} onChange={e=>setPageSize(Number(e.target.value))}><option>10</option><option>25</option><option>50</option><option>100</option></select></div>
            <button type="button" className="pwm-icon-button" onClick={load} disabled={loading} title="Refresh list"><Icon name="refresh" size={15}/> Refresh</button>
        </div>
        {loading&&<div className="pwm-spinner-line"/>}
        {!loading&&!items.length ? <EmptyState icon={mode==='files'?'files':'mods'} title={copy.empty}>{query||provider||sideFilter?'Try clearing one or more filters.':'Add content using the tools above.'}</EmptyState> : <>
            <div className="pwm-table-wrap">
                <table className="pwm-table">
                    <thead><tr><th><button type="button" className="pwm-icon-button" style={{border:0,background:'transparent',padding:0,textTransform:'uppercase',fontSize:10}} onClick={()=>toggleSort('name')}>Item {sort==='name'&&(direction==='asc'?'↑':'↓')}</button></th><th><button type="button" className="pwm-icon-button" style={{border:0,background:'transparent',padding:0,textTransform:'uppercase',fontSize:10}} onClick={()=>toggleSort('provider')}>Provider {sort==='provider'&&(direction==='asc'?'↑':'↓')}</button></th><th><button type="button" className="pwm-icon-button" style={{border:0,background:'transparent',padding:0,textTransform:'uppercase',fontSize:10}} onClick={()=>toggleSort('path')}>Target {sort==='path'&&(direction==='asc'?'↑':'↓')}</button></th><th>Side</th><th>Integrity</th><th style={{width:82}}>Actions</th></tr></thead>
                    <tbody>{items.map(i=><tr key={i.id}>
                        <td><div className="pwm-name-cell"><Artwork src={i.metadata?.icon_url} name={i.display_name} provider={i.provider}/><div style={{minWidth:0}}><div className="pwm-name">{i.display_name}</div><div className="pwm-secondary">{i.metadata?.author?`by ${i.metadata.author} · `:''}{i.kind}{i.provider_project_id?` · ${i.provider_project_id}`:''}</div></div></div></td>
                        <td><Pill tone={i.provider==='modrinth'?'good':i.provider==='curseforge'?'blue':i.provider==='custom'?'warn':'default'}>{i.provider}</Pill></td>
                        <td><div className="pwm-code">{i.target_path}</div>{i.filename&&i.filename!==i.target_path&&<div className="pwm-secondary">{i.filename}</div>}</td>
                        <td>{i.provider==='local'||i.provider==='url'?<Pill>{i.side}</Pill>:<select className="pwm-select" style={{minWidth:88,padding:'6px 8px'}} aria-label={`Side for ${i.display_name}`} value={i.side} onChange={e=>setSide(i.id,e.target.value)}><option value="both">both</option><option value="server">server</option><option value="client">client</option></select>}</td>
                        <td>{i.sha256?<button type="button" className="pwm-icon-button" style={{padding:'5px 7px'}} onClick={()=>copyHash(i.sha256)} title="Copy SHA-256"><Icon name="shield" size={13}/><span className="pwm-code">{i.sha256.slice(0,10)}…</span><Icon name="copy" size={12}/></button>:<span className="pwm-secondary">Provider metadata</span>}</td>
                        <td><Button isSecondary onClick={()=>remove(i)}><span style={{display:'inline-flex',alignItems:'center',gap:5}}><Icon name="trash" size={14}/> Remove</span></Button></td>
                    </tr>)}</tbody>
                </table>
            </div>
            <Pagination page={page} total={total} pageSize={pageSize} onPage={setPage} loading={loading}/>
        </>}
    </div>;
};
