@echo off
echo Building for Linux (AMD64)...

REM 设置交叉编译环境变量
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64

REM 执行编译
go build -o goemail-linux-amd64 main.go

if %errorlevel% neq 0 (
    echo [ERROR] Build failed!
    pause
    exit /b %errorlevel%
)

echo [SUCCESS] Build completed: goemail-linux-amd64
echo You can upload this file to your Linux server and run it with:
echo   chmod +x goemail-linux-amd64
echo   ./goemail-linux-amd64
pause
