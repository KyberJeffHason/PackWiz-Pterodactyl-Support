import React, { useState } from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import { Callout, Card, Icon, Pill, SectionHeading, errorMessage } from './ui';
import type { Project } from './ProjectSettingsV2';

export default function ClientLauncher({ base, project, onOpenReleases }: { base: string; project: Project; onOpenReleases?: () => void }) {
    const [exporting, setExporting] = useState(false);
    const [error, setError] = useState('');
    const exportClient = async () => {
        if (!project.current_revision) return;
        setExporting(true); setError('');
        try {
            const response = await http.get(`${base}/projects/${project.id}/client-export`, { responseType: 'blob' });
            const blob = response.data instanceof Blob ? response.data : new Blob([response.data], { type: 'application/zip' });
            const url = URL.createObjectURL(blob); const anchor = document.createElement('a');
            anchor.href = url; anchor.download = `${project.slug}-client.zip`; document.body.appendChild(anchor); anchor.click(); anchor.remove();
            window.setTimeout(() => URL.revokeObjectURL(url), 1000);
        } catch (reason: any) {
            let message = errorMessage(reason); const data = reason?.response?.data;
            if (data instanceof Blob) { try { const parsed = JSON.parse(await data.text()); message = parsed?.message || parsed?.error || message; } catch {} }
            setError(message);
        } finally { setExporting(false); }
    };
    return <div className="pwm-grid" style={{ gap: 14 }}><Card>
        <SectionHeading icon="package" title="Client launcher" description="Build the small launcher ZIP players import once. The instance then follows the current published Packwiz revision." actions={<Pill tone={project.current_revision ? 'good' : 'warn'}>{project.current_revision ? 'Ready to export' : 'Publish required'}</Pill>}/>
        {error && <div style={{ marginBottom: 12 }}><Callout tone="error" icon="warning" title="Client export failed">{error}</Callout></div>}
        {project.current_revision ? <><div className="pwm-client-summary"><div><span>Pack</span><strong>{project.display_name}</strong></div><div><span>Minecraft</span><strong>{project.minecraft_version}</strong></div><div><span>Loader</span><strong>{project.loader} {project.loader_version}</strong></div><div><span>Revision</span><strong>{project.current_revision.slice(0, 12)}…</strong></div></div>
        <div className="pwm-client-note"><Icon name="refresh" size={17}/><div><strong>Import once</strong><p>The generated archive is compatible with Prism Launcher, Freesm and other Prism/MultiMC-style launchers. Before each launch, the included Packwiz bootstrap checks the hosted pack and applies the current client-side files.</p></div></div>
        <div className="pwm-actions-right" style={{ marginTop: 16 }}><Button type="button" onClick={exportClient} disabled={exporting}><span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}><Icon name="package" size={14}/>{exporting ? 'Building ZIP…' : 'Download client ZIP'}</span></Button></div></> : <div className="pwm-client-empty"><Icon name="warning" size={20}/><div><strong>Publish the pack before distributing a client.</strong><p>The launcher ZIP points to the stable hosted pack URL, so it needs at least one published revision.</p></div>{onOpenReleases && <Button type="button" isSecondary onClick={onOpenReleases}>Open Releases</Button>}</div>}
    </Card><Card muted><SectionHeading icon="info" title="What players receive" description="The ZIP contains launcher metadata and the pinned Packwiz bootstrap, not a duplicate copy of the entire modpack."/><div className="pwm-kv"><div className="pwm-kv-key">Distribution</div><div className="pwm-kv-value">Hosted Packwiz project</div><div className="pwm-kv-key">Updates</div><div className="pwm-kv-value">Applied before game launch</div><div className="pwm-kv-key">Client content</div><div className="pwm-kv-value">Client and both-side mods, configs and resources</div><div className="pwm-kv-key">Server content</div><div className="pwm-kv-value">Not downloaded by the client instance</div></div></Card></div>;
}
