// La API como app de Express, sin nada de Lambda: corre igual en local (local.mjs), en pruebas y en Lambda
// (lambda.mjs). CORS primero, luego request id, plazo propio, tope de cuerpo y un manejador de errores que no
// filtra detalles internos (BEM-C11, C12, C13, C25).
import { randomBytes } from 'node:crypto';
import express from 'express';
import { cors } from './cors.mjs';

// __APP_VERSION__ lo reemplaza esbuild con el commit (--define); en pruebas y en local queda «dev».
export const version = typeof __APP_VERSION__ === 'string' && __APP_VERSION__ !== '' ? __APP_VERSION__ : 'dev';

export class TiempoAgotado extends Error {}

const pdfDeEjemplo = Buffer.from([0x25, 0x50, 0x44, 0x46, 0x2d, 0x31, 0x2e, 0x34, 0x0a, 0xe2, 0xe3, 0xcf, 0xd3, 0x0a]);

/**
 * @param {ReturnType<import('./config.mjs').loadConfig>} config
 * @param {{ consultar: (signal: AbortSignal) => Promise<string> }} deps puertos; en pruebas se simulan
 */
export function crearApp(config, deps) {
  const app = express();
  app.disable('x-powered-by');
  app.use(cors({ origenes: config.allowedOrigins, credenciales: false }));

  app.use((req, res, next) => {
    req.id = req.lambdaContext?.awsRequestId || randomBytes(8).toString('hex');
    res.set('X-Request-Id', req.id);
    // Plazo propio menor que el corte del gateway: toda llamada externa recibe esta señal.
    req.plazo = AbortSignal.timeout(config.handlerBudgetMs);
    next();
  });
  app.use(express.json({ limit: `${config.maxBodyKb}kb` }));

  app.get('/', (req, res) => res.json({ api: 'ok' }));
  app.get('/health', (req, res) => res.json({ status: 'ok', version })); // sin datos internos

  app.get('/v1/consulta', async (req, res) => {
    const valor = await conPlazo(deps.consultar(req.plazo), req.plazo); // siempre con await: nada queda suelto
    res.json({ valor });
  });
  app.post('/v1/eco', (req, res) => res.json(req.body));
  app.get('/v1/archivo', (req, res) => res.type('application/pdf').send(pdfDeEjemplo));

  // Manejador de errores: el detalle va al log con el request id, nunca al cliente.
  app.use((err, req, res, _next) => {
    let codigo = 500;
    let mensaje = 'error interno';
    if (err?.type === 'entity.too.large') [codigo, mensaje] = [413, 'cuerpo demasiado grande'];
    if (err instanceof TiempoAgotado || err?.name === 'TimeoutError') [codigo, mensaje] = [504, 'la operación tardó demasiado; intenta de nuevo'];
    console.error(JSON.stringify({ nivel: 'error', request_id: req.id, ruta: req.path, codigo, error: String(err?.message ?? err) }));
    res.status(codigo).json({ error: mensaje });
  });
  return app;
}

// conPlazo corta la espera cuando vence la señal, aunque la dependencia no la respete.
function conPlazo(promesa, signal) {
  return new Promise((resolve, reject) => {
    const alAbortar = () => reject(new TiempoAgotado('plazo vencido'));
    if (signal.aborted) return alAbortar();
    signal.addEventListener('abort', alAbortar, { once: true });
    promesa.then(resolve, reject).finally(() => signal.removeEventListener('abort', alAbortar));
  });
}
