# Referencias de back-end-manager

Implementaciones mínimas, en Go y en Node, que pasan los escenarios marcados en sus pruebas. Son un punto de
partida, no una librería: cópialas y adáptalas a la arquitectura de tu proyecto. Las dos aplican las mismas reglas.

| Archivo | Qué muestra | Escenarios |
|---|---|---|
| `go/cmd/api-lambda/main.go` | Entrada Go: arranque en frío que falla visible, mismo router en local, pruebas y Lambda (aws-lambda-go-api-proxy), presupuesto de tiempo, tope de cuerpo, request id, recuperador, errores sin detalles internos | BEM-C01, C02, C11, C12, C13, C15, C25, H01 |
| `go/config` · `node/src/config.mjs` | Cargador de configuración: secretos sin default, vacío = ausente, error que nombra variables y nunca valores | BEM-C04, C05 |
| `go/cors` · `node/src/cors.mjs` | Middleware CORS con lista o comodín, Vary: Origin, preflight 204 sin autenticación | BEM-C10, C11, K01, K02, K03 |
| `go/manifiesto` · `node/test/variables.test.mjs` | Prueba de que toda variable que lee el código está declarada en el manifiesto | BEM-C08 |
| `node/src/app.mjs` · `node/src/lambda.mjs` | Entrada Node: app Express sin nada de Lambda, envuelta por serverless-http; configuración en el nivel superior del módulo, binarios en base64, request id de la invocación | BEM-C01, C02, C11, C12, C13, C15, C25, H01 |
| `node/package.json` | Versiones fijadas, ESM, build con esbuild (bundle único con la versión del commit) y pruebas con node:test | BEM-N02 |
| `serverless.example.yml` | Manifiesto con HTTP API `$default`, arm64, retención de logs, concurrencia reservada, rol mínimo y recursos por ambiente (notas para Go y Node) | BEM-H01, C07, C09, C23 |
| `github-actions/deploy-api.example.yml` | GitHub Actions: pruebas → build Go → despliegue con OIDC → smoke test con versión, concurrencia por ambiente y vuelta atrás por ref | BEM-C19, C20, C21, C22, C24, O01 |
| `azure-pipelines/deploy-api.example.yml` | Azure Pipelines: lo mismo para Node, con environment (aprobaciones y lock) y variable group por ambiente | BEM-C19, C20, C21, C24 |
| `github-actions/deploy-frontend.example.yml` | Frontend a su bucket propio, caché por tipo de archivo e invalidación | BEM-F02, F03 |

```sh
cd go && go test ./...
cd node && npm ci && npm test && APP_VERSION=$(git rev-parse HEAD) npm run build
```

## Trampas que estas referencias evitan

1. **Arranque que falla en silencio.** Un `return` en `init()` (Go) o un `try/catch` alrededor de la configuración en el nivel superior del módulo (Node) deja la función a medias y cada invocación falla. Se hace fallar el arranque.
2. **Vacío no es ausente.** `${env:X, ''}` + un `getEnvOrDefault` (o `process.env.X || 'valor'`) activan el valor por defecto del código, también para secretos.
3. **El despliegue borra variables.** El manifiesto reemplaza todas las variables de la función: lo agregado en la consola desaparece, y lo que el CI pasa al job pero el manifiesto no declara nunca llega.
4. **Trabajo tras responder.** Lambda congela el entorno al responder: una goroutine o una promesa sin `await` queda a medias. Cola o servicio aparte.
5. **CORS reflejando el Origin con credenciales, o `*` en todo.** Lo primero abre la API a cualquier sitio; lo segundo, con cookies, el navegador lo rechaza. Lista explícita o comodín sin credenciales y con métodos y cabeceras explícitos.
6. **CORS en dos capas.** Si el gateway tiene CORS, ignora las cabeceras de la aplicación; con las dos, cabeceras duplicadas.
7. **Errores del gateway sin CORS.** Un timeout del gateway llega sin cabeceras y el navegador lo muestra como error CORS: responde antes con un presupuesto de tiempo.
8. **Archivos por la función.** El límite real es el de Lambda (6 MB), no el del gateway (10 MB), y el base64 lo achica más: URL prefirmada.
9. **Sentencias preparadas con un pooler en modo transacción.** «prepared statement already exists»: en Go, pgx en modo exec o simple protocol; en Node, consultas de `pg` sin `name`.
10. **`/{proxy+}` no incluye `/`.** Usa `$default` (en Serverless, `httpApi: '*'`).
11. **Sync con `--delete` a un bucket compartido.** Borra lo que no es del frontend. Bucket propio, variable propia y el paso falla si la variable está vacía.
12. **Secretos en variables públicas del frontend.** `VITE_*` quedan en el JavaScript descargable.
13. **Credenciales de despliegue en la función.** La función usa solo su rol de ejecución.
14. **Versiones que flotan.** `npm install -g serverless` sin versión, un runtime de Node vencido, Go distinto entre jobs: todo con versión fijada en el proyecto.
15. **Borrar el stack para desplegar.** `serverless remove` antes de `deploy` corta la API y puede borrar recursos con datos. Un despliegue actualiza en su lugar.
16. **Lambda en VPC sin salida.** Sin NAT ni VPC endpoints, las llamadas a S3, SQS o internet cuelgan hasta el timeout.
17. **Desplegado no es funcionando.** Smoke test que compara la versión de `/health` con el commit.
18. **Propiedades que el runtime ya define.** En Node ≥ 24, `req.signal` es de solo lectura en `IncomingMessage`: guarda el plazo propio con otro nombre (`req.plazo`).
