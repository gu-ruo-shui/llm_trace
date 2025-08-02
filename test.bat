@echo off
setlocal enabledelayedexpansion

echo 🧪 运行所有测试...

REM 下载依赖
echo 📦 下载依赖...
go mod tidy
if !errorlevel! neq 0 (
    echo ❌ 依赖下载失败
    exit /b 1
)

REM 运行单元测试
echo 🔬 运行单元测试...
go test -v ./...
if !errorlevel! neq 0 (
    echo ❌ 单元测试失败
    exit /b 1
)

REM 运行集成测试
echo 🔗 运行集成测试...
go test -v -run TestIntegration ./...
if !errorlevel! neq 0 (
    echo ❌ 集成测试失败
    exit /b 1
)

REM 运行性能测试
echo ⚡ 运行性能测试...
go test -v -run TestIntegration_Performance ./...
if !errorlevel! neq 0 (
    echo ❌ 性能测试失败
    exit /b 1
)

REM 运行覆盖率测试
echo 📊 运行覆盖率测试...
go test -v -cover ./...
if !errorlevel! neq 0 (
    echo ❌ 覆盖率测试失败
    exit /b 1
)

echo ✅ 所有测试完成！
pause