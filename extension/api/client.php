<?php

use Illuminate\Support\Facades\Route;
use {appcontext}\PackwizProxyController;

Route::get('/servers/{server}/projects', [PackwizProxyController::class, 'projects']);
Route::post('/servers/{server}/projects', [PackwizProxyController::class, 'create']);
Route::post('/servers/{server}/projects/{project}/publish', [PackwizProxyController::class, 'publish']);
Route::get('/servers/{server}/projects/{project}/revisions', [PackwizProxyController::class, 'revisions']);
Route::get('/servers/{server}/projects/{project}/revisions/diff', [PackwizProxyController::class, 'diff']);
Route::post('/servers/{server}/projects/{project}/rollback/{revision}', [PackwizProxyController::class, 'rollback']);
Route::post('/servers/{server}/projects/{project}/custom-jars', [PackwizProxyController::class, 'upload']);
Route::post('/servers/{server}/projects/{project}/files', [PackwizProxyController::class, 'uploadFile']);
Route::post('/servers/{server}/projects/{project}/mods', [PackwizProxyController::class, 'addMod']);
Route::get('/servers/{server}/providers/{provider}/search', [PackwizProxyController::class, 'search']);
