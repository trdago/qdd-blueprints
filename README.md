# qdd-blueprints

Blueprints QDD públicos: el esqueleto conceptual (vocabulario, preguntas de diseño, invariantes y escenarios de
conformidad) de módulos que se reutilizan en todos los proyectos. Cualquiera puede leerlos, importarlos y adaptarlos
(licencia [MIT](LICENSE)).

| Blueprint | Qué define |
|---|---|
| [`estado-por-frente`](estado-por-frente/blueprint.yaml) | Reporte de avance en una tabla de frentes × etapas con reglas de evidencia auditables. |
| [`consola-claude-code`](consola-claude-code/blueprint.yaml) | Autocompactación, modelo y esfuerzo por defecto, barra de estado (proyecto · modelo · contexto · 5h · semana) y, con `consola_qdd`, panel de estado con acciones. Referencia en `referencias/`. |
| [`estado-por-tareas`](estado-por-tareas/blueprint.yaml) | Tarea en curso y pulsos de vida: «● running» con un latido sutil en el panel, que se apaga solo si nada late (requiere `estado-por-frente`). |

## Usarlos

Con [QDD](https://github.com/trdago/qdd-framework) instalado (`npm install -g qdd-framework`), en la raíz de tu proyecto:

```sh
qdd blueprint add https://github.com/trdago/qdd-blueprints.git --paquete base
qdd blueprint apply estado-por-frente
qdd modules approve estado-por-frente
```

La URL HTTPS funciona sin cuenta de GitHub. Si tienes una llave SSH en GitHub también sirve
`git@github.com:trdago/qdd-blueprints.git`.

Puedes usarlos junto a los de tu organización: el lockfile del proyecto recuerda de qué repositorio salió cada uno
(QDD ≥ 2.5.0).

## Versionado

Semver por blueprint: patch = redacción, minor = opciones nuevas, major = cambio de invariantes. Lo calcula QDD al
proponer un cambio (`qdd blueprint propose`); cada versión publicada es un tag `vX.Y.Z` del repositorio.
