<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Support\Facades\DB;
use Pterodactyl\BlueprintFramework\Libraries\ExtensionLibrary\Admin\BlueprintAdminLibrary as BlueprintExtensionLibrary;

return new class extends Migration {
    public function up(): void { app(BlueprintExtensionLibrary::class)->dbSetMany('packwizmanager', ['service_url' => 'http://127.0.0.1:8090/api/v1', 'public_url' => '']); }
    public function down(): void { DB::table('settings')->where('key', 'like', 'packwizmanager::%')->delete(); }
};
