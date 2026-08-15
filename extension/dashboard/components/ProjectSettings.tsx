import React from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';

export type Project = { id:string;slug:string;display_name:string;minecraft_version:string;loader:string;loader_version?:string;pack_version:string;current_revision?:string };
export default ({base,project,reload}:{base:string;project:Project;reload:()=>void})=>{const save=async(e:React.FormEvent<HTMLFormElement>)=>{e.preventDefault();await http.patch(`${base}/projects/${project.id}`,Object.fromEntries(new FormData(e.currentTarget)));reload()};return <form onSubmit={save} css="display:grid;gap:10px;max-width:520px"><label>Pack name<Input name="display_name" defaultValue={project.display_name} required/></label><label>Minecraft<Input name="minecraft_version" defaultValue={project.minecraft_version} required/></label><label>Loader<Input name="loader" defaultValue={project.loader} required/></label><label>Loader version<Input name="loader_version" defaultValue={project.loader_version} required/></label><label>Pack version<Input name="pack_version" defaultValue={project.pack_version} required/></label><Button type="submit">Save metadata</Button></form>};
