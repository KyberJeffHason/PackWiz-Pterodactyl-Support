<?php

namespace Pterodactyl\BlueprintFramework\Extensions\packwizmanager;

use Illuminate\Http\Client\PendingRequest;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Crypt;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Str;
use Pterodactyl\BlueprintFramework\Libraries\ExtensionLibrary\Admin\BlueprintAdminLibrary as BlueprintExtensionLibrary;
use Pterodactyl\Facades\Activity;
use Pterodactyl\Http\Controllers\Controller;
use Pterodactyl\Models\Server;
use Pterodactyl\Models\User;
use Pterodactyl\Repositories\Wings\DaemonFileRepository;
use Pterodactyl\Repositories\Wings\DaemonServerRepository;
use Pterodactyl\Services\Servers\StartupModificationService;
use RuntimeException;
use Symfony\Component\HttpFoundation\Response;

class PackwizIntegrationController extends Controller
{
    private const BOOTSTRAP_VERSION = 'v0.0.3';
    private const BOOTSTRAP_URL = 'https://github.com/packwiz/packwiz-installer-bootstrap/releases/download/v0.0.3/packwiz-installer-bootstrap.jar';
    private const BOOTSTRAP_SHA256 = 'a8fbb24dc604278e97f4688e82d3d91a318b98efc08d5dbfcbcbcab6443d116c';

    public function __construct(
        private BlueprintExtensionLibrary $blueprint,
        private DaemonFileRepository $files,
        private DaemonServerRepository $daemon,
        private StartupModificationService $startup,
    ) {}

    private function server(Request $request, Server $server): Server
    {
        abort_unless($request->user()->root_admin, 403, 'Administrator permission required.');
        return $server;
    }

    private function service(Request $request, string $permission): PendingRequest
    {
        $encrypted = (string) $this->blueprint->dbGet('packwizmanager', 'service_token', '');
        abort_if($encrypted === '', 503, 'Packwiz Manager is not configured.');
        return Http::baseUrl((string) $this->blueprint->dbGet('packwizmanager', 'service_url', 'http://127.0.0.1:8090/api/v1'))
            ->withToken(Crypt::decryptString($encrypted))->acceptJson()->timeout(120)->withHeaders([
                'X-Packwiz-Permissions' => $permission,
                'X-Pterodactyl-Actor' => (string) $request->user()->id,
                'X-Request-ID' => (string) Str::uuid(),
            ]);
    }

    private function plan(Request $request, Server $server, string $project): array
    {
        $response = $this->service($request, 'packwiz.read')->get("/projects/{$project}");
        abort_unless($response->successful(), $response->status(), $response->body());
        $pack = $response->json();
        abort_if(empty($pack['current_revision']), 422, 'Publish project before integration.');
        $base = rtrim((string) $this->blueprint->dbGet('packwizmanager', 'public_url', ''), '/');
        abort_if($base === '', 503, 'Public URL is not configured.');
        return [
            'old_startup' => $server->startup,
            'new_startup' => 'bash ./packwiz-start.sh',
            'pack_url' => $base.'/'.$pack['slug'].'/pack.toml',
            'bootstrap_version' => self::BOOTSTRAP_VERSION,
            'bootstrap_sha256' => self::BOOTSTRAP_SHA256,
        ];
    }

    private function requireOffline(Server $server): void
    {
        $details = $this->daemon->setServer($server)->getDetails();
        abort_unless(($details['state'] ?? null) === 'offline', 409, 'Stop server before changing integration.');
    }

    public function preview(Request $request, Server $server, string $project): Response
    {
        $model = $this->server($request, $server);
        return response()->json($this->plan($request, $model, $project));
    }

    public function install(Request $request, Server $server, string $project): Response
    {
        $model = $this->server($request, $server);
        $plan = $this->plan($request, $model, $project);
        $key = 'startup:'.$model->uuid;
        abort_if((string) $this->blueprint->dbGet('packwizmanager', $key, '') !== '', 409, 'Integration is already installed.');
        $this->requireOffline($model);
        $repo = $this->files->setServer($model);

        // Wings intentionally does not follow remote-download redirects. GitHub
        // release assets redirect to their asset CDN, so download the pinned
        // bootstrap through the Panel, verify it, then upload the bytes to Wings.
        $bootstrap = Http::timeout(60)
            ->withOptions(['allow_redirects' => true])
            ->get(self::BOOTSTRAP_URL);

        abort_unless($bootstrap->successful(), 502, 'Failed to download Packwiz bootstrap.');

        $jar = $bootstrap->body();
        abort_if(strlen($jar) > 200000, 502, 'Packwiz bootstrap exceeded expected size.');
        abort_unless(
            hash_equals(self::BOOTSTRAP_SHA256, hash('sha256', $jar)),
            502,
            'Bootstrap checksum mismatch.'
        );

        $repo->putContent('/packwiz-installer-bootstrap.jar', $jar);
        $old = str_replace(["\r", "\n"], ' ', $model->startup);
        $url = str_replace("'", "'\"'\"'", $plan['pack_url']);
        $script = "#!/usr/bin/env bash\nset -Eeuo pipefail\njava -jar packwiz-installer-bootstrap.jar -g -s server '{$url}'\nexec {$old}\n";
        $repo->putContent('/packwiz-start.sh', $script);
        $repo->chmodFiles('/', [['file' => 'packwiz-start.sh', 'mode' => '755']]);
        $this->blueprint->dbSet('packwizmanager', $key, $model->startup);
        $applied = false;
        try {
            $this->startup->setUserLevel(User::USER_LEVEL_ADMIN)->handle($model, ['startup' => $plan['new_startup']]);
            $applied = true;
            $state = $this->service($request, 'packwiz.integration')->put("/projects/{$project}/server-links/{$model->uuid}", [
                'server_identifier' => $model->uuidShort,
                'update_on_start' => true,
                'bootstrap_state' => 'installed',
                'bootstrap_version' => self::BOOTSTRAP_VERSION,
                'startup_integration_state' => 'applied',
                'last_sync_status' => 'ready',
            ]);
            if (!$state->successful()) {
                throw new RuntimeException('Manager rejected integration state.');
            }
        } catch (\Throwable $error) {
            if ($applied) {
                $this->startup->setUserLevel(User::USER_LEVEL_ADMIN)->handle($model, ['startup' => $plan['old_startup']]);
            }
            $repo->deleteFiles('/', ['packwiz-start.sh', 'packwiz-installer-bootstrap.jar']);
            $this->blueprint->dbForget('packwizmanager', $key);
            throw $error;
        }
        Activity::event('server:packwiz.install')->subject($model)->property('project', $project)->log();
        return response()->json($plan);
    }

    public function revert(Request $request, Server $server, string $project): Response
    {
        $model = $this->server($request, $server);
        $key = 'startup:'.$model->uuid;
        $old = (string) $this->blueprint->dbGet('packwizmanager', $key, '');
        abort_if($old === '', 409, 'No saved startup command.');
        $this->requireOffline($model);
        $this->startup->setUserLevel(User::USER_LEVEL_ADMIN)->handle($model, ['startup' => $old]);
        $this->files->setServer($model)->deleteFiles('/', ['packwiz-start.sh', 'packwiz-installer-bootstrap.jar']);
        $this->blueprint->dbForget('packwizmanager', $key);
        $state = $this->service($request, 'packwiz.integration')->put("/projects/{$project}/server-links/{$model->uuid}", [
            'server_identifier' => $model->uuidShort,
            'update_on_start' => false,
            'bootstrap_state' => 'absent',
            'bootstrap_version' => self::BOOTSTRAP_VERSION,
            'startup_integration_state' => 'reverted',
            'last_sync_status' => 'reverted',
        ]);
        abort_unless($state->successful(), 502, 'Integration reverted, but manager state update failed.');
        Activity::event('server:packwiz.revert')->subject($model)->property('project', $project)->log();
        return response()->json(['startup' => $old]);
    }
}
