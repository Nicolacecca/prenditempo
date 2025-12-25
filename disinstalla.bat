@echo off
setlocal

echo ========================================
echo   PrendiTempo - Disinstallazione
echo ========================================
echo.

set "INSTALL_DIR=%LOCALAPPDATA%\PrendiTempo"

:: Verifica se installato
if not exist "%INSTALL_DIR%" (
    echo PrendiTempo non risulta installato.
    pause
    exit /b 0
)

echo Questa operazione rimuovera' PrendiTempo dal sistema.
echo Il database con i tuoi dati verra' MANTENUTO.
echo.
choice /c SN /m "Vuoi procedere con la disinstallazione"
if errorlevel 2 goto :end

:: Chiudi app se in esecuzione
echo.
echo Chiusura applicazione...
taskkill /f /im PrendiTempo.exe >nul 2>&1
timeout /t 2 /nobreak >nul

:: Rimuovi collegamento desktop
echo Rimozione collegamento desktop...
del "%USERPROFILE%\Desktop\PrendiTempo.lnk" >nul 2>&1

:: Rimuovi dall'avvio automatico
echo Rimozione dall'avvio automatico...
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\PrendiTempo.lnk" >nul 2>&1

:: Rimuovi file applicazione (mantieni database)
echo Rimozione file applicazione...
del "%INSTALL_DIR%\PrendiTempo.exe" >nul 2>&1
del "%INSTALL_DIR%\icon.ico" >nul 2>&1
del "%INSTALL_DIR%\icon_tracking.ico" >nul 2>&1
rmdir /s /q "%INSTALL_DIR%\web" >nul 2>&1

echo.
echo ========================================
echo   Disinstallazione completata!
echo ========================================
echo.
echo Il database (timetracker.db) e' stato mantenuto in:
echo %INSTALL_DIR%
echo.
echo Se vuoi eliminare anche i dati, elimina manualmente la cartella.
echo.

:end
pause
