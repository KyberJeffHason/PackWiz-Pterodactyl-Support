@extends('layouts.admin')
@section('title') Packwiz Manager @endsection
@section('content')
<style>
.pwm-admin .box{border-radius:5px;overflow:hidden}.pwm-admin .box-header{padding:16px 18px}.pwm-admin .box-body{padding:18px}.pwm-admin .lead-copy{color:#6f7c86;margin:4px 0 0;line-height:1.55}.pwm-admin .status-row{display:flex;gap:12px;flex-wrap:wrap;margin-bottom:18px}.pwm-admin .status-card{flex:1;min-width:210px;background:#fff;border:1px solid #e4e9ed;border-radius:5px;padding:15px 16px;display:flex;gap:12px;align-items:flex-start}.pwm-admin .status-icon{width:38px;height:38px;border-radius:5px;display:flex;align-items:center;justify-content:center;background:#edf7f5;color:#168c78;font-size:18px;flex:none}.pwm-admin .status-icon.blue{background:#eef5fb;color:#3c8dbc}.pwm-admin .status-icon.gold{background:#fff8e6;color:#c58c18}.pwm-admin .status-title{font-size:12px;text-transform:uppercase;letter-spacing:.04em;color:#7b8a95;font-weight:700}.pwm-admin .status-value{font-size:15px;color:#263238;font-weight:600;margin-top:3px;word-break:break-word}.pwm-admin .section-title{font-size:15px;font-weight:600;color:#263238;margin:0 0 4px}.pwm-admin .section-copy{color:#7b8a95;font-size:12px;margin-bottom:14px}.pwm-admin .form-group label{font-size:12px;color:#37474f}.pwm-admin .help-block{font-size:11px;line-height:1.5}.pwm-admin .permission-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px}.pwm-admin .architecture{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-top:10px}.pwm-admin .architecture span{background:#f4f7f9;border:1px solid #dce3e8;border-radius:4px;padding:6px 9px;font-size:11px;color:#52616b}.pwm-admin .architecture i{color:#9aa7af}.pwm-admin .box-footer{padding:14px 18px;display:flex;align-items:center;justify-content:space-between;gap:12px}.pwm-admin .muted-note{font-size:11px;color:#87949d}@media(max-width:800px){.pwm-admin .permission-grid{grid-template-columns:1fr}.pwm-admin .box-footer{align-items:flex-start;flex-direction:column}}
</style>
<div class="pwm-admin">
    @if(session('success'))
        <div class="alert alert-success"><i class="fa fa-check-circle"></i> {{ session('success') }}</div>
    @endif

    <div class="box box-primary">
        <div class="box-header with-border">
            <h3 class="box-title"><i class="fa fa-cubes"></i> Packwiz Manager</h3>
            <p class="lead-copy">Configure the private manager connection, public pack distribution URL, and delegated Packwiz permissions used by the Pterodactyl extension.</p>
        </div>
        <div class="box-body" style="background:#f7f9fb">
            <div class="status-row">
                <div class="status-card">
                    <div class="status-icon"><i class="fa {{ $configured ? 'fa-check' : 'fa-exclamation' }}"></i></div>
                    <div><div class="status-title">Manager connection</div><div class="status-value">{{ $configured ? 'Service token configured' : 'Setup required' }}</div></div>
                </div>
                <div class="status-card">
                    <div class="status-icon blue"><i class="fa fa-exchange"></i></div>
                    <div><div class="status-title">Private API</div><div class="status-value">Panel proxy only</div></div>
                </div>
                <div class="status-card">
                    <div class="status-icon gold"><i class="fa fa-globe"></i></div>
                    <div><div class="status-title">Published packs</div><div class="status-value">{{ $publicUrl ?: 'Public URL not set' }}</div></div>
                </div>
            </div>
        </div>

        <form method="POST">
            {{ csrf_field() }} {{ method_field('PATCH') }}
            <div class="box-body">
                <h4 class="section-title"><i class="fa fa-plug text-aqua"></i> Service connection</h4>
                <p class="section-copy">The browser never talks to Packwiz Manager directly. Pterodactyl authenticates the user, checks permissions, and proxies requests to this private URL.</p>
                <div class="row">
                    <div class="col-md-7">
                        <div class="form-group">
                            <label for="service_url">Private service URL</label>
                            <input id="service_url" name="service_url" type="url" class="form-control" value="{{ $serviceUrl }}" placeholder="http://host.docker.internal:8090/api/v1" required>
                            <p class="help-block">For a Dockerized Panel with the manager on the host, use a host-reachable address such as <code>http://host.docker.internal:8090/api/v1</code>. Do not expose the management listener publicly.</p>
                        </div>
                    </div>
                    <div class="col-md-5">
                        <div class="form-group">
                            <label for="service_token">Service token</label>
                            <input id="service_token" name="service_token" type="password" class="form-control" autocomplete="new-password" placeholder="{{ $configured ? 'Configured — leave blank to preserve' : 'Required, 32+ characters' }}">
                            <p class="help-block">Stored encrypted by the Panel. Existing tokens are intentionally never sent back to the browser.</p>
                        </div>
                    </div>
                </div>

                <hr>
                <h4 class="section-title"><i class="fa fa-cloud-download text-blue"></i> Public distribution</h4>
                <p class="section-copy">This base URL is embedded into server integration plans and points clients or servers to immutable published Packwiz releases.</p>
                <div class="form-group">
                    <label for="public_url">Public pack base URL</label>
                    <input id="public_url" name="public_url" type="url" class="form-control" value="{{ $publicUrl }}" placeholder="https://packs.example.com/public" required>
                    <p class="help-block">Use the externally reachable <code>/public</code> endpoint, usually provided through your reverse proxy or Cloudflare Tunnel.</p>
                </div>
                <div class="architecture" aria-label="Request architecture">
                    <span><i class="fa fa-user"></i> Browser</span><i class="fa fa-long-arrow-right"></i><span><i class="fa fa-shield"></i> Pterodactyl permission proxy</span><i class="fa fa-long-arrow-right"></i><span><i class="fa fa-cog"></i> Private Manager API</span>
                </div>

                <hr>
                <h4 class="section-title"><i class="fa fa-users text-green"></i> Delegated permissions</h4>
                <p class="section-copy">Server owners and root administrators keep their normal access. These optional user-ID lists delegate higher-risk Packwiz actions to additional users.</p>
                <div class="permission-grid">
                    <div class="form-group">
                        <label for="permission_edit"><i class="fa fa-pencil"></i> Edit pack</label>
                        <input id="permission_edit" name="permission_edit" class="form-control" value="{{ $permissionEdit }}" placeholder="12, 24, 31">
                        <p class="help-block">Add/remove provider mods, change sides, and edit project metadata.</p>
                    </div>
                    <div class="form-group">
                        <label for="permission_upload"><i class="fa fa-upload"></i> Upload content</label>
                        <input id="permission_upload" name="permission_upload" class="form-control" value="{{ $permissionUpload }}" placeholder="12, 24, 31">
                        <p class="help-block">Upload custom JARs or managed files and import approved remote URLs.</p>
                    </div>
                    <div class="form-group">
                        <label for="permission_publish"><i class="fa fa-history"></i> Publish / rollback</label>
                        <input id="permission_publish" name="permission_publish" class="form-control" value="{{ $permissionPublish }}" placeholder="12, 24, 31">
                        <p class="help-block">Create immutable releases, publish changes, and move the current revision backward.</p>
                    </div>
                </div>

                <div class="callout callout-info" style="margin:4px 0 0">
                    <h4><i class="fa fa-key"></i> Provider credentials stay service-side</h4>
                    <p style="margin:0">The CurseForge API key is configured in the Packwiz Manager service environment and is never returned through the extension or browser.</p>
                </div>
            </div>
            <div class="box-footer">
                <span class="muted-note"><i class="fa fa-lock"></i> Sensitive settings are stored by Blueprint's extension database helpers.</span>
                <button class="btn btn-primary" type="submit"><i class="fa fa-save"></i> Save Packwiz settings</button>
            </div>
        </form>
    </div>
</div>
@endsection
