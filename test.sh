#!/bin/bash

# 测试脚本 - 运行所有测试

set -e

echo "🧪 运行所有测试..."

# 下载依赖
echo "📦 下载依赖..."
go mod tidy

# 运行单元测试
echo "🔬 运行单元测试..."
go test -v ./...

# 运行集成测试（需要网络连接）
echo "🔗 运行集成测试..."
go test -v -run TestIntegration ./...

# 运行性能测试
echo "⚡ 运行性能测试..."
go test -v -run TestIntegration_Performance ./...

# 运行覆盖率测试
echo "📊 运行覆盖率测试..."
go test -v -cover ./...

echo "✅ 所有测试完成！"