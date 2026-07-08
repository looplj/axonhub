from __future__ import annotations
import json, re
from pathlib import Path
import yaml

ROOT = Path(__file__).resolve().parent
OPENAI_YAML = ROOT / 'openai-openapi.github.yaml'
OUT = ROOT / 'openai-fields.json'
MD = ROOT / 'openai-fields.md'

data = yaml.safe_load(OPENAI_YAML.read_text())
schemas = data['components']['schemas']


def ref_name(ref: str) -> str:
    return ref.rsplit('/', 1)[-1]


def resolve(schema):
    seen=set()
    while isinstance(schema, dict) and '$ref' in schema:
        name=ref_name(schema['$ref'])
        if name in seen:
            break
        seen.add(name)
        schema=schemas[name]
    return schema


def merge_props(schema, depth=0, seen=None):
    if seen is None: seen=set()
    if not isinstance(schema, dict): return {}, []
    if '$ref' in schema:
        name=ref_name(schema['$ref'])
        if name in seen: return {}, []
        seen.add(name)
        props, req = merge_props(schemas[name], depth+1, seen)
        seen.remove(name)
        return props, req
    props=dict(schema.get('properties') or {})
    req=list(schema.get('required') or [])
    for key in ('allOf','anyOf','oneOf'):
        for sub in schema.get(key) or []:
            sp, sr = merge_props(sub, depth+1, seen)
            for k,v in sp.items(): props.setdefault(k,v)
            for r in sr:
                if r not in req: req.append(r)
    return props, req


def type_of(schema):
    if not isinstance(schema, dict): return ''
    if '$ref' in schema: return ref_name(schema['$ref'])
    if 'type' in schema:
        t=schema['type']
        if t=='array':
            return 'array['+type_of(schema.get('items') or {})+']'
        return str(t)
    for key in ('oneOf','anyOf','allOf'):
        if key in schema:
            vals=[type_of(x) for x in schema[key] if type_of(x)]
            return key+'('+ ' | '.join(vals[:8]) + ((' | …') if len(vals)>8 else '') +')'
    return ''


def desc(schema):
    if not isinstance(schema, dict): return ''
    d=schema.get('description') or schema.get('summary') or ''
    return re.sub(r'\s+', ' ', str(d)).strip()


def enum_of(schema):
    if not isinstance(schema, dict): return []
    if 'enum' in schema: return schema['enum']
    vals=[]
    for key in ('oneOf','anyOf'):
        for sub in schema.get(key) or []:
            vals += enum_of(resolve(sub))
    return vals


def fields_for_schema(name):
    schema=schemas[name]
    props, required=merge_props(schema)
    rows=[]
    for fname, fschema in props.items():
        rs=resolve(fschema)
        rows.append({
            'field': fname,
            'required': fname in required,
            'type': type_of(fschema),
            'description': desc(rs) or desc(fschema),
            'enum': enum_of(rs)[:80],
            'schema_ref': ref_name(fschema['$ref']) if isinstance(fschema,dict) and '$ref' in fschema else '',
        })
    return rows

# top-level request/response fields
result={
    'openai_responses_request': fields_for_schema('CreateResponse'),
    'openai_responses_response': fields_for_schema('Response'),
    'openai_chat_request': fields_for_schema('CreateChatCompletionRequest'),
    'openai_chat_response': fields_for_schema('CreateChatCompletionResponse'),
}

# collect likely streaming/event schemas for Responses and Chat
schema_names=sorted(schemas)
result['openai_responses_stream_schemas']=[n for n in schema_names if n.startswith('Response') and ('Event' in n or n.endswith('DeltaEvent') or 'Delta' in n)]
result['openai_chat_stream_schemas']=[n for n in schema_names if 'ChatCompletion' in n and ('Chunk' in n or 'Stream' in n)]

# also include nested schema field summaries for all referenced/important schemas matching protocol prefixes
important=[]
for n in schema_names:
    if n in {'CreateResponse','Response','CreateChatCompletionRequest','CreateChatCompletionResponse'}:
        continue
    if n.startswith(('Response','CreateResponse','ChatCompletion','CreateChatCompletion','Chat')):
        props,_=merge_props(schemas[n])
        if props:
            important.append({'schema':n,'fields':[f['field'] for f in fields_for_schema(n)]})
result['openai_related_nested_schemas']=important

OUT.write_text(json.dumps(result, ensure_ascii=False, indent=2))

lines=[]
lines.append('# OpenAI official schema fields')
lines.append('')
lines.append(f'Source: `{OPENAI_YAML}`')
for key,title in [
    ('openai_responses_request','OpenAI Responses request: CreateResponse'),
    ('openai_responses_response','OpenAI Responses response: Response'),
    ('openai_chat_request','OpenAI Chat request: CreateChatCompletionRequest'),
    ('openai_chat_response','OpenAI Chat response: CreateChatCompletionResponse'),
]:
    lines.append('')
    lines.append(f'## {title}')
    lines.append('')
    lines.append('| Field | Required | Type | Meaning | Enum |')
    lines.append('|---|---:|---|---|---|')
    for f in result[key]:
        enum=', '.join(map(str,f['enum'][:20]))
        lines.append(f"| `{f['field']}` | {'yes' if f['required'] else 'no'} | `{f['type']}` | {f['description'].replace('|','\\|')} | {enum.replace('|','\\|')} |")
lines.append('')
lines.append('## OpenAI Responses stream/event schema names')
for n in result['openai_responses_stream_schemas']:
    lines.append(f'- `{n}`')
lines.append('')
lines.append('## OpenAI Chat stream schema names')
for n in result['openai_chat_stream_schemas']:
    lines.append(f'- `{n}`')
lines.append('')
lines.append('## OpenAI related nested schemas')
for item in important:
    lines.append(f"- `{item['schema']}`: " + ', '.join(f'`{x}`' for x in item['fields']))
MD.write_text('\n'.join(lines)+'\n')
print('wrote', OUT)
print('wrote', MD)
for k in ['openai_responses_request','openai_responses_response','openai_chat_request','openai_chat_response']:
    print(k, len(result[k]), [x['field'] for x in result[k]])
print('responses stream schemas', len(result['openai_responses_stream_schemas']))
print('chat stream schemas', len(result['openai_chat_stream_schemas']))
