<?php

namespace {appcontext};

use Illuminate\Http\Client\PendingRequest;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Crypt;
use Illuminate\Support\Str;
use Pterodactyl\BlueprintFramework\Libraries\ExtensionLibrary\Admin\BlueprintAdminLibrary as BlueprintExtensionLibrary;
use Pterodactyl\Http\Controllers\Controller;
use Pterodactyl\Models\Server;
use Symfony\Component\HttpFoundation\Response;

class PackwizProxyController extends Controller
{
    public function __construct(private BlueprintExtensionLibrary $blueprint) {}

    private function authorizeServer(Request $request, string $identifier): Server
    {
        $server = Server::query()->where('uuidShort', $identifier)->orWhere('uuid', $identifier)->firstOrFail();
        $user = $request->user();
        abort_unless($user->root_admin || $server->owner_id === $user->id || $user->servers()->whereKey($server->id)->exists(), 403);
        $request->attributes->set('packwiz_owner', $user->root_admin || $server->owner_id === $user->id);
        return $server;
    }

    private function service(Request $request, string $permission): PendingRequest
    {
        if ($permission !== 'packwiz.read' && !$request->attributes->get('packwiz_owner')) {
            $ids = array_filter(array_map('trim', explode(',', (string) $this->blueprint->dbGet('packwizmanager', 'permission_'.$permission, ''))));
            abort_unless(in_array((string) $request->user()->id, $ids, true), 403, 'Packwiz permission denied.');
        }
        $encrypted = (string) $this->blueprint->dbGet('packwizmanager', 'service_token', '');
        $token = $encrypted === '' ? '' : Crypt::decryptString($encrypted);
        abort_if(strlen($token) < 32, 503, 'Packwiz Manager service is not configured.');
        return Http::baseUrl((string) $this->blueprint->dbGet('packwizmanager', 'service_url', 'http://127.0.0.1:8090/api/v1'))
            ->withToken($token)->acceptJson()->timeout(120)->withHeaders([
                'X-Packwiz-Permissions' => $permission,
                'X-Pterodactyl-Actor' => (string) $request->user()->id,
                'X-Request-ID' => (string) ($request->header('X-Request-ID') ?: Str::uuid()),
            ]);
    }

    private function relay($response): Response
    {
        return response($response->body(), $response->status())->header('Content-Type', 'application/json');
    }

    public function projects(Request $request, string $server): Response { $this->authorizeServer($request, $server); return $this->relay($this->service($request, 'packwiz.read')->get('/projects')); }
    public function create(Request $request, string $server): Response { $this->authorizeServer($request, $server); return $this->relay($this->service($request, 'packwiz.edit')->post('/projects', $request->json()->all())); }
    public function importProject(Request $request, string $server): Response { $this->authorizeServer($request,$server);$request->validate(['archive'=>'required|file','slug'=>'required|string','display_name'=>'required|string','minecraft_version'=>'required|string','loader'=>'required|string','loader_version'=>'required|string','pack_version'=>'required|string']);$file=$request->file('archive');return $this->relay($this->service($request,'packwiz.edit')->attach('archive',fopen($file->getRealPath(),'rb'),$file->getClientOriginalName())->post('/projects/import',$request->only(['slug','display_name','minecraft_version','loader','loader_version','pack_version']))); }
    public function publish(Request $request, string $server, string $project): Response { $this->authorizeServer($request, $server); return $this->relay($this->service($request, 'packwiz.publish')->post("/projects/{$project}/publish", $request->json()->all())); }
    public function revisions(Request $request, string $server, string $project): Response { $this->authorizeServer($request, $server); return $this->relay($this->service($request, 'packwiz.read')->get("/projects/{$project}/revisions")); }
    public function diff(Request $request, string $server, string $project): Response { $this->authorizeServer($request, $server); return $this->relay($this->service($request, 'packwiz.read')->get("/projects/{$project}/revisions/diff", $request->query())); }
    public function addMod(Request $request, string $server, string $project): Response { $this->authorizeServer($request, $server); $request->validate(['provider' => 'required|in:modrinth,curseforge', 'project_id' => 'required|string', 'version_id' => 'nullable|string', 'display_name' => 'required|string|max:128', 'side' => 'required|in:client,server,both']); return $this->relay($this->service($request, 'packwiz.edit')->post("/projects/{$project}/mods", $request->only(['provider', 'project_id', 'version_id', 'display_name', 'side']))); }
    public function items(Request $request,string $server,string $project):Response{$this->authorizeServer($request,$server);return $this->relay($this->service($request,'packwiz.read')->get("/projects/{$project}/items"));}
    public function itemSide(Request $request,string $server,string $project,string $item):Response{$this->authorizeServer($request,$server);$request->validate(['side'=>'required|in:client,server,both']);return $this->relay($this->service($request,'packwiz.edit')->patch("/projects/{$project}/items/{$item}/side",$request->only('side')));}
    public function removeItem(Request $request,string $server,string $project,string $item):Response{$this->authorizeServer($request,$server);return $this->relay($this->service($request,'packwiz.edit')->delete("/projects/{$project}/items/{$item}"));}
    public function updateProject(Request $request,string $server,string $project):Response{$this->authorizeServer($request,$server);$request->validate(['display_name'=>'required|string','minecraft_version'=>'required|string','loader'=>'required|string','loader_version'=>'required|string','pack_version'=>'required|string']);return $this->relay($this->service($request,'packwiz.edit')->patch("/projects/{$project}",$request->only(['display_name','minecraft_version','loader','loader_version','pack_version'])));}
    public function rollback(Request $request, string $server, string $project, int $revision): Response { $this->authorizeServer($request, $server); return $this->relay($this->service($request, 'packwiz.publish')->post("/projects/{$project}/rollback/{$revision}")); }
    public function search(Request $request, string $server, string $provider): Response { $this->authorizeServer($request, $server); abort_unless(in_array($provider, ['modrinth', 'curseforge'], true), 404); return $this->relay($this->service($request, 'packwiz.read')->get("/providers/{$provider}/search", $request->query())); }
    public function upload(Request $request, string $server, string $project): Response
    {
        $this->authorizeServer($request, $server); $request->validate(['file' => 'required|file', 'display_name' => 'required|string|max:128', 'side' => 'required|in:client,server,both', 'destination' => ['required', 'regex:/^mods\/[A-Za-z0-9._-]+\.jar$/']]);
        $file = $request->file('file'); $pending = $this->service($request, 'packwiz.upload')->attach('file', fopen($file->getRealPath(), 'rb'), $file->getClientOriginalName());
        return $this->relay($pending->post("/projects/{$project}/custom-jars", $request->only(['display_name', 'side', 'destination'])));
    }
    public function uploadFile(Request $request, string $server, string $project): Response
    {
        $this->authorizeServer($request, $server); $request->validate(['file' => 'required|file', 'target_path' => ['required', 'regex:/^(config|defaultconfigs|kubejs|datapacks|resourcepacks)\/[A-Za-z0-9._\/-]+$/']]); $file=$request->file('file');
        return $this->relay($this->service($request, 'packwiz.upload')->attach('file', fopen($file->getRealPath(), 'rb'), $file->getClientOriginalName())->post("/projects/{$project}/files", ['target_path'=>$request->input('target_path')]));
    }
    public function importUrl(Request $request,string $server,string $project):Response{$this->authorizeServer($request,$server);$request->validate(['url'=>'required|url','target_path'=>'required|string','display_name'=>'required|string','kind'=>'required|in:file,config,kubejs,datapack,resourcepack','side'=>'required|in:client,server,both']);return $this->relay($this->service($request,'packwiz.upload')->post("/projects/{$project}/url-imports",$request->only(['url','target_path','display_name','kind','side'])));}
}
