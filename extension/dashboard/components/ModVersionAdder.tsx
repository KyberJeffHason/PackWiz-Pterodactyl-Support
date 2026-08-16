import React, { useState } from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import { Icon, errorMessage } from './ui';

type Provider = 'modrinth'|'curseforge';
type ProviderVersion = {
    id:string;
    name?:string;
    version_number?:string;
    channel?:string;
    published?:string;
    filename?:string;
};
type VersionResponse = { items:ProviderVersion[]; total:number; page:number; page_size:number };

type Props = {
    base:string;
    projectId:string;
    minecraftVersion:string;
    loader:string;
    provider:Provider;
    providerProjectId:string;
    displayName:string;
    iconUrl?:string;
    author?:string;
    onAdded:(message:string)=>void;
    onError:(message:string)=>void;
};

const versionLabel=(version:ProviderVersion)=>{
    const primary=version.version_number||version.name||version.id;
    const name=version.name&&version.name!==primary?` — ${version.name}`:'';
    const channel=version.channel&&version.channel!=='unknown'?` · ${version.channel}`:'';
    return `${primary}${name}${channel}`;
};

export default function ModVersionAdder({
    base,
    projectId,
    minecraftVersion,
    loader,
    provider,
    providerProjectId,
    displayName,
    iconUrl,
    author,
    onAdded,
    onError,
}:Props){
    const [versions,setVersions]=useState<ProviderVersion[]|null>(null);
    const [selectedVersion,setSelectedVersion]=useState('');
    const [loadingVersions,setLoadingVersions]=useState(false);
    const [adding,setAdding]=useState(false);
    const [localError,setLocalError]=useState('');

    const loadVersions=async()=>{
        setLoadingVersions(true);
        setLocalError('');
        try{
            const response=await http.get(`${base}/providers/${provider}/search`,{params:{
                versions_for:providerProjectId,
                minecraft:minecraftVersion,
                loader,
                page_size:50,
            }});
            const data:VersionResponse=Array.isArray(response.data)
                ?{items:response.data,total:response.data.length,page:1,page_size:response.data.length}
                :response.data;
            const items=data.items||[];
            setVersions(items);
            if(selectedVersion&&!items.some(version=>version.id===selectedVersion))setSelectedVersion('');
            if(!items.length)setLocalError(`No ${minecraftVersion} ${loader} versions were returned by ${provider==='modrinth'?'Modrinth':'CurseForge'}.`);
        }catch(error:any){
            const message=errorMessage(error);
            setLocalError(message);
            onError(message);
        }finally{
            setLoadingVersions(false);
        }
    };

    const add=async()=>{
        setAdding(true);
        setLocalError('');
        onError('');
        try{
            await http.post(`${base}/projects/${projectId}/mods`,{
                provider,
                project_id:providerProjectId,
                version_id:selectedVersion,
                display_name:displayName,
                side:'both',
                icon_url:iconUrl||'',
                author:author||'',
            });
            const selected=versions?.find(version=>version.id===selectedVersion);
            onAdded(selectedVersion
                ?`${displayName} ${versionLabel(selected||{id:selectedVersion})} added. Packwiz pinned the selected provider version and resolved dependencies.`
                :`${displayName} added. Packwiz selected the latest compatible version and resolved dependencies.`
            );
        }catch(error:any){
            const message=errorMessage(error);
            setLocalError(message);
            onError(message);
        }finally{
            setAdding(false);
        }
    };

    return <div className="pwm-grid" style={{gap:8,marginTop:4}}>
        <div className="pwm-toolbar" style={{alignItems:'end',gap:8}}>
            <div className="pwm-field grow">
                <label className="pwm-label">Version</label>
                <select className="pwm-select" value={selectedVersion} onChange={event=>setSelectedVersion(event.target.value)} disabled={loadingVersions||adding}>
                    <option value="">Latest compatible (automatic)</option>
                    {(versions||[]).map(version=><option key={version.id} value={version.id}>{versionLabel(version)}</option>)}
                </select>
                <span className="pwm-help">{versions===null?'Load versions to pin an exact provider release.':`${versions.length} compatible version${versions.length===1?'':'s'} loaded.`}</span>
            </div>
            <Button isSecondary onClick={loadVersions} disabled={loadingVersions||adding}>
                <span style={{display:'inline-flex',alignItems:'center',gap:5}}><Icon name="refresh" size={13}/>{loadingVersions?'Loading…':versions===null?'Load versions':'Refresh'}</span>
            </Button>
        </div>
        {localError&&<div className="pwm-help" style={{color:'#fca5a5'}}>{localError}</div>}
        <Button onClick={add} disabled={adding||loadingVersions}>
            <span style={{display:'inline-flex',alignItems:'center',gap:5}}><Icon name="plus" size={13}/>{adding?'Adding…':selectedVersion?'Add selected version':'Add to pack'}</span>
        </Button>
    </div>;
}
