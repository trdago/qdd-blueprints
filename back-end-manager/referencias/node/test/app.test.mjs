import { test, after } from 'node:test';
import assert from 'node:assert/strict';
import { crearApp } from '../src/app.mjs';

const APP = 'https://app.ejemplo.com';
const config = { allowedOrigins: [APP], handlerBudgetMs: 200, maxBodyKb: 1 };
const servidores = [];
after(() => servidores.forEach((s) => s.close()));

async function levantar(deps = { consultar: async () => 'ok' }) {
  const s = crearApp(config, deps).listen(0);
  servidores.push(s);
  await new Promise((r) => s.once('listening', r));
  return `http://127.0.0.1:${s.address().port}`;
}

test('BEM-C10: preflight 204 sin autenticación, con métodos, cabeceras y Max-Age', async () => {
  const url = await levantar();
  const r = await fetch(`${url}/v1/consulta`, { method: 'OPTIONS', headers: { Origin: APP, 'Access-Control-Request-Method': 'POST' } });
  assert.equal(r.status, 204);
  for (const h of ['access-control-allow-origin', 'access-control-allow-methods', 'access-control-allow-headers', 'access-control-max-age']) {
    assert.ok(r.headers.get(h), `falta ${h}`);
  }
});

test('BEM-C11: 404 y 500 traen CORS y X-Request-Id, sin detalles internos', async () => {
  const url = await levantar({ consultar: async () => { throw new Error('detalle interno'); } });
  for (const ruta of ['/no-existe', '/v1/consulta']) {
    const r = await fetch(url + ruta, { headers: { Origin: APP } });
    assert.ok(r.status >= 400, ruta);
    assert.equal(r.headers.get('access-control-allow-origin'), APP, ruta);
    assert.ok(r.headers.get('x-request-id'), ruta);
    assert.ok(!(await r.text()).includes('detalle interno'), ruta);
  }
});

test('BEM-K01: un origen fuera de la lista no recibe CORS', async () => {
  const url = await levantar();
  const r = await fetch(`${url}/health`, { headers: { Origin: 'https://malo.example' } });
  assert.equal(r.headers.get('access-control-allow-origin'), null);
  assert.equal(r.headers.get('access-control-allow-credentials'), null);
});

test('BEM-K02: un origen de la lista recibe ese origen exacto y Vary: Origin', async () => {
  const url = await levantar();
  const r = await fetch(`${url}/health`, { headers: { Origin: APP } });
  assert.equal(r.headers.get('access-control-allow-origin'), APP);
  assert.match(r.headers.get('vary') ?? '', /Origin/);
});

test('BEM-C12: con una dependencia lenta responde 504 propio dentro del presupuesto', async () => {
  const url = await levantar({ consultar: () => new Promise((r) => setTimeout(() => r('tarde'), 5000).unref()) });
  const inicio = Date.now();
  const r = await fetch(`${url}/v1/consulta`, { headers: { Origin: APP } });
  assert.equal(r.status, 504);
  assert.ok(Date.now() - inicio < 1000);
  assert.equal(r.headers.get('access-control-allow-origin'), APP);
});

test('BEM-C13: un cuerpo sobre el tope responde 413', async () => {
  const url = await levantar();
  const r = await fetch(`${url}/v1/eco`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ x: 'a'.repeat(4096) }) });
  assert.equal(r.status, 413);
});
