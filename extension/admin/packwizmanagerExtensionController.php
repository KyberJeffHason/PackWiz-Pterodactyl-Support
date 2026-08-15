<?php

namespace Pterodactyl\Http\Controllers\Admin\Extensions\packwizmanager;

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Crypt;
use Pterodactyl\Http\Controllers\Controller;
use Pterodactyl\BlueprintFramework\Libraries\ExtensionLibrary\Admin\BlueprintAdminLibrary as BlueprintExtensionLibrary;

class packwizmanagerExtensionController extends Controller
{
    public function __construct(private BlueprintExtensionLibrary $blueprint) {}
    public function index() { return view('admin.extensions.packwizmanager.index', [
        'configured' => $this->blueprint->dbGet('packwizmanager', 'service_token', '') !== '',
        'serviceUrl' => $this->blueprint->dbGet('packwizmanager', 'service_url', 'http://127.0.0.1:8090/api/v1'),
        'publicUrl' => $this->blueprint->dbGet('packwizmanager', 'public_url', ''),
        'permissionEdit' => $this->blueprint->dbGet('packwizmanager', 'permission_packwiz.edit', ''),
        'permissionUpload' => $this->blueprint->dbGet('packwizmanager', 'permission_packwiz.upload', ''),
        'permissionPublish' => $this->blueprint->dbGet('packwizmanager', 'permission_packwiz.publish', ''),
    ]); }
    public function update(Request $request)
    {
        $data = $request->validate(['service_url' => 'required|url', 'service_token' => 'nullable|string|min:32', 'public_url' => 'required|url', 'permission_edit' => ['nullable','regex:/^[0-9, ]*$/'], 'permission_upload' => ['nullable','regex:/^[0-9, ]*$/'], 'permission_publish' => ['nullable','regex:/^[0-9, ]*$/']]);
        if ($data['service_token']) { $this->blueprint->dbSet('packwizmanager', 'service_token', Crypt::encryptString($data['service_token'])); }
        abort_if($this->blueprint->dbGet('packwizmanager', 'service_token', '') === '', 422, 'Service token required on first save.');
        $this->blueprint->dbSetMany('packwizmanager', ['service_url' => $data['service_url'], 'public_url' => $data['public_url'], 'permission_packwiz.edit' => $data['permission_edit'] ?? '', 'permission_packwiz.upload' => $data['permission_upload'] ?? '', 'permission_packwiz.publish' => $data['permission_publish'] ?? '']);
        return redirect()->back()->with('success', 'Settings saved. Existing token remains hidden.');
    }
}
