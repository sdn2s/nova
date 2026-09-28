## Install

1. Download `nova-<version>.zip` below.
2. Open PowerShell as administrator and add the folder to Windows Defender exclusions, otherwise WinDivert may be quarantined:
   ```powershell
   Add-MpPreference -ExclusionPath "C:\nova"
   ```
3. Unpack and pick the best strategy:
   ```powershell
   Expand-Archive "$env:USERPROFILE\Downloads\nova-<version>.zip" -DestinationPath C:\
   cd C:\nova
   Get-ChildItem -Recurse | Unblock-File
   .\nova.exe probe --install
   ```

If something does not work, run `.\nova.exe diag`.
