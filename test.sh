#!/bin/bash

echo "Тестирование конкурентных запросов..."

send_request() {
    local key=$1
    curl -s -X POST http://localhost:8080/v1/withdrawals \
        -H "Authorization: Bearer test-token" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"user-123\",
            \"amount\": 300.00,
            \"currency\": \"USDT\",
            \"destination\": \"0x$key\",
            \"idempotency_key\": \"$key\"
        }"
}

echo "Отправляем 5 конкурентных запросов..."
for i in {1..5}; do
    send_request "key-$i" &
done

wait
echo "Все запросы выполнены"