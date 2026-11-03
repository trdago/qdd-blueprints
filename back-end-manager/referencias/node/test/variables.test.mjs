// BEM-C08: toda variable que lee el código está declarada en el manifiesto (serverless.example.yml).
// El manifiesto reemplaza todas las variables de la función en cada despliegue: una leída sin declarar no existe en la nube.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const SRC = new URL('../src/', import.meta.url).pathname;
const MANIFIESTO = new URL('../../serverless.example.yml', import.meta.url).pathname;
const IGNORAR = new Set(['AWS_LAMBDA_FUNCTION_NAME', 'AWS_REGION']); // las pone Lambda

function leidas() {
  const out = new Set();
  const patrones = [/process\.env\.([A-Z][A-Z0-9_]*)/g, /process\.env\[['"]([A-Z][A-Z0-9_]*)['"]\]/g, /\b(?:obligatoria|secreto|entero|lista)\(\s*['"]([A-Z][A-Z0-9_]*)['"]/g];
  for (const f of readdirSync(SRC).filter((n) => n.endsWith('.mjs'))) {
    const texto = readFileSync(join(SRC, f), 'utf8');
    for (const p of patrones) for (const m of texto.matchAll(p)) out.add(m[1]);
  }
  return out;
}

// Lee las claves de provider.environment (y de functions.*.environment) sin dependencias: bloques indentados.
function declaradas() {
  const out = new Set();
  let indentBloque = -1;
  for (const linea of readFileSync(MANIFIESTO, 'utf8').split('\n')) {
    const indent = linea.length - linea.trimStart().length;
    if (/^\s*environment:\s*$/.test(linea)) { indentBloque = indent; continue; }
    if (indentBloque < 0 || linea.trim() === '' || linea.trim().startsWith('#')) continue;
    if (indent <= indentBloque) { indentBloque = -1; continue; }
    const m = linea.match(/^\s*([A-Z][A-Z0-9_]*)\s*:/);
    if (m) out.add(m[1]);
  }
  return out;
}

test('BEM-C08: toda variable leída está declarada en el manifiesto', () => {
  const d = declaradas();
  const faltan = [...leidas()].filter((v) => !d.has(v) && !IGNORAR.has(v));
  assert.deepEqual(faltan, [], `leídas sin declarar: ${faltan.join(', ')}`);
  for (const v of ['JWT_SECRET', 'DATABASE_URL', 'CORS_ALLOWED_ORIGINS']) assert.ok(leidas().has(v), v);
});
