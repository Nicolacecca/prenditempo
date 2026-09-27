//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

// ShowIdleNotification mostra una notifica toast quando viene rilevato l'idle
func (a *App) ShowIdleNotification(minutes int) error {
	// Formatta il messaggio
	var durationText string
	if minutes >= 60 {
		hours := minutes / 60
		mins := minutes % 60
		if mins > 0 {
			durationText = fmt.Sprintf("%d ore e %d minuti", hours, mins)
		} else {
			durationText = fmt.Sprintf("%d ore", hours)
		}
	} else {
		durationText = fmt.Sprintf("%d minuti", minutes)
	}

	title := "PrendiTempo - Inattivita rilevata"
	message := fmt.Sprintf("Sei stato inattivo per %s. Apri l'app per attribuire il tempo.", durationText)

	err := showWindowsBalloon(title, message)
	if err != nil {
		log.Printf("[NOTIFICATION] Errore invio notifica: %v", err)
		return err
	}

	log.Printf("[NOTIFICATION] Notifica inviata per %d minuti di idle", minutes)
	return nil
}

// ShowSimpleNotification mostra una notifica semplice
func (a *App) ShowSimpleNotification(title, message string) error {
	return showWindowsBalloon(title, message)
}

// showWindowsBalloon usa PowerShell per mostrare un balloon notification
func showWindowsBalloon(title, message string) error {
	// Crea script PowerShell temporaneo
	psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$balloon = New-Object System.Windows.Forms.NotifyIcon
$balloon.Icon = [System.Drawing.SystemIcons]::Information
$balloon.BalloonTipTitle = '%s'
$balloon.BalloonTipText = '%s'
$balloon.BalloonTipIcon = 'Info'
$balloon.Visible = $true
$balloon.ShowBalloonTip(10000)
Start-Sleep -Milliseconds 500
$balloon.Dispose()
`, escapeForPS(title), escapeForPS(message))

	// Usa un nome univoco: istanze o notifiche concorrenti non si sovrascrivono.
	scriptFile, err := os.CreateTemp("", "prenditempo_notification_*.ps1")
	if err != nil {
		return fmt.Errorf("errore creazione script: %v", err)
	}
	scriptPath := scriptFile.Name()
	defer os.Remove(scriptPath)
	if _, err := scriptFile.WriteString(psScript); err != nil {
		scriptFile.Close()
		return fmt.Errorf("errore scrittura script: %v", err)
	}
	if err := scriptFile.Close(); err != nil {
		return fmt.Errorf("errore chiusura script: %v", err)
	}

	// Esegui lo script
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[NOTIFICATION] Output: %s", string(output))
		return fmt.Errorf("errore PowerShell: %v", err)
	}

	return nil
}

// escapeForPS escape caratteri speciali per PowerShell string literals
func escapeForPS(s string) string {
	// Escape single quotes raddoppiandole
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "''"
		} else {
			result += string(c)
		}
	}
	return result
}
