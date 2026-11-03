import { test } from 'node:test';
import assert from 'node:assert/strict';
import express from 'express';
import { cors } from '../src/cors.mjs';

test('BEM-K03: comodín responde * y nunca credenciales, aunque se pidan', async () => {
  const app = express();
  app.use(cors({ origenes: [], credenciales: true }));
  app.get('/x', (req, res) => res.send('ok'));
  const s = app.listen(0);
  await new Promise((r) => s.once('listening', r));
  try {
    const r = await fetch(`http://127.0.0.1:${s.address().port}/x`, { headers: { Origin: 'https://cualquiera.example' } });
    assert.equal(r.headers.get('access-control-allow-origin'), '*');
    assert.equal(r.headers.get('access-control-allow-credentials'), null);
  } finally {
    s.close();
  }
});
