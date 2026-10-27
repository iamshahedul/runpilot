<?php
/**
 * RunPilot Server Provisioning Module for WHMCS
 * Product: RunPilot (Native Node.js & Python Application Hosting)
 *
 * Supported Actions:
 * - TestConnection (Tests API credentials and host connectivity)
 * - CreateAccount (Provisions user, assigns package limits, creates native Linux app)
 * - SuspendAccount (Stops native application process)
 * - UnsuspendAccount (Restarts application process)
 * - TerminateAccount (Stops process and cleans up app workspace)
 * - ChangePackage (Dynamically updates cgroups v2 CPU & RAM limits)
 * - ClientArea (cPanel-style credentials view & direct 1-Click SSO)
 */

if (!defined("WHMCS")) {
    die("This file cannot be accessed directly");
}

function runpilot_MetaData()
{
    return [
        'DisplayName' => 'RunPilot Application Hosting',
        'APIVersion' => '1.2',
        'RequiresServer' => true,
    ];
}

/**
 * Configuration fields in WHMCS Product Setup -> Module Settings
 */
function runpilot_ConfigOptions()
{
    return [
        'Hosting Package' => [
            'Type' => 'dropdown',
            'Options' => 'Starter,Professional,Business,Custom',
            'Description' => 'cPanel-style Package (Starter: 512M, Pro: 1G, Business: 2G)',
            'Default' => 'Starter',
        ],
        'Runtime Type' => [
            'Type' => 'dropdown',
            'Options' => 'nodejs,python',
            'Description' => 'Default application runtime environment',
            'Default' => 'nodejs',
        ],
        'Runtime Version' => [
            'Type' => 'dropdown',
            'Options' => '20.x,22.x,18.x,3.11,3.12,3.10',
            'Description' => 'Runtime version (Node.js or Python line)',
            'Default' => '20.x',
        ],
        'Custom Memory Limit' => [
            'Type' => 'text',
            'Size' => '10',
            'Default' => '512M',
            'Description' => 'Custom RAM quota if Package is Custom (e.g. 512M, 1G, 2G)',
        ],
        'Custom CPU Quota' => [
            'Type' => 'text',
            'Size' => '10',
            'Default' => '100%',
            'Description' => 'Custom CPU quota if Package is Custom (100% = 1 core, 200% = 2 cores)',
        ],
    ];
}

/**
 * WHMCS Test Connection Button Handler (in Servers config)
 */
function runpilot_TestConnection(array $params)
{
    try {
        $res = runpilot_ApiRequest($params, 'GET', '/api/v1/servers');
        if (!empty($res['error'])) {
            return [
                'success' => false,
                'error' => $res['error'],
            ];
        }
        return [
            'success' => true,
            'error' => '',
        ];
    } catch (Exception $e) {
        return [
            'success' => false,
            'error' => $e->getMessage(),
        ];
    }
}

/**
 * Internal Helper to authenticate and get JWT token & active base URL
 */
function runpilot_GetAuthSession($params)
{
    $rawHost = !empty($params['serverip']) ? $params['serverip'] : ($params['serverhostname'] ?? '103.42.5.195');
    $rawHost = preg_replace('#^https?://#i', '', trim($rawHost));
    $rawHost = rtrim($rawHost, '/');

    $port = 0;
    if (strpos($rawHost, ':') !== false) {
        $parts = explode(':', $rawHost, 2);
        $rawHost = $parts[0];
        $port = (int)$parts[1];
    } elseif (!empty($params['serverport'])) {
        $port = (int)$params['serverport'];
    }

    $candidatePorts = [];
    if ($port > 0) {
        $candidatePorts[] = $port;
        if ($port !== 80) {
            $candidatePorts[] = 80;
        }
    } else {
        $candidatePorts = [80, 8080];
    }

    $adminUser = trim($params['serverusername'] ?? '');
    if (empty($adminUser)) {
        $adminUser = 'admin';
    }

    $adminPass = $params['serverpassword'] ?? '';
    if (empty($adminPass)) {
        return ['error' => 'Server Password is empty in WHMCS Server settings.'];
    }

    $token = null;
    $workingBaseUrl = ($candidatePorts[0] === 80) ? "http://{$rawHost}" : "http://{$rawHost}:{$candidatePorts[0]}";
    $lastError = '';

    foreach ($candidatePorts as $candidatePort) {
        $candidateBase = ($candidatePort === 80) ? "http://{$rawHost}" : "http://{$rawHost}:{$candidatePort}";
        $loginUrl = "{$candidateBase}/api/v1/auth/login";

        $loginCh = curl_init($loginUrl);
        curl_setopt($loginCh, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($loginCh, CURLOPT_POST, true);
        curl_setopt($loginCh, CURLOPT_POSTFIELDS, json_encode([
            'email' => $adminUser,
            'password' => $adminPass,
        ]));
        curl_setopt($loginCh, CURLOPT_HTTPHEADER, ['Content-Type: application/json']);
        curl_setopt($loginCh, CURLOPT_TIMEOUT, 10);
        curl_setopt($loginCh, CURLOPT_CONNECTTIMEOUT, 6);
        curl_setopt($loginCh, CURLOPT_SSL_VERIFYPEER, false);
        curl_setopt($loginCh, CURLOPT_SSL_VERIFYHOST, false);

        $loginResp = curl_exec($loginCh);
        $httpCode = curl_getinfo($loginCh, CURLINFO_HTTP_CODE);
        $curlErr = curl_error($loginCh);
        $curlErrNo = curl_errno($loginCh);
        curl_close($loginCh);

        if ($loginResp === false) {
            $lastError = "Connection failed to {$loginUrl} (cURL code {$curlErrNo}: {$curlErr}).";
            continue;
        }

        if ($httpCode === 401 || $httpCode === 403) {
            return [
                'error' => "RunPilot Login Rejected (HTTP {$httpCode}). Email: '{$adminUser}'."
            ];
        }

        if ($httpCode !== 200) {
            $lastError = "HTTP {$httpCode} from {$loginUrl}: {$loginResp}";
            continue;
        }

        $loginData = json_decode($loginResp, true);
        if (!empty($loginData['token'])) {
            return [
                'token' => $loginData['token'],
                'baseUrl' => $candidateBase,
            ];
        }
    }

    return [
        'error' => "RunPilot Connection Error: {$lastError}",
        'baseUrl' => $workingBaseUrl,
    ];
}

/**
 * Internal API Helper for RunPilot REST endpoints
 */
function runpilot_ApiRequest($params, $method, $path, $data = null)
{
    $auth = runpilot_GetAuthSession($params);
    if (!empty($auth['error'])) {
        return ['error' => $auth['error']];
    }

    $token = $auth['token'];
    $baseUrl = $auth['baseUrl'];

    $targetUrl = "{$baseUrl}{$path}";
    $ch = curl_init($targetUrl);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $method);
    curl_setopt($ch, CURLOPT_TIMEOUT, 30);
    curl_setopt($ch, CURLOPT_CONNECTTIMEOUT, 10);
    curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false);
    curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, false);

    $headers = [
        "Authorization: Bearer {$token}",
        "Content-Type: application/json",
    ];

    if ($data !== null) {
        curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
    }

    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    $response = curl_exec($ch);
    $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $curlErr = curl_error($ch);
    curl_close($ch);

    if ($response === false) {
        return ['error' => "RunPilot API request failed to {$targetUrl}: {$curlErr}"];
    }

    $decoded = json_decode($response, true);
    if ($code >= 400) {
        $msg = $decoded['error'] ?? "HTTP error {$code}: {$response}";
        return ['error' => $msg];
    }

    return ['success' => true, 'data' => $decoded];
}

/**
 * Provision new client user and application on order activation
 */
function runpilot_CreateAccount($params)
{
    $serviceId = $params['serviceid'];
    $domain = !empty($params['domain']) ? trim($params['domain']) : "app-{$serviceId}.local";
    
    // Options
    $packageName = !empty($params['configoption1']) ? $params['configoption1'] : 'Starter';
    $runtimeType = !empty($params['configoption2']) ? $params['configoption2'] : 'nodejs';
    $runtimeVersion = !empty($params['configoption3']) ? $params['configoption3'] : '20.x';
    $ramLimit = !empty($params['configoption4']) ? $params['configoption4'] : '512M';
    $cpuQuota = !empty($params['configoption5']) ? $params['configoption5'] : '100%';

    $appId = "app-{$serviceId}";
    $appName = !empty($params['customfields']['App Name']) 
        ? $params['customfields']['App Name'] 
        : (!empty($params['domain']) ? $params['domain'] : "Service-{$serviceId}");

    // 1. Provision client user in RunPilot with username, email, and password
    $clientUsername = !empty($params['username']) ? $params['username'] : 'user' . $serviceId;
    $clientPassword = !empty($params['password']) ? $params['password'] : 'Pass' . rand(1000, 9999) . '!';
    $clientEmail    = !empty($params['clientsdetails']['email']) ? $params['clientsdetails']['email'] : $clientUsername . '@' . $domain;
    $clientFullName = trim(($params['clientsdetails']['firstname'] ?? '') . ' ' . ($params['clientsdetails']['lastname'] ?? ''));
    if (empty($clientFullName)) {
        $clientFullName = $clientUsername;
    }

    $userRes = runpilot_ApiRequest($params, 'POST', '/api/v1/users', [
        'username'  => $clientUsername,
        'email'     => $clientEmail,
        'password'  => $clientPassword,
        'full_name' => $clientFullName,
        'role'      => 'user',
    ]);

    $clientId = 1;
    if (!empty($userRes['data']['id'])) {
        $clientId = (int)$userRes['data']['id'];
    }

    // 2. Generate internal reverse-proxy backend port (range 3000-9999)
    $port = 3000 + ($serviceId % 6000);

    $payload = [
        'id' => $appId,
        'name' => $appName,
        'owner_id' => $clientId,
        'package' => $packageName,
        'runtime_type' => $runtimeType,
        'runtime_version' => $runtimeVersion,
        'git_repo' => 'https://github.com/iamshahedul/runpilot.git',
        'git_branch' => 'main',
        'port' => $port,
        'start_command' => ($runtimeType === 'nodejs') ? 'node server.js' : 'python3 app.py',
        'resource_limits' => [
            'cpu_quota' => $cpuQuota,
            'memory_max' => $ramLimit,
            'tasks_max' => 128,
        ],
    ];

    $res = runpilot_ApiRequest($params, 'POST', '/api/v1/applications', $payload);
    if (!empty($res['error'])) {
        return $res['error'];
    }

    // 3. Attach domain if customer specified one
    if (!empty($params['domain']) && strpos($params['domain'], '.') !== false) {
        runpilot_ApiRequest($params, 'POST', "/api/v1/applications/{$appId}/domains", [
            'domain' => $params['domain'],
        ]);
    }

    return 'success';
}

/**
 * Suspend application process on overdue invoice
 */
function runpilot_SuspendAccount($params)
{
    $appId = "app-{$params['serviceid']}";
    $res = runpilot_ApiRequest($params, 'POST', "/api/v1/applications/{$appId}/stop");
    if (!empty($res['error'])) {
        return $res['error'];
    }
    return 'success';
}

/**
 * Unsuspend application process on payment
 */
function runpilot_UnsuspendAccount($params)
{
    $appId = "app-{$params['serviceid']}";
    $res = runpilot_ApiRequest($params, 'POST', "/api/v1/applications/{$appId}/start");
    if (!empty($res['error'])) {
        return $res['error'];
    }
    return 'success';
}

/**
 * Terminate and clean up application on cancellation
 */
function runpilot_TerminateAccount($params)
{
    $appId = "app-{$params['serviceid']}";
    $res = runpilot_ApiRequest($params, 'DELETE', "/api/v1/applications/{$appId}");
    if (!empty($res['error'])) {
        return $res['error'];
    }
    return 'success';
}

/**
 * Update CPU and RAM limits when client upgrades package
 */
function runpilot_ChangePackage($params)
{
    $appId = "app-{$params['serviceid']}";
    $packageName = !empty($params['configoption1']) ? $params['configoption1'] : 'Professional';
    $ramLimit = $params['configoption4'] ?? '1G';
    $cpuQuota = $params['configoption5'] ?? '150%';

    $payload = [
        'package' => $packageName,
        'resource_limits' => [
            'cpu_quota' => $cpuQuota,
            'memory_max' => $ramLimit,
            'tasks_max' => 128,
        ],
    ];

    $res = runpilot_ApiRequest($params, 'PUT', "/api/v1/applications/{$appId}", $payload);
    if (!empty($res['error'])) {
        return $res['error'];
    }
    return 'success';
}

/**
 * Client Area Dashboard Integration & Single Sign-On (SSO) Box (cPanel Style)
 */
function runpilot_ClientArea($params)
{
    $auth = runpilot_GetAuthSession($params);
    $serverIp = !empty($params['serverip']) ? $params['serverip'] : ($params['serverhostname'] ?? '103.42.5.195');
    $serverIp = preg_replace('#^https?://#i', '', trim($serverIp));
    $defaultBase = "http://{$serverIp}";
    $panelUrl = $auth['baseUrl'] ?? $defaultBase;

    $username = !empty($params['username']) ? htmlspecialchars($params['username']) : 'softcart';
    $password = !empty($params['password']) ? htmlspecialchars($params['password']) : '';
    $domain = !empty($params['domain']) ? htmlspecialchars($params['domain']) : 'softcarto.com';
    $serviceId = (int)$params['serviceid'];
    $appId = "app-{$serviceId}";
    $clientEmail = !empty($params['clientsdetails']['email']) ? htmlspecialchars($params['clientsdetails']['email']) : '';
    $packageName = !empty($params['configoption1']) ? htmlspecialchars($params['configoption1']) : 'Starter';

    // Request specific SSO token for this client username
    $token = '';
    $ssoRes = runpilot_ApiRequest($params, 'POST', '/api/v1/auth/sso-token', [
        'identifier' => $params['username'],
    ]);
    if (!empty($ssoRes['data']['token'])) {
        $token = $ssoRes['data']['token'];
    } elseif (!empty($auth['token'])) {
        $token = $auth['token'];
    }

    // Direct Single Sign-On (SSO) URL
    $ssoUrl = !empty($token) ? "{$panelUrl}/?token=" . urlencode($token) : $panelUrl;

    $html = <<<HTML
    <div style="background: #111827; border: 1px solid #1F2937; border-radius: 12px; padding: 24px; color: #F9FAFB; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin-top: 15px; box-shadow: 0 4px 20px rgba(0,0,0,0.25);">
        <div style="display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #1F2937; padding-bottom: 16px; margin-bottom: 20px;">
            <div style="display: flex; align-items: center; gap: 12px;">
                <div style="background: rgba(74, 222, 128, 0.15); border: 1px solid rgba(74, 222, 128, 0.3); border-radius: 8px; width: 38px; height: 38px; display: flex; align-items: center; justify-content: center; font-size: 18px; color: #4ADE80;">
                    ⚡
                </div>
                <div>
                    <h3 style="margin: 0; font-size: 16px; font-weight: 600; color: #FFFFFF;">RunPilot Hosting Control Panel</h3>
                    <p style="margin: 2px 0 0; font-size: 13px; color: #9CA3AF;">Package: <strong style="color: #4ADE80;">{$packageName}</strong> • Node.js & Python Isolation</p>
                </div>
            </div>
            <span style="background: rgba(74, 222, 128, 0.15); color: #4ADE80; border: 1px solid rgba(74, 222, 128, 0.3); padding: 4px 12px; border-radius: 20px; font-size: 12px; font-weight: 600;">
                ● Active Service
            </span>
        </div>

        <!-- Credentials Grid (cPanel Style) -->
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 14px; margin-bottom: 20px;">
            <div style="background: #0B1120; border: 1px solid #1F2937; border-radius: 8px; padding: 14px 16px;">
                <div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: #9CA3AF; margin-bottom: 5px;">Control Panel URL</div>
                <a href="{$panelUrl}" target="_blank" style="color: #38BDF8; font-size: 14px; font-weight: 600; text-decoration: none; word-break: break-all;">{$panelUrl} &rarr;</a>
            </div>

            <div style="background: #0B1120; border: 1px solid #1F2937; border-radius: 8px; padding: 14px 16px;">
                <div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: #9CA3AF; margin-bottom: 5px;">Application / Domain</div>
                <div style="color: #F3F4F6; font-size: 14px; font-weight: 600;">{$appId} ({$domain})</div>
            </div>

            <div style="background: #0B1120; border: 1px solid #1F2937; border-radius: 8px; padding: 14px 16px;">
                <div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: #9CA3AF; margin-bottom: 5px;">Hosting Username</div>
                <div style="color: #4ADE80; font-size: 14px; font-family: monospace; font-weight: 700;">{$username}</div>
            </div>

            <div style="background: #0B1120; border: 1px solid #1F2937; border-radius: 8px; padding: 14px 16px;">
                <div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.05em; color: #9CA3AF; margin-bottom: 5px;">Hosting Password</div>
                <div style="display: flex; align-items: center; justify-content: space-between;">
                    <input type="password" id="rp-pass-val-{$serviceId}" value="{$password}" readonly style="background: transparent; border: none; color: #F3F4F6; font-family: monospace; font-size: 14px; width: 120px; outline: none;">
                    <div style="display: flex; gap: 4px;">
                        <button type="button" onclick="var p=document.getElementById('rp-pass-val-{$serviceId}'); p.type = p.type==='password'?'text':'password'; this.textContent = p.type==='password'?'Show':'Hide';" style="background: #1F2937; color: #D1D5DB; border: 1px solid #374151; border-radius: 4px; padding: 2px 7px; font-size: 11px; cursor: pointer;">Show</button>
                        <button type="button" onclick="navigator.clipboard.writeText('{$password}'); this.textContent='Copied!'; setTimeout(()=>this.textContent='Copy', 1500);" style="background: #1F2937; color: #D1D5DB; border: 1px solid #374151; border-radius: 4px; padding: 2px 7px; font-size: 11px; cursor: pointer;">Copy</button>
                    </div>
                </div>
            </div>
        </div>

        <!-- 1-Click Auto Login Bar -->
        <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 14px; background: rgba(74, 222, 128, 0.07); border: 1px solid rgba(74, 222, 128, 0.25); border-radius: 8px; padding: 14px 18px;">
            <div style="color: #E5E7EB; font-size: 13px;">
                ⚡ <strong style="color: #4ADE80;">One-Click Auto Login (SSO):</strong> Log directly into RunPilot as <code style="background: #1F2937; color: #4ADE80; padding: 2px 6px; border-radius: 4px;">{$username}</code> without typing your password.
            </div>
            <a href="{$ssoUrl}" target="_blank" style="background: #4ADE80; color: #064E3B; font-weight: 700; text-decoration: none; padding: 10px 22px; border-radius: 6px; font-size: 13px; display: inline-flex; align-items: center; gap: 6px; box-shadow: 0 2px 10px rgba(74, 222, 128, 0.3);">
                <span>Log In to Control Panel</span> &rarr;
            </a>
        </div>
    </div>
HTML;

    return $html;
}
