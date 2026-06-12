#!/usr/bin/env python3
"""Сгенерировать токен для агента через gRPC"""

import sys
sys.path.insert(0, "/root/msg/hermes-agent")
import grpc
import messenger_pb2 as msg_pb
import messenger_pb2_grpc as msg_grpc

channel = grpc.insecure_channel("localhost:50052")
stub = msg_grpc.ChatServiceStub(channel)

# Используем первого пользователя как админа
admin_id = "73255fa6-8fce-4c8a-8807-89a98f7b7be0"

response = stub.GenerateAgentToken(msg_pb.GenerateAgentTokenRequest(
    agent_id="hermes-agent-1",
    agent_name="Hermes Agent",
    capabilities=["shell", "git", "build", "file", "docker", "ai"],
    ttl_hours=24 * 30,
    admin_user_id=admin_id,
))

print(f"Success: {response.success}")
if response.success:
    print(f"Token: {response.token}")
    print(f"Expires at: {response.expires_at}")
else:
    print(f"Error: {response.error}")
