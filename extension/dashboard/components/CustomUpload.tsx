import React,{useState} from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';
import ItemsPanel from './ItemsPanel';
import {Callout,Card,SectionHeading,errorMessage} from './ui';
import type {Project} from './ProjectSettingsV2';

const jarDestination=(value:string)=>{
    const base=value.replace(/\.jar$/i,'').trim().replace(/[^A-Za-z0-9._+-]+/g,'-').replace(/^[-.]+|[-.]+$/g,'')||'custom';
    return `mods/${base}.jar`;
};

export default function CustomUpload({base,project,refreshKey,onChanged,onError}:{base:string;project:Project;refreshKey:number;onChanged:(m:string)=>void;onError:(m:string)=>void}){
    const[busy,setBusy]=useState(false),[modName,setModName]=useState(''),[destination,setDestination]=useState('mods/custom.jar');
    const chooseFile=(e:React.ChangeEvent<HTMLInputElement>)=>{
        const file=e.currentTarget.files?.[0];
        if(!file)return;
        setModName(file.name.replace(/\.jar$/i,''));
        setDestination(jarDestination(file.name));
    };
    const upload=async(e:React.FormEvent<HTMLFormElement>)=>{
        e.preventDefault();
        const form=e.currentTarget;
        setBusy(true);
        try{
            await http.post(`${base}/projects/${project.id}/custom-jars`,new FormData(form));
            form.reset();
            setModName('');setDestination('mods/custom.jar');
            onChanged('Custom JAR uploaded.');
        }catch(x:any){onError(errorMessage(x))}finally{setBusy(false)}
    };
    return <div className="pwm-grid">
        <Card><SectionHeading icon="upload" title="Custom JAR" description="Upload a trusted mod JAR directly into the Minecraft mods/ directory."/>
            <Callout tone="warn" icon="warning"><strong>Custom Uploads are for mods only.</strong>JARs uploaded here are installed directly under <span className="pwm-code">mods/</span>. Use Files for other locations such as <span className="pwm-code">config/bluemap/packs</span>.</Callout>
            <form onSubmit={upload} className="pwm-form-grid" style={{marginTop:12}}>
                <div className="pwm-field full"><label className="pwm-label">JAR file</label><Input name="file" type="file" accept=".jar,application/java-archive" onChange={chooseFile} required/><span className="pwm-help">Selecting a JAR fills the display name and destination filename as starting values. You can change either field independently.</span></div>
                <div className="pwm-field"><label className="pwm-label">Mod name</label><Input name="display_name" value={modName} onChange={e=>setModName(e.currentTarget.value)} placeholder="My Mod" required/><span className="pwm-help">Display name only. Changing this does not rename the JAR.</span></div>
                <div className="pwm-field"><label className="pwm-label">Destination (mods only)</label><Input name="destination" value={destination} onChange={e=>setDestination(e.currentTarget.value)} pattern="mods/[A-Za-z0-9._+-]+\\.jar" required/><span className="pwm-help">Must be a JAR directly under <span className="pwm-code">mods/</span>, for example <span className="pwm-code">mods/Create+Addon.jar</span>.</span></div>
                <div className="pwm-field"><label className="pwm-label">Side</label><select className="pwm-select" name="side" defaultValue="both"><option value="both">Both</option><option value="server">Server</option><option value="client">Client</option></select></div>
                <div className="pwm-actions-right" style={{alignItems:'end'}}><Button type="submit" disabled={busy}>{busy?'Uploading…':'Upload JAR'}</Button></div>
            </form>
        </Card>
        <Card><ItemsPanel base={base} project={project.id} mode="custom" refreshKey={refreshKey}/></Card>
    </div>;
}
