import React,{useEffect,useRef,useState} from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';
import { Callout, Card, Icon, Pill, SectionHeading, errorMessage } from './ui';

type Project={id:string;slug:string;display_name:string;minecraft_version:string;loader:string;loader_version?:string;pack_version:string;current_revision?:string};
type Mode='create'|'replace';

// Pterodactyl's shared Axios instance defaults every request to application/json.
// Remove that merged default only for real FormData payloads so Axios/the browser
// can emit multipart/form-data with the required boundary. ImportProject is loaded
// by the Packwiz route itself, so this also fixes the custom-JAR and managed-file
// upload forms that use the same HTTP instance.
const packwizHttp=http as typeof http&{__packwizMultipartInterceptor?:boolean};
if(!packwizHttp.__packwizMultipartInterceptor){
    packwizHttp.interceptors.request.use(config=>{
        if(typeof FormData!=='undefined'&&config.data instanceof FormData){
            const headers=config.headers as any;
            if(headers&&typeof headers.delete==='function')headers.delete('Content-Type');
            else if(headers)delete headers['Content-Type'];
        }
        return config;
    });
    packwizHttp.__packwizMultipartInterceptor=true;
}

const cookieName=(base:string)=>{
    const id=base.match(/\/servers\/([^/]+)/)?.[1]||'server';
    return `packwiz_project_${id.replace(/-/g,'_')}`;
};

export default({base,reload,currentProject}:{base:string;reload?:()=>void;currentProject?:Project})=>{
    const[mode,setMode]=useState<Mode>(currentProject?'replace':'create');
    const[busy,setBusy]=useState(false),[error,setError]=useState(''),[success,setSuccess]=useState('');
    const[file,setFile]=useState<File>(),[dragging,setDragging]=useState(false),[confirmText,setConfirmText]=useState('');
    const[projects,setProjects]=useState<Project[]>([]);
    const input=useRef<HTMLInputElement>(null);

    const loadProjects=async()=>{try{const r=await http.get(`${base}/projects`);setProjects(r.data||[])}catch{}};
    useEffect(()=>{loadProjects()},[base]);

    const selectProject=(id:string)=>{
        document.cookie=`${cookieName(base)}=${encodeURIComponent(id)}; path=/; SameSite=Lax`;
        window.location.reload();
    };

    const choose=(candidate?:File)=>{
        if(!candidate)return;
        if(!candidate.name.toLowerCase().endsWith('.zip')){setError('Choose a .zip archive containing a Packwiz project.');return;}
        setFile(candidate);setError('');setSuccess('');
    };

    const submit=async(e:React.FormEvent<HTMLFormElement>)=>{
        e.preventDefault();
        if(!file){setError('Choose a Packwiz ZIP archive first.');return;}
        if(mode==='replace'&&currentProject&&confirmText!==currentProject.slug){setError(`Type ${currentProject.slug} exactly to confirm replacement.`);return;}
        setBusy(true);setError('');setSuccess('');
        try{
            const data=new FormData(e.currentTarget);
            data.set('archive',file,file.name);
            data.delete('confirmation');
            if(mode==='replace'&&currentProject){data.set('replace_project_id',currentProject.id);data.delete('slug');}
            else data.delete('replace_project_id');
            const r=await http.post(`${base}/projects/import`,data);
            const imported:Project=r.data;
            if(imported?.id)document.cookie=`${cookieName(base)}=${encodeURIComponent(imported.id)}; path=/; SameSite=Lax`;
            setSuccess(mode==='replace'?'Working project replaced successfully. Published revisions were left untouched.':'Packwiz project imported successfully.');
            await loadProjects();
            if(reload)reload();
            window.setTimeout(()=>window.location.reload(),350);
        }catch(err:any){setError(errorMessage(err))}finally{setBusy(false)}
    };

    const project=currentProject;
    const defaultName=project?.display_name||'';
    const defaultMinecraft=project?.minecraft_version||'1.21.1';
    const defaultPackVersion=project?.pack_version||'0.1.0';
    const defaultLoader=project?.loader||'neoforge';
    const defaultLoaderVersion=project?.loader_version||'';

    return <div className="pwm-grid" style={{gap:14}}>
        {currentProject&&projects.length>1&&<Card muted>
            <SectionHeading icon="package" title="Projects on this manager" description="Switch which Packwiz project this server workspace is currently displaying." actions={<Pill>{projects.length} projects</Pill>}/>
            <div className="pwm-field"><label className="pwm-label">Active project</label><select className="pwm-select" value={currentProject.id} onChange={e=>selectProject(e.target.value)}>{projects.map(p=><option key={p.id} value={p.id}>{p.display_name} · {p.slug}</option>)}</select><span className="pwm-help">Your selection is remembered for this Pterodactyl server.</span></div>
        </Card>}

        <Card>
            <SectionHeading icon="upload" title="Import / Replace Packwiz project" description="Import another Packwiz ZIP or safely replace the current working tree without touching already-published revisions." actions={currentProject?<Pill tone="blue">{currentProject.slug}</Pill>:undefined}/>
            {error&&<div style={{marginBottom:12}}><Callout tone="error" icon="warning" title="Import failed">{error}</Callout></div>}
            {success&&<div style={{marginBottom:12}}><Callout tone="good" icon="check">{success}</Callout></div>}

            {currentProject&&<div style={{display:'flex',gap:8,flexWrap:'wrap',marginBottom:14}}>
                <button type="button" className="pwm-icon-button" style={mode==='replace'?{borderColor:'#2dd4bf',background:'rgba(45,212,191,.08)'}:{}} onClick={()=>{setMode('replace');setError('');setSuccess('')}}><Icon name="refresh" size={14}/> Replace current project</button>
                <button type="button" className="pwm-icon-button" style={mode==='create'?{borderColor:'#60a5fa',background:'rgba(96,165,250,.08)'}:{}} onClick={()=>{setMode('create');setError('');setSuccess('')}}><Icon name="plus" size={14}/> Create another project</button>
            </div>}

            {mode==='replace'&&currentProject?<Callout tone="warn" icon="warning" title="This replaces the working tree, not your published history">The uploaded archive becomes the editable working project. Existing immutable revisions and the currently published revision remain available until you explicitly publish a new revision. The imported files and .pw.toml entries are re-indexed into the Mods/Files tables.</Callout>:<Callout icon="info" title="Create an independent managed project">The imported archive receives its own project ID and stable slug. After import it becomes the selected project; use the project switcher here to move between projects.</Callout>}

            <form onSubmit={submit} className="pwm-form-grid" style={{marginTop:14}}>
                <div className="pwm-field full">
                    <label className="pwm-label">Packwiz ZIP archive</label>
                    <div role="button" tabIndex={0} onClick={()=>input.current?.click()} onKeyDown={e=>{if(e.key==='Enter'||e.key===' ')input.current?.click()}} onDragEnter={e=>{e.preventDefault();setDragging(true)}} onDragOver={e=>e.preventDefault()} onDragLeave={e=>{e.preventDefault();setDragging(false)}} onDrop={e=>{e.preventDefault();setDragging(false);choose(e.dataTransfer.files?.[0])}} style={{border:`1px dashed ${dragging?'#2dd4bf':'#526277'}`,borderRadius:6,padding:'26px 18px',textAlign:'center',cursor:'pointer',background:dragging?'rgba(45,212,191,.08)':'rgba(15,23,42,.28)',transition:'150ms ease'}}>
                        <input ref={input} type="file" accept=".zip,application/zip" style={{display:'none'}} onChange={e=>choose(e.target.files?.[0])}/>
                        <div className="pwm-empty-icon" style={{margin:'0 auto 10px'}}><Icon name={file?'check':'upload'} size={22}/></div>
                        <div className="pwm-name">{file?file.name:'Drop a Packwiz ZIP here'}</div>
                        <div className="pwm-secondary" style={{marginTop:5}}>{file?`${(file.size/1024/1024).toFixed(1)} MiB · click to choose another archive`:'or click to browse · pack.toml may be at the root or inside one wrapping directory'}</div>
                    </div>
                </div>

                <div className="pwm-field full"><label className="pwm-label">Pack name</label><Input name="display_name" defaultValue={defaultName} placeholder="KyberCreate" required/><span className="pwm-help">Used by Packwiz Manager and written into pack.toml during replacement.</span></div>
                {mode==='create'&&<div className="pwm-field full"><label className="pwm-label">New project slug</label><Input name="slug" placeholder="kybercreate" pattern="[a-z0-9][a-z0-9-]{1,62}[a-z0-9]" required/><span className="pwm-help">Permanent identifier used in published pack URLs.</span></div>}
                <div className="pwm-field"><label className="pwm-label">Minecraft version</label><Input name="minecraft_version" defaultValue={defaultMinecraft} required/></div>
                <div className="pwm-field"><label className="pwm-label">Pack version</label><Input name="pack_version" defaultValue={defaultPackVersion} required/></div>
                <div className="pwm-field"><label className="pwm-label">Loader</label><select className="pwm-select" name="loader" defaultValue={defaultLoader}><option value="neoforge">NeoForge</option><option value="fabric">Fabric</option><option value="forge">Forge</option><option value="quilt">Quilt</option></select></div>
                <div className="pwm-field"><label className="pwm-label">Loader version</label><Input name="loader_version" defaultValue={defaultLoaderVersion} placeholder="21.1.247" required/></div>

                {mode==='replace'&&currentProject&&<div className="pwm-field full"><label className="pwm-label">Confirm replacement</label><Input name="confirmation" value={confirmText} onChange={e=>setConfirmText(e.target.value)} placeholder={`Type ${currentProject.slug}`}/><span className="pwm-help">This guard prevents an accidental replacement of the current working project.</span></div>}

                <div className="full"><Callout icon="shield" title="Validated before activation">ZIP extraction is size/path constrained, Packwiz must accept the imported tree, and database changes are committed together with the filesystem replacement. A failed replacement restores the previous working tree.</Callout></div>
                <div className="full pwm-actions-right"><Button type="submit" disabled={busy||(mode==='replace'&&!!currentProject&&confirmText!==currentProject.slug)}><span style={{display:'inline-flex',alignItems:'center',gap:6}}><Icon name={mode==='replace'?'refresh':'upload'} size={14}/>{busy?'Importing…':mode==='replace'?'Replace working project':'Import as new project'}</span></Button></div>
            </form>
        </Card>
    </div>;
};