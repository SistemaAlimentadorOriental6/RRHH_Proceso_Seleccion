# Plan de acción: fluidez de la web de RRHH (vacantes, análisis en vivo y avatar)

> Fecha: 2026-10-26 · Alcance: `go-rrhh/` (API, base de datos, SSE, proceso automático) y `frontend/` (almacén, vistas, avatar).
> Este documento es solo diagnóstico y plan; no se cambió código.

---

## 1. Resumen ejecutivo

Lo que siente el usuario tiene **cinco causas principales**. Ninguna es "el servidor es lento" en general: son decisiones de diseño que se suman.

| # | Causa | Síntoma que produce | Gravedad |
|---|-------|---------------------|----------|
| C1 | **El navegador se queda sin conexiones** (máximo 6 por servidor en HTTP/1.1). Cada pestaña deja abiertas 1–2 conexiones SSE para siempre, más una petición de conteo que puede durar minutos, más los módulos de Vite en modo dev. | Con 2–3 pestañas abiertas las peticiones nuevas quedan en "Stalled" y **nada carga hasta recargar** (y a veces ni así, porque las otras pestañas siguen ocupando las conexiones). | Crítica |
| C2 | **La lista de vacantes depende de Chrome en vivo.** Si la caché tiene más de 2 minutos y no hay análisis en curso, `/api/computrabajo/vacantes` abre Chrome, inicia sesión y cuenta inscritos; y si el proceso automático está contando en ese momento, **espera en la cola del semáforo `semCT`** hasta que termine. | "Ver las vacantes es demasiado lento" (segundos a minutos), sobre todo al arrancar el servidor y entre análisis. | Crítica |
| C3 | **Sondeo agresivo que satura la base de datos.** El frontend pregunta `/api/jobs` cada 2 s por pestaña y, sin análisis, **relee todos los candidatos de todas las vacantes cada 4 s**. Cada relectura son ~8 consultas a la MariaDB remota + leer del disco el texto del perfil de cada candidato (que luego se descarta). El pool tiene solo 10 conexiones. | Todo el backend se vuelve lento, incluido el guardado de cada candidato, que **retrasa el evento en vivo**. | Alta |
| C4 | **Los candidatos ya analizados hacen fila detrás de los nuevos.** Con concurrencia 1, una persona guardada (que solo hay que "confirmar") espera a que termine el análisis lento (captchas de ADRES/RUNT/SIMIT) de quien va delante. | Tarjetas "En cola" durante minutos aunque ya tengan resultado; "no muestra a las personas hasta luego". | Alta |
| C5 | **El frontend reprocesa todo en cada evento.** Al unirse a un análisis se repiten miles de eventos uno por uno; cada uno dispara ordenamientos, animaciones `flip` sobre todas las tarjetas y el efecto del avatar que recorre a todos los candidatos. Hay un avatar con su propio bucle por cada tarjeta de vacante. | Congelamientos al entrar/recargar, tirones del avatar, animaciones que "saltan". | Media-alta |

**Orden recomendado:** Fase 0 (medir, medio día) → Fase 1 (arreglos rápidos, 2–3 días, resuelven la mayoría del dolor) → Fase 2 (cambios estructurales, 1–2 semanas) → Fase 3 (pulido de UX y avatar) → Fase 4 (pruebas de carga y regresión).

---

## 2. Cómo funciona hoy (flujo real)

```
Navegador (Vite dev :5173, HTTP/1.1)  ──proxy──►  go-rrhh :8080  ──►  MariaDB 192.168.90.32 (pool 10)
                                                       │
                                                       └──► Chrome (1 solo, semáforo semCT) ──► Computrabajo
```

Por cada pestaña abierta en la sección de vacantes:

1. `GET /api/computrabajo/vacantes?contar=0` y luego `GET /api/computrabajo/vacantes` (con conteo) — `almacenAnalisis.svelte.ts:943` y `:948`.
2. `GET /api/computrabajo/vacantes/{id}/candidatos` **por cada vacante visible**, en paralelo — `VistaVacantesEnVivo.svelte:282-285`, `VistaDescartados.svelte:26-28`.
3. `EventSource /api/cambios` permanente — `almacenAnalisis.svelte.ts:833`.
4. `setInterval` cada 2 s → `GET /api/jobs` — `almacenAnalisis.svelte.ts:822`.
5. Si no hay análisis: cada 4 s → **repetir el paso 2 para todas las sesiones** — `almacenAnalisis.svelte.ts:890-893`.
6. Si hay análisis: `EventSource /api/jobs/{id}/eventos` permanente que **repite todo lo ocurrido** desde el inicio — `servidor/analisis.go:504-576`.

En el servidor, en paralelo:

- Proceso automático: cada 1 minuto cuenta los inscritos de todas las vacantes con Chrome (`correo.go:241-271`), ocupando `semCT` todo ese tiempo, y lanza análisis.
- Trabajador de reanudaciones que también toma `semCT` (`reanudacion.go:134`).

---

## 3. Diagnóstico detallado (con evidencia)

### C1 — Límite de 6 conexiones por servidor en el navegador

- El frontend se sirve con `vite` en modo desarrollo y el proxy `/api` (`frontend/vite.config.ts`). Es HTTP/1.1 sin TLS: **Chrome permite 6 conexiones simultáneas por host:puerto, compartidas entre todas las pestañas**.
- Conexiones que quedan ocupadas indefinidamente por pestaña:
  - `/api/cambios` (SSE) — siempre, desde que se crea la primera sesión (`almacenAnalisis.svelte.ts:800` → `escucharCambios`).
  - `/api/jobs/{id}/eventos` o `/api/computrabajo/vacantes/{id}/analizar` (SSE) — mientras hay análisis.
- Conexiones que duran minutos: `/api/computrabajo/vacantes` con conteo (plazo de hasta `6 min × cuentas`, `computrabajo.go:263`).
- En modo dev, además, cada recarga pide decenas de módulos ES sueltos, que compiten por esas mismas 6 conexiones.
- **Resultado:** con 3 pestañas hay 6 SSE abiertos y cualquier `fetch` nuevo queda en "Stalled" para siempre. Recargar una pestaña libera solo sus conexiones; por eso "hay que recargar varias veces".

**Cómo confirmarlo:** DevTools → Network → columna *Timing* de una petición lenta: si la mayor parte es "Stalled/Queueing", es esto. También `chrome://net-internals/#sockets`.

### C2 — Lista de vacantes atada a Chrome

- `ManejarVacantesComputrabajo` (`computrabajo.go:233-315`) solo responde rápido si:
  - hay un análisis en curso (usa la caché), **o**
  - la caché tiene menos de `vigenciaListaFresca = 2 min` (`analisis.go:32`) **y** (`contar=0` **o** la caché trae conteos).
- En cualquier otro caso llama a `consultarVacantesComputrabajo` (`computrabajo.go:84`), que:
  1. **Espera el semáforo `semCT`** (`computrabajo.go:86-91`). Lo pueden tener: el conteo del proceso automático (cada minuto, `correo.go:246`), la primera lectura al arrancar (`correo.go:104`), una reanudación (`reanudacion.go:134`) o el precalentamiento.
  2. Abre Chrome / inicia sesión / lee las vacantes de **todas las cuentas**.
  3. Si `contar=true`, abre una pestaña por vacante para contar inscritos.
- Al **arrancar el servidor** la caché está vacía y el proceso automático está contando con `semCT` tomado: la primera visita de la pantalla espera minutos.
- Entre análisis, la caché puede tener más de 2 min (el ciclo es 1 min de espera + la duración del conteo + la duración de los análisis), así que la pantalla cae a la ruta lenta de forma intermitente.
- El frontend hace **dos** llamadas seguidas (`contar=0` y luego con conteo) — `almacenAnalisis.svelte.ts:943-949`. La segunda casi siempre vuelve a la ruta lenta si la caché no trae conteos.

### C3 — Sondeo que satura la base de datos

**Frontend:**
- `setInterval(() => this.seguirAnalisisEnCurso(), 2000)` — `almacenAnalisis.svelte.ts:822`. Por pestaña, 30 peticiones/min a `/api/jobs`.
- Sin análisis en curso, cada 4 s: `this.sesiones.forEach((s) => void s.refrescar())` — `almacenAnalisis.svelte.ts:890-893`. Con 6 vacantes abiertas son **90 lecturas completas por minuto por pestaña**.
- `refrescar` compara cada candidato con `JSON.stringify` completo (`almacenAnalisis.svelte.ts:314-332`) y reemplaza objetos, lo que dispara re-render y animaciones.

**Backend, por cada `GET /api/computrabajo/vacantes/{id}/candidatos`** (`servidor/analisis.go:655-684`):
1. Validar sesión: 1 consulta (`autenticacion.go:52`, `usuarios.go:179`) — esto ocurre en **toda** petición, también en el sondeo de `/api/jobs`.
2. `RepositorioMySQL.ofertas()` → agregado `GROUP BY` sobre todas las postulaciones (`repositorio_mysql.go:140`, `leer.go:26-29`) + un `os.Stat`.
3. `SELECT id FROM ofertas` (`leer.go:52`).
4. `candidatosDonde`: 5 consultas (postulaciones+perfil+evaluación con subconsulta correlacionada, motivos, validaciones con `GROUP BY`, IA, archivos) (`leer.go:143-271`).
5. **Lee del disco el `texto_perfil` de cada candidato** (`leer.go:392-395`) y **luego se descarta** con `sinTextoPerfil` (`analisis.go:683`).
6. Descifra la cédula (y fecha de nacimiento) de cada fila; deserializa el JSON de cada validación.

Son ~8 viajes a una MariaDB remota + N lecturas de disco, con un pool de **10 conexiones** (`almacen.go:71`). Con varias pestañas, el pool se agota y las peticiones hacen fila — incluida la escritura de cada candidato del análisis (`orquestador/persistencia.go:25-31`), que ocurre **antes** de emitir `candidato.completo` (`orquestador/analisis.go:818` → `:836`). Es decir: **el sondeo retrasa el evento en vivo**.

Otros endpoints con el mismo patrón:
- `ManejarVacanteCandidato` (`vacantes.go:109-132`): `ResolverOferta` (agregado) + `buscarCandidato` (agregado + carga completa) + `ObtenerRankingOferta` (agregado + carga completa **otra vez** + JSON del ranking del disco). Para mostrar **una** persona carga la vacante entera dos veces.
- `reintentarPendientes` del proceso automático carga todas las vacantes completas (`correo.go:212`).

Índices: `postulaciones` tiene `(oferta_id, dictamen)` pero las consultas filtran por `(oferta_id, activa)`; el agregado de `OfertasProcesadas` recorre todas las postulaciones.

### C4 — Los guardados esperan detrás de los nuevos

- `Analizar` mete a todos en **una sola cola** con `Concurrencia: 1` (`orquestador/analisis.go:378-412`, `servidor/analisis.go:266`).
- Un candidato con resultado válido (`t.previo != nil`) solo necesita `reutilizar` (instantáneo salvo la IA), pero hace fila detrás de un candidato nuevo que puede tardar minutos (perfil en Chrome + captchas).
- En pantalla: `candidato.descubierto` llega enseguida (tarjeta "En cola"), pero `candidato.completo` del guardado llega mucho después.
- Agravante en el frontend: si la sesión se unió al análisis **antes** de cargar los guardados (`cargarGuardados` sale si `this.activo`, `almacenAnalisis.svelte.ts:280`), `conservados` está vacío y esas personas aparecen como "En cola" sin su resultado, aunque la base ya lo tiene.

### C5 — El frontend hace demasiado trabajo por evento

- **Repetición completa al unirse/recargar:** el servidor manda todos los eventos desde el inicio del job (`servidor/analisis.go:537-543`; hasta 200 000 en memoria, `maxEventosJob`). `EventSource` despacha cada evento como una tarea separada, así que Svelte recalcula y repinta **por cada evento**. Con 200 candidatos × ~15 eventos son ~3 000 actualizaciones seguidas.
- Por cada actualización:
  - `SesionAnalisis.candidatos` reordena todo (`almacenAnalisis.svelte.ts:253-259`).
  - `SesionGrupo.candidatos` es un **getter sin memoria**: `flatMap + filter + sort` en cada acceso (`almacenAnalisis.svelte.ts:677-685`), y se accede desde muchos sitios (vista, avatar, `porSlug`, `resumen`…).
  - `candidatosVisibles` vuelve a filtrar y ordenar (`VistaVacantesEnVivo.svelte:205-239`) y `candidatosCola` ordena otra vez (`:305-307`).
  - `animate:flip` de 900 ms sobre **todas** las tarjetas de cola y resultados (`VistaVacantesEnVivo.svelte:946`, `:978`) → mide cada tarjeta en cada reordenamiento.
  - El efecto "fotos" del avatar recorre todos los candidatos × 10 etapas (`AvatarAnalisis.svelte:250-297`) y, durante la repetición, **reacciona a eventos viejos** (frases y sonidos en ráfaga) porque no mira `repitiendo`.
- **Avatar:**
  - Se crea **un avatar por tarjeta de vacante** (`VistaVacantesEnVivo.svelte:498`), cada uno con su `requestAnimationFrame`, un `setInterval` de 1 s con `querySelector` (`AvatarAnalisis.svelte:771`), un listener `pointermove` global que lo despierta en cada movimiento del mouse (`:769`, `:755-759`) y un temporizador de paseo (`:650-665`).
  - Cada cuadro hace varias lecturas de layout: `ancla.getBoundingClientRect()`, `areaVisible()`, `elegirObjetivo()` (hasta 4 `querySelector`), `tarjeta.getBoundingClientRect()`, `globoEl.offsetWidth/offsetHeight` (`:683-737`). Mientras las tarjetas se animan con `flip`/`fly`, eso fuerza recálculos de layout en cada cuadro → tirones.
- **Carga de guardados en paralelo:** en el listado se piden los candidatos de **todas** las vacantes visibles a la vez solo para pintar los contadores de cada tarjeta (`VistaVacantesEnVivo.svelte:282-297`).

### Hallazgos menores

- `ResponderJSON` serializa todo a un `bytes.Buffer` antes de escribir (`controlador.go:166-177`): con listas grandes duplica memoria; aceptable, pero no ayuda.
- `candidato.completo` lleva el candidato completo con todos los `resultado_*` (RUNT, SIMIT…): eventos de varios KB que se guardan todos en memoria y se repiten en cada reconexión.
- `listarCandidatosGuardados` no usa paginación aunque el backend la soporta en `/api/vacantes/{slug}/candidatos`.
- `retratoIniciales` genera un SVG por candidato; barato, pero se regenera al reconstruir objetos en `refrescar`.

---

## 4. Plan de acción

Formato de cada tarea: **qué** · dónde · esfuerzo (S ≤ ½ día, M ≤ 2 días, L > 2 días) · criterio de aceptación.

### Fase 0 — Medir antes de tocar (½ día)

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 0.1 | Registrar una línea base: tiempo de `/api/computrabajo/vacantes` (frío, caliente, durante conteo automático), `/candidatos` por vacante, y "fin de candidato en backend → tarjeta en pantalla". El log `conRegistro` ya imprime la duración de cada petición (`middleware.go:95-105`). | logs del servidor | S | Tabla con p50/p95 de cada caso. |
| 0.2 | Reproducir C1: abrir 3 pestañas en la sección de vacantes durante un análisis y capturar en DevTools el *Timing* de una petición atascada. | navegador | S | Captura con "Stalled" > 10 s. |
| 0.3 | Activar `slow_query_log` en MariaDB (umbral 200 ms) durante una hora de uso normal. | MariaDB | S | Lista de consultas lentas. |
| 0.4 | Grabar un perfil en DevTools → Performance al recargar con un análisis en curso (repetición de eventos). | navegador | S | Duración total de *long tasks* al entrar. |
| 0.5 | Añadir `net/http/pprof` detrás de una variable de entorno (solo local) para ver goroutines bloqueadas en `semCT` y en el pool de la base. | `main.go` | S | `/debug/pprof/goroutine` accesible en local. |

### Fase 1 — Arreglos rápidos de alto impacto (2–3 días)

**1A. Conexiones (C1)**

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 1.1 | **Servir el frontend compilado**, no `vite dev`: `pnpm build` y que Go sirva `frontend/dist` (con `http.FileServer` + fallback a `index.html` para las rutas del SPA), o un nginx delante. Elimina las decenas de módulos sueltos por recarga y el proxy de Vite. | `servidor/servidor.go` (ruta `/` hoy devuelve 404 JSON) | M | Recarga completa < 1 s con 1 sola petición de JS/CSS. |
| 1.2 | **Activar HTTP/2** (TLS): con HTTP/2 todas las peticiones viajan multiplexadas en una conexión y el límite de 6 desaparece. Opción simple: nginx/Caddy con certificado interno delante de `:8080`; o `ListenAndServeTLS` en Go (HTTP/2 viene incluido). | despliegue / `servidor.go:146` | M | `chrome://net-export` muestra `h2`; 5 pestañas abiertas sin peticiones "Stalled". |
| 1.3 | Mientras 1.2 no esté: **una sola conexión SSE por navegador**, no por pestaña. Elegir una pestaña "líder" con `BroadcastChannel` + `navigator.locks` (o un `SharedWorker`) que abre `/api/cambios` y reenvía los eventos a las demás. | `almacenAnalisis.svelte.ts:818-881` | M | Con 4 pestañas, en `chrome://net-internals` hay 1 SSE, no 4–8. |

**1B. Lista de vacantes instantánea (C2)**

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 1.4 | **Responder siempre desde la caché si existe (aunque esté vieja)** y refrescarla en segundo plano (patrón *stale-while-revalidate*). La petición HTTP **nunca** espera a `semCT`. Añadir un "single-flight": si ya hay un refresco en curso, no lanzar otro. Devolver en una cabecera la edad de la lista (`X-Vacantes-Edad`) para que la pantalla muestre "actualizado hace X". | `computrabajo.go:241-281` | M | `/api/computrabajo/vacantes` < 200 ms p95 en todos los casos salvo el primer arranque sin caché. |
| 1.5 | Al terminar un refresco, avisar por `/api/cambios` con `event: vacantes` (el `difusor` ya existe, `cambios.go:43`). El frontend vuelve a pedir la lista (ahora instantánea) al recibirlo, en vez de esperar con la petición abierta. | `computrabajo.go:120-122`, `almacenAnalisis.svelte.ts:832-881` | S | El conteo de inscritos se actualiza solo, sin peticiones colgadas de minutos. |
| 1.6 | **Persistir la última lista** (con conteos y fecha) en un archivo JSON o en la tabla `ofertas`, y cargarla al arrancar el servidor. Así el primer acceso tras un reinicio también es inmediato. | `controlador.go` (al crear), `analisis.go:209-214` | S | Tras reiniciar el servidor, la lista aparece en < 300 ms. |
| 1.7 | Frontend: hacer **una sola** llamada a la lista (quitar la doble `contar=0` + `contar=1`) ya que el backend responde de caché; mostrar "actualizando conteos…" mientras llega el aviso de 1.5. | `almacenAnalisis.svelte.ts:935-963` | S | Una petición por carga de pantalla. |

**1C. Quitar el sondeo y aligerar las lecturas (C3)**

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 1.8 | **Avisar el inicio y fin de cada análisis por `/api/cambios`** (`job.iniciado` con `id`, `oferta_id`, `titulo`; `job.terminado`). Emitirlo en `iniciarJobAnalisis` y en `terminar`. Eliminar el `setInterval` de 2 s; dejar solo una consulta a `/api/jobs` al abrir la conexión SSE (para ponerse al día) y en `visibilitychange`. | `servidor/analisis.go:307-325`, `:111-118`; `almacenAnalisis.svelte.ts:822`, `:883-901` | M | En reposo, 0 peticiones/min por pestaña (verificable en Network). |
| 1.9 | **Eliminar el refresco cada 4 s.** Refrescar solo cuando el backend avise que esa vacante cambió: emitir `cambio` con `oferta_id` al guardar un candidato fuera de un análisis (reanudación, formulario) y al terminar un job. El frontend refresca **solo** esa sesión. | `almacenAnalisis.svelte.ts:888-894`, `cambios.go:38-40`, `reanudacion.go`, `datos_faltantes.go` | M | Los cambios llegan a la pantalla en < 2 s sin sondeo. |
| 1.10 | **No leer `texto_perfil` del disco en los listados.** Agregar un parámetro (p. ej. `conTexto bool`) a `candidatosDonde`/`CandidatosDeOferta`, y leer el archivo solo cuando la API lo pide (`incluir_cv=1`, exportar, IA). | `almacen/leer.go:247-270`, `:392-395`; `repositorio_mysql.go:157` | S | El listado no abre ningún archivo `texto_perfil` (verificable con Process Monitor). |
| 1.11 | **Caché en memoria de `OfertasProcesadas`** (5–10 s, o invalidada al guardar un candidato) para no repetir el `GROUP BY` en cada petición. `ObtenerCandidatosOferta` y `ResolverOferta` la usan. | `repositorio_mysql.go:60-89` | S | 1 agregado cada pocos segundos como máximo, no 1 por petición. |
| 1.12 | **Caché corta de sesiones** (token → usuario, 30–60 s) para no ir a la base en cada petición; invalidar en logout. | `autenticacion.go:47-58` | S | Las peticiones de lectura no consultan `sesiones` más de 1 vez/min por usuario. |
| 1.13 | Subir el pool a 20–25 conexiones y `SetMaxIdleConns` igual al máximo (evita abrir/cerrar conexiones TCP a la base remota). Revisar `max_connections` de MariaDB antes. | `almacen/almacen.go:71-73` | S | En pprof no hay goroutines esperando `database/sql.(*DB).conn`. |
| 1.14 | Índice `postulaciones (oferta_id, activa)` en una migración 007 repetible (`ADD KEY IF NOT EXISTS`). | `almacen/migraciones/007_indices.sql` | S | `EXPLAIN` del agregado y de `candidatosDe` usa el índice. |

**1D. Guardados primero (C4)**

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 1.15 | **Separar la fila de reutilizados de la fila de nuevos.** En el callback del listado, si `t.previo != nil`, procesarlo en un trabajador aparte (o en línea si no requiere IA) en vez de ponerlo en la misma cola. Los nuevos siguen con concurrencia 1. Mantener el orden de `Indice` en el resumen final. | `orquestador/analisis.go:378-412` | M | Al reanalizar una vacante, todos los ya analizados aparecen como completos en < 5 s desde que se leen del listado. |
| 1.16 | Frontend: cargar los guardados **antes** de unirse a un análisis (`unirse` espera a `cargarGuardados`), y permitir `cargarGuardados` aunque la sesión esté activa (fusionando por `url_detalle`). | `almacenAnalisis.svelte.ts:279-302`, `:390-398`, `:896-897` | S | Al recargar en mitad de un análisis, no aparece ninguna persona ya analizada como "En cola". |

### Fase 2 — Cambios estructurales (1–2 semanas)

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 2.1 | **Instantánea en vez de repetición.** Al conectarse a un job, enviar primero un evento `estado` con la foto actual (lista de candidatos con su fase y etapas, y el resumen) y después solo los eventos nuevos. Mantener `Last-Event-ID` para reconexiones cortas. Reducir lo que se guarda por job (no hace falta el historial completo si existe la foto). | `servidor/analisis.go:49-150`, `:504-576`; `almacenAnalisis.svelte.ts:400-468` | L | Unirse a un análisis con 300 candidatos transfiere < 300 KB y pinta en < 500 ms. |
| 2.2 | **Endpoint de resumen por vacante** (`postulados`, `aptos`, `rechazados`, `pendientes`) a partir de `OfertasProcesadas` (ampliar con conteo de pendientes). Las tarjetas del listado usan esto y **no** cargan los candidatos de cada vacante. | `vacantes.go`, `almacen/leer.go:25-46`, `VistaVacantesEnVivo.svelte:282-297` | M | Abrir el listado de vacantes hace 1 petición de datos, no 1 + N. |
| 2.3 | **Lectura incremental:** `GET .../candidatos?desde=<timestamp>` que devuelva solo postulaciones con `actualizado_en` posterior, más `ETag`/`If-None-Match` para responder `304` cuando nada cambió. | `servidor/analisis.go:655-684`, `almacen/leer.go` | M | Un refresco sin cambios devuelve 304 en < 50 ms. |
| 2.4 | **Emitir el evento antes de persistir, o persistir en segundo plano.** Hoy `candidato.completo` espera la escritura en la base y los JSON de aptos. Opción segura: emitir `candidato.completo` en cuanto hay resultado y luego `candidato.guardado` (o `candidato.etapa guardado/error`) cuando termine la escritura; el frontend ya muestra errores de la etapa "guardado". | `orquestador/analisis.go:805-844` | M | Latencia "resultado → tarjeta" < 300 ms aun con la base lenta. |
| 2.5 | `ManejarVacanteCandidato`: buscar por slug/cédula con una consulta directa (por `slug_persona`, ya indexado en 002) en vez de cargar la vacante completa dos veces. | `vacantes.go:93-132`, `almacen/` | M | Detalle de una persona < 100 ms. |
| 2.6 | Separar la lectura de vacantes de Computrabajo (lista) del conteo de inscritos: conteos en segundo plano por vacante, con resultados parciales publicados por `/api/cambios` a medida que llegan. | `computrabajo.go:183-214` | M | La lista aparece al instante y cada conteo se completa por separado. |
| 2.7 | Reducir el tamaño del payload de `candidato.completo` y de los listados: enviar los `resultado_*` completos solo en el detalle de la persona; en listas, solo lo que la tarjeta muestra (veredicto, motivos, etapas, foto, score). | `orquestador/analisis.go:839-844`, `servidor/vacantes.go` | M | Listado de 300 candidatos < 200 KB gzip. |

### Fase 3 — Fluidez en pantalla y avatar (3–5 días, en paralelo con Fase 2)

| ID | Tarea | Dónde | Esf. | Aceptación |
|----|-------|-------|------|------------|
| 3.1 | **Aplicar eventos por lotes**: acumular los eventos SSE en un arreglo y aplicarlos una vez por cuadro (`requestAnimationFrame`) o al recibir `repeticion.fin`. Una sola actualización reactiva por cuadro en vez de una por evento. | `almacenAnalisis.svelte.ts:415-422` | S | Ninguna *long task* > 50 ms al unirse a un análisis grande. |
| 3.2 | Convertir `SesionGrupo.candidatos` y `resumen` en `$derived` (memorizados) en lugar de getters que recalculan en cada acceso. | `almacenAnalisis.svelte.ts:660-726` | S | El perfil de rendimiento ya no muestra `flatMap/sort` repetidos. |
| 3.3 | Índice `slug → candidato` para `porSlug` y para el avatar (`sesion.candidatos.find`) en vez de búsquedas lineales. | `almacenAnalisis.svelte.ts:270-272`, `:728-730`, `AvatarAnalisis.svelte:199` | S | — |
| 3.4 | **Limitar `animate:flip`**: solo con menos de ~40 tarjetas visibles; por encima, sin flip. Reducir la duración a 300–400 ms. Respetar `prefers-reduced-motion`. | `VistaVacantesEnVivo.svelte:946`, `:978` | S | Reordenar 200 tarjetas no baja de 50 fps. |
| 3.5 | **Paginar o virtualizar "Analizados"** (mostrar 30–50 y "ver más", o lista virtual). La fila "Por verificar" ya es horizontal: limitarla a los próximos ~20. | `VistaVacantesEnVivo.svelte:930-990` | M | El DOM de la vista no supera ~100 tarjetas. |
| 3.6 | **Un solo avatar activo.** En el listado de vacantes, montar el avatar solo en la tarjeta de la vacante que se analiza (o uno global que viaja a ella); las demás muestran un icono estático. | `VistaVacantesEnVivo.svelte:494-498` | S | 1 bucle `requestAnimationFrame` como máximo. |
| 3.7 | Avatar: **no despertar con cada movimiento del mouse** si no está visible o no hay análisis (usar `IntersectionObserver` sobre su tarjeta); quitar el vigilante de 1 s y arrancar el bucle solo por eventos de estado. | `AvatarAnalisis.svelte:755-778` | S | Con el mouse moviéndose y sin análisis, 0 cuadros de animación del avatar fuera de pantalla. |
| 3.8 | Avatar: **cachear las medidas** (`getBoundingClientRect` del área y de la tarjeta objetivo) y recalcularlas solo en `scroll`/`resize`/`ResizeObserver` o cuando cambia el objetivo; leer todo antes de escribir `style.transform`. | `AvatarAnalisis.svelte:583-637`, `:677-744` | M | DevTools no muestra "Forced reflow" en los cuadros del avatar. |
| 3.9 | Avatar: **ignorar la repetición** en el efecto de reacciones (no reaccionar mientras `repitiendo` sea verdadero; exponerlo como `$state` público) y limitar las reacciones a 1 cada ~1,5 s. | `AvatarAnalisis.svelte:250-297`, `almacenAnalisis.svelte.ts:240` | S | Al recargar en mitad de un análisis el avatar no dispara ráfagas de frases ni sonidos. |
| 3.10 | Avatar: el efecto de "fotos" debería reaccionar al **evento** (callback desde la sesión: `alEtapa`/`alCompletar` emiten "novedad") en vez de comparar todo el estado de todos los candidatos en cada cambio. | `AvatarAnalisis.svelte:246-297`, `almacenAnalisis.svelte.ts:556-628` | M | Costo por evento O(1) en el avatar. |
| 3.11 | Estados de carga claros: esqueletos en tarjetas de vacantes y candidatos, "actualizado hace X" en la lista, y un indicador de conexión en vivo (conectado / reconectando) visible. Hoy un estado "colgado" no se distingue de "no hay datos". | `VistaVacantesEnVivo.svelte:440-490` | M | El usuario siempre ve si está cargando, en vivo o desconectado. |
| 3.12 | Precargar la foto del candidato en cuanto se descubre (si `foto_url` llega en el evento) y fijar `width/height` en `<img>` para evitar saltos de layout. | `VistaVacantesEnVivo.svelte:734`, `:795` | S | CLS ≈ 0 al aparecer tarjetas. |

### Fase 4 — Verificación y no-regresión (2–3 días)

| ID | Tarea | Esf. | Aceptación |
|----|-------|------|------------|
| 4.1 | Repetir las mediciones de la Fase 0 y comparar. | S | Cumplir las metas de la sección 5. |
| 4.2 | Prueba de 5 pestañas abiertas durante un análisis completo de una vacante grande (≥ 200 postulados). | S | Ninguna petición "Stalled"; todas las pestañas ven cada candidato en < 1 s. |
| 4.3 | Pruebas Go: `ManejarVacantesComputrabajo` responde de caché aunque `semCT` esté tomado (simular con la fuente falsa de `servidor_test.go`); `Analizar` emite los reutilizados antes que el nuevo lento. | M | Pruebas nuevas en `servidor/` y `orquestador/` pasan con `go test ./...`. |
| 4.4 | Prueba de reinicio del servidor en mitad de un análisis: la pantalla se reconecta sola y muestra el estado correcto sin recargar. | S | Sin recargas manuales. |
| 4.5 | Revisar en `slow_query_log` que no queden consultas > 200 ms en uso normal. | S | Log vacío o justificado. |

---

## 5. Metas medibles

| Indicador | Hoy (estimado, confirmar en Fase 0) | Meta |
|-----------|-------------------------------------|------|
| Abrir lista de vacantes | de 1 s a varios minutos | < 300 ms siempre (< 1 s tras reinicio) |
| Candidatos guardados de una vacante | 1–5 s, peor con varias pestañas | < 500 ms; refresco sin cambios < 50 ms (304) |
| Candidato terminado en backend → tarjeta en pantalla | variable, a veces "nunca" sin recargar | < 1 s |
| Reanalizar vacante: ya analizados visibles como completos | minutos (detrás de los nuevos) | < 5 s |
| Peticiones por minuto por pestaña en reposo | ~30 a `/api/jobs` + ~15×N relecturas | 0 (solo SSE) |
| Conexiones abiertas con 4 pestañas | 4–8 SSE (bloquea el navegador) | 1 (o HTTP/2) |
| Unirse a un análisis en curso | segundos congelado + ráfaga del avatar | < 500 ms, sin ráfagas |
| Animación del avatar | tirones al reordenar | 60 fps, sin "Forced reflow" |

---

## 6. Riesgos y cuidados

- **Caché "vieja" de vacantes (1.4):** la pantalla puede mostrar por unos segundos una vacante ya cerrada. Mitigar mostrando la edad de la lista y refrescando en segundo plano; el análisis igual valida la vacante en vivo (`resolverOferta`).
- **Emitir antes de persistir (2.4):** si la escritura falla, la pantalla debe enterarse (evento de error de la etapa "guardado"); no ocultar ese error.
- **Separar la cola de reutilizados (1.15):** `acum` y el resumen se tocan desde dos trabajadores; ya usan mutex (`contar`), pero revisar `acumuladorResultados.agregar` y el orden final por `Indice`.
- **SSE único entre pestañas (1.3):** si la pestaña líder se cierra, otra debe tomar el relevo (`navigator.locks` lo resuelve).
- **HTTP/2 con TLS (1.2):** el certificado debe ser de confianza en los equipos del equipo de RRHH; la cookie de sesión pasa a `Secure` automáticamente (`autenticacion.go:68`).
- **Servir `dist` desde Go (1.1):** mantener CORS para desarrollo; las rutas del SPA deben devolver `index.html` y las de `/api/` seguir devolviendo 404 JSON.
- **Cambios sin confirmar en el repositorio:** hay modificaciones en curso en `AvatarAnalisis.svelte`, `almacenAnalisis.svelte.ts`, `descarteAnimado.svelte.ts`, `VistaVacantesEnVivo.svelte`, `almacen/*.go` y `servidor/analisis.go`. Confirmarlas o guardarlas antes de empezar este plan para no mezclar cambios.

---

## 7. Orden sugerido de trabajo (lista de tareas)

1. [ ] 0.1–0.5 Medir línea base.
2. [ ] 1.1 Servir `dist` desde Go + 1.2 HTTP/2 (o 1.3 SSE único si HTTP/2 tarda).
3. [ ] 1.4 + 1.5 + 1.6 + 1.7 Lista de vacantes desde caché con refresco en segundo plano.
4. [ ] 1.8 + 1.9 Quitar sondeo de 2 s y 4 s; avisos por `/api/cambios`.
5. [ ] 1.10 + 1.11 + 1.12 + 1.13 + 1.14 Aligerar lecturas de la base.
6. [ ] 1.15 + 1.16 Guardados primero.
7. [ ] 3.1 + 3.2 + 3.6 + 3.9 Frontend por lotes, getters memorizados, un solo avatar, sin ráfagas.
8. [ ] 2.1 Instantánea en vez de repetición.
9. [ ] 2.2 + 2.3 Resumen por vacante e incremental/304.
10. [ ] 2.4 + 2.5 + 2.6 + 2.7 Latencia del evento, detalle directo, conteos parciales, payload liviano.
11. [ ] 3.3–3.5, 3.7, 3.8, 3.10–3.12 Pulido de animaciones y estados de carga.
12. [ ] 4.1–4.5 Verificación final.

Los pasos 2 a 6 (Fase 1) deberían eliminar casi por completo el "tengo que recargar varias veces" y la lentitud de la lista de vacantes; el resto lleva la experiencia a fluida de verdad.

---

## 8. Estado de implementación (2026-10-27)

| Tarea | Estado | Dónde |
|-------|--------|-------|
| 1.1 Frontend compilado servido por Go | Hecho | `servidor/servidor.go` (`RRHH_FRONTEND_DIR`, por defecto `../frontend/dist`) |
| 1.2 HTTPS/HTTP2 opcional | Hecho (falta el certificado) | `RRHH_TLS_CERT` / `RRHH_TLS_KEY` |
| 1.3 Conexiones en vivo | Hecho de otra forma: la pestaña oculta >15 s suelta sus streams y al volver retoma desde el último evento | `almacenAnalisis.svelte.ts` (`pausar`/`reanudar`) |
| 1.4–1.7 Lista de vacantes al instante | Hecho | `servidor/computrabajo.go` (`refrescarVacantes`), `analisis.go` (lista en disco), `clienteRrhh.ts` |
| 1.8–1.9 Sin sondeo de 2 s y 4 s | Hecho | evento `job` en `/api/cambios`; el frontend relee solo al recibir avisos |
| 1.10 Listados sin `texto_perfil` | Hecho | `almacen/leer.go`, `repositorio_mysql.go`, `controlador.go` |
| 1.11 Caché de vacantes de la base (3 s) | Hecho | `repositorio_mysql.go` |
| 1.12 Caché de sesiones (30 s) | Hecho | `servidor/autenticacion.go` |
| 1.13 Pool 20 conexiones | Hecho | `almacen/almacen.go` |
| 1.14 Índice | Hecho, **falta aplicarlo** | `almacen/migraciones/007_indices_lectura.sql` |
| 1.15 Reutilizados en su propia fila | Hecho | `orquestador/analisis.go` |
| 1.16 Guardados antes de unirse | Hecho | `almacenAnalisis.svelte.ts` (`unirseConGuardados`) |
| 3.1 Eventos por lotes | Hecho | `almacenAnalisis.svelte.ts` (`encolar`) |
| 3.2 `SesionGrupo` memorizado | Hecho | `almacenAnalisis.svelte.ts` |
| 3.4 `flip` solo con ≤ 40 tarjetas | Hecho | `VistaVacantesEnVivo.svelte` (`acomodar`) |
| 3.5 "Analizados" por tandas de 60 | Hecho | `VistaVacantesEnVivo.svelte` |
| 3.6 Un solo avatar | Hecho | `VistaVacantesEnVivo.svelte` (`idConAvatar`) |
| 3.7 Avatar quieto fuera de pantalla | Hecho | `AvatarAnalisis.svelte` (`IntersectionObserver`) |
| 3.9 Avatar ignora la repetición | Hecho | `AvatarAnalisis.svelte` |
| 2.1 Repetición compactada (en vez de foto aparte) | Hecho | `servidor/analisis.go` (`compactar`: con ≥ 50 eventos pendientes, solo descubrimiento + resultado de cada persona, última noticia por etapa de quien sigue en análisis y último resumen; ids intactos para reconectar) |
| 2.2 Resumen por vacante | Hecho | campo `resumen` en `/api/computrabajo/vacantes`; la lista ya no carga los candidatos de cada vacante |
| 2.3 ETag / 304 | Hecho | `ResponderJSONConETag`; `listarCandidatosGuardados` manda `If-None-Match` |
| 2.4 Resultado antes de guardar | Hecho | `orquestador/analisis.go` (`cerrarCandidato`) + evento `candidato.guardado` con el id de la postulación |
| 2.5 Detalle con una sola lectura | Hecho | `vacantes.go` (`buscarEn`, `rankingDeAptos`) |
| 2.6 Conteos parciales | Hecho | `computrabajo.go` (`anotarConteo`, aviso `vacantes` con `parcial`, máx. 1 cada 2 s) |
| 2.7 Payload liviano | No hecho a propósito | las pantallas de detalle y flujo leen los `resultado_*` del evento; recortarlos exige un endpoint de detalle. Con la repetición compactada y gzip el costo ya es bajo |
| 3.3 Índice por slug | Hecho | `porSlug` con `Map` derivado |
| 3.8 Medidas del avatar | Hecho (parcial) | el área visible se mide una vez y se recuerda; la tarjeta que sigue sí se mide cada cuadro porque se mueve |
| 3.10 Avatar por eventos | No hace falta | con los eventos por lotes (3.1) su efecto corre como mucho una vez por cuadro |
| 3.11 Estados visibles | Hecho | indicador «En vivo / Reconectando / En pausa», «Lista actualizada hace X» y botón Actualizar |
| 3.12 Imágenes con tamaño | Hecho | `width`/`height`/`decoding="async"` |
| Fase 0 y Fase 4 (medir antes/después) | Pendiente | requiere el servidor y la base reales |
