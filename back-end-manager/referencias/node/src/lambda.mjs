// Entrada de Lambda (handler: dist/lambda.handler). Todo lo caro se crea en el nivel superior del módulo, una sola
// vez por entorno (arranque en frío). Si la configuración falla, loadConfig lanza y el módulo no carga: Lambda lo
// informa como error de arranque y no queda una función a medias (BEM-C01, BEM-C03).
import serverless from 'serverless-http';
import { crearApp } from './app.mjs';
import { loadConfig } from './config.mjs';

const config = loadConfig(); // sin try/catch: un error aquí debe hacer fallar el arranque
const deps = { consultar: async () => 'ok' }; // aquí van el pool de la base, los clientes del SDK, etc.

export const handler = serverless(crearApp(config, deps), {
  binary: ['application/pdf', 'image/*', 'application/octet-stream'], // llegan en base64 e íntegros (BEM-C15)
  request(req, _event, context) {
    req.lambdaContext = context; // request id de la invocación para X-Request-Id y los logs (BEM-C25)
  },
});
