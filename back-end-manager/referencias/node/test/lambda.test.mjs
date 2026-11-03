import { test } from 'node:test';
import assert from 'node:assert/strict';

const evento = (method, path) => ({
  version: '2.0',
  routeKey: '$default',
  rawPath: path,
  rawQueryString: '',
  headers: { origin: 'https://app.ejemplo.com', host: 'api.ejemplo.com' },
  requestContext: { http: { method, path, sourceIp: '127.0.0.1' }, requestId: 'req-gw' },
  isBase64Encoded: false,
});
const contexto = { awsRequestId: 'req-lambda-123' };

async function cargarHandler(env) {
  Object.assign(process.env, env);
  return (await import(`../src/lambda.mjs?${Math.random()}`)).handler;
}

const envOK = {
  STAGE: 'test',
  DATABASE_URL: 'postgres://u:p@h:5432/db',
  JWT_SECRET: 'x'.repeat(32),
  CORS_ALLOWED_ORIGINS: 'https://app.ejemplo.com',
};

test('BEM-C02: el handler de Lambda llega al router, incluida la raíz', async () => {
  const handler = await cargarHandler(envOK);
  const r = await handler(evento('GET', '/health'), contexto);
  assert.equal(r.statusCode, 200);
  assert.equal(JSON.parse(r.body).version, 'dev');
  assert.equal(r.headers['access-control-allow-origin'], 'https://app.ejemplo.com');
  assert.equal(r.headers['x-request-id'], 'req-lambda-123');
  const raiz = await handler(evento('GET', '/'), contexto);
  assert.equal(raiz.statusCode, 200, 'BEM-H01');
});

test('BEM-C15: un PDF sale en base64 e íntegro', async () => {
  const handler = await cargarHandler(envOK);
  const r = await handler(evento('GET', '/v1/archivo'), contexto);
  assert.equal(r.isBase64Encoded, true);
  assert.deepEqual([...Buffer.from(r.body, 'base64').subarray(0, 5)], [0x25, 0x50, 0x44, 0x46, 0x2d]);
});

test('BEM-C01: sin un secreto el módulo no carga (el arranque falla visible)', async () => {
  const guardado = process.env.JWT_SECRET;
  try {
    await assert.rejects(() => cargarHandler({ JWT_SECRET: '' }), /JWT_SECRET: falta/);
  } finally {
    process.env.JWT_SECRET = guardado;
  }
});
