# qdd-blueprints

Blueprints QDD públicos: el esqueleto conceptual (vocabulario, preguntas de diseño, invariantes y escenarios de
conformidad) de módulos que se reutilizan en todos los proyectos. Cualquiera puede leerlos, importarlos y adaptarlos
(licencia [MIT](LICENSE)).

| Blueprint | Qué define |
|---|---|
| [`estado-por-tareas`](estado-por-tareas/blueprint.yaml) | Reporte de avance en una tabla de tareas × etapas con reglas de evidencia auditables y, si lo quieres, la tarea en curso: «● running» con un latido sutil en el panel, que se apaga solo si nada late. Reemplaza a `estado-por-frente`. |
| [`consola-claude-code`](consola-claude-code/blueprint.yaml) | Autocompactación, modelo y esfuerzo por defecto, barra de estado (proyecto · modelo · contexto · 5h · semana) y, con `consola_qdd`, panel de estado con acciones y con los comandos que la IA deja para ti (un clic y «s  Sí»). Referencia en `referencias/`. |
| [`ciclo-de-vida-agentico`](ciclo-de-vida-agentico/blueprint.yaml) | Las 14 fases para construir un sistema agéntico con modelos entrenados o ajustados (del problema y su KPI al golden set, el umbral y la producción), con un tutor que explica, mide con números reales y marca cada fase solo con evidencia y una persona que aprueba. Si entra a un proyecto en curso, primero lo diagnostica: nivel de madurez por fase, qué está bien, qué falta y qué mejorar primero. Viene en el paquete `ia` junto a `estado-por-tareas`. |
| [`auth`](auth/blueprint.yaml) | Usuarios, roles, permisos y acceso: identidad siempre propia en tablas del proyecto, sin amarrarse a un proveedor (nada de Supabase Auth ni Cognito), login con correo o RUT, federación con Google o Clave Única, sesiones con refresh rotativo, recuperación, alta, baja y fusión de cuentas, matriz de roles × permisos, multi-organización, segundo factor, API keys, tokens para una capa de datos con RLS, sesiones de dispositivo, enlaces para terceros y auditoría. Matriz y validación de RUT de ejemplo en `referencias/`. |
| [`back-end-manager`](back-end-manager/blueprint.yaml) | API serverless en AWS Lambda, en Go o en Node: arranque en frío que falla visible, configuración con secretos obligatorios, variables declaradas en el manifiesto, CORS en una sola capa (lista o comodín sin credenciales), presupuesto de tiempo bajo el corte del gateway, archivos por URL prefirmada, base de datos con pooler, trabajo largo en cola, VPC, frontend en S3 + CloudFront, y despliegue solo por CI (GitHub Actions o Azure Pipelines) con OIDC, un despliegue por ambiente, smoke test con versión y vuelta atrás. Referencias en Go y Node con sus pruebas, manifiesto y pipelines de ejemplo en `referencias/`. |
| [`estado-por-frente`](estado-por-frente/blueprint.yaml) | *Reemplazado por `estado-por-tareas`.* Se mantiene para los proyectos que aún no migran: con QDD ≥ 2.8.0, `qdd blueprint add estado-por-tareas` los migra conservando su módulo y sus respuestas. |
| [`correo`](correo/blueprint.yaml) | Correo transaccional por SMTP estándar, con Gmail (contraseña de aplicación) por defecto o un servidor con dominio propio: cola persistente con reintentos e idempotencia, plantillas en español e inglés, cabeceras sin inyección, imágenes incrustadas, límites por destinatario, por IP y tope diario, SPF/DKIM/DMARC, rebotes y supresión, sin secretos en código ni registros y pruebas con un SMTP falso. |

## Usarlos

Con [QDD](https://github.com/trdago/qdd-framework) instalado (`npm install -g qdd-framework`), en la raíz de tu proyecto:

```sh
qdd blueprint add https://github.com/trdago/qdd-blueprints.git --paquete base
qdd blueprint apply estado-por-tareas
qdd modules approve estado-por-tareas
```

La URL HTTPS funciona sin cuenta de GitHub. Si tienes una llave SSH en GitHub también sirve
`git@github.com:trdago/qdd-blueprints.git`.

Puedes usarlos junto a los de tu organización: el lockfile del proyecto recuerda de qué repositorio salió cada uno
(QDD ≥ 2.5.0).

## Versionado

Semver por blueprint: patch = redacción, minor = opciones nuevas, major = cambio de invariantes. Lo calcula QDD al
proponer un cambio (`qdd blueprint propose`); cada versión publicada es un tag `vX.Y.Z` del repositorio.
