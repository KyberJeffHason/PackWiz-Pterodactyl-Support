import React from 'react';
import http from '@/api/http';
import Button from '@/components/elements/Button';
import Input from '@/components/elements/Input';

export default ({base,reload}:{base:string;reload:()=>void})=>{const submit=async(e:React.FormEvent<HTMLFormElement>)=>{e.preventDefault();await http.post(`${base}/projects/import`,new FormData(e.currentTarget));reload()};return <details><summary>Import existing Packwiz ZIP</summary><form onSubmit={submit} css="display:grid;gap:8px;max-width:520px"><Input type="file" name="archive" accept=".zip" required/><Input name="display_name" placeholder="Pack name" required/><Input name="slug" placeholder="pack-slug" required/><Input name="minecraft_version" placeholder="1.21.1" required/><Input name="loader" placeholder="neoforge" required/><Input name="loader_version" placeholder="Loader version" required/><Input name="pack_version" placeholder="0.1.0" required/><Button type="submit">Import archive</Button></form></details>};
