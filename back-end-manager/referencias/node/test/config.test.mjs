import { test } from 'node:test';
import assert from 'node:assert/strict';
import { loadConfig } from '../src/config.mjs';

const base = () => ({
  STAGE: 'qa',
  DATABASE_URL: 'postgres://u:p@h:5432/db',
  JWT_SECRET: 'x'.repeat(32),
  CORS_ALLOWED_ORIGINS: 'https://app.ejemplo.com/, http://localhost:5173',
});

test('BEM-C04: un secreto ausente o vacío no arranca y no usa default', () => {
  for (const v of [undefined, '', '   ']) {
    const env = base();
    if (v === undefined) delete env.JWT_SECRET;
    else env.JWT_SECRET = v;
    assert.throws(() => loadConfig(env), /JWT_SECRET: falta/);
  }
});

test('BEM-C05: el error nombra la variable, nunca su valor', () => {
  const env = { ...base(), JWT_SECRET: 'corto-y-secreto' };
  assert.throws(() => loadConfig(env), (err) => !err.message.includes('corto-y-secreto') && err.message.includes('JWT_SECRET'));
});

test('junta todos los problemas de una vez', () => {
  assert.throws(() => loadConfig({ MAX_BODY_KB: '-1' }), (err) =>
    ['STAGE: falta', 'DATABASE_URL: falta', 'JWT_SECRET: falta', 'MAX_BODY_KB'].every((p) => err.message.includes(p)));
});

test('defaults solo para lo que no es secreto', () => {
  const c = loadConfig(base());
  assert.equal(c.handlerBudgetMs, 25000);
  assert.equal(c.maxBodyKb, 1024);
  assert.deepEqual(c.allowedOrigins, ['https://app.ejemplo.com', 'http://localhost:5173']);
});
