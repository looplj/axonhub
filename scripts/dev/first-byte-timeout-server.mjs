import http from 'node:http';
import { pathToFileURL } from 'node:url';

const DEFAULT_HOST = '127.0.0.1';
const DEFAULT_PORT = 18090;
const TEST_MODEL_ID = 'first-byte-timeout-test';
const HOLD_MODE_NO_HEADERS = 'no-headers';
const HOLD_MODE_SSE_NO_EVENTS = 'sse-no-events';

function sendJson(response, statusCode, body) {
  const payload = JSON.stringify(body);

  response.writeHead(statusCode, {
    'access-control-allow-origin': '*',
    'content-length': Buffer.byteLength(payload),
    'content-type': 'application/json; charset=utf-8',
  });
  response.end(payload);
}

function sendOptions(response) {
  response.writeHead(204, {
    'access-control-allow-headers': 'authorization,content-type,x-api-key,x-project-id',
    'access-control-allow-methods': 'GET,POST,OPTIONS',
    'access-control-allow-origin': '*',
    'access-control-max-age': '600',
  });
  response.end();
}

function holdSseWithoutEvents(request, response, pathname, logger) {
  request.resume();
  request.socket.setTimeout(0);
  response.setTimeout(0);
  response.writeHead(200, {
    'access-control-allow-origin': '*',
    'cache-control': 'no-cache',
    connection: 'keep-alive',
    'content-type': 'text/event-stream; charset=utf-8',
  });
  response.flushHeaders();

  logger.log(`[hold] ${request.method} ${pathname} accepted; SSE headers sent, no events will be sent`);
  request.on('close', () => {
    logger.log(`[close] ${request.method} ${pathname} client disconnected`);
  });
}

function holdWithoutResponseBytes(request, response, pathname, logger) {
  request.resume();
  request.socket.setTimeout(0);
  response.setTimeout(0);

  logger.log(`[hold] ${request.method} ${pathname} accepted; no response bytes will be sent`);
  request.on('close', () => {
    logger.log(`[close] ${request.method} ${pathname} client disconnected`);
  });
}

function shouldHoldResponse(request, pathname) {
  if (request.method === 'POST') {
    return true;
  }

  return pathname === '/v1/chat/completions' || pathname === '/v1/responses' || pathname === '/v1/messages';
}

export function createFirstByteTimeoutServer({ logger = console, holdMode = HOLD_MODE_NO_HEADERS } = {}) {
  const server = http.createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://${request.headers.host ?? 'localhost'}`);

    if (request.method === 'OPTIONS') {
      sendOptions(response);
      return;
    }

    if (request.method === 'GET' && url.pathname === '/health') {
      sendJson(response, 200, { status: 'ok' });
      return;
    }

    if (request.method === 'GET' && url.pathname === '/v1/models') {
      sendJson(response, 200, {
        object: 'list',
        data: [
          {
            id: TEST_MODEL_ID,
            object: 'model',
            created: 0,
            owned_by: 'first-byte-timeout-server',
          },
        ],
      });
      return;
    }

    if (shouldHoldResponse(request, url.pathname)) {
      if (holdMode === HOLD_MODE_SSE_NO_EVENTS) {
        holdSseWithoutEvents(request, response, url.pathname, logger);
        return;
      }

      holdWithoutResponseBytes(request, response, url.pathname, logger);
      return;
    }

    sendJson(response, 404, {
      error: {
        message: 'not found',
        type: 'not_found',
      },
    });
  });

  server.timeout = 0;
  server.requestTimeout = 0;
  server.headersTimeout = 0;
  server.keepAliveTimeout = 0;

  return server;
}

export function startFirstByteTimeoutServer({
  host = process.env.FIRST_BYTE_TIMEOUT_HOST || DEFAULT_HOST,
  port = Number(process.env.FIRST_BYTE_TIMEOUT_PORT || DEFAULT_PORT),
  holdMode = process.env.FIRST_BYTE_TIMEOUT_MODE || HOLD_MODE_NO_HEADERS,
  logger = console,
} = {}) {
  const server = createFirstByteTimeoutServer({ logger, holdMode });

  server.listen(port, host, () => {
    logger.log(`[first-byte-timeout] listening on http://${host}:${port}`);
    logger.log(`[first-byte-timeout] health: GET http://${host}:${port}/health`);
    logger.log(`[first-byte-timeout] model id: ${TEST_MODEL_ID}`);
    logger.log(`[first-byte-timeout] mode: ${holdMode}`);
    if (holdMode === HOLD_MODE_SSE_NO_EVENTS) {
      logger.log('[first-byte-timeout] POST requests send SSE headers and then hold without events');
    } else {
      logger.log('[first-byte-timeout] POST requests are accepted and held without response bytes');
    }
  });

  return server;
}

function readArg(name, fallback) {
  const equalsPrefix = `--${name}=`;
  const equalsValue = process.argv.find((arg) => arg.startsWith(equalsPrefix));

  if (equalsValue) {
    return equalsValue.slice(equalsPrefix.length);
  }

  const index = process.argv.indexOf(`--${name}`);
  if (index >= 0 && process.argv[index + 1]) {
    return process.argv[index + 1];
  }

  return fallback;
}

function parsePort(value) {
  const port = Number(value);

  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`invalid port: ${value}`);
  }

  return port;
}

function parseHoldMode(value) {
  if (value === HOLD_MODE_NO_HEADERS || value === HOLD_MODE_SSE_NO_EVENTS) {
    return value;
  }

  throw new Error(`invalid mode: ${value}. Use "${HOLD_MODE_NO_HEADERS}" or "${HOLD_MODE_SSE_NO_EVENTS}".`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const host = readArg('host', process.env.FIRST_BYTE_TIMEOUT_HOST || DEFAULT_HOST);
  const port = parsePort(readArg('port', process.env.FIRST_BYTE_TIMEOUT_PORT || String(DEFAULT_PORT)));
  const holdMode = parseHoldMode(readArg('mode', process.env.FIRST_BYTE_TIMEOUT_MODE || HOLD_MODE_NO_HEADERS));
  const server = startFirstByteTimeoutServer({ host, port, holdMode });

  const shutdown = () => {
    server.close(() => {
      process.exit(0);
    });
  };

  process.once('SIGINT', shutdown);
  process.once('SIGTERM', shutdown);
}
