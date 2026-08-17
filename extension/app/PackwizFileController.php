<?php

namespace Pterodactyl\BlueprintFramework\Extensions\packwizmanager;

use Illuminate\Http\Client\PendingRequest;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Crypt;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Str;
use Pterodactyl\BlueprintFramework\Libraries\ExtensionLibrary\Admin\BlueprintAdminLibrary as BlueprintExtensionLibrary;
use Pterodactyl\Http\Controllers\Controller;
use Pterodactyl\Models\Server;
use Symfony\Component\HttpFoundation\Response;

class PackwizFileController extends Controller
{
    public function __construct(private BlueprintExtensionLibrary $blueprint) {}

    private function authorizeServer(Request $request, Server $server): void
    {
        $user = $request->user();
        abort_unless($user->root_admin || $server->owner_id === $user->id || $user->servers()->whereKey($server->id)->exists(), 403);
    }

    private function service(Request $request): PendingRequest
    {
        $encrypted = (string) $this->blueprint->dbGet('packwizmanager', 'service_token', '');
        $token = $encrypted === '' ? '' : Crypt::decryptString($encrypted);
        abort_if(strlen($token) < 32, 503, 'Packwiz Manager service is not configured.');

        return Http::baseUrl((string) $this->blueprint->dbGet('packwizmanager', 'service_url', 'http://127.0.0.1:8090/api/v1'))
            ->withToken($token)
            ->timeout(120)
            ->withHeaders([
                'X-Packwiz-Permissions' => 'packwiz.read',
                'X-Pterodactyl-Actor' => (string) $request->user()->id,
                'X-Request-ID' => (string) ($request->header('X-Request-ID') ?: Str::uuid()),
            ]);
    }

    public function download(Request $request, Server $server, string $project): Response
    {
        $this->authorizeServer($request, $server);
        $request->validate(['path' => 'required|string|max:4096']);

        $upstream = $this->service($request)->get("/projects/{$project}/items", [
            'view' => 'download',
            'path' => (string) $request->query('path'),
        ]);

        $response = response($upstream->body(), $upstream->status());
        foreach (['Content-Type', 'Content-Disposition', 'Content-Length', 'Last-Modified', 'X-Content-Type-Options'] as $header) {
            $value = $upstream->header($header);
            if ($value !== null && $value !== '') {
                $response->header($header, $value);
            }
        }

        return $response;
    }
}
