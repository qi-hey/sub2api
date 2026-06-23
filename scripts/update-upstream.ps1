param(
    [string]$Version = "",
    [string]$PrivacyPatchRef = "privacyfilter-v137",
    [string]$ToolingPatchRef = "tooling",
    [string]$TargetBranch = "main"
)

$ErrorActionPreference = "Stop"

function Invoke-Git {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    & git @Args
    if ($LASTEXITCODE -ne 0) {
        throw "git $($Args -join ' ') failed"
    }
}

$dirty = (& git status --porcelain)
if ($dirty) {
    throw "Working tree is not clean. Commit or stash changes before updating."
}

Invoke-Git fetch upstream --tags --prune

if (-not $Version) {
    $Version = (& git tag --list "v*" --sort=-version:refname | Select-Object -First 1).Trim()
}
if (-not $Version) {
    throw "No upstream version tag found."
}

$privacyCommit = (& git rev-parse $PrivacyPatchRef).Trim()
$toolingCommit = (& git rev-parse $ToolingPatchRef).Trim()

Invoke-Git switch -C $TargetBranch $Version
Invoke-Git cherry-pick $privacyCommit
Invoke-Git cherry-pick $toolingCommit
Invoke-Git branch -f deploy HEAD

Write-Host "Updated $TargetBranch and deploy to $Version with privacyfilter changes."
Write-Host "Next: run tests/build, then push: git push origin $TargetBranch deploy --tags"
