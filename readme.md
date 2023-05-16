# Midjourney HTTP

## Example

```bash
cp config.template.yaml config.yaml
# edit config.yaml
# get your discordToken in browser devtools
# get appId、channelId、sessionId in devtools when chatting with midjourney official bot
go run main.go
```

- POST /generation-task

  ```json
  {
    "prompt": "hello world",
    "params": "--ar 16:9",
    "fast_mode": false // optional
  }
  ```

- GET /image?prompt=hello world&params=--ar 16:9&fast_mode=false
