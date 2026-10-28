# qdd-blueprints

Blueprints QDD de la organización: el esqueleto conceptual (vocabulario, preguntas de diseño, invariantes y escenarios de
conformidad) de módulos que se reutilizan en todos los proyectos.

| Blueprint | Qué define |
|---|---|
| [`estado-por-frente`](estado-por-frente/blueprint.yaml) | Reporte de avance en una tabla de frentes × etapas con reglas de evidencia auditables. |
| [`consola-claude-code`](consola-claude-code/blueprint.yaml) | Autocompactación, modelo y esfuerzo por defecto, y barra de estado (proyecto · modelo · contexto · 5h · semana). Referencia en `referencias/`. |

```sh
qdd blueprint add git@github.com:trdago/qdd-blueprints.git --ref v1.0.0 --paquete base
qdd blueprint apply estado-por-frente
qdd modules approve estado-por-frente
```

Versionado (semver por blueprint): patch = redacción, minor = opciones nuevas, major = cambio de invariantes.
