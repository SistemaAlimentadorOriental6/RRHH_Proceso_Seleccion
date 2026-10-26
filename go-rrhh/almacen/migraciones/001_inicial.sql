-- 001_inicial.sql
-- Esquema inicial de RRHH. Pensado para MariaDB 10.6+ o MySQL 8.0.1+ (aún no probado en el servidor).
--
-- Convenciones:
--   * IDs UUID v7 en BINARY(16), generados en Go (uuid.NewV7). Excepción: tablas de bitácora
--     de alto volumen (eventos_trabajo, auditoria), que usan BIGINT AUTO_INCREMENT.
--   * Fechas en UTC, DATETIME(3).
--   * Dinero en DECIMAL(14,2). Nunca FLOAT.
--   * Listas que se leen completas van en JSON; las que se consultan van en tabla hija.
--   * Datos personales: cédula y contactos cifrados (AES-256-GCM) + HMAC-SHA256 para buscar.
--     Textos largos (perfil, CV) viven en archivos del disco; aquí solo referencia + SHA-256.

SET time_zone = '+00:00';

-- A. Catálogos y configuración

CREATE TABLE ofertas (
  id               BINARY(16)   NOT NULL,
  id_computrabajo  VARCHAR(32)  NOT NULL,
  titulo           VARCHAR(255) NOT NULL,
  slug             VARCHAR(255) NOT NULL,
  excluida         TINYINT(1)   NOT NULL DEFAULT 0,
  creado_en        DATETIME(3)  NOT NULL,
  actualizado_en   DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_oferta_computrabajo (id_computrabajo),
  KEY ix_oferta_slug (slug)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE fuentes (
  codigo    VARCHAR(30)  NOT NULL,
  nombre    VARCHAR(120) NOT NULL,
  tipo      VARCHAR(20)  NOT NULL,
  es_nucleo TINYINT(1)   NOT NULL DEFAULT 0,  -- descarta si falla (núcleo) o solo informa
  orden     TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (codigo),
  CONSTRAINT ck_fuentes_tipo CHECK (tipo IN ('navegador','api','memoria'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO fuentes (codigo, nombre, tipo, es_nucleo, orden) VALUES
  ('adres',        'ADRES (BDUA)',          'navegador', 1, 1),
  ('runt',         'RUNT',                  'api',       1, 2),
  ('simit',        'SIMIT',                 'navegador', 1, 3),
  ('procuraduria', 'Procuraduría General',  'navegador', 0, 4),
  ('ofac',         'OFAC (EE. UU.)',        'navegador', 0, 5),
  ('sarlaft',      'SARLAFT ONU',           'memoria',   0, 6);

CREATE TABLE reglas_idoneidad (
  id                    BINARY(16)   NOT NULL,
  version               INT UNSIGNED NOT NULL,
  genero_requerido      VARCHAR(20)  NULL,       -- 'Hombre' hoy; PENDIENTE revisión legal. NULL = sin requisito
  eps_autorizadas       JSON         NOT NULL,
  runt_antiguedad_anios DECIMAL(4,1) NOT NULL,
  runt_categorias       JSON         NOT NULL,   -- ej. ["C2","C3"]
  simit_max_multas      INT UNSIGNED NOT NULL,
  simit_deuda_max       DECIMAL(14,2) NOT NULL,
  vigente_desde         DATETIME(3)  NOT NULL,
  vigente_hasta         DATETIME(3)  NULL,
  creado_en             DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_regla_version (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Versión 1: refleja exactamente las reglas que hoy aplica el código.
INSERT INTO reglas_idoneidad
  (id, version, genero_requerido, eps_autorizadas, runt_antiguedad_anios, runt_categorias,
   simit_max_multas, simit_deuda_max, vigente_desde, vigente_hasta, creado_en)
VALUES
  (0x01990000000070008000000000000001, 1, 'Hombre',
   JSON_ARRAY('Sura','Nueva EPS','Savia Salud','Sanitas'), 2.0, JSON_ARRAY('C2','C3'),
   0, 1000000.00, UTC_TIMESTAMP(3), NULL, UTC_TIMESTAMP(3));

-- B. Ejecución (jobs)

CREATE TABLE trabajos (
  id               BINARY(16)   NOT NULL,
  tipo             VARCHAR(40)  NOT NULL,         -- analisis_vacante
  oferta_id        BINARY(16)   NOT NULL,
  regla_version_id BINARY(16)   NOT NULL,         -- reglas con las que corrió este job
  opciones         JSON         NOT NULL,
  estado           VARCHAR(20)  NOT NULL,
  cancelar         TINYINT(1)   NOT NULL DEFAULT 0,
  intentos         TINYINT UNSIGNED NOT NULL DEFAULT 0,
  worker_id        VARCHAR(64)  NULL,
  bloqueado_hasta  DATETIME(3)  NULL,
  resumen          JSON         NULL,             -- solo conteos
  error            VARCHAR(500) NULL,             -- sanitizado, sin datos personales
  solicitado_por   VARCHAR(120) NULL,             -- NULL hasta que exista autenticación
  creado_en        DATETIME(3)  NOT NULL,
  actualizado_en   DATETIME(3)  NOT NULL,
  terminado_en     DATETIME(3)  NULL,
  PRIMARY KEY (id),
  KEY ix_trabajo_cola (estado, creado_en),
  KEY ix_trabajo_oferta (oferta_id, creado_en),
  CONSTRAINT ck_trabajos_estado CHECK (estado IN ('pendiente','en_curso','completado','error','cancelado')),
  CONSTRAINT fk_trabajos_oferta FOREIGN KEY (oferta_id) REFERENCES ofertas (id),
  CONSTRAINT fk_trabajos_regla  FOREIGN KEY (regla_version_id) REFERENCES reglas_idoneidad (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE eventos_trabajo (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,  -- Last-Event-ID del SSE
  trabajo_id BINARY(16)      NOT NULL,
  tipo       VARCHAR(60)     NOT NULL,
  datos      JSON            NOT NULL,                 -- solo IDs y conteos
  creado_en  DATETIME(3)     NOT NULL,
  PRIMARY KEY (id),
  KEY ix_evento_trabajo (trabajo_id, id),
  CONSTRAINT fk_eventos_trabajo FOREIGN KEY (trabajo_id) REFERENCES trabajos (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE auditoria (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  quien         VARCHAR(120)    NULL,
  accion        VARCHAR(60)     NOT NULL,   -- analisis.iniciar, analisis.cancelar, supresion.persona
  objetivo_tipo VARCHAR(40)     NULL,
  objetivo_id   BINARY(16)      NULL,       -- sin FK: la auditoría sobrevive a las purgas
  detalle       JSON            NULL,       -- sin datos personales
  creado_en     DATETIME(3)     NOT NULL,
  PRIMARY KEY (id),
  KEY ix_auditoria_creado (creado_en)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- C. Persona y postulación

CREATE TABLE personas (
  id             BINARY(16)    NOT NULL,
  cedula_hmac    CHAR(64)      NOT NULL,        -- HMAC-SHA256, clave RRHH_CLAVE_PSEUDONIMOS
  cedula_cifrada VARBINARY(256) NOT NULL,       -- AES-256-GCM, clave RRHH_CLAVE_CIFRADO
  creado_en      DATETIME(3)   NOT NULL,
  actualizado_en DATETIME(3)   NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_persona_cedula (cedula_hmac)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE postulaciones (
  id                     BINARY(16)  NOT NULL,
  persona_id             BINARY(16)  NOT NULL,
  oferta_id              BINARY(16)  NOT NULL,
  trabajo_id             BINARY(16)  NULL,      -- último job que la procesó
  etapa_actual           VARCHAR(30) NOT NULL,
  dictamen               VARCHAR(20) NOT NULL DEFAULT 'pendiente',
  verificacion_pendiente TINYINT(1)  NOT NULL DEFAULT 0,
  error_proceso          VARCHAR(255) NULL,
  creado_en              DATETIME(3) NOT NULL,
  actualizado_en         DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_postulacion_persona_oferta (persona_id, oferta_id),
  KEY ix_postulacion_oferta_dictamen (oferta_id, dictamen),
  CONSTRAINT ck_postulaciones_dictamen CHECK (dictamen IN ('apto','no_apto','pendiente')),
  CONSTRAINT fk_postulaciones_persona FOREIGN KEY (persona_id) REFERENCES personas (id) ON DELETE CASCADE,
  CONSTRAINT fk_postulaciones_oferta  FOREIGN KEY (oferta_id)  REFERENCES ofertas (id),
  CONSTRAINT fk_postulaciones_trabajo FOREIGN KEY (trabajo_id) REFERENCES trabajos (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE checkpoints (
  postulacion_id BINARY(16)  NOT NULL,
  etapa          VARCHAR(30) NOT NULL,
  resultado      VARCHAR(20) NOT NULL,
  trabajo_id     BINARY(16)  NULL,
  registrado     DATETIME(3) NOT NULL,
  PRIMARY KEY (postulacion_id, etapa),
  CONSTRAINT fk_checkpoints_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_checkpoints_trabajo     FOREIGN KEY (trabajo_id)     REFERENCES trabajos (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- D. Perfil, contacto y archivos

CREATE TABLE archivos (
  id             BINARY(16)   NOT NULL,
  postulacion_id BINARY(16)   NOT NULL,
  tipo           VARCHAR(30)  NOT NULL,
  fuente_codigo  VARCHAR(30)  NULL,            -- solo para capturas: de qué fuente
  ruta           VARCHAR(500) NOT NULL,        -- relativa a descargas/
  sha256         CHAR(64)     NOT NULL,
  bytes          INT UNSIGNED NOT NULL,
  creado_en      DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  KEY ix_archivos_postulacion (postulacion_id, tipo),
  CONSTRAINT ck_archivos_tipo CHECK (tipo IN ('cv_original','cv_pdf','perfil_pdf','foto','captura','texto_perfil','texto_cv')),
  CONSTRAINT fk_archivos_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_archivos_fuente      FOREIGN KEY (fuente_codigo)  REFERENCES fuentes (codigo)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE perfiles (
  postulacion_id          BINARY(16)  NOT NULL,
  genero_declarado        VARCHAR(30) NULL,    -- de la ficha de Computrabajo
  genero_inferido         VARCHAR(30) NULL,    -- de perfil / diccionario / sufijo
  genero_fuente           VARCHAR(20) NULL,
  url_detalle             VARCHAR(500) NULL,
  url_cv                  VARCHAR(500) NULL,
  texto_perfil_archivo_id BINARY(16)  NULL,
  leido_en                DATETIME(3) NOT NULL,
  PRIMARY KEY (postulacion_id),
  CONSTRAINT ck_perfiles_genero_fuente CHECK (genero_fuente IS NULL OR genero_fuente IN ('perfil','diccionario','sufijo','ninguna')),
  CONSTRAINT fk_perfiles_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_perfiles_texto       FOREIGN KEY (texto_perfil_archivo_id) REFERENCES archivos (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE contactos_candidato (
  id             BINARY(16)    NOT NULL,
  postulacion_id BINARY(16)    NOT NULL,
  tipo           VARCHAR(10)   NOT NULL,       -- correo | telefono
  valor_hmac     CHAR(64)      NOT NULL,       -- deduplica sin descifrar
  valor_cifrado  VARBINARY(256) NOT NULL,
  origen         VARCHAR(20)   NOT NULL DEFAULT 'cv',
  detectado_en   DATETIME(3)   NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_contacto (postulacion_id, tipo, valor_hmac),
  CONSTRAINT ck_contactos_tipo CHECK (tipo IN ('correo','telefono')),
  CONSTRAINT fk_contactos_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- E. Validaciones: encabezado común + detalle por fuente

CREATE TABLE validaciones (
  id             BINARY(16)   NOT NULL,
  postulacion_id BINARY(16)   NOT NULL,
  fuente_codigo  VARCHAR(30)  NOT NULL,
  trabajo_id     BINARY(16)   NULL,
  resultado      VARCHAR(20)  NOT NULL,        -- ok | rechazado | error | alerta
  codigo_estado  VARCHAR(60)  NULL,            -- estado crudo del portal: INSCRITO_ACTIVO, ERROR_CONSULTA...
  detalle        TEXT         NULL,
  intentos       TINYINT UNSIGNED NOT NULL DEFAULT 1,
  duracion_ms    INT UNSIGNED NULL,
  consultado_en  DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  KEY ix_validaciones_persona_fuente (postulacion_id, fuente_codigo, consultado_en),
  CONSTRAINT ck_validaciones_resultado CHECK (resultado IN ('ok','rechazado','error','alerta')),
  CONSTRAINT fk_validaciones_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_validaciones_fuente      FOREIGN KEY (fuente_codigo)  REFERENCES fuentes (codigo),
  CONSTRAINT fk_validaciones_trabajo     FOREIGN KEY (trabajo_id)     REFERENCES trabajos (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE val_adres (
  validacion_id     BINARY(16)   NOT NULL,
  estado_portal     VARCHAR(60)  NULL,
  nombres           VARCHAR(120) NULL,
  apellidos         VARCHAR(120) NULL,
  departamento      VARCHAR(120) NULL,
  municipio         VARCHAR(120) NULL,
  eps               VARCHAR(120) NULL,
  regimen           VARCHAR(60)  NULL,
  estado_afiliacion VARCHAR(60)  NULL,
  tipo_afiliado     VARCHAR(60)  NULL,
  fecha_afiliacion  DATE         NULL,
  cumple_eps        TINYINT(1)   NULL,          -- NULL = no verificado (p. ej. portal en mantenimiento)
  en_mantenimiento  TINYINT(1)   NOT NULL DEFAULT 0,
  PRIMARY KEY (validacion_id),
  CONSTRAINT fk_val_adres_validacion FOREIGN KEY (validacion_id) REFERENCES validaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE val_runt (
  validacion_id     BINARY(16)   NOT NULL,
  estado_portal     VARCHAR(60)  NULL,
  inscrito          TINYINT(1)   NULL,
  id_persona_runt   VARCHAR(60)  NULL,
  apellido_consultado VARCHAR(120) NULL,
  fecha_inscripcion DATETIME(3)  NULL,          -- NULL si el texto del portal no se pudo parsear
  estado_persona    VARCHAR(60)  NULL,
  conductor_activo  TINYINT(1)   NULL,
  estado_conductor  VARCHAR(60)  NULL,
  tiene_licencias   TINYINT(1)   NULL,
  categorias_activas JSON        NULL,
  tiene_multas      TINYINT(1)   NULL,
  paz_y_salvo       TINYINT(1)   NULL,
  numero_paz_y_salvo VARCHAR(60) NULL,
  antiguedad_anios  DECIMAL(6,2) NULL,          -- calculada al momento de la consulta
  cumple_categoria  TINYINT(1)   NULL,          -- C2/C3 activa
  intentos_captcha  TINYINT UNSIGNED NULL,
  PRIMARY KEY (validacion_id),
  CONSTRAINT fk_val_runt_validacion FOREIGN KEY (validacion_id) REFERENCES validaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE runt_licencias (
  id              BINARY(16)   NOT NULL,
  validacion_id   BINARY(16)   NOT NULL,
  numero_licencia VARCHAR(60)  NULL,
  fecha_expedicion DATE        NULL,
  estado          VARCHAR(40)  NULL,
  retenida        TINYINT(1)   NOT NULL DEFAULT 0,
  ot_expide       VARCHAR(120) NULL,
  restricciones   TEXT         NULL,
  PRIMARY KEY (id),
  KEY ix_licencias_validacion (validacion_id),
  CONSTRAINT fk_runt_licencias_validacion FOREIGN KEY (validacion_id) REFERENCES val_runt (validacion_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE licencia_categorias (
  id               BINARY(16)  NOT NULL,
  licencia_id      BINARY(16)  NOT NULL,
  categoria        VARCHAR(10) NOT NULL,        -- C2, C3, ...
  fecha_vencimiento DATE       NULL,
  restricciones    TEXT        NULL,
  limitaciones     TEXT        NULL,
  PRIMARY KEY (id),
  KEY ix_categoria (categoria),
  KEY ix_categoria_licencia (licencia_id),
  CONSTRAINT fk_licencia_categorias_licencia FOREIGN KEY (licencia_id) REFERENCES runt_licencias (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE val_simit (
  validacion_id   BINARY(16)   NOT NULL,
  estado_portal   VARCHAR(60)  NULL,
  paz_y_salvo     TINYINT(1)   NULL,
  comparendos     INT UNSIGNED NULL,
  multas          INT UNSIGNED NULL,
  acuerdos        INT UNSIGNED NULL,
  deuda_texto     VARCHAR(60)  NULL,            -- monto tal como lo muestra el portal
  deuda_total     DECIMAL(14,2) NULL,           -- monto parseado; revisar centavos
  cumple          TINYINT(1)   NULL,
  PRIMARY KEY (validacion_id),
  CONSTRAINT fk_val_simit_validacion FOREIGN KEY (validacion_id) REFERENCES validaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE val_procuraduria (
  validacion_id   BINARY(16)  NOT NULL,
  estado_portal   VARCHAR(60) NULL,
  coincidencia    TINYINT(1)  NOT NULL DEFAULT 0,
  con_antecedentes TINYINT(1) NOT NULL DEFAULT 0,
  detalle         TEXT        NULL,
  PRIMARY KEY (validacion_id),
  CONSTRAINT fk_val_procuraduria_validacion FOREIGN KEY (validacion_id) REFERENCES validaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE val_ofac (
  validacion_id       BINARY(16)   NOT NULL,
  estado_portal       VARCHAR(60)  NULL,
  total_coincidencias INT UNSIGNED NOT NULL DEFAULT 0,
  registros           JSON         NOT NULL,    -- lista de coincidencias, se lee completa
  PRIMARY KEY (validacion_id),
  CONSTRAINT fk_val_ofac_validacion FOREIGN KEY (validacion_id) REFERENCES validaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE val_sarlaft (
  validacion_id     BINARY(16)   NOT NULL,
  tipo_coincidencia VARCHAR(40)  NULL,         -- NOMBRE_EXACTO | SIMILITUD_NOMBRE | DOCUMENTO_IDENTIDAD | ALIAS_REGISTRADO | LISTA_NO_DISPONIBLE
  id_onu            VARCHAR(40)  NULL,
  nombre_sancionado VARCHAR(255) NULL,
  version_lista     DATETIME(3)  NULL,         -- cuándo se descargó la lista usada
  PRIMARY KEY (validacion_id),
  CONSTRAINT fk_val_sarlaft_validacion FOREIGN KEY (validacion_id) REFERENCES validaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- F. Evaluaciones

CREATE TABLE evaluaciones_idoneidad (
  id                     BINARY(16)  NOT NULL,
  postulacion_id         BINARY(16)  NOT NULL,
  regla_version_id       BINARY(16)  NOT NULL,
  trabajo_id             BINARY(16)  NULL,
  es_apto                TINYINT(1)  NOT NULL,
  cumple_genero          TINYINT(1)  NULL,     -- NULL = no evaluado (descarte temprano)
  cumple_eps             TINYINT(1)  NULL,
  cumple_runt            TINYINT(1)  NULL,
  cumple_simit           TINYINT(1)  NULL,
  descarte_temprano      TINYINT(1)  NOT NULL DEFAULT 0,
  verificacion_pendiente TINYINT(1)  NOT NULL DEFAULT 0,
  vigente                TINYINT(1)  NOT NULL DEFAULT 1,  -- una sola vigente por postulación (lo asegura la app)
  evaluado_en            DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY ix_evaluacion_postulacion (postulacion_id, vigente),
  CONSTRAINT fk_evaluaciones_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_evaluaciones_regla       FOREIGN KEY (regla_version_id) REFERENCES reglas_idoneidad (id),
  CONSTRAINT fk_evaluaciones_trabajo     FOREIGN KEY (trabajo_id) REFERENCES trabajos (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE postulacion_motivos (
  id            BINARY(16)  NOT NULL,
  evaluacion_id BINARY(16)  NOT NULL,
  tipo          VARCHAR(20) NOT NULL,           -- genero | cedula | eps | runt | simit | pendiente
  detalle       TEXT        NOT NULL,
  PRIMARY KEY (id),
  KEY ix_motivo_evaluacion (evaluacion_id),
  KEY ix_motivo_tipo (tipo),
  CONSTRAINT ck_motivos_tipo CHECK (tipo IN ('genero','cedula','eps','runt','simit','pendiente')),
  CONSTRAINT fk_motivos_evaluacion FOREIGN KEY (evaluacion_id) REFERENCES evaluaciones_idoneidad (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE evaluaciones_ia (
  id                     BINARY(16)   NOT NULL,
  postulacion_id         BINARY(16)   NOT NULL,
  evaluacion_id          BINARY(16)   NOT NULL,   -- la evaluación de idoneidad que la originó
  modelo                 VARCHAR(60)  NOT NULL,
  prompt_version         VARCHAR(20)  NOT NULL,
  cargo                  VARCHAR(255) NOT NULL,
  texto_cv_sha256        CHAR(64)     NULL,
  score_total            TINYINT UNSIGNED NOT NULL,      -- 0 a 100
  score_experiencia      TINYINT UNSIGNED NOT NULL,      -- 0 a 30
  score_conduccion_normativo TINYINT UNSIGNED NOT NULL,  -- 0 a 30
  score_estabilidad      TINYINT UNSIGNED NOT NULL,      -- 0 a 20
  score_afinidad         TINYINT UNSIGNED NOT NULL,      -- 0 a 20
  dictamen               VARCHAR(60)  NOT NULL,
  por_que                TEXT         NULL,
  fortalezas             JSON         NULL,
  alerta                 TEXT         NULL,
  nivel_educativo        VARCHAR(60)  NULL,
  anos_experiencia       TINYINT UNSIGNED NULL,
  habilidades            JSON         NULL,
  observaciones          TEXT         NULL,
  sintesis               JSON         NULL,      -- análisis previo del CV (AnalizarExperienciaCV)
  puesto                 INT UNSIGNED NULL,      -- posición en el ranking de la vacante
  duracion_ms            INT UNSIGNED NULL,
  evaluado_en            DATETIME(3)  NOT NULL,
  vigente                TINYINT(1)   NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY ix_ia_postulacion (postulacion_id, vigente),
  KEY ix_ia_cargo_puesto (cargo, puesto),
  CONSTRAINT ck_ia_score_total CHECK (score_total <= 100),
  CONSTRAINT fk_ia_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_ia_evaluacion  FOREIGN KEY (evaluacion_id)  REFERENCES evaluaciones_idoneidad (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- G. Comunicación con el candidato

CREATE TABLE notificaciones (
  id             BINARY(16)   NOT NULL,
  postulacion_id BINARY(16)   NOT NULL,
  contacto_id    BINARY(16)   NULL,
  canal          VARCHAR(10)  NOT NULL,        -- correo | whatsapp
  plantilla      VARCHAR(60)  NOT NULL,        -- ej. solicitud_datos
  estado         VARCHAR(12)  NOT NULL DEFAULT 'pendiente',
  intentos       TINYINT UNSIGNED NOT NULL DEFAULT 0,
  ultimo_error   VARCHAR(500) NULL,
  enviada_en     DATETIME(3)  NULL,
  actualizado_en DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_notificacion (postulacion_id, canal, plantilla),  -- nunca repetir el mismo mensaje
  CONSTRAINT ck_notificaciones_canal  CHECK (canal IN ('correo','whatsapp')),
  CONSTRAINT ck_notificaciones_estado CHECK (estado IN ('pendiente','enviada','fallida')),
  CONSTRAINT fk_notificaciones_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE,
  CONSTRAINT fk_notificaciones_contacto    FOREIGN KEY (contacto_id)    REFERENCES contactos_candidato (id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Registro de la migración

CREATE TABLE schema_migraciones (
  version     INT UNSIGNED NOT NULL,
  nombre      VARCHAR(120) NOT NULL,
  aplicada_en DATETIME(3)  NOT NULL,
  PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (1, '001_inicial', UTC_TIMESTAMP(3));
