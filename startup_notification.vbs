Set objShell = CreateObject("WScript.Shell")
scriptPath = CreateObject("Scripting.FileSystemObject").GetParentFolderName(WScript.ScriptFullName)
psScript = scriptPath & "\startup_notification.ps1"
objShell.Run "powershell.exe -WindowStyle Hidden -ExecutionPolicy Bypass -File """ & psScript & """", 0, False
