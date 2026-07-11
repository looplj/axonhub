# Design — Chat Web Search Options

## Classification

`web_search_options` 是 Chat native request control，和以下对象不同：

```text
Responses hosted web_search tool
Anthropic web-search server tool
Chat tools[] declaration
```

## Seam

OpenAI Chat native request typed/raw top-level replay. 不进入 common `llm.Tool`。

## Tests

1. Chat request captures known object fields.
2. unknown nested key survives Chat replay.
3. Chat tool list remains unchanged.
4. target adapter produces no synthetic tool conversion.

