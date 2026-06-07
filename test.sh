#!/bin/bash

# Intimate Agent 测试脚本

echo "=========================================="
echo "  Intimate Agent 测试脚本"
echo "=========================================="

BASE_URL="http://localhost:9090"

# 1. 健康检查
echo ""
echo "1️⃣  健康检查..."
HEALTH=$(curl -s $BASE_URL/health)
if [ "$HEALTH" = '{"status":"ok"}' ]; then
    echo "✅ 服务正常运行"
else
    echo "❌ 服务未运行，请先启动服务: ./start.sh"
    exit 1
fi

# 2. 测试对话
echo ""
echo "2️⃣  测试对话..."
echo "发送: 你好，我叫小明"
RESPONSE=$(curl -s -X POST $BASE_URL/chat \
    -H "Content-Type: application/json" \
    -d '{"user_id": "test_user", "message": "你好，我叫小明"}')

REPLY=$(echo $RESPONSE | python3 -c "import sys,json; print(json.load(sys.stdin)['reply'])")
echo "回复: $REPLY"

# 3. 查看状态
echo ""
echo "3️⃣  查看用户状态..."
STATUS=$(curl -s $BASE_URL/status/test_user)
echo "用户状态:"
echo $STATUS | python3 -m json.tool

# 4. 第二轮对话
echo ""
echo "4️⃣  第二轮对话..."
echo "发送: 我今天心情不太好"
RESPONSE2=$(curl -s -X POST $BASE_URL/chat \
    -H "Content-Type: application/json" \
    -d '{"user_id": "test_user", "message": "我今天心情不太好"}')

REPLY2=$(echo $RESPONSE2 | python3 -c "import sys,json; print(json.load(sys.stdin)['reply'])")
echo "回复: $REPLY2"

# 5. 再次查看状态
echo ""
echo "5️⃣  查看更新后的状态..."
STATUS2=$(curl -s $BASE_URL/status/test_user)
echo "用户状态:"
echo $STATUS2 | python3 -m json.tool

echo ""
echo "=========================================="
echo "  ✅ 测试完成"
echo "=========================================="
