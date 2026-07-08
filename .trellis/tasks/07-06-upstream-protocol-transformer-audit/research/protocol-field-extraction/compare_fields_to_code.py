from __future__ import annotations
import json,re
from pathlib import Path
ROOT=Path(__file__).resolve().parent
UP=Path('/tmp/axonhub-upstream-20260706-175405')
CUR=Path('/Users/asuan/项目/AI/axonhub')
openai=json.loads((ROOT/'openai-fields.json').read_text())
anth=json.loads((ROOT/'anthropic-fields.json').read_text())

def go_struct_fields(root:Path, rel:str, struct:str):
    txt=(root/rel).read_text(errors='replace')
    m=re.search(r'type\s+'+re.escape(struct)+r'\s+struct\s*\{',txt)
    if not m: return {}
    i=m.end(); depth=1; j=i
    while j<len(txt) and depth:
        if txt[j]=='{': depth+=1
        elif txt[j]=='}': depth-=1
        j+=1
    body=txt[i:j-1]
    out={}
    for line in body.splitlines():
        s=line.strip()
        if not s or s.startswith('//'): continue
        mm=re.match(r'([A-Z][A-Za-z0-9_]*)\s+(.+?)(?:\s+`([^`]+)`)?$',s)
        if not mm: continue
        name, typ, tag=mm.group(1), mm.group(2).strip(), mm.group(3) or ''
        jm=re.search(r'json:"([^",]+)', tag)
        json_name=jm.group(1) if jm else name
        if json_name=='-': continue
        out[json_name]={'go':name,'type':typ,'tag':tag}
    return out

def all_json_tags(root:Path, rels:list[str]):
    tags=set(); locations={}
    for rel in rels:
        p=root/rel
        if not p.exists(): continue
        txt=p.read_text(errors='replace')
        for m in re.finditer(r'([A-Z][A-Za-z0-9_]*)\s+[^`\n]+`json:"([^",]+)', txt):
            f=m.group(2)
            if f!='-':
                tags.add(f); locations.setdefault(f,[]).append(f'{rel}:{txt[:m.start()].count(chr(10))+1}:{m.group(1)}')
    return tags, locations

up_tags, up_locs = all_json_tags(UP, ['llm/transformer/openai/model.go','llm/transformer/openai/responses/model.go','llm/transformer/anthropic/model.go'])
cur_tags, cur_locs = all_json_tags(CUR, ['llm/transformer/openai/model.go','llm/transformer/openai/responses/model.go','llm/transformer/anthropic/model.go'])

struct_specs={
 'openai_chat_request': ('OpenAI Chat request', 'llm/transformer/openai/model.go','Request', openai['openai_chat_request']),
 'openai_chat_response': ('OpenAI Chat response', 'llm/transformer/openai/model.go','Response', openai['openai_chat_response']),
 'openai_responses_request': ('OpenAI Responses request', 'llm/transformer/openai/responses/model.go','Request', openai['openai_responses_request']),
 'openai_responses_response': ('OpenAI Responses response', 'llm/transformer/openai/responses/model.go','Response', openai['openai_responses_response']),
 'anthropic_request': ('Anthropic Messages request', 'llm/transformer/anthropic/model.go','MessageRequest', anth['anthropic_request']),
 'anthropic_response': ('Anthropic Message response', 'llm/transformer/anthropic/model.go','MessageResponse', anth['anthropic_response']),
}

# fallback Anthropic response struct name in code may be Response
if not go_struct_fields(UP,'llm/transformer/anthropic/model.go','MessageResponse'):
    struct_specs['anthropic_response']=('Anthropic Message response','llm/transformer/anthropic/model.go','Message', anth['anthropic_response'])

def handling(field, in_top, in_any, protocol):
    if in_top: return 'native top-level struct'
    if in_any: return 'nested/response/helper struct only'
    if protocol=='responses' and field in {'conversation','context_management'}: return 'missing in upstream request; should be native/opaque request field'
    if protocol=='chat' and field in {'web_search_options','prediction','audio','prompt_cache_retention'}: return 'missing in upstream request; modern Chat native field candidate'
    if protocol=='anthropic' and field in {'mcp_servers','container','inference_geo','context_management'}: return 'missing or companion-native field candidate'
    return 'missing/not modeled'

matrix={}
for key,(title,rel,struct,fields) in struct_specs.items():
    up_struct=go_struct_fields(UP,rel,struct)
    cur_struct=go_struct_fields(CUR,rel,struct)
    proto='chat' if 'chat' in key else 'responses' if 'responses' in key else 'anthropic'
    rows=[]
    for f in fields:
        name=f['field']
        rows.append({
            'field': name,
            'required': f.get('required',False),
            'official_type': f.get('type',''),
            'meaning': f.get('description',''),
            'upstream_top': name in up_struct,
            'current_top': name in cur_struct,
            'upstream_any': name in up_tags,
            'current_any': name in cur_tags,
            'upstream_location': '; '.join(up_locs.get(name,[])[:5]),
            'current_location': '; '.join(cur_locs.get(name,[])[:5]),
            'author_handling': handling(name, name in up_struct, name in up_tags, proto),
        })
    matrix[key]={'title':title,'file':rel,'struct':struct,'rows':rows}

# stream handling by string presence in packages
def grep_present(root:Path, rels:list[str], term:str):
    for rel in rels:
        p=root/rel
        if p.exists() and term in p.read_text(errors='replace'):
            return True
    return False
stream_rows=[]
for ev in openai['openai_responses_stream_schemas']:
    # convert schema name to likely event type? keep schema and presence by schema name or snake-ish event suffix impossible; use schema name presence
    stream_rows.append({'protocol':'openai_responses','event_or_schema':ev,'upstream_present':grep_present(UP,['llm/transformer/openai/responses/model.go','llm/transformer/openai/responses/inbound_stream.go','llm/transformer/openai/responses/outbound_stream.go','llm/transformer/openai/responses/aggregator.go'],ev),'current_present':grep_present(CUR,['llm/transformer/openai/responses/model.go','llm/transformer/openai/responses/inbound_stream.go','llm/transformer/openai/responses/outbound_stream.go','llm/transformer/openai/responses/aggregator.go'],ev)})
for ev in openai['openai_chat_stream_schemas']:
    stream_rows.append({'protocol':'openai_chat','event_or_schema':ev,'upstream_present':grep_present(UP,['llm/transformer/openai/model.go','llm/transformer/openai/inbound_stream.go','llm/transformer/openai/outbound_stream.go','llm/transformer/openai/aggregator.go'],ev),'current_present':grep_present(CUR,['llm/transformer/openai/model.go','llm/transformer/openai/inbound_stream.go','llm/transformer/openai/outbound_stream.go','llm/transformer/openai/aggregator.go'],ev)})
for ev in anth['anthropic_stream_events']+anth['anthropic_stream_delta_types']:
    stream_rows.append({'protocol':'anthropic','event_or_schema':ev,'upstream_present':grep_present(UP,['llm/transformer/anthropic/model.go','llm/transformer/anthropic/inbound_stream.go','llm/transformer/anthropic/outbound_stream.go','llm/transformer/anthropic/aggregator.go'],ev),'current_present':grep_present(CUR,['llm/transformer/anthropic/model.go','llm/transformer/anthropic/inbound_stream.go','llm/transformer/anthropic/outbound_stream.go','llm/transformer/anthropic/aggregator.go'],ev)})

result={'matrices':matrix,'stream_rows':stream_rows,'anthropic_mcp_connector_fields':anth['anthropic_mcp_connector_fields'],'openai_nested_schemas':openai['openai_related_nested_schemas']}
(ROOT/'code-field-coverage.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))

lines=['# Complete protocol field inventory against AxonHub code','','Sources: OpenAI OpenAPI YAML, Anthropic official raw docs + MCP connector companion docs, upstream/current Go structs.','']
for key,val in matrix.items():
    lines += [f"## {val['title']}", '', f"Code target: `{val['file']}` struct `{val['struct']}`", '', '| Field | Required | Official type | Upstream top-level? | Current top-level? | Upstream any json tag? | Author handling |','|---|---:|---|---:|---:|---:|---|']
    for r in val['rows']:
        lines.append(f"| `{r['field']}` | {'yes' if r['required'] else 'no'} | `{r['official_type']}` | {'yes' if r['upstream_top'] else 'no'} | {'yes' if r['current_top'] else 'no'} | {'yes' if r['upstream_any'] else 'no'} | {r['author_handling']} |")
    lines.append('')
lines += ['## Stream/event schema coverage','','| Protocol | Event/schema | Upstream present by name/string? | Current present by name/string? |','|---|---|---:|---:|']
for r in stream_rows:
    lines.append(f"| `{r['protocol']}` | `{r['event_or_schema']}` | {'yes' if r['upstream_present'] else 'no'} | {'yes' if r['current_present'] else 'no'} |")
lines += ['','## Anthropic MCP connector companion fields','','| Field | Required | Type |','|---|---:|---|']
for x in anth['anthropic_mcp_connector_fields']:
    lines.append(f"| `{x['field']}` | {'yes' if x['required'] else 'no'} | `{x['type']}` |")
lines += ['','## OpenAI nested schema field lists','','See `openai-fields.md` and `openai-fields.json` for full nested schema field lists. This matrix compares top-level request/response and stream/event coverage first.']
(ROOT/'complete-protocol-field-inventory.md').write_text('\n'.join(lines)+'\n')
print('wrote complete inventory')
for key,val in matrix.items():
    missing=[r['field'] for r in val['rows'] if not r['upstream_top']]
    print(key, 'fields', len(val['rows']), 'missing_upstream_top', missing)
print('stream rows', len(stream_rows))
