param(
    [ValidateSet("install", "uninstall", "start", "stop", "restart", "status")]
    [string]$Command = "status"
)

$ErrorActionPreference = "Stop"
$rootDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$serviceName = if ($env:ROUTER_SERVICE_NAME) { $env:ROUTER_SERVICE_NAME } else { "YeyingRouter" }
$binaryPath = if ($env:ROUTER_BINARY) { $env:ROUTER_BINARY } else { Join-Path $rootDir "build\router.exe" }
$port = if ($env:ROUTER_PORT) { $env:ROUTER_PORT } else { "3011" }
$logDir = if ($env:ROUTER_LOG_DIR) { $env:ROUTER_LOG_DIR } else { Join-Path $rootDir "logs" }
function Fail([string]$Message) {
    throw $Message
}

function Require-Binary {
    if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
        Fail "Router binary not found: $binaryPath. Build with: go build -o build/router.exe ./cmd/router"
    }
}

function Get-Task {
    Get-ScheduledTask -TaskName $serviceName -ErrorAction SilentlyContinue
}

function Install-RouterTask {
    Require-Binary
    New-Item -ItemType Directory -Force -Path $logDir | Out-Null
    $action = New-ScheduledTaskAction -Execute $binaryPath -Argument "--port $port --log-dir `"$logDir`"" -WorkingDirectory $rootDir
    $trigger = New-ScheduledTaskTrigger -AtLogOn
    $settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
    $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Highest
    Register-ScheduledTask -TaskName $serviceName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null
    Start-ScheduledTask -TaskName $serviceName
    Write-Output "Installed and started $serviceName."
}

function Uninstall-RouterTask {
    Stop-ScheduledTask -TaskName $serviceName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $serviceName -Confirm:$false -ErrorAction SilentlyContinue
    Write-Output "Uninstalled $serviceName."
}

switch ($Command) {
    "install" { Install-RouterTask }
    "uninstall" { Uninstall-RouterTask }
    "start" {
        if (-not (Get-Task)) { Fail "Task is not installed. Run: .\scripts\service.ps1 install" }
        Start-ScheduledTask -TaskName $serviceName
        Write-Output "Started $serviceName."
    }
    "stop" {
        Stop-ScheduledTask -TaskName $serviceName -ErrorAction SilentlyContinue
        Write-Output "Stopped $serviceName."
    }
    "restart" {
        if (-not (Get-Task)) { Fail "Task is not installed. Run: .\scripts\service.ps1 install" }
        Restart-ScheduledTask -TaskName $serviceName
        Write-Output "Restarted $serviceName."
    }
    "status" {
        $task = Get-Task
        if (-not $task) {
            Write-Output "$serviceName is not installed."
            exit 3
        }
        Get-ScheduledTaskInfo -TaskName $serviceName | Format-List
    }
}
