# Tools Usage Examples

## Capture Command

### OpenAI Responses API
```bash
./tools capture -url=http://127.0.0.1:8090/v1/responses -payload=responses -key='ah-xxx'
```

### Anthropic Messages API
```bash
./tools capture -url=http://127.0.0.1:8090/anthropic/v1/messages -payload=messages -key='ah-xxx'
```

### OpenAI Chat Completions API
```bash
./tools capture -url=http://127.0.0.1:8090/v1/chat/completions -payload=chat -key='ah-xxx'
```
