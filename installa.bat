@echo off
setlocal enabledelayedexpansion

echo ========================================
echo   PrendiTempo - Installazione
echo ========================================
echo.

:: Directory di installazione
set "INSTALL_DIR=%LOCALAPPDATA%\PrendiTempo"
set "SRC_DIR=%~dp0"

:: Verifica se gia' installato
if exist "%INSTALL_DIR%\PrendiTempo.exe" (
    echo Rilevata installazione precedente.
    echo.
    choice /c SN /m "Vuoi aggiornare l'installazione esistente"
    if errorlevel 2 goto :end

    :: Chiudi app se in esecuzione
    taskkill /f /im PrendiTempo.exe >nul 2>&1
    timeout /t 2 /nobreak >nul
)

:: Crea cartella di installazione
echo [1/3] Creazione cartella installazione...
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%INSTALL_DIR%\web" mkdir "%INSTALL_DIR%\web"

:: Copia file
echo [2/3] Copia file...
copy /y "%SRC_DIR%PrendiTempo.exe" "%INSTALL_DIR%\" >nul
xcopy /s /e /y "%SRC_DIR%web\*" "%INSTALL_DIR%\web\" >nul
if exist "%SRC_DIR%icon.ico" copy /y "%SRC_DIR%icon.ico" "%INSTALL_DIR%\" >nul
if exist "%SRC_DIR%icon_tracking.ico" copy /y "%SRC_DIR%icon_tracking.ico" "%INSTALL_DIR%\" >nul

:: Crea collegamento desktop
echo [3/3] Creazione collegamento sul desktop...
set "SHORTCUT=%USERPROFILE%\Desktop\PrendiTempo.lnk"
set "VBS_SCRIPT=%TEMP%\create_shortcut.vbs"

echo Set oWS = WScript.CreateObject("WScript.Shell") > "%VBS_SCRIPT%"
echo sLinkFile = "%SHORTCUT%" >> "%VBS_SCRIPT%"
echo Set oLink = oWS.CreateShortcut(sLinkFile) >> "%VBS_SCRIPT%"
echo oLink.TargetPath = "%INSTALL_DIR%\PrendiTempo.exe" >> "%VBS_SCRIPT%"
echo oLink.WorkingDirectory = "%INSTALL_DIR%" >> "%VBS_SCRIPT%"
echo oLink.IconLocation = "%INSTALL_DIR%\icon.ico" >> "%VBS_SCRIPT%"
echo oLink.Description = "PrendiTempo - Time Tracker" >> "%VBS_SCRIPT%"
echo oLink.Save >> "%VBS_SCRIPT%"

cscript //nologo "%VBS_SCRIPT%"
del "%VBS_SCRIPT%"

echo.
echo ========================================
echo   Installazione completata!
echo ========================================
echo.
echo Installato in: %INSTALL_DIR%
echo Collegamento creato sul Desktop
echo.

:: Chiedi se avviare l'app
choice /c SN /m "Vuoi avviare PrendiTempo ora"
if errorlevel 2 goto :end

start "" "%INSTALL_DIR%\PrendiTempo.exe"

:end
echo.
echo Grazie per aver installato PrendiTempo!
echo.
pause
