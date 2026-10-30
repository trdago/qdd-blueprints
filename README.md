# qdd-blueprints

Blueprints QDD públicos: el esqueleto conceptual (vocabulario, preguntas de diseño, invariantes y escenarios de
conformidad) de módulos que se reutilizan en todos los proyectos. Cualquiera puede leerlos, importarlos y adaptarlos
(licencia [MIT](LICENSE)).

| Blueprint | Qué define |
|---|---|
| [`estado-por-tareas`](estado-por-tareas/blueprint.yaml) | Reporte de avance en una tabla de tareas × etapas con reglas de evidencia auditables y, si lo quieres, la tarea en curso: «● running» con un latido sutil en el panel, que se apaga solo si nada late. Reemplaza a `estado-por-frente`. |
| [`consola-claude-code`](consola-claude-code/blueprint.yaml) | Autocompactación, modelo y esfuerzo por defecto, barra de estado (proyecto · modelo · contexto · 5h · semana) y, con `consola_qdd`, panel de estado con acciones y con los comandos que la IA deja para ti (un clic y «s  Sí»). Referencia en `referencias/`. |
| [`estado-por-frente`](estado-por-frente/blueprint.yaml) | *Reemplazado por `estado-por-tareas`.* Se mantiene para los proyectos que aún no migran: con QDD ≥ 2.8.0, `qdd blueprint add estado-por-tareas` los migra conservando su módulo y sus respuestas. |

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
