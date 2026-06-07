#!/bin/bash

# Intimate Agent 一键启动脚本
# 所有配置从 .env 文件读取，请确保已创建 .env 文件

echo "=========================================="
echo "  Intimate Agent 启动脚本"
echo "=========================================="

# 1. 设置 Go 环境
export PATH=$HOME/sdk/go/bin:$PATH
export GOPROXY=https://goproxy.cn,direct

# 2. 检查 .env 文件是否存在
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ ! -f "$SCRIPT_DIR/.env" ]; then
    echo "❌ 未找到 .env 文件"
    echo "   请复制 .env.example 为 .env 并填入你的配置："
    echo "   cp .env.example .env"
    exit 1
fi
echo "✅ 配置文件: $SCRIPT_DIR/.env"

# 3. 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装 Go 1.22.5"
    echo "   下载地址: https://go.dev/dl/"
    exit 1
fi
echo "✅ Go 已安装: $(go version)"

# 4. 进入项目目录
cd "$SCRIPT_DIR"
echo "✅ 项目目录: $(pwd)"

# 5. 下载依赖
echo "📦 下载依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "❌ 依赖下载失败"
    exit 1
fi
echo "✅ 依赖下载完成"

# 6. 编译程序
echo "🔨 编译程序..."
go build -o intimate-agent ./cmd/server/
if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi
echo "✅ 编译完成"

# 7. 启动程序
echo ""
echo "=========================================="
echo "  🚀 启动 Intimate Agent"
echo "=========================================="
echo ""
echo "按 Ctrl+C 停止程序"
echo "=========================================="
echo ""

# 运行程序（配置从 .env 自动加载）
./intimate-agent
