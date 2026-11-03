// Cargador de configuración (BEM-C04, BEM-C05). Mismas reglas que la referencia en Go:
// un secreto u obligatorio vacío cuenta como ausente; un secreto nunca tiene valor por defecto; el error nombra cada
// variable que falta o está mal, nunca su valor. Un default solo existe para lo que no es secreto y está aquí.

export class ConfigError extends Error {}

export function loadConfig(env = process.env) {
  const problemas = [];
  const valor = (nombre) => (env[nombre] ?? '').trim();
  const obligatoria = (nombre) => {
    const v = valor(nombre);
    if (v === '') problemas.push(`${nombre}: falta`);
    return v;
  };
  const secreto = obligatoria; // aparte para que nadie le ponga un default
  const entero = (nombre, porDefecto) => {
    const v = valor(nombre);
    if (v === '') return porDefecto;
    const n = Number(v);
    if (!Number.isInteger(n) || n <= 0) {
      problemas.push(`${nombre}: debe ser un entero positivo`);
      return porDefecto;
    }
    return n;
  };
  const lista = (nombre) =>
    valor(nombre)
      .split(',')
      .map((o) => o.trim().replace(/\/+$/, ''))
      .filter(Boolean);

  const config = {
    stage: obligatoria('STAGE'),
    databaseUrl: secreto('DATABASE_URL'),
    jwtSecret: secreto('JWT_SECRET'),
    allowedOrigins: lista('CORS_ALLOWED_ORIGINS'),
    handlerBudgetMs: entero('HANDLER_BUDGET_SECONDS', 25) * 1000,
    maxBodyKb: entero('MAX_BODY_KB', 1024),
    presignedUrlMinutes: entero('PRESIGNED_URL_MINUTES', 15),
  };
  if (config.jwtSecret && config.jwtSecret.length < 32) problemas.push('JWT_SECRET: debe tener al menos 32 caracteres');
  if (problemas.length > 0) throw new ConfigError(`configuración inválida: ${problemas.join('; ')}`);
  return Object.freeze(config);
}
