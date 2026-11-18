import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import ts from 'typescript';
import { getApiPath } from './curl-paths.ts';
import { escapeShellValue } from './curl-shell.ts';

// 直接执行生产代码的函数，显式协议使测试无需加载渠道图标与 React 依赖。
const generatorSource = readFileSync(new URL('./curl-generator.ts', import.meta.url), 'utf8');
const generatorAst = ts.createSourceFile('curl-generator.ts', generatorSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
const generatorModule = [
  `import { getApiPath } from '${new URL('./curl-paths.ts', import.meta.url).href}';`,
  `import { escapeShellValue } from '${new URL('./curl-shell.ts', import.meta.url).href}';`,
  ...generatorAst.statements.filter(ts.isFunctionDeclaration).map((statement) => statement.getText(generatorAst)),
].join('\n');
const { outputText } = ts.transpileModule(generatorModule, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { generateRequestCurl, generateExecutionCurl } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

test('uses the ZenMux native video endpoint for generated cURL', () => {
  assert.equal(getApiPath('zenmux/video'), '/v1/videos');
});

test('keeps OpenAI video cURL on the existing video endpoint', () => {
  assert.equal(getApiPath('openai/video'), '/v1/videos');
});

test('uses the Seedance task endpoint for generated cURL', () => {
  assert.equal(getApiPath('seedance/video'), '/api/v3/contents/generations/tasks');
});

test('uses native embedding and decision gateway routes without changing the body', () => {
  assert.equal(getApiPath('cohere/embed'), '/v2/embed');
  const request = { kind: 'choice', question: 'Which team?', options: ['billing', 'support'] };
  assert.equal(getApiPath('autotrust/decide', request), '/v1/decide');
  assert.deepEqual(request, { kind: 'choice', question: 'Which team?', options: ['billing', 'support'] });
});

test('copies the AutoTrust gateway model header with an unchanged native body', () => {
  const body = { kind: 'choice', question: 'Which team?', options: ['billing', 'support'] };
  const command = generateRequestCurl({ 'X-AxonHub-Model': ['my-decider'], 'Content-Type': ['application/json'] }, body, 'autotrust/decide');
  assert.ok(command.startsWith("curl '/v1/decide'"));
  assert.ok(command.includes("-H 'X-AxonHub-Model: my-decider'"));
  assert.ok(command.includes(`-d '${JSON.stringify(body)}'`));
});

test('retains the AutoTrust body model selector and actual upstream execution URL', () => {
  const body = { model: 'my-decider', kind: 'noul', question: 'Is it urgent?' };
  assert.ok(generateRequestCurl({}, body, 'autotrust/decide').includes('"model":"my-decider"'));
  const execution = generateExecutionCurl({}, { kind: 'noul', question: 'Is it urgent?' }, undefined, 'autotrust/decide', 'http://inference-host:8000/custom/v1/decide');
  assert.ok(execution.startsWith("curl 'http://inference-host:8000/custom/v1/decide'"));
  assert.ok(!execution.includes('X-AxonHub-Model'));
});

test('copies Cohere image content and native dimensions without OpenAI conversion', () => {
  const body = { model: 'qwen-vision', inputs: [{ content: [{ type: 'image_url', image_url: { url: 'data:image/png;base64,aGVsbG8=' } }] }], input_type: 'image', output_dimension: 2048 };
  const command = generateRequestCurl({ 'Content-Type': 'application/json' }, body, 'cohere/embed');
  assert.ok(command.startsWith("curl '/v2/embed'"));
  assert.ok(command.includes(`-d '${JSON.stringify(body)}'`));
});

test('shell-escapes model-derived URLs in generated cURL', () => {
  assert.equal(
    escapeShellValue("https://example.com/models/a'b:generateContent"),
    "https://example.com/models/a'\\''b:generateContent",
  );
});
