from __future__ import annotations
import json, re
from pathlib import Path
ROOT=Path(__file__).resolve().parent
VENDOR=Path('docs/specs/vendor/protocol-canonical-2026-07-06')
api=(VENDOR/'anthropic-messages-api.official-raw.md').read_text(errors='replace')
stream=(VENDOR/'anthropic-messages-streaming.official-raw.md').read_text(errors='replace')
mcp=(VENDOR/'anthropic-mcp-connector.official-raw.md').read_text(errors='replace')
body=api[api.find('### Body Parameters'):api.find('### Returns')]
returns=api[api.find('### Returns'):api.find('### Response')]

def clean(s): return re.sub(r'\s+', ' ', s).strip()

def extract_desc(field, section):
    pat=f'`{re.escape(field)}:'
    i=section.find(pat)
    if i<0: return ''
    j=len(section)
    # next top-level known marker will be handled by caller; fallback next " - `xxx: optional/number/string/array/Model/ToolChoice" after 100 chars
    m=re.search(r' - `([a-zA-Z_][a-zA-Z0-9_]*): (optional |number|string|array|boolean|Model|Metadata|OutputConfig|ToolChoice|ThinkingConfigParam)', section[i+len(pat):])
    if m and m.start()>80:
        j=i+len(pat)+m.start()
    return clean(section[i:j])

request_fields=[
 ('max_tokens','number','required'),
 ('messages','array[MessageParam]','required'),
 ('model','Model','required'),
 ('container','string','optional'),
 ('inference_geo','string','optional'),
 ('metadata','Metadata','optional'),
 ('output_config','OutputConfig','optional'),
 ('service_tier','auto | standard_only','optional'),
 ('stop_sequences','array[string]','optional'),
 ('stream','boolean','optional'),
 ('system','string | array[TextBlockParam]','optional'),
 ('temperature','number','optional'),
 ('thinking','ThinkingConfigParam','optional'),
 ('tool_choice','ToolChoice','optional'),
 ('tools','array[ToolUnion]','optional'),
 ('top_k','number','optional'),
 ('top_p','number','optional'),
]
request=[{'field':f,'type':t,'required':req=='required','description':extract_desc(f,body)} for f,t,req in request_fields]

response_fields=[
 ('id','string'),('container','Container'),('content','array[ContentBlock]'),('model','Model'),('role','assistant'),('stop_details','StopDetails'),('stop_reason','string'),('stop_sequence','string|null'),('type','message'),('usage','Usage')
]
response=[{'field':f,'type':t,'required': f not in {'container','stop_details','stop_sequence'},'description':extract_desc(f,returns)} for f,t in response_fields]

# Streaming events from official doc prose
stream_events=[]
for ev in ['message_start','content_block_start','content_block_delta','content_block_stop','message_delta','message_stop','ping','error']:
    if ev in stream:
        stream_events.append(ev)
# delta types
stream_delta_types=[]
for ev in ['text_delta','input_json_delta','thinking_delta','signature_delta','citations_delta']:
    if ev in stream:
        stream_delta_types.append(ev)

mcp_fields=[]
if 'mcp_servers' in mcp:
    mcp_fields.append({'field':'mcp_servers','type':'array[MCPServer]','required':False,'description':'Remote MCP server definitions used with Messages API MCP connector companion feature.'})
if 'mcp_toolset' in mcp:
    mcp_fields.append({'field':'tools[].type=mcp_toolset','type':'ToolUnion variant','required':False,'description':'Toolset entry referencing an MCP server by name.'})
for f,t in [('name','string'),('url','string'),('authorization_token','string'),('tool_configuration','object')]:
    if f in mcp:
        mcp_fields.append({'field':'mcp_servers[].'+f,'type':t,'required': f in {'name','url'},'description':'MCP server definition field.'})

result={'anthropic_request':request,'anthropic_response':response,'anthropic_stream_events':stream_events,'anthropic_stream_delta_types':stream_delta_types,'anthropic_mcp_connector_fields':mcp_fields}
(ROOT/'anthropic-fields.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
lines=['# Anthropic official/companion fields','',f'Sources: `{VENDOR}/anthropic-messages-api.official-raw.md`, `anthropic-messages-streaming.official-raw.md`, `anthropic-mcp-connector.official-raw.md`','']
for key,title in [('anthropic_request','Anthropic Messages request'),('anthropic_response','Anthropic Message response')]:
    lines += [f'## {title}','', '| Field | Required | Type | Meaning |','|---|---:|---|---|']
    for x in result[key]:
        lines.append(f"| `{x['field']}` | {'yes' if x['required'] else 'no'} | `{x['type']}` | {x['description'].replace('|','\\|')} |")
    lines.append('')
lines += ['## Anthropic stream events',''] + [f'- `{x}`' for x in stream_events] + ['','## Anthropic stream delta types',''] + [f'- `{x}`' for x in stream_delta_types]
lines += ['','## Anthropic MCP connector companion fields','', '| Field | Required | Type | Meaning |','|---|---:|---|---|']
for x in mcp_fields:
    lines.append(f"| `{x['field']}` | {'yes' if x['required'] else 'no'} | `{x['type']}` | {x['description']} |")
(ROOT/'anthropic-fields.md').write_text('\n'.join(lines)+'\n')
print('request', len(request), [x['field'] for x in request])
print('response', len(response), [x['field'] for x in response])
print('events', stream_events)
print('delta', stream_delta_types)
print('mcp', [x['field'] for x in mcp_fields])
