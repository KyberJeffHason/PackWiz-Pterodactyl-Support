import React, { useEffect, useState } from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';

type Item = { id:string; display_name:string; provider:string; target_path:string; side:string; sha256?:string };
export default ({ base, project }: { base:string; project:string }) => {
    const [items,setItems]=useState<Item[]>([]),[error,setError]=useState('');
    const load=()=>http.get(`${base}/projects/${project}/items`).then(r=>setItems(r.data||[])).catch(e=>setError(e.message));useEffect(()=>{load()},[project]);
    const side=(id:string,value:string)=>http.patch(`${base}/projects/${project}/items/${id}/side`,{side:value}).then(load).catch(e=>setError(e.response?.data?.error||e.message));
    const remove=(id:string)=>confirm('Remove this managed item?')&&http.delete(`${base}/projects/${project}/items/${id}`).then(load).catch(e=>setError(e.message));
    return <section><h3>Installed items</h3>{error&&<p role="alert">{error}</p>}<ul>{items.map(i=><li key={i.id}><strong>{i.display_name}</strong> · {i.provider} · <code>{i.target_path}</code> <select aria-label={`Side for ${i.display_name}`} value={i.side} onChange={e=>side(i.id,e.target.value)} disabled={i.provider==='local'||i.provider==='url'}><option>both</option><option>server</option><option>client</option></select> <Button isSecondary onClick={()=>remove(i.id)}>Remove</Button></li>)}</ul></section>;
};
