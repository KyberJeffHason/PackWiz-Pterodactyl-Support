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

class PackwizClientExportController extends Controller
{
    private const BOOTSTRAP_URL = 'https://github.com/packwiz/packwiz-installer-bootstrap/releases/download/v0.0.3/packwiz-installer-bootstrap.jar';
    private const BOOTSTRAP_SHA256 = 'a8fbb24dc604278e97f4688e82d3d91a318b98efc08d5dbfcbcbcab6443d116c';
    private const BOOTSTRAP_MAX_BYTES = 200000;

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
            ->acceptJson()
            ->timeout(120)
            ->withHeaders([
                'X-Packwiz-Permissions' => 'packwiz.read',
                'X-Pterodactyl-Actor' => (string) $request->user()->id,
                'X-Request-ID' => (string) ($request->header('X-Request-ID') ?: Str::uuid()),
            ]);
    }

    private function loaderUid(string $loader): string
    {
        return match (strtolower($loader)) {
            'fabric' => 'net.fabricmc.fabric-loader',
            'forge' => 'net.minecraftforge',
            'neoforge' => 'net.neoforged',
            'quilt' => 'org.quiltmc.quilt-loader',
            default => abort(422, 'Unsupported launcher loader.'),
        };
    }

    private function bootstrap(): string
    {
        $response = Http::timeout(60)
            ->withOptions(['allow_redirects' => true])
            ->get(self::BOOTSTRAP_URL);

        abort_unless($response->successful(), 502, 'Failed to download Packwiz client bootstrap.');
        $jar = $response->body();
        abort_if(strlen($jar) > self::BOOTSTRAP_MAX_BYTES, 502, 'Packwiz client bootstrap exceeded expected size.');
        abort_unless(hash_equals(self::BOOTSTRAP_SHA256, hash('sha256', $jar)), 502, 'Packwiz client bootstrap checksum mismatch.');

        return $jar;
    }

    private function buildZip(array $files): string
    {
        $body = '';
        $directory = '';
        $offset = 0;
        $count = 0;

        foreach ($files as $name => $data) {
            $name = str_replace('\\', '/', (string) $name);
            $data = (string) $data;
            $crc = hexdec(hash('crc32b', $data));
            $size = strlen($data);

            $local = pack('VvvvvvVVVvv', 0x04034b50, 20, 0, 0, 0, 0, $crc, $size, $size, strlen($name), 0)
                .$name
                .$data;

            $directory .= pack('VvvvvvvVVVvvvvvVV', 0x02014b50, 20, 20, 0, 0, 0, 0, $crc, $size, $size, strlen($name), 0, 0, 0, 0, 0, $offset)
                .$name;

            $body .= $local;
            $offset += strlen($local);
            ++$count;
        }

        $body .= $directory;
        $body .= pack('VvvvvVVv', 0x06054b50, 0, 0, $count, $count, strlen($directory), $offset, 0);

        return $body;
    }

    public function export(Request $request, Server $server, string $project): Response
    {
        $this->authorizeServer($request, $server);

        $upstream = $this->service($request)->get("/projects/{$project}");
        if (!$upstream->successful()) {
            return response($upstream->body(), $upstream->status())->header('Content-Type', 'application/json');
        }

        $pack = $upstream->json();
        abort_unless(is_array($pack), 502, 'Packwiz Manager returned invalid project metadata.');
        abort_if(empty($pack['current_revision']), 409, 'Publish at least one revision before exporting a client ZIP.');

        $publicBase = rtrim((string) $this->blueprint->dbGet('packwizmanager', 'public_url', ''), '/');
        abort_if($publicBase === '', 503, 'Configure the Packwiz public URL before exporting a client ZIP.');

        $slug = (string) ($pack['slug'] ?? 'packwiz-pack');
        $packUrl = $publicBase.'/'.$slug.'/pack.toml';
        $displayName = trim((string) preg_replace('/[\r\n=]+/', ' ', (string) ($pack['display_name'] ?? $slug)));
        if ($displayName === '') {
            $displayName = $slug;
        }

        $instance = "[General]\n"
            ."name={$displayName}\n"
            ."InstanceType=OneSix\n"
            ."MCLaunchMethod=LauncherPart\n"
            ."OverrideCommands=true\n"
            ."PreLaunchCommand=\"\$INST_JAVA\" -jar packwiz-installer-bootstrap.jar {$packUrl}\n";

        $manifest = json_encode([
            'formatVersion' => 1,
            'components' => [
                [
                    'uid' => 'net.minecraft',
                    'version' => (string) ($pack['minecraft_version'] ?? ''),
                    'important' => true,
                ],
                [
                    'uid' => $this->loaderUid((string) ($pack['loader'] ?? '')),
                    'version' => (string) ($pack['loader_version'] ?? ''),
                ],
            ],
        ], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES);
        abort_if($manifest === false, 500, 'Failed to build launcher manifest.');

        $readme = "Packwiz managed client\n\n"
            ."Import this ZIP into Prism Launcher or a compatible Prism/MultiMC fork.\n"
            ."The Packwiz updater runs before Minecraft launches and synchronizes the client from:\n"
            .$packUrl."\n";

        $zip = $this->buildZip([
            'instance.cfg' => $instance,
            'mmc-pack.json' => $manifest."\n",
            'README.txt' => $readme,
            'minecraft/packwiz-installer-bootstrap.jar' => $this->bootstrap(),
        ]);

        return response($zip, 200, [
            'Content-Type' => 'application/zip',
            'Content-Disposition' => 'attachment; filename="'.$slug.'-client.zip"',
            'Cache-Control' => 'no-store',
            'X-Content-Type-Options' => 'nosniff',
        ]);
    }
}
