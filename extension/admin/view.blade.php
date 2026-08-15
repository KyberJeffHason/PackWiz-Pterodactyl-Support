@extends('layouts.admin')
@section('title') Packwiz Manager @endsection
@section('content')
<div class="row"><div class="col-xs-12"><div class="box box-primary"><div class="box-header"><h3 class="box-title">Packwiz Manager</h3></div>
<form method="POST"><div class="box-body">{{ csrf_field() }} {{ method_field('PATCH') }}
<div class="form-group"><label for="service_url">Private service URL</label><input id="service_url" name="service_url" class="form-control" value="{{ $serviceUrl }}" required></div>
<div class="form-group"><label for="service_token">Service token</label><input id="service_token" name="service_token" type="password" class="form-control" placeholder="{{ $configured ? 'Configured — leave blank to preserve' : 'Required, 32+ characters' }}"></div>
<div class="form-group"><label for="public_url">Public pack URL</label><input id="public_url" name="public_url" class="form-control" value="{{ $publicUrl }}" placeholder="https://pack.example.com/public" required></div>
<div class="form-group"><label for="permission_edit">Subuser IDs allowed to edit (comma-separated)</label><input id="permission_edit" name="permission_edit" class="form-control" value="{{ $permissionEdit }}"></div>
<div class="form-group"><label for="permission_upload">Subuser IDs allowed to upload</label><input id="permission_upload" name="permission_upload" class="form-control" value="{{ $permissionUpload }}"></div>
<div class="form-group"><label for="permission_publish">Subuser IDs allowed to publish/rollback</label><input id="permission_publish" name="permission_publish" class="form-control" value="{{ $permissionPublish }}"></div>
<p>CurseForge key is configured in service environment and is never returned to browser.</p></div><div class="box-footer"><button class="btn btn-primary">Save</button></div></form></div></div></div>
@endsection
