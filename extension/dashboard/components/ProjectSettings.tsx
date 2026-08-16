import React,{useState} from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';
import ImportProject from './ImportProject';
import { Callout, Card, Icon, Pill, SectionHeading, errorMessage } from './ui';

export type Project={id:string;slug:string;display_name:string;minecraft_version:string;loader:string;loader_version?:string;pack_version:string;current_revision?:string;created_at?:string;updated_at?:string};

export default({base,project,reload}:{base:string;project:Project;reload:()=>void})=>{
    const[busy,setBusy]=useState(false),[error,setError]=useState(''),[saved,setSaved]=useState(false);
    const save=async(e:React.FormEvent<HTMLFormElement>)=>{e.preventDefault();setBusy(true);setError('');setSaved(false);try{await http.patch(`${base}/projects/${project.id}`,Object.fromEntries(new FormData(e.currentTarget)));setSaved(true);reload()}catch(err:any){setError(errorMessage(err))}finally{setBusy(false)}};
    return <div className="pwm-grid" style={{gap:14}}>
        <Card><SectionHeading icon="settings" title="Project settings" description="Update pack metadata. Packwiz is re-initialized with these values while preserving the managed project tree." actions={<Pill><Icon name="package" size={13}/>{project.slug}</Pill>}/>
            {error&&<div style={{marginBottom:12}}><Callout tone="error" icon="warning" title="Could not save settings">{error}</Callout></div>}{saved&&<div style={{marginBottom:12}}><Callout tone="good" icon="check">Project metadata saved successfully.</Callout></div>}
            <form onSubmit={save} className="pwm-form-grid">
                <div className="pwm-field full"><label className="pwm-label">Pack name</label><Input name="display_name" defaultValue={project.display_name} required/><span className="pwm-help">Human-readable name shown in the manager and generated Packwiz metadata.</span></div>
                <div className="pwm-field"><label className="pwm-label">Minecraft version</label><Input name="minecraft_version" defaultValue={project.minecraft_version} required/><span className="pwm-help">Example: 1.21.1</span></div>
                <div className="pwm-field"><label className="pwm-label">Pack version</label><Input name="pack_version" defaultValue={project.pack_version} required/><span className="pwm-help">Your modpack release version, independent of revision number.</span></div>
                <div className="pwm-field"><label className="pwm-label">Loader</label><select className="pwm-select" name="loader" defaultValue={project.loader} required><option value="neoforge">NeoForge</option><option value="fabric">Fabric</option><option value="forge">Forge</option><option value="quilt">Quilt</option></select></div>
                <div className="pwm-field"><label className="pwm-label">Loader version</label><Input name="loader_version" defaultValue={project.loader_version} required/></div>
                <div className="full pwm-actions-right"><Button type="submit" disabled={busy}><span style={{display:'inline-flex',alignItems:'center',gap:6}}><Icon name="check" size={14}/>{busy?'Saving…':'Save metadata'}</span></Button></div>
            </form>
        </Card>
        <Card muted><SectionHeading icon="info" title="Project identity" description="Stable identifiers are intentionally not editable because published URLs and integrations reference them."/><div className="pwm-kv"><div className="pwm-kv-key">Project ID</div><div className="pwm-kv-value pwm-code">{project.id}</div><div className="pwm-kv-key">Slug</div><div className="pwm-kv-value pwm-code">{project.slug}</div><div className="pwm-kv-key">Published revision</div><div className="pwm-kv-value">{project.current_revision||'Not published yet'}</div></div></Card>
        <ImportProject base={base} reload={reload} currentProject={project}/>
    </div>;
};
