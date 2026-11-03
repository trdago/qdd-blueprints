// Servidor local con la misma app que corre en Lambda: npm run local
import { crearApp } from './app.mjs';
import { loadConfig } from './config.mjs';

const app = crearApp(loadConfig(), { consultar: async () => 'ok' });
app.listen(8080, () => console.log(JSON.stringify({ nivel: 'info', msg: 'servidor local', addr: ':8080' })));
