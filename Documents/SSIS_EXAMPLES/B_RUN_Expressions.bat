
@echo off
setlocal ENABLEDELAYEDEXPANSION

REM ====== User-configurable settings ======
REM Path to the SSIS package (.dtsx)
set "PackagePath=.\Expressions.dtsx"

REM Value to inject via environment variable (used by the package)
set "ConfigFile=.\Config_B.config"

REM Optional: SSIS package parameter/variable name (uncomment the relevant line below)
REM For SSIS 2012+ package parameters:
REM set "PackageParameter=ConfigFile"
REM For classic SSIS variable path (legacy):
REM set "VariablePath=\Package.Variables[User::ConfigFile].Properties[Value]"
REM ========================================

REM Timestamped log file
for /f "tokens=1-3 delims=/ " %%a in ("%date%") do set "d=%%c-%%a-%%b"
for /f "tokens=1-3 delims=:." %%a in ("%time%") do set "t=%%a%%b%%c"
set "LogDir=%~dp0logs"
if not exist "%LogDir%" mkdir "%LogDir%"
set "LogFile=%LogDir%\dtexec_%d%_%t%.log"

echo === Starting SSIS package ===
echo Package: %PackagePath%
echo ConfigFile env: %ConfigFile%
echo Log: %LogFile%
echo.

REM Make the environment variable available to the process (current session)
set "ConfigFile=%ConfigFile%"

REM ---- Choose ONE of the following lines depending on how your package reads the value ----
REM If your SSIS package reads the environment variable directly (Package Configurations or Script):
REM Just run dtexec; the env var is already set:
REM "DTExec.exe" /F "%PackagePath%" /Rep E /Logger "DTS.LOGGERFILE;LogFile=%LogFile%"

REM If your SSIS package expects a **parameter** named ConfigFile (SSIS 2012+):
REM "DTExec.exe" /F "%PackagePath%" /Par "$Package::ConfigFile=%ConfigFile%" /Rep E /Logger "DTS.LOGGERFILE;LogFile=%LogFile%"

REM If your SSIS package expects a **variable** User::ConfigFile (legacy):
REM "DTExec.exe" /F "%PackagePath%" /Set "\Package.Variables[User::ConfigFile].Properties[Value];%ConfigFile%" /Rep E /Logger "DTS.LOGGERFILE;LogFile=%LogFile%"

REM --- Active command (pick the one that matches your package) ---
"DTExec.exe" /F "%PackagePath%" /Par "$Package::ConfigFile=%ConfigFile%" /Rep E /Logger "DTS.LOGGERFILE;LogFile=%LogFile%"
set "RC=%ERRORLEVEL%"

echo.
if %RC% NEQ 0 (
    echo ERROR: DTExec failed with exit code %RC%
    echo See log: "%LogFile%"
) else (
    echo SUCCESS: DTExec completed successfully.
)

endlocal & exit /b %RC%
