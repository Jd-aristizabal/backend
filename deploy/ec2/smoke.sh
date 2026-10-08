#!/bin/sh
set -eu
api=http://172.30.10.3:8080
client=$(cat /proc/sys/kernel/random/uuid)
curl -fsS "$api/health"
curl -fsS "$api/ready"
curl -fsS -H "X-Client-ID: $client" -H 'Content-Type: application/json' -d '{"title":"Prueba EC2","priority":"media"}' "$api/api/v1/focus/tasks" > /tmp/johan-smoke-task.json
task=$(python3 -c 'import json;print(json.load(open("/tmp/johan-smoke-task.json"))["data"]["id"])')
curl -fsS -H "X-Client-ID: $client" "$api/api/v1/focus/tasks"
curl -fsS -X DELETE -H "X-Client-ID: $client" "$api/api/v1/focus/tasks/$task"
curl -fsS -H "X-Client-ID: $client" -H 'Content-Type: application/json' -d '{"message":"Que proyectos tiene Johan?"}' "$api/api/v1/chat" > /tmp/johan-smoke-chat.json
python3 -c 'import json;d=json.load(open("/tmp/johan-smoke-chat.json"))["data"];assert "Focus Flow" in d["message"];print(d["message"])'
conversation=$(python3 -c 'import json;print(json.load(open("/tmp/johan-smoke-chat.json"))["data"]["conversation_id"])')
curl -fsS -H "X-Client-ID: $client" "$api/api/v1/chat/conversations/$conversation/messages" > /tmp/johan-smoke-history.json
python3 -c 'import json;assert len(json.load(open("/tmp/johan-smoke-history.json"))["data"])==2;print("Historial persistido correctamente")'
curl -fsS -X DELETE -H "X-Client-ID: $client" "$api/api/v1/chat/conversations/$conversation"
rm /tmp/johan-smoke-task.json /tmp/johan-smoke-chat.json /tmp/johan-smoke-history.json
