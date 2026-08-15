import React, { useEffect, useState } from 'react';
import http from '@/api/http';
import { ServerContext } from '@/state/server';
import Spinner from '@/components/elements/Spinner';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';
import ItemsPanel from './components/ItemsPanel';
import ImportProject from './components/ImportProject';
import ProjectSettings, { Project } from './components/ProjectSettings';
import Integration from './components/Integration';

type ProviderHit = { project_id?: string; id?: number; title?: string; name?: string; author?: string; icon_url?: string };
const sections = ['Overview', 'Mods', 'Files', 'Custom uploads', 'Releases', 'Server integration', 'Settings'] as const;

export default () => {
    const server = ServerContext.useStoreState(s => s.server.data!);
    const [projects, setProjects] = useState<Project[]>([]), [active, setActive] = useState<typeof sections[number]>('Overview');
    const [busy, setBusy] = useState(true), [error, setError] = useState(''), [results, setResults] = useState<ProviderHit[]>([]);
    const [releases, setReleases] = useState<any[]>([]);
    const base = `/api/client/extensions/packwizmanager/servers/${server.uuid}`;
    const load = () => http.get(`${base}/projects`).then(r => setProjects(r.data || [])).catch(e => setError(e.response?.data?.error || e.message)).finally(() => setBusy(false));
    useEffect(() => { load(); }, [server.uuid]);
    useEffect(() => { if (active === 'Releases' && projects[0]) http.get(`${base}/projects/${projects[0].id}/revisions`).then(r => setReleases(r.data || [])).catch(e => setError(e.message)); }, [active, projects[0]?.id]);
    const project = projects[0];
    const create = async (e: React.FormEvent<HTMLFormElement>) => { e.preventDefault(); setBusy(true); const f = new FormData(e.currentTarget); try { await http.post(`${base}/projects`, Object.fromEntries(f)); await load(); } catch (e: any) { setError(e.response?.data?.error || e.message); setBusy(false); } };
    const search = async (e: React.FormEvent<HTMLFormElement>) => { e.preventDefault(); const f = new FormData(e.currentTarget); try { const r = await http.get(`${base}/providers/${f.get('provider')}/search`, { params: { q: f.get('q'), minecraft: project.minecraft_version, loader: project.loader } }); setResults(r.data || []); } catch (e: any) { setError(e.response?.data?.error || e.message); } };
    const addMod = async (hit: ProviderHit) => { const provider = hit.project_id ? 'modrinth' : 'curseforge'; await http.post(`${base}/projects/${project.id}/mods`, { provider, project_id: String(hit.project_id || hit.id), version_id: '', display_name: hit.title || hit.name, side: 'both' }); alert('Mod added. Packwiz selected compatible version and dependencies.'); };
    const upload = async (e: React.FormEvent<HTMLFormElement>) => { e.preventDefault(); setBusy(true); try { await http.post(`${base}/projects/${project.id}/custom-jars`, new FormData(e.currentTarget)); alert('Custom JAR stored. SHA-256 shown in project item data. Publish only trusted code.'); } catch (e: any) { setError(e.response?.data?.error || e.message); } finally { setBusy(false); } };
    const uploadFile = async (e: React.FormEvent<HTMLFormElement>) => { e.preventDefault(); setBusy(true); try { await http.post(`${base}/projects/${project.id}/files`, new FormData(e.currentTarget)); alert('Managed file saved and Packwiz index refreshed.'); } catch (e: any) { setError(e.response?.data?.error || e.message); } finally { setBusy(false); } };
    const importUrl = async (e: React.FormEvent<HTMLFormElement>) => { e.preventDefault(); const data=Object.fromEntries(new FormData(e.currentTarget)); try { await http.post(`${base}/projects/${project.id}/url-imports`,data); alert('URL fetched, validated, and stored.'); } catch(e:any){setError(e.response?.data?.error||e.message)} };
    const publish = async () => { setBusy(true); try { await http.post(`${base}/projects/${project.id}/publish`, { changelog: 'Published from Pterodactyl' }); await load(); } catch (e: any) { setError(e.response?.data?.error || e.message); setBusy(false); } };
    const rollback = async (revision: number) => { if (!confirm(`Switch current release to revision ${revision}?`)) return; await http.post(`${base}/projects/${project.id}/rollback/${revision}`); await load(); };
    if (busy && !projects.length) return <Spinner centered />;
    return <div css="max-width:1100px;margin:auto"><h1 css="font-size:2rem;font-weight:600;margin-bottom:1rem">Packwiz</h1>
        {error && <div role="alert" css="background:#7f1d1d;padding:12px;border-radius:6px;margin-bottom:12px">{error}</div>}
        {!project ? <><form onSubmit={create} css="display:grid;gap:12px;max-width:520px"><h2>Create project</h2><label>Pack name<Input name="display_name" required /></label><label>Slug<Input name="slug" pattern="[a-z0-9][a-z0-9-]{1,62}[a-z0-9]" required /></label><label>Minecraft version<Input name="minecraft_version" defaultValue="1.21.1" required /></label><label>Loader<select name="loader" defaultValue="neoforge"><option>neoforge</option><option>fabric</option><option>forge</option><option>quilt</option></select></label><label>Loader version<Input name="loader_version" required /></label><label>Pack version<Input name="pack_version" defaultValue="0.1.0" required /></label><Button type="submit">Create Packwiz project</Button></form><ImportProject base={base} reload={load}/></> : <>
            <nav aria-label="Packwiz sections" css="display:flex;gap:8px;flex-wrap:wrap;margin-bottom:18px">{sections.map(s => <Button key={s} isSecondary={active !== s} onClick={() => setActive(s)}>{s}</Button>)}</nav>
            {active === 'Overview' && <section><h2>{project.display_name}</h2><p>Minecraft {project.minecraft_version} · {project.loader} · pack {project.pack_version}</p><p>Published revision: {project.current_revision || 'None'}</p></section>}
            {active === 'Mods' && <section><form onSubmit={search} css="display:flex;gap:8px"><label>Provider<select name="provider"><option value="modrinth">Modrinth</option><option value="curseforge">CurseForge</option></select></label><label>Search<Input name="q" required /></label><Button type="submit">Search compatible mods</Button></form><ul>{results.map((r, i) => <li key={r.project_id || r.id || i}>{r.title || r.name} {r.author && `— ${r.author}`} <Button onClick={() => addMod(r)}>Add compatible version</Button></li>)}</ul><ItemsPanel base={base} project={project.id}/></section>}
            {active === 'Files' && <section><form onSubmit={uploadFile} css="display:grid;gap:10px;max-width:520px"><label>File<Input type="file" name="file" required /></label><label>Target path<Input name="target_path" placeholder="config/example.toml" required /></label><Button type="submit">Upload managed file</Button></form><form onSubmit={importUrl} css="display:grid;gap:10px;max-width:520px"><h3>Import URL</h3><Input name="url" type="url" placeholder="https://example.com/config.toml" required/><Input name="display_name" placeholder="Display name" required/><Input name="target_path" placeholder="config/example.toml" required/><select name="kind"><option>config</option><option>kubejs</option><option>datapack</option><option>resourcepack</option><option>file</option></select><select name="side"><option>both</option><option>server</option><option>client</option></select><Button type="submit">Import URL</Button></form><ItemsPanel base={base} project={project.id}/></section>}
            {active === 'Custom uploads' && <section><div role="alert" css="background:#78350f;padding:12px;border-radius:6px;margin-bottom:12px"><strong>Warning:</strong> custom JARs execute Minecraft code. Upload only trusted files.</div><form onSubmit={upload} css="display:grid;gap:10px;max-width:520px"><label>JAR file<Input type="file" name="file" accept=".jar,application/java-archive" required /></label><label>Display name<Input name="display_name" required /></label><label>Destination<Input name="destination" defaultValue="mods/custom.jar" pattern="mods/[A-Za-z0-9._-]+\.jar" required /></label><label>Side<select name="side" defaultValue="both"><option>both</option><option>server</option><option>client</option></select></label><Button type="submit">Upload custom JAR</Button></form></section>}
            {active === 'Releases' && <section><p>Publishing validates and snapshots immutable pack files.</p><Button onClick={publish} disabled={busy}>Publish revision</Button><ul>{releases.map(r => <li key={r.id}><strong>Revision {r.revision}</strong> · {r.created_at} · actor {r.actor || 'unknown'} · <code>{String(r.content_digest).slice(0, 12)}</code> — {r.changelog || 'No changelog'} <Button isSecondary onClick={() => rollback(r.revision)}>Rollback</Button></li>)}</ul></section>}
            {active === 'Server integration' && <Integration base={base} project={project.id}/>}
            {active === 'Settings' && <ProjectSettings base={base} project={project} reload={load}/>}
        </>}
    </div>;
};
