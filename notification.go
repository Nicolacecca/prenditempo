//go:build !wails

package main

import (
	"log"
	"os/exec"
	"runtime"
)

// ShowWindowsNotification mostra una notifica toast di Windows
func ShowWindowsNotification(title, message string) error {
	log.Printf("[NOTIFICA] Tentativo di mostrare notifica: %s - %s", title, message)

	if runtime.GOOS != "windows" {
		log.Println("Notifiche supportate solo su Windows")
		return nil
	}

	// Usa PowerShell per mostrare una notifica toast
	script := `
		[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
		[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
		[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null

		$APP_ID = 'PrendiTempo'

		$template = @"
<toast scenario="reminder" duration="long">
	<visual>
		<binding template="ToastGeneric">
			<text>` + title + `</text>
			<text>` + message + `</text>
		</binding>
	</visual>
	<audio src="ms-winsoundevent:Notification.Reminder" loop="false"/>
</toast>
"@

		try {
			$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
			$xml.LoadXml($template)
			$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
			$toast.Priority = [Windows.UI.Notifications.ToastNotificationPriority]::High
			[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($APP_ID).Show($toast)
			Write-Host "Notifica mostrata con successo"
		} catch {
			Write-Error "Errore nella notifica: $_"
			exit 1
		}
	`

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[NOTIFICA] Errore notifica Windows: %v", err)
		log.Printf("[NOTIFICA] Output PowerShell: %s", string(output))
		return err
	}

	log.Printf("[NOTIFICA] Notifica mostrata con successo")
	return nil
}
