# Diagnóstico · archivador-licitaciones · 2026-11-01

> Ejemplo del blueprint `ciclo-de-vida-agentico`. Es lo que el tutor escribe en `.qdd/project/tutor/diagnostico.md`
> al sumarse a un proyecto en curso (`entrada: en-curso`). Este es el segundo diagnóstico del proyecto; el anterior es
> del 2026-10-15.

**El sistema:** un grafo de LangGraph con tres agentes (clasificador, archivador y detector de dependencias) que usa
DeepSeek para clasificar documentos de licitaciones, archivarlos en carpetas estándar con su metadata y detectar qué
documento aclara o modifica a otro. Clasifica con un prompt few-shot. Un notebook informa accuracy 0,91. El split se
hizo al azar, no hay golden set y todo pasa sin revisión humana.

**Respuestas del blueprint:** entrada en-curso · nivel aprendiz · modo copiloto · tipo_tarea clasificacion ·
kappa_minimo 0.8 · tolerancia_drift 0.02 · registro estado · comprension preguntas · datos_personales si.

**Fuentes:** el repositorio (commit `4e1a9c2`), sin corridas guardadas, y una entrevista de 20 minutos con la persona
responsable.

## Semáforo

🟢 nivel 3–4 · 🟡 nivel 2 · 🔴 nivel 0–1 · ⚪ sin evidencia («?»). Un 0 es ausente confirmado por la persona; «?»,
que no se pudo comprobar. El diagnóstico no marca ✅: una puerta pasa a ✅ solo con nivel 3 o más y la aprobación
de una persona, y sin una corrida que muestre que no se cumple, lo que falta queda en ⏳, no en ❌.

| Fase | Nivel | Antes | | Evidencia | Puerta |
|---|---|---|---|---|---|
| F01 Problema y KPI | 1 | 1 | 🔴 | `README.md` dice «clasificar documentos de licitaciones», sin métrica ni meta. | ⏳ falta el KPI con su meta y su error tolerado, y lo que queda fuera |
| F02 Taxonomía y esquema | 3 | 1 | 🟢 | `taxonomia.yaml` v2 (9 clases y OTRO, cada una con ejemplo y contraejemplo); `esquema.json` validó las salidas de la corrida `tax-2026-10-28-a`. *Encontrada.* | ⏳ espera la aprobación de la persona |
| F03 Corpus, limpieza y split | 1 | 1 | 🔴 | `notebooks/explorar.ipynb`: `train_test_split(..., random_state=42)` al azar; 31 licitaciones con documentos en train y en test; RUT y correos sin anonimizar en los PDF. | ⏳ falta el split por grupo y la limpieza de PII |
| F04 Anotación y acuerdo | 0 | 0 | 🔴 | Entrevista: las etiquetas salieron del mismo modelo; nadie anotó. | ⏳ bloqueada: falta la limpieza de PII de F03 |
| F05 Golden set | 0 | 0 | 🔴 | No hay `eval/` ni `golden/`; la entrevista lo confirma. | ⏳ falta el golden |
| F06 Baseline | ? | ? | ⚪ | Sin artefacto, sin corrida y la entrevista no supo decir si se comparó con algo simple. | ⏳ sin evidencia |
| F07 Método | 1 | 1 | 🔴 | `prompts/clasificar.md`: few-shot con 12 ejemplos; se eligió sin comparar con zero-shot ni con otro modelo. | ⏳ falta la tabla comparativa |
| F08 Entrenamiento o ajuste | — | — | — | Few-shot sin ajuste de pesos: el tutor propone «—». | ⏳ espera que la persona decida «—» y su motivo |
| F09 Evaluación | 2 | 2 | 🟡 | `notebooks/metricas.ipynb`: accuracy 0,91 sin id de corrida, sin versión de datos ni semilla; no se pudo reproducir (dio 0,87 y 0,93). *Encontrada.* | ⏳ falta una corrida reproducible sobre el golden |
| F10 Calibración y umbral | 0 | 0 | 🔴 | Todo se archiva solo; las salidas no traen confianza. | ⏳ falta el umbral y a quién escala |
| F11 Equipo de agentes | 2 | 1 | 🟡 | `agentes/grafo.py` (3 nodos de LangGraph) y `agentes.md` con roles y herramientas, escrito tras el diagnóstico anterior. | ⏳ falta una métrica por agente |
| F12 Dependencias y contratos | 1 | 1 | 🔴 | `EstadoLicitacion(TypedDict)` sin validación al entrar ni al salir; las relaciones «aclara a» no tienen precisión ni recall. | ⏳ falta validar el contrato |
| F13 Salidas, metadata y trazabilidad | 2 | 2 | 🟡 | `salidas/<licitación>/<clase>/` con convención de nombres; `metadata.json` por documento sin confianza ni versión del prompt. | ⏳ falta la confianza y las versiones |
| F14 Producción y mejora continua | 1 | 1 | 🔴 | Corre en producción con un cron; sin regresión, tablero ni rollback. | ⏳ falta la regresión, el tablero y el rollback |

Resumen: 🟢 1 · 🟡 3 · 🔴 8 · ⚪ 1 · — 1. F06 cuenta como «?», no como 0.

## Fortalezas

Todas las fases en nivel 3 o 4:

1. **F02 Taxonomía y esquema (nivel 3).** Las 9 clases y OTRO tienen definición, ejemplo y contraejemplo, y el
   esquema ya valida las salidas. Es la base de la guía de anotación de F04.

F11 y F13 (nivel 2) van bien encaminadas, pero todavía no son fortalezas: les falta medir.

## Las 3 brechas de mayor riesgo

El riesgo es cuántas fases dependen de la fase, directa o indirectamente; el esfuerzo, cuántos niveles le faltan
para llegar a 3.

| # | Brecha | Riesgo | Esfuerzo | Qué se arriesga |
|---|---|---|---|---|
| 1 | **F01: no hay KPI.** Nadie escribió la meta ni el error tolerado. | 13 | 2 | Ninguna evaluación ni umbral tiene contra qué compararse: F09 y F10 no pueden pasar. |
| 2 | **F03: split al azar y PII sin tratar.** Documentos de la misma licitación están en train y en test. | 11 | 2 | El 0,91 está inflado por la fuga, y hay RUT y correos que llegan al proveedor del modelo. |
| 3 | **F04: nadie anotó.** Las etiquetas salieron del mismo modelo. | 10 | 3 | Sin acuerdo entre personas no hay golden firme, y todo lo que se mide después hereda el error. |

## Plan de mejora

Las fases bajo nivel 3, de mayor a menor riesgo ÷ esfuerzo, sin poner una antes de otra del plan de la que
depende:

| Orden | Fase | Nivel | Riesgo ÷ esfuerzo | Qué hacer |
|---|---|---|---|---|
| 1 | F01 Problema y KPI | 1 | 13 ÷ 2 | Escribir en problema.md el KPI con su métrica, su meta y su error tolerado, y lo que queda fuera. |
| 2 | F03 Corpus, limpieza y split | 1 | 11 ÷ 2 | Rehacer el split por licitación, anonimizar RUT y correos (con su propia corrida) y congelar el test con su hash. |
| 3 | F04 Anotación y acuerdo | 0 | 10 ÷ 3 | Escribir la guía desde `taxonomia.yaml` y anotar con dos personas al menos 50 ejemplos comunes hasta κ ≥ 0.8. |
| 4 | F05 Golden set | 0 | 9 ÷ 3 | Congelar el golden v1 con el sha256 de cada ejemplo, revisado por 2 personas. |
| 5 | F11 Equipo de agentes | 2 | 3 ÷ 1 | Una métrica por agente y una corrida de punta a punta sobre golden v1. |
| 6 | F07 Método | 1 | 5 ÷ 2 | Comparar few-shot, zero-shot y otro modelo en métrica, costo por caso y latencia. |
| 7 | F09 Evaluación | 2 | 3 ÷ 1 | Medir sobre golden v1 con id de corrida, contra el baseline y el KPI. |
| 8 | F10 Calibración y umbral | 0 | 2 ÷ 3 | Sumar la confianza a cada salida, trazar la curva y mandar a revisión lo que quede bajo el umbral. |
| 9 | F13 Salidas, metadata y trazabilidad | 2 | 1 ÷ 1 | Sumar la confianza y las versiones a `metadata.json`. |
| 10 | F12 Dependencias y contratos | 1 | 0 ÷ 2 | Validar `EstadoLicitacion` al entrar y al salir, y medir las relaciones «aclara a». |
| 11 | F14 Producción y mejora continua | 1 | 0 ÷ 2 | Regresión antes de desplegar, tablero, alertas y un rollback probado. |

F11 (3 ÷ 1) va antes que F09 (3 ÷ 1) porque F09 depende de F07, que todavía no está; F12 y F14 empatan en 0 y va
primero la que va antes en el orden.

**Por averiguar:** F06 Baseline («?»). Nadie supo decir si se comparó con algo simple; F09 depende de ella, así que
hay que averiguarlo antes del paso 7.

**Fuera del plan:** F08 («—» propuesto, espera que la persona lo decida con su motivo).

## Progreso desde el diagnóstico del 2026-10-15

| Fase | Antes | Ahora | Por qué |
|---|---|---|---|
| F02 Taxonomía y esquema | 1 | 3 | Se escribió `taxonomia.yaml` v2 con contraejemplos y la clase OTRO, y el esquema validó una corrida. |
| F11 Equipo de agentes | 1 | 2 | Se escribió `agentes.md` con roles y herramientas. |

Ninguna fase bajó de nivel. Las demás siguen igual; F06 sigue sin evidencia («?»).

## En la bitácora

El diagnóstico quedó registrado como una entrada `tipo: diagnostico` con fecha, quién lo hizo (`tutor`, y la
persona entrevistada por su usuario del equipo), el commit leído
(`4e1a9c2`) y este archivo citado por ruta y hash. No copia ningún documento ni dato personal.
