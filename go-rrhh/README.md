# Automatización de RRHH en Go (Golang)

Sistema de alto rendimiento para la extracción, validación gubernamental y clasificación de candidatos laborales desde **Computrabajo**, integrando consultas paralelas contra **Procuraduría General de la Nación**, **Lista SARLAFT ONU**, **OFAC (EE. UU.)**, **ADRES (BDUA EPS)**, **RUNT** y **SIMIT (Federación Colombiana de Municipios)**.

---

## 🏛️ Arquitectura del Sistema (Principios SOLID)

El proyecto fue diseñado bajo una arquitectura desacoplada basada en **Agentes Especializados**:

1. **Single Responsibility Principle (SRP)**:
   - Cada agente tiene una única responsabilidad y gestiona una única fuente de datos (`AgenteComputrabajo`, `AgenteSarlaft`, `AgenteOfac`, `AgenteProcuraduria`, `AgenteAdres`, `AgenteRunt`, `AgenteSimit`, `AgenteClasificador`).
2. **Open/Closed Principle (OCP)**:
   - Se definió la interfaz `AgenteValidador` en `agentes/interfaz.go`. Es posible incorporar nuevas fuentes de validación (por ejemplo, Contraloría, Policía o Antecedentes Judiciales) sin modificar el orquestador principal.
3. **Liskov Substitution Principle (LSP)**:
   - Cualquier implementación de `AgenteValidador` puede ser sustituida o ejecutada concurrentemente de forma transparente por el motor.
4. **Interface Segregation Principle (ISP)**:
   - Contratos atómicos y bien definidos que evitan dependencias innecesarias en los agentes.
5. **Dependency Inversion Principle (DIP)**:
   - El motor orquestador interactúa mediante contratos y modelos fuertemente tipados, sin acoplamiento a los detalles de bajo nivel de transporte (HTTP REST vs Chrome DevTools Protocol).

---

## 🚀 Ventajas frente a la versión en Python

| Criterio | Versión Python (Playwright + Threads) | Versión Go (`go-rrhh`) |
| :--- | :--- | :--- |
| **Concurrencia** | `asyncio` limitado por GIL y eventos de greenlet en Playwright. | Goroutines nativas ligeras y canales (`chan`) sin bloqueo de GIL. |
| **Consumo de Memoria** | ~350 MB a 600 MB en ejecuciones paralelas. | ~45 MB a 90 MB. |
| **Tiempo de Arranque** | 3.5 a 5.0 segundos (carga del runtime e imports). | < 50 milisegundos (código máquina compilado). |
| **Resolución OCR RUNT** | Subproceso o microservicio externo dependiente. | OCR directo y cliente HTTP ultrarrápido con reintentos automáticos. |
| **Distribución** | Requiere Python 3.12+, `pip install`, Playwright install. | Un único ejecutable binario autónomo (`rrhh.exe`). |

---

## 📦 Estructura del Proyecto

```text
go-rrhh/
├── agentes/
│   ├── interfaz.go              # Contrato AgenteValidador
│   ├── agente_computrabajo.go   # Login, ofertas, candidatos y PDF
│   ├── agente_sarlaft.go        # Lista ONU en memoria (1.010 sancionados)
│   ├── agente_ofac.go           # Lista Clinton / SDN OFAC (EE. UU.)
│   ├── agente_procuraduria.go   # Solucionador de retos y antecedentes disciplinarios
│   ├── agente_adres.go          # Afiliación EPS en BDUA con detección de CortinaWeb
│   ├── agente_runt.go           # Consulta REST RUNT con resolución de LibreCaptcha
│   ├── agente_simit.go          # Multas y comparendos en Angular SIMIT
│   └── agente_clasificador.go   # Evaluación de 4 filtros y segregación de aptos
├── configuracion/
│   └── configuracion.go         # Variables .env, rutas y EPS autorizadas
├── modelos/
│   └── candidato.go             # Structs fuertemente tipados
├── ocr/
│   └── ocr.go                   # Solucionador OCR para captchas del RUNT
├── orquestador/
│   └── motor.go                 # Pipeline concurrente con semáforos reguladores
├── go.mod                       # Módulos y dependencias Go 1.26
├── main.go                      # CLI y punto de entrada
└── rrhh.exe                     # Binario ejecutable compilado
```

---

## 🎯 Reglas de Negocio para Candidatos Aptos

El `AgenteClasificador` califica y segrega a la carpeta `candidatos_aptos/` únicamente a las personas que cumplan:

1. **Género**: Hombre (determinado por ficha de perfil o análisis onomástico).
2. **EPS**: Afiliación activa en una de las EPS autorizadas (**Sura**, **Nueva EPS**, **Savia Salud**, **Sanitas**).
3. **RUNT**: Antigüedad de registro en el RUNT mayor o igual a **2 años**, con **licencia activa en categoría C2 o C3**.
4. **SIMIT**: **0 multas de tránsito** o comparendos hasta un máximo de **$1.000.000** siempre que cuente con acuerdo de pago vigente.

---

## 💻 Instrucciones de Uso

### 1. Compilación
```powershell
cd C:\Users\daniel.ochoa\Proyectos\PruebaRRHH\go-rrhh
go build -o rrhh.exe .
```

### 2. Procesamiento masivo de ofertas (Concurrencia 20)
```powershell
.\rrhh.exe --antecedentes --sarlaft --ofac --adres --runt --simit --concurrencia 20
```

### 3. Consultas directas individuales
- **SARLAFT ONU**:
  ```powershell
  .\rrhh.exe -validar-sarlaft "Juan Perez"
  ```
- **Procuraduría**:
  ```powershell
  .\rrhh.exe -validar-cedula "80766779"
  ```
- **OFAC**:
  ```powershell
  .\rrhh.exe -validar-ofac "1111111111111"
  ```
- **RUNT**:
  ```powershell
  .\rrhh.exe -validar-runt "80766779" -apellido "RIOS"
  ```
- **SIMIT**:
  ```powershell
  .\rrhh.exe -validar-simit "1111111111111"
  ```
- **ADRES (BDUA)**:
  ```powershell
  .\rrhh.exe -validar-adres "80766779"
  ```

## 🌐 API REST (`-web`, solo API: sin panel HTML; el índice de endpoints está en `GET /`)

| Endpoint | Descripción |
|---|---|
| `GET /api/salud` | Estado del servicio |
| `GET /api/computrabajo/vacantes[?contar=0]` | Vacantes activas **en vivo** en Computrabajo con `total_candidatos` (inscritos reales; `contar=0` lo omite), `procesada` y `excluida` |
| `GET /api/vacantes` | Vacantes con `slug` (una por slug; gana la carpeta más reciente) |
| `GET /api/vacantes/{slug}` | Resumen de la vacante |
| `GET /api/vacantes/{slug}/candidatos?filtro=aptos\|no_aptos&limite=&offset=` | Candidatos (cabecera `X-Total-Count`) |
| `GET /api/vacantes/{slug}/candidatos/{cedula\|slug-persona}` | Persona + su evaluación de IA |
| `GET /api/vacantes/{slug}/ranking` | Ranking de IA de la vacante (puestos 1..n) |
| `GET /api/ranking[?oferta=slug]` | Ranking global o por vacante |
| `POST /api/evaluar-ia[?async=1]` | Evalúa con IA. `async=1` responde 202; avance en `GET /api/evaluar-ia/estado` |
| `GET /api/pdf?ruta=` | Hoja de vida (solo archivos dentro de `descargas/`) |
| `POST /api/candidato/preguntar` | Pregunta a la IA sobre un candidato |

Los listados omiten `texto_cv`/`texto_perfil`; use `?incluir_cv=1` para incluirlos.
Variables: `RRHH_CORS_ORIGINS` (orígenes permitidos, por defecto los puertos locales de Vite y 8080),
`RRHH_PRECALENTAR_COMPUTRABAJO=true` (abre Chrome y la sesión al arrancar, para que la primera consulta de vacantes sea rápida), `OLLAMA_CONCURRENCIA` (candidatos evaluados a la vez, por defecto 2) y `SARLAFT_TLS_INSECURE=true`
(solo si la descarga de la lista ONU falla por certificado).

## Análisis en vivo de una vacante (SSE)

`GET /api/computrabajo/vacantes/{id}/analizar` — `{id}` es el `id` que devuelve `/api/computrabajo/vacantes`.
Abre la oferta, descarga perfil y hoja de vida de cada postulado y aplica las reglas (género, RUNT C2/C3 ≥ 2 años,
SIMIT, EPS) transmitiendo el avance como Server-Sent Events. Solo corre **un análisis a la vez**.

Parámetros opcionales: `completo=1` (sin descarte temprano), `forzar=1` (reprocesa lo ya analizado),
`limite=N`, `concurrencia=1..20` (por defecto 1: una persona a la vez, de principio a fin), `antecedentes|sarlaft|ofac=0` (consultas informativas, activas por defecto).

Descarte temprano (por defecto): perfil → género → RUNT → SIMIT → ADRES; se corta en la primera regla que falla.
Eventos: `job.iniciado`, `candidato.descubierto`, `candidato.etapa`, `candidato.completo`, `resumen`,
`listado.completo`, `job.completado` | `job.error` | `job.cancelado` (y `: ping` cada 15 s).

El análisis no depende de la conexión: `GET /api/jobs/{id}/eventos` (o reconectar con `Last-Event-ID`) retoma el
historial; `GET /api/jobs` lista los recientes; `DELETE /api/jobs/{id}` cancela.

```js
const es = new EventSource(`/api/computrabajo/vacantes/${id}/analizar`);
es.addEventListener('candidato.completo', e => pintar(JSON.parse(e.data)));
es.addEventListener('job.completado', () => es.close());
```

## Candidatos sin cédula: pausa, solicitud con token y reanudación

Requiere la base de datos (migraciones `almacen/migraciones/001` a `004`, todas seguras de repetir).

Cuando no se puede leer la cédula de un candidato (ficha de Computrabajo, texto del PDF o Word de la hoja de
vida, perfil impreso, texto de la ficha, o una postulación anterior de la misma persona con el mismo correo y
nombre), **no se descarta**: su análisis queda en pausa. Cada método probado queda registrado en
`postulacion_historial` (`GET /api/postulaciones/{id}/historial`).

| Estado (`estado_proceso`) | Significa |
|---|---|
| `esperando_datos` | Se le pidió (o se le pedirá) su cédula y fecha de nacimiento |
| `sin_contacto` | La hoja de vida no trae correo ni celular: lo resuelve un reclutador |
| `datos_recibidos` | Respondió; el análisis se reanuda en segundo plano |
| `solicitud_vencida` | El enlace venció sin respuesta; se puede reenviar |
| `identidad_en_revision` | La cédula declarada no cuadra con el nombre del perfil (o ya tiene otra postulación en la vacante) |

**Enlace con token.** El mensaje lleva `…/validar-datos/<token>` y nada más. El token es aleatorio (256 bits),
de un solo uso, vence a los 7 días y en la base solo se guarda su huella SHA-256. Reenviar
(`POST /api/postulaciones/{id}/solicitar-datos`) genera uno nuevo e invalida el anterior. El formulario pide
nombre, correo y cargo al backend con ese token (`GET /api/validar-datos/{token}`) y entrega los datos con
`POST /api/validar-datos/{token}`. Si la fecha de nacimiento da menos de la edad mínima (18) no se guarda nada.
El token no se escribe en los logs; las rutas públicas tienen límite de peticiones por IP.

**Qué se guarda.** Cédula y fecha de nacimiento cifradas (AES-256-GCM), la edad al declarar, la versión de la
política que aceptó y la huella de su IP. La API solo expone la `edad`, no la fecha.

**Reanudación.** Al entregar sus datos se guardan y se encola un trabajo (`trabajos`, tipo
`reanudar_postulacion`) en la misma transacción. Un trabajador en segundo plano retoma desde ADRES (no vuelve
a abrir Computrabajo), compara el titular que devuelve ADRES con el nombre del perfil y evalúa. Reintenta hasta
3 veces; si el proceso se cae, el trabajo se recupera al reiniciar.

**Envío.** Por defecto no sale ningún mensaje solo: se envía desde la interfaz, y solo hacia las personas de
prueba (`RRHH_SOLICITUD_SOLO_PRUEBA=true`). Ver las variables `RRHH_SOLICITUD_*`, `RRHH_EDAD_MINIMA` y
`RRHH_POLITICA_VERSION` en el `.env`.

## Inicio de sesión y usuarios

Con la base configurada, toda la API (`/api/...`) exige sesión, salvo `/api/salud`, `/api/auth/*` y la página pública del candidato (`/api/validar-datos/<token>`). Sin base de datos la API queda abierta y el servidor lo avisa al arrancar.

1. Aplicar `almacen/migraciones/005_usuarios.sql` completo (crea `usuarios` y `sesiones`; segura de repetir).
2. Crear el usuario (la contraseña va en una variable de entorno para que no quede en el historial):

```powershell
$env:RRHH_NUEVA_CLAVE = "<contraseña>"
go run . -crear-usuario -usuario-correo sharitha.ramirez@sao6.com.co -usuario-nombre "RAMIREZ PALACIO SHARITHA" -usuario-cargo "AUXILIAR DE DESARROLLO"
Remove-Item Env:RRHH_NUEVA_CLAVE
```

Repetir el comando con otra contraseña la cambia (y desbloquea la cuenta). La contraseña se guarda como hash PBKDF2-SHA256 con sal; la sesión es una cookie HttpOnly/SameSite=Strict de 12 h. 5 intentos fallidos seguidos bloquean la cuenta 15 minutos.
