import sys
import json

if len(sys.argv) < 2:
    print("Usage: python script.py <json_string>")
    sys.exit(1)

json_string = sys.argv[1]

try:
    data = json.loads(json_string)
except json.JSONDecodeError:
    print("Invalid JSON string")
    sys.exit(1)

application_id = data.get("application_id")
guild_id = data.get("guild_id")
channel_id = data.get("channel_id")
session_id = data.get("session_id")

print(f"application_id: {application_id}")
print(f"guild_id: {guild_id}")
print(f"channel_id: {channel_id}")
print(f"session_id: {session_id}")