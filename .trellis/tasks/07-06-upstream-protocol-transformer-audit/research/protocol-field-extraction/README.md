# Protocol field extraction artifacts

This folder exists because implementation must not proceed until request, response, and stream/event fields are inventoried.

## Python/YAML setup

PyYAML was installed globally into the uv-managed Python used by `python3`:

```bash
uv pip install --python "$(python3 -c 'import sys; print(sys.executable)')" --break-system-packages PyYAML
```

Verified:

```text
PyYAML 6.0.3
```

## OpenAI source correction

`docs/specs/vendor/protocol-canonical-2026-07-06/openai-api-definition.fetch.yaml` is not parseable YAML because the previous fetch path stripped indentation. The parseable source used here is:

```text
https://raw.githubusercontent.com/openai/openai-openapi/master/openapi.yaml
```

Saved as:

```text
openai-openapi.github.yaml
```

## Generated files

- `extract_openai_fields.py` -> `openai-fields.md`, `openai-fields.json`
- `extract_anthropic_fields.py` -> `anthropic-fields.md`, `anthropic-fields.json`
- `compare_fields_to_code.py` -> `complete-protocol-field-inventory.md`, `code-field-coverage.json`
- `openai-response-stream-event-types.json` -> Responses event schema to event type mapping
