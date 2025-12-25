@echo off
setlocal enabledelayedexpansion

echo ========================================
echo   PrendiTempo - Build Release Package
echo ========================================
echo.

:: Directory corrente
set "SRC_DIR=%~dp0"
set "DIST_DIR=%SRC_DIR%dist\PrendiTempo"

:: Pulisci e ricrea cartella dist
echo [1/5] Pulizia cartella distribuzione...
if exist "%SRC_DIR%dist" rmdir /s /q "%SRC_DIR%dist"
mkdir "%DIST_DIR%"
mkdir "%DIST_DIR%\web"

:: Compila versione release
echo [2/5] Compilazione versione release...
cd /d "%SRC_DIR%"
go build -ldflags="-H windowsgui -s -w" -o "%DIST_DIR%\PrendiTempo.exe"
if errorlevel 1 (
    echo ERRORE: Compilazione fallita!
    pause
    exit /b 1
)

:: Copia file necessari
echo [3/5] Copia file...
xcopy /s /e /y "%SRC_DIR%web\*" "%DIST_DIR%\web\" >nul
copy /y "%SRC_DIR%icon.ico" "%DIST_DIR%\" >nul
copy /y "%SRC_DIR%icon_tracking.ico" "%DIST_DIR%\" >nul 2>nul

:: Crea installer e disinstaller
echo [4/5] Creazione installer...
copy /y "%SRC_DIR%installa.bat" "%DIST_DIR%\" >nul
copy /y "%SRC_DIR%disinstalla.bat" "%DIST_DIR%\" >nul

:: Crea ZIP (se 7zip disponibile)
echo [5/5] Creazione archivio ZIP...
where 7z >nul 2>&1
if %errorlevel% equ 0 (
    cd /d "%SRC_DIR%dist"
    7z a -tzip "PrendiTempo_Setup.zip" "PrendiTempo" >nul
    echo    Creato: dist\PrendiTempo_Setup.zip
) else (
    echo    7-Zip non trovato, archivio ZIP non creato.
    echo    Puoi comprimere manualmente la cartella dist\PrendiTempo
)

echo.
echo ========================================
echo   Build completata!
echo ========================================
echo.
echo Contenuto cartella dist\PrendiTempo:
dir /b "%DIST_DIR%"
echo.
echo Per installare su un altro PC:
echo 1. Copia la cartella "dist\PrendiTempo" sul PC
echo 2. Esegui "installa.bat" come amministratore
echo.
pause
