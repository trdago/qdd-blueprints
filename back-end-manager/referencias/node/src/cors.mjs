// Middleware CORS de la aplicación (capa_cors=aplicacion): va primero, antes que la autenticación, el logger y el
// manejo de errores, para que todo error traiga sus cabeceras (BEM-C10, BEM-C11).
// Con lista, solo un Origin de la lista recibe ese origen exacto, Vary: Origin y, si se pide, credenciales.
// Con comodín (lista vacía), * y nunca credenciales (BEM-K01, K02, K03).

export function cors({ origenes = [], credenciales = false, maxAge = 600 } = {}) {
  const permitidos = new Set(origenes.map((o) => o.replace(/\/+$/, '')));
  const comodin = permitidos.size === 0;
  const metodos = 'GET, POST, PUT, PATCH, DELETE, OPTIONS';
  const cabeceras = 'Authorization, Content-Type, Accept, X-Request-Id';

  return function corsMiddleware(req, res, next) {
    const origen = req.headers.origin;
    const permitido = comodin || permitidos.has(origen);
    if (!comodin) res.append('Vary', 'Origin');
    if (origen && permitido) {
      res.set('Access-Control-Allow-Origin', comodin ? '*' : origen);
      res.set('Access-Control-Expose-Headers', 'X-Request-Id');
    }
    if (origen && permitido && credenciales && !comodin) res.set('Access-Control-Allow-Credentials', 'true');

    const esPreflight = req.method === 'OPTIONS' && req.headers['access-control-request-method'];
    if (!esPreflight) return next();
    if (permitido) {
      res.set('Access-Control-Allow-Methods', metodos);
      res.set('Access-Control-Allow-Headers', cabeceras);
      res.set('Access-Control-Max-Age', String(maxAge));
    }
    return res.status(204).end();
  };
}
