import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { test } from 'node:test';
import { DefaultChatTransport, readUIMessageStream } from 'ai';

test('chat transport consumes the backend AI SDK stream fixture', async () => {
  const fixture = await readFile(
    new URL('../../../../llm/transformer/aisdk/testdata/aisdk-strop.stream.jsonl', import.meta.url),
    'utf8'
  );
  const events = fixture.trim().split('\n').map((line) => JSON.parse(line).Data);
  const expectedText = events.filter((event) => event !== '[DONE]').map(JSON.parse)
    .filter((event) => event.type === 'text-delta').map((event) => event.delta).join('');
  const messages = [{ id: 'user-message', role: 'user', parts: [{ type: 'text', text: 'Hello' }] }];
  const transport = new DefaultChatTransport({
    api: '/admin/playground/chat',
    body: { model: 'test-model' },
    fetch: async (url, init) => {
      assert.equal(url, '/admin/playground/chat');
      const body = JSON.parse(init.body);
      assert.equal(body.model, 'test-model');
      assert.deepEqual(body.messages, messages);
      return new Response(events.map((event) => `data: ${event}\n\n`).join(''), {
        headers: { 'Content-Type': 'text/event-stream', 'x-vercel-ai-ui-message-stream': 'v1' },
      });
    },
  });
  const stream = await transport.sendMessages({ trigger: 'submit-message', chatId: 'chat', messages });
  let result;
  for await (const message of readUIMessageStream({ stream, terminateOnError: true })) result = message;
  assert.equal(result.role, 'assistant');
  assert.equal(result.parts.filter((part) => part.type === 'text').map((part) => part.text).join(''), expectedText);
});
