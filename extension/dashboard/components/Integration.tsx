import React,{useState} from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import { Callout, Card, Icon, Pill, SectionHeading, errorMessage } from './ui';

type Plan={old_startup?:string;new_startup?:string;pack_url?:string;bootstrap_version?:string;bootstrap_sha256?:string;startup?:string};

export default({base,project}:{base:string;project:string})=>{
    const[plan,setPlan]=useState<Plan>(),[error,setError]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false);
    const action=async(kind:'preview'|'install'|'revert')=>{
        if(kind==='install'&&!confirm('The server must be fully offline. Install Packwiz startup integration now?'))return;
        if(kind==='revert'&&!confirm('Restore the saved startup command and remove Packwiz integration files?'))return;
        setBusy(true);setError('');setNotice('');
        try{
            const r=kind==='preview'?await http.get(`${base}/projects/${project}/integration/preview`):await http.post(`${base}/projects/${project}/integration/${kind}`);
            setPlan(r.data);
            setNotice(kind==='install'?'Integration installed. The next server start will update the pack before launching Minecraft.':kind==='revert'?'Integration reverted and the previous startup command was restored.':'Preview loaded. Review the startup change below before applying it.');
        }catch(e:any){setError(errorMessage(e));}
        finally{setBusy(false)}
    };
    return <div className="pwm-grid" style={{gap:14}}>
        <Card><SectionHeading icon="server" title="Server integration" description="Keep the server synchronized with the current published Packwiz release at startup." actions={<Pill tone="blue"><Icon name="shield" size={13}/> Admin-only</Pill>}/>
            <div className="pwm-progress"><span className="pwm-progress-step done"><Icon name="check" size={14}/> Publish pack</span><span className="pwm-progress-line"/><span className="pwm-progress-step done"><Icon name="check" size={14}/> Stop server</span><span className="pwm-progress-line"/><span className="pwm-progress-step"><Icon name="server" size={14}/> Install wrapper</span><span className="pwm-progress-line"/><span className="pwm-progress-step"><Icon name="refresh" size={14}/> Auto-update on boot</span></div>
            <div style={{marginTop:14}}><Callout tone="warn" icon="warning" title="The server must be offline">Installation and revert both modify the startup command and files through Wings. Wait until Pterodactyl shows the server as fully offline before applying either action.</Callout></div>
            {error&&<div style={{marginTop:12}}><Callout tone="error" icon="warning" title="Integration action failed">{error}</Callout></div>}
            {notice&&<div style={{marginTop:12}}><Callout tone="good" icon="check">{notice}</Callout></div>}
            <div className="pwm-split-actions" style={{marginTop:14}}><Button onClick={()=>action('preview')} disabled={busy}><span style={{display:'inline-flex',alignItems:'center',gap:6}}><Icon name="search" size={14}/> Preview change</span></Button><Button onClick={()=>action('install')} disabled={busy}><span style={{display:'inline-flex',alignItems:'center',gap:6}}><Icon name="link" size={14}/> Install integration</span></Button><Button isSecondary onClick={()=>action('revert')} disabled={busy}><span style={{display:'inline-flex',alignItems:'center',gap:6}}><Icon name="refresh" size={14}/> Revert</span></Button></div>
        </Card>
        {plan&&<Card muted><SectionHeading icon="code" title="Integration plan" description="These are the exact values the extension will use. The bootstrap is pinned and checksum-verified before being uploaded to Wings."/>
            <div className="pwm-kv">
                {'pack_url' in plan&&<><div className="pwm-kv-key">Published pack URL</div><div className="pwm-kv-value pwm-code">{plan.pack_url}</div></>}
                {'bootstrap_version' in plan&&<><div className="pwm-kv-key">Bootstrap</div><div className="pwm-kv-value">{plan.bootstrap_version}</div></>}
                {'bootstrap_sha256' in plan&&<><div className="pwm-kv-key">Bootstrap SHA-256</div><div className="pwm-kv-value pwm-code">{plan.bootstrap_sha256}</div></>}
                {'old_startup' in plan&&<><div className="pwm-kv-key">Current startup</div><div className="pwm-kv-value pwm-code">{plan.old_startup}</div></>}
                {'new_startup' in plan&&<><div className="pwm-kv-key">Integrated startup</div><div className="pwm-kv-value pwm-code">{plan.new_startup}</div></>}
                {'startup' in plan&&<><div className="pwm-kv-key">Restored startup</div><div className="pwm-kv-value pwm-code">{plan.startup}</div></>}
            </div>
        </Card>}
        <div className="pwm-grid pwm-grid-3"><div className="pwm-stat"><div className="pwm-stat-label"><Icon name="shield" size={14}/> Integrity</div><div className="pwm-stat-value" style={{fontSize:14}}>Pinned bootstrap</div><div className="pwm-stat-help">SHA-256 is verified before installation.</div></div><div className="pwm-stat"><div className="pwm-stat-label"><Icon name="refresh" size={14}/> Updates</div><div className="pwm-stat-value" style={{fontSize:14}}>Before startup</div><div className="pwm-stat-help">Packwiz installer runs before the original command.</div></div><div className="pwm-stat"><div className="pwm-stat-label"><Icon name="server" size={14}/> Recovery</div><div className="pwm-stat-value" style={{fontSize:14}}>Reversible</div><div className="pwm-stat-help">The previous startup command is saved for revert.</div></div></div>
    </div>;
};
