# Script per mostrare notifica all'avvio di Windows
# Questo script viene eseguito automaticamente all'accensione del PC

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

# Percorso dell'eseguibile - usa percorso assoluto
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$global:EXE_PATH = Join-Path $scriptDir "PrendiTempo.exe"
$global:DASHBOARD_URL = "http://localhost:8080/index_v2.html"

# Crea una finestra form personalizzata
$form = New-Object System.Windows.Forms.Form
$form.Text = "PrendiTempo"
$form.Size = New-Object System.Drawing.Size(400, 180)
$form.StartPosition = "CenterScreen"
$form.TopMost = $true
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$form.BackColor = [System.Drawing.Color]::FromArgb(26, 26, 26)

# Icona
$iconLabel = New-Object System.Windows.Forms.Label
$iconLabel.Text = [char]0x23F1
$iconLabel.Font = New-Object System.Drawing.Font("Segoe UI Emoji", 32, [System.Drawing.FontStyle]::Regular)
$iconLabel.ForeColor = [System.Drawing.Color]::FromArgb(255, 107, 43)
$iconLabel.AutoSize = $true
$iconLabel.Location = New-Object System.Drawing.Point(20, 20)
$form.Controls.Add($iconLabel)

# Titolo
$titleLabel = New-Object System.Windows.Forms.Label
$titleLabel.Text = "PrendiTempo"
$titleLabel.Font = New-Object System.Drawing.Font("Segoe UI", 14, [System.Drawing.FontStyle]::Bold)
$titleLabel.ForeColor = [System.Drawing.Color]::White
$titleLabel.AutoSize = $true
$titleLabel.Location = New-Object System.Drawing.Point(90, 25)
$form.Controls.Add($titleLabel)

# Messaggio
$messageLabel = New-Object System.Windows.Forms.Label
$messageLabel.Text = "Ricordati di avviare il tracking del tempo!"
$messageLabel.Font = New-Object System.Drawing.Font("Segoe UI", 10)
$messageLabel.ForeColor = [System.Drawing.Color]::FromArgb(204, 204, 204)
$messageLabel.AutoSize = $true
$messageLabel.Location = New-Object System.Drawing.Point(90, 55)
$form.Controls.Add($messageLabel)

# Pulsante "Avvia PrendiTempo"
$btnOpen = New-Object System.Windows.Forms.Button
$btnOpen.Text = "Avvia PrendiTempo"
$btnOpen.Size = New-Object System.Drawing.Size(150, 35)
$btnOpen.Location = New-Object System.Drawing.Point(30, 100)
$btnOpen.BackColor = [System.Drawing.Color]::FromArgb(255, 107, 43)
$btnOpen.ForeColor = [System.Drawing.Color]::White
$btnOpen.FlatStyle = 'Flat'
$btnOpen.FlatAppearance.BorderSize = 0
$btnOpen.Font = New-Object System.Drawing.Font("Segoe UI", 10, [System.Drawing.FontStyle]::Bold)
$btnOpen.Cursor = [System.Windows.Forms.Cursors]::Hand
$btnOpen.Tag = @{ExePath = $global:EXE_PATH; DashboardUrl = $global:DASHBOARD_URL}
$btnOpen.Add_Click({
    $data = $this.Tag
    # Avvia l'applicazione
    Start-Process -FilePath $data.ExePath
    # Attendi che il server sia pronto (max 10 secondi)
    $maxRetries = 20
    $retryCount = 0
    $serverReady = $false
    while (-not $serverReady -and $retryCount -lt $maxRetries) {
        Start-Sleep -Milliseconds 500
        try {
            $tcp = New-Object System.Net.Sockets.TcpClient
            $tcp.Connect("localhost", 8080)
            $tcp.Close()
            $serverReady = $true
        } catch {
            $retryCount++
        }
    }
    # Apri la dashboard nel browser
    Start-Process $data.DashboardUrl
    $this.FindForm().Close()
})
$form.Controls.Add($btnOpen)

# Pulsante "Chiudi"
$btnClose = New-Object System.Windows.Forms.Button
$btnClose.Text = "Chiudi"
$btnClose.Size = New-Object System.Drawing.Size(150, 35)
$btnClose.Location = New-Object System.Drawing.Point(200, 100)
$btnClose.BackColor = [System.Drawing.Color]::FromArgb(107, 114, 128)
$btnClose.ForeColor = [System.Drawing.Color]::White
$btnClose.FlatStyle = 'Flat'
$btnClose.FlatAppearance.BorderSize = 0
$btnClose.Font = New-Object System.Drawing.Font("Segoe UI", 10)
$btnClose.Cursor = [System.Windows.Forms.Cursors]::Hand
$btnClose.Add_Click({
    $this.FindForm().Close()
})
$form.Controls.Add($btnClose)

# Mostra la finestra
$form.ShowDialog() | Out-Null
