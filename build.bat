@echo off
setlocal

set BINARY_NAME=lion
set MAIN_PATH=./cmd/cli
set DIST_DIR=dist

set VERSION=dev
for /f "tokens=*" %%i in ('git describe --tags --always --dirty 2^>nul') do set VERSION=%%i

set LDFLAGS=-s -w -X main.version=%VERSION%

if "%1"=="" goto :build_all
if "%1"=="clean" goto :clean
if "%1"=="help" goto :help
goto :build_one

:build_all
echo === 构建所有平台 ===
echo.

echo [1/6] windows/amd64
set GOOS=windows& set GOARCH=amd64
go build -ldflags "%LDFLAGS%" -o %DIST_DIR%\%BINARY_NAME%-windows-amd64.exe %MAIN_PATH%

echo [2/6] windows/arm64
set GOOS=windows& set GOARCH=arm64
go build -ldflags "%LDFLAGS%" -o %DIST_DIR%\%BINARY_NAME%-windows-arm64.exe %MAIN_PATH%

echo [3/6] darwin/amd64
set GOOS=darwin& set GOARCH=amd64
go build -ldflags "%LDFLAGS%" -o %DIST_DIR%\%BINARY_NAME%-darwin-amd64 %MAIN_PATH%

echo [4/6] darwin/arm64
set GOOS=darwin& set GOARCH=arm64
go build -ldflags "%LDFLAGS%" -o %DIST_DIR%\%BINARY_NAME%-darwin-arm64 %MAIN_PATH%

echo [5/6] linux/amd64
set GOOS=linux& set GOARCH=amd64
go build -ldflags "%LDFLAGS%" -o %DIST_DIR%\%BINARY_NAME%-linux-amd64 %MAIN_PATH%

echo [6/6] linux/arm64
set GOOS=linux& set GOARCH=arm64
go build -ldflags "%LDFLAGS%" -o %DIST_DIR%\%BINARY_NAME%-linux-arm64 %MAIN_PATH%

echo.
echo === 构建完成 ===
dir /b %DIST_DIR%
goto :eof

:build_one
echo === 构建 %1 ===
if "%1"=="windows/amd64" set GOOS=windows& set GOARCH=amd64& set OUT=%DIST_DIR%\%BINARY_NAME%-windows-amd64.exe
if "%1"=="windows/arm64" set GOOS=windows& set GOARCH=arm64& set OUT=%DIST_DIR%\%BINARY_NAME%-windows-arm64.exe
if "%1"=="darwin/amd64"  set GOOS=darwin&  set GOARCH=amd64& set OUT=%DIST_DIR%\%BINARY_NAME%-darwin-amd64
if "%1"=="darwin/arm64"  set GOOS=darwin&  set GOARCH=arm64& set OUT=%DIST_DIR%\%BINARY_NAME%-darwin-arm64
if "%1"=="linux/amd64"   set GOOS=linux&   set GOARCH=amd64& set OUT=%DIST_DIR%\%BINARY_NAME%-linux-amd64
if "%1"=="linux/arm64"   set GOOS=linux&   set GOARCH=arm64& set OUT=%DIST_DIR%\%BINARY_NAME%-linux-arm64
go build -ldflags "%LDFLAGS%" -o %OUT% %MAIN_PATH%
echo === 完成: %OUT% ===
goto :eof

:clean
if exist %DIST_DIR% rmdir /s /q %DIST_DIR%
echo === 已清理 ===
goto :eof

:help
echo 用法:
echo   build.bat              构建所有平台 (win/mac/linux, amd64/arm64)
echo   build.bat windows/amd64  构建单个平台
echo   build.bat darwin/arm64
echo   build.bat linux/amd64
echo   build.bat clean          清理构建产物
goto :eof
