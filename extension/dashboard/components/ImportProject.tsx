import React,{useState} from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';
import { Callout, Card, Icon, SectionHeading, errorMessage } from './ui';

export default({base,reload}:{base:string;reload:()=>void})=>{
    const[busy,setBusy]=useState(false),[error,setError]=useState('');
    const submit=async(e:React.FormEvent<HTMLFormElement>)=>{e.preventDefault();setBusy(true);setError('');try{await http.post(`${base}/projects/import`,new FormData(e.currentTarget));reload()}catch(err:any){setError(errorMessage(err))}finally{setBusy(false)}};
    return <Card><SectionHeading icon="upload" title="Import an existing Packwiz project" description="Upload a ZIP whose project root contains pack.toml. The archive is validated before it becomes a managed project."/>
        {error&&<div style={{marginBottom:12}}><Callout tone="error" icon="warning" title="Import failed">{error}</Callout></div>}
        <form onSubmit={submit} className="pwm-form-grid">
            <div className="pwm-field full"><label className="pwm-label">Packwiz ZIP archive</label><Input type="file" name="archive" accept=".zip,application/zip" required/><span className="pwm-help">The ZIP may contain a single wrapping directory, but the detected pack root must include pack.toml.</span></div>
            <div className="pwm-field"><label className="pwm-label">Pack name</label><Input name="display_name" placeholder="KyberLand" required/></div>
            <div className="pwm-field"><label className="pwm-label">Slug</label><Input name="slug" placeholder="kyberland" pattern="[a-z0-9][a-z0-9-]{1,62}[a-z0-9]" required/></div>
            <div className="pwm-field"><label className="pwm-label">Minecraft version</label><Input name="minecraft_version" placeholder="1.21.1" required/></div>
            <div className="pwm-field"><label className="pwm-label">Pack version</label><Input name="pack_version" placeholder="0.1.0" required/></div>
            <div className="pwm-field"><label className="pwm-label">Loader</label><select className="pwm-select" name="loader" defaultValue="neoforge"><option value="neoforge">NeoForge</option><option value="fabric">Fabric</option><option value="forge">Forge</option><option value="quilt">Quilt</option></select></div>
            <div className="pwm-field"><label className="pwm-label">Loader version</label><Input name="loader_version" placeholder="21.1.200" required/></div>
            <div className="full"><Callout icon="shield" title="Safe import">Archive extraction is size-limited and path-validated. Existing project files are copied into Packwiz Manager's own project directory.</Callout></div>
            <div className="full pwm-actions-right"><Button type="submit" disabled={busy}><span style={{display:'inline-flex',alignItems:'center',gap:6}}><Icon name="upload" size={14}/>{busy?'Importing…':'Import Packwiz archive'}</span></Button></div>
        </form>
    </Card>;
};
