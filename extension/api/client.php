<?php

use Illuminate\Support\Facades\Route;
use Pterodactyl\BlueprintFramework\Extensions\packwizmanager\PackwizProxyController;
use Pterodactyl\BlueprintFramework\Extensions\packwizmanager\PackwizIntegrationController;
use Pterodactyl\BlueprintFramework\Extensions\packwizmanager\PackwizClientExportController;

Route::get('/servers/{server}/projects', [PackwizProxyController::class, 'projects']);
Route::post('/servers/{server}/projects', [PackwizProxyController::class, 'create']);
Route::post('/servers/{server}/projects/import', [PackwizProxyController::class, 'importProject']);
Route::get('/servers/{server}/projects/{project}/client-export', [PackwizClientExportController::class, 'export']);
Route::post('/servers/{server}/projects/{project}/publish', [PackwizProxyController::class, 'publish']);
Route::get('/servers/{server}/projects/{project}/revisions', [PackwizProxyController::class, 'revisions']);
Route::get('/servers/{server}/projects/{project}/revisions/diff', [PackwizProxyController::class, 'diff']);
Route::post('/servers/{server}/projects/{project}/rollback/{revision}', [PackwizProxyController::class, 'rollback']);
Route::post('/servers/{server}/projects/{project}/custom-jars', [PackwizProxyController::class, 'upload']);
Route::post('/servers/{server}/projects/{project}/files', [PackwizProxyController::class, 'uploadFile']);
Route::post('/servers/{server}/projects/{project}/url-imports', [PackwizProxyController::class, 'importUrl']);
Route::post('/servers/{server}/projects/{project}/mods', [PackwizProxyController::class, 'addMod']);
Route::get('/servers/{server}/projects/{project}/items', [PackwizProxyController::class, 'items']);
Route::patch('/servers/{server}/projects/{project}/items/{item}/side', [PackwizProxyController::class, 'itemSide']);
Route::delete('/servers/{server}/projects/{project}/items/{item}', [PackwizProxyController::class, 'removeItem']);
Route::patch('/servers/{server}/projects/{project}', [PackwizProxyController::class, 'updateProject']);
Route::get('/servers/{server}/providers/{provider}/search', [PackwizProxyController::class, 'search']);
Route::get('/servers/{server}/projects/{project}/integration/preview', [PackwizIntegrationController::class, 'preview']);
Route::post('/servers/{server}/projects/{project}/integration/install', [PackwizIntegrationController::class, 'install']);
Route::post('/servers/{server}/projects/{project}/integration/revert', [PackwizIntegrationController::class, 'revert']);
