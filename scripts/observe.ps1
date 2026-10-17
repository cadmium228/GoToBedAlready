param(
    [ValidateSet('list', 'read')][string]$Mode = 'list',
    [int]$TargetId = 0,
    [string]$StartedAt = ''
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$culture = [Globalization.CultureInfo]::InvariantCulture

function Send-Result($value) {
    ConvertTo-Json -InputObject $value -Depth 5 -Compress
}

function Send-Unavailable([string]$detail) {
    Send-Result @{ available = $false; percent = 0; source = ''; detail = $detail }
}

if ($Mode -eq 'list') {
    $items = @(Get-Process | ForEach-Object {
        try {
            if ($_.MainWindowHandle -ne 0 -and $_.MainWindowTitle) {
                @{
                    pid = $_.Id
                    name = $_.ProcessName + '.exe'
                    title = $_.MainWindowTitle
                    startedAt = $_.StartTime.ToUniversalTime().Ticks.ToString($culture)
                }
            }
        } catch {}
    } | Sort-Object { $_.name })
    Send-Result $items
    exit 0
}

try {
    $process = Get-Process -Id $TargetId
    if ($process.StartTime.ToUniversalTime().Ticks.ToString($culture) -ne $StartedAt) {
        Send-Unavailable 'Process restarted; select it again'
        exit 0
    }
    if ($process.MainWindowHandle -eq 0) {
        Send-Unavailable 'Process has no accessible window'
        exit 0
    }
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    $scope = [Windows.Automation.TreeScope]
    $elementType = [Windows.Automation.AutomationElement]
    $controlType = [Windows.Automation.ControlType]
    $rangeType = [Windows.Automation.RangeValuePattern]
    $root = $elementType::FromHandle($process.MainWindowHandle)
    $candidates = New-Object 'System.Collections.Generic.List[object]'

    function Read-Progress($element, [string]$prefix, [bool]$allowName) {
        $pattern = $null
        $value = $null
        if (-not $element.Current.IsEnabled) { return }
        if ($element.TryGetCurrentPattern($rangeType::Pattern, [ref]$pattern)) {
            $range = $pattern.Current
            if ($range.Maximum -gt $range.Minimum -and $range.Value -ge $range.Minimum -and $range.Value -le $range.Maximum) {
                $value = 100 * ($range.Value - $range.Minimum) / ($range.Maximum - $range.Minimum)
            }
        } elseif ($allowName -and $element.Current.Name -match '(?<![\d.,])(100|[1-9]?\d)([.,]\d+)?\s*%') {
            $value = [double]::Parse(($Matches[1] + $Matches[2]).Replace(',', '.'), $culture)
        }
        if ($null -ne $value -and $value -ge 0 -and $value -le 100) {
            $identity = ($element.GetRuntimeId() -join '.')
            $candidates.Add(@{ available = $true; percent = $value; source = $prefix + ':' + $identity; detail = '' })
        }
    }

    $progressCondition = New-Object Windows.Automation.PropertyCondition($elementType::ControlTypeProperty, $controlType::ProgressBar)
    $bars = $root.FindAll($scope::Descendants, $progressCondition)
    foreach ($bar in $bars) {
        Read-Progress $bar 'window' $false
    }
    if ($candidates.Count -eq 1) {
        Send-Result $candidates[0]
        exit 0
    }
    if ($candidates.Count -gt 1) {
        Send-Unavailable 'Multiple progress bars; cannot safely choose one'
        exit 0
    }

    $taskbarCondition = New-Object Windows.Automation.OrCondition(
        (New-Object Windows.Automation.PropertyCondition($elementType::ClassNameProperty, 'Shell_TrayWnd')),
        (New-Object Windows.Automation.PropertyCondition($elementType::ClassNameProperty, 'Shell_SecondaryTrayWnd'))
    )
    $taskbars = $elementType::RootElement.FindAll($scope::Children, $taskbarCondition)
    $buttonCondition = New-Object Windows.Automation.PropertyCondition($elementType::ControlTypeProperty, $controlType::Button)
    $percentExpression = '(?<![\d.,])(?:100|[1-9]?\d)(?:[.,]\d+)?\s*%'
    $title = ($process.MainWindowTitle -replace $percentExpression, '').Trim(' ', '-', ':', '|')
    $matchingTitles = @(Get-Process | Where-Object {
        $_.MainWindowHandle -ne 0 -and (($_.MainWindowTitle -replace $percentExpression, '').Trim(' ', '-', ':', '|')) -eq $title
    })
    if ($title.Length -gt 0 -and $matchingTitles.Count -eq 1) {
        foreach ($taskbar in $taskbars) {
            foreach ($button in $taskbar.FindAll($scope::Descendants, $buttonCondition)) {
                $name = ($button.Current.Name -replace $percentExpression, '').Trim(' ', '-', ':', '|')
                if ($name -eq $title) {
                    Read-Progress $button 'taskbar' $true
                }
            }
        }
    }
    if ($candidates.Count -eq 1) {
        Send-Result $candidates[0]
    } else {
        Send-Unavailable 'No unambiguous UI Automation progress. Keep the download window open; this launcher may be unsupported'
    }
} catch {
    Send-Unavailable 'Window closed, access denied, or UI Automation provider unavailable'
}
