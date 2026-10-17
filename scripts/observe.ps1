param(
    [ValidateSet('list', 'read', 'inspect')][string]$Mode = 'list',
    [int]$TargetId = 0,
    [string]$StartedAt = ''
)

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$culture = [Globalization.CultureInfo]::InvariantCulture

function Send-Result($value) {
    ConvertTo-Json -InputObject $value -Depth 8 -Compress
}

function Unavailable([string]$detail) {
    return @{ available = $false; percent = 0; source = ''; detail = $detail }
}

function Process-Identity($process) {
    return $process.StartTime.ToUniversalTime().Ticks.ToString($culture)
}

$session = (Get-Process -Id $PID).SessionId
$processes = @{}
foreach ($process in Get-Process) {
    try {
        if ($process.SessionId -eq $session -and $process.Id -ne $PID) {
            $null = Process-Identity $process
            $processes[$process.Id] = $process
        }
    } catch {}
}

if ($Mode -eq 'list') {
    $items = @($processes.Values | ForEach-Object {
        if ($_.ProcessName -ne 'GoToBedAlready') {
            @{
                pid = $_.Id
                name = $_.ProcessName + '.exe'
                title = $_.MainWindowTitle
                startedAt = Process-Identity $_
            }
        }
    } | Sort-Object @{Expression={ -not $_.title }}, @{Expression={ $_.name }})
    Send-Result $items
    exit 0
}

$diagnostic = @{ selectedPID = $TargetId; familyPIDs = @(); relationError = ''; windows = @(); controls = @(); sample = $null }
try {
    if (-not $processes.ContainsKey($TargetId) -or (Process-Identity $processes[$TargetId]) -ne $StartedAt) {
        throw 'The selected process exited or restarted. Select it again.'
    }
    $parents = @{}
    try {
        Get-CimInstance Win32_Process -Filter "SessionId = $session" -OperationTimeoutSec 3 | ForEach-Object { $parents[[int]$_.ProcessId] = [int]$_.ParentProcessId }
    } catch {
        $parents = @{}
        $diagnostic.relationError = $_.Exception.Message
    }
    $rootId = $TargetId
    if ($processes[$TargetId].ProcessName -eq 'SocialClubHelper') {
        $cursor = $TargetId
        $visited = @{}
        while ($parents.ContainsKey($cursor) -and -not $visited.ContainsKey($cursor)) {
            $visited[$cursor] = $true
            $parent = $parents[$cursor]
            if (-not $processes.ContainsKey($parent) -or $processes[$parent].StartTime -gt $processes[$cursor].StartTime) { break }
            if ($processes[$parent].ProcessName -eq 'Launcher') { $rootId = $parent; break }
            $cursor = $parent
        }
    }
    $family = @{$rootId = $true}
    for ($depth = 0; $depth -lt 32; $depth++) {
        $added = $false
        foreach ($child in @($parents.Keys)) {
            $parent = $parents[$child]
            if ($family.ContainsKey($parent) -and -not $family.ContainsKey($child) -and $processes.ContainsKey($child) -and $processes[$child].StartTime -ge $processes[$parent].StartTime) {
                $family[$child] = $true
                $added = $true
            }
        }
        if (-not $added) { break }
    }
    $diagnostic.familyPIDs = @($family.Keys | Sort-Object)
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    Add-Type -Path (Join-Path $PSScriptRoot 'native-progress.cs')
    $scope = [Windows.Automation.TreeScope]
    $elementType = [Windows.Automation.AutomationElement]
    $controlType = [Windows.Automation.ControlType]
    $rangeType = [Windows.Automation.RangeValuePattern]
    $valueType = [Windows.Automation.ValuePattern]
    $candidates = @{}
    $controls = New-Object 'System.Collections.Generic.List[object]'
    $windows = @([ProgressWindows]::All())
    $relatedWindows = @($windows | Where-Object { $family.ContainsKey($_.PID) })
    $diagnostic.windows = @($relatedWindows | ForEach-Object { @{ pid = $_.PID; title = $_.Title; handle = $_.Handle.ToInt64().ToString() } })
    $percentExpression = '(?<![-\d.,])(?:100|[1-9]?\d)(?:[.,]\d+)?\s*%'

    function Parse-Percent([string]$text) {
        $found = [regex]::Matches($text, $percentExpression)
        if ($found.Count -eq 1) {
            return [double]::Parse(($found[0].Value -replace '[\s%]', '').Replace(',', '.'), $culture)
        }
        return $null
    }

    function Read-Progress($element, [string]$prefix, [bool]$allowName) {
        $pattern = $null
        $value = $null
        $kind = 'range'
        if ($element.TryGetCurrentPattern($rangeType::Pattern, [ref]$pattern)) {
            $range = $pattern.Current
            if ($range.Maximum -gt $range.Minimum -and $range.Value -ge $range.Minimum -and $range.Value -le $range.Maximum) {
                $value = 100 * ($range.Value - $range.Minimum) / ($range.Maximum - $range.Minimum)
            }
        } elseif ($element.Current.ClassName -like '*msctls_progress32*' -and $element.Current.NativeWindowHandle -ne 0) {
            $kind = 'native'
            $value = [ProgressWindows]::Value([IntPtr]$element.Current.NativeWindowHandle)
        } elseif ($element.TryGetCurrentPattern($valueType::Pattern, [ref]$pattern)) {
            $kind = 'value'
            $value = Parse-Percent $pattern.Current.Value
        }
        if ($null -eq $value -and $allowName) {
            $kind = 'name'
            $value = Parse-Percent $element.Current.Name
        }
        if ($null -ne $value -and $value -ge 0 -and $value -le 100) {
            $identity = $prefix + ':' + $kind + ':' + ($element.GetRuntimeId() -join '.')
            $candidates[$identity] = @{ available = $true; percent = $value; source = $identity; detail = '' }
        }
    }

    foreach ($window in $relatedWindows) {
        $root = $elementType::FromHandle($window.Handle)
        $elements = $root.FindAll($scope::Descendants, [Windows.Automation.Condition]::TrueCondition)
        foreach ($element in $elements) {
            if ($Mode -eq 'inspect' -and $controls.Count -lt 150) {
                $controls.Add(@{ pid = $window.PID; name = $element.Current.Name; class = $element.Current.ClassName; type = $element.Current.ControlType.ProgrammaticName; patterns = @($element.GetSupportedPatterns() | ForEach-Object { $_.ProgrammaticName }) })
            }
            if ($element.Current.ControlType -eq $controlType::ProgressBar -or $element.Current.ClassName -like '*msctls_progress32*') {
                Read-Progress $element ('window:' + $window.PID + ':' + (Process-Identity $processes[$window.PID])) $false
            }
        }
    }
    if ($candidates.Count -eq 0) {
        $taskbarCondition = New-Object Windows.Automation.OrCondition(
            (New-Object Windows.Automation.PropertyCondition($elementType::ClassNameProperty, 'Shell_TrayWnd')),
            (New-Object Windows.Automation.PropertyCondition($elementType::ClassNameProperty, 'Shell_SecondaryTrayWnd'))
        )
        $buttonCondition = New-Object Windows.Automation.PropertyCondition($elementType::ControlTypeProperty, $controlType::Button)
        foreach ($window in $relatedWindows) {
            $title = ($window.Title -replace $percentExpression, '').Trim(' ', '-', ':', '|')
            $matching = @($windows | Where-Object { (($_.Title -replace $percentExpression, '').Trim(' ', '-', ':', '|')) -eq $title })
            if (-not $title -or $matching.Count -ne 1) { continue }
            foreach ($taskbar in $elementType::RootElement.FindAll($scope::Children, $taskbarCondition)) {
                foreach ($button in $taskbar.FindAll($scope::Descendants, $buttonCondition)) {
                    $name = ($button.Current.Name -replace $percentExpression, '').Trim(' ', '-', ':', '|')
                    if ($name -eq $title) { Read-Progress $button ('taskbar:' + $window.PID + ':' + (Process-Identity $processes[$window.PID])) $true }
                }
            }
        }
    }
    $diagnostic.controls = @($controls.ToArray())
    if ($candidates.Count -eq 1) {
        $diagnostic.sample = @($candidates.Values)[0]
    } elseif ($candidates.Count -gt 1) {
        $diagnostic.sample = Unavailable 'MULTIPLE_PROGRESS'
    } elseif ($relatedWindows.Count -eq 0) {
        if ($diagnostic.relationError) { $diagnostic.sample = Unavailable 'PROCESS_TREE_UNAVAILABLE' } else { $diagnostic.sample = Unavailable 'NO_RELATED_WINDOW' }
    } else {
        $diagnostic.sample = Unavailable 'NO_ACCESSIBLE_PROGRESS'
    }
} catch {
    $diagnostic.sample = Unavailable ('UI Automation: ' + $_.Exception.Message)
}
if ($Mode -eq 'inspect') { Send-Result $diagnostic } else { Send-Result $diagnostic.sample }
