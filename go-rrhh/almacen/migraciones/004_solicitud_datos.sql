-- 004_solicitud_datos.sql
-- Flujo de datos faltantes: cuando no se pudo leer la cédula de un candidato, su análisis queda en
-- pausa, se le pide la información con un enlace de un solo uso (token) y, al responder, se
-- reanuda desde donde se detuvo.
--
--   solicitudes_datos     control de cada solicitud: estado, etapa pausada y vigencia del token
--   datos_declarados      lo que escribió el candidato (cédula y fecha de nacimiento cifradas) y su consentimiento
--   postulacion_historial línea de tiempo de cada candidato: qué se intentó, qué se dejó en pausa y por qué
--
-- Las reanudaciones usan la tabla trabajos (tipo reanudar_postulacion) creada en la 001.
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

CREATE TABLE IF NOT EXISTS solicitudes_datos (
  id                BINARY(16)   NOT NULL,
  postulacion_id    BINARY(16)   NOT NULL,
  token_hash        CHAR(64)     NULL,                    -- SHA-256 del token; el token nunca se guarda. NULL hasta el primer envío
  motivo            VARCHAR(40)  NOT NULL,                -- cedula_no_encontrada
  etapa_pausada     VARCHAR(30)  NOT NULL,                -- primera etapa que depende del dato que falta (adres)
  estado            VARCHAR(20)  NOT NULL DEFAULT 'pendiente_envio',
  envios            TINYINT UNSIGNED NOT NULL DEFAULT 0,
  aperturas         INT UNSIGNED NOT NULL DEFAULT 0,
  intentos_fallidos TINYINT UNSIGNED NOT NULL DEFAULT 0,  -- datos rechazados con este token; al pasar de 10 se revoca
  creada_en         DATETIME(3)  NOT NULL,
  actualizada_en    DATETIME(3)  NOT NULL,
  enviada_en        DATETIME(3)  NULL,
  vence_en          DATETIME(3)  NULL,                    -- se fija al enviar
  abierta_en        DATETIME(3)  NULL,                    -- primera vez que se abrió el enlace
  completada_en     DATETIME(3)  NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_solicitud_token (token_hash),
  KEY ix_solicitud_postulacion (postulacion_id, estado),
  KEY ix_solicitud_vencimiento (estado, vence_en),
  CONSTRAINT ck_solicitudes_estado CHECK (estado IN ('pendiente_envio','enviada','abierta','completada','vencida','revocada')),
  CONSTRAINT fk_solicitudes_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS datos_declarados (
  id                       BINARY(16)    NOT NULL,
  solicitud_id             BINARY(16)    NOT NULL,
  postulacion_id           BINARY(16)    NOT NULL,
  cedula_hmac              CHAR(64)      NOT NULL,
  cedula_cifrada           VARBINARY(256) NOT NULL,
  fecha_nacimiento_cifrada VARBINARY(256) NOT NULL,
  edad_al_declarar         TINYINT UNSIGNED NOT NULL,     -- validada contra la edad mínima antes de guardar
  politica_version         VARCHAR(40)   NOT NULL,        -- política de datos personales que aceptó
  politica_aceptada_en     DATETIME(3)   NOT NULL,
  ip_hmac                  CHAR(64)      NULL,            -- huella de la IP, no la IP
  creado_en                DATETIME(3)   NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_declarados_solicitud (solicitud_id),
  KEY ix_declarados_postulacion (postulacion_id),
  CONSTRAINT fk_declarados_solicitud FOREIGN KEY (solicitud_id) REFERENCES solicitudes_datos (id) ON DELETE CASCADE,
  CONSTRAINT fk_declarados_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS postulacion_historial (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  postulacion_id BINARY(16)   NOT NULL,
  tipo           VARCHAR(40)  NOT NULL,   -- cedula.busqueda, analisis.pausado, solicitud.creada, solicitud.enviada...
  detalle        JSON         NULL,       -- sin datos personales
  creado_en      DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  KEY ix_historial_postulacion (postulacion_id, id),
  CONSTRAINT fk_historial_postulacion FOREIGN KEY (postulacion_id) REFERENCES postulaciones (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Un correo puede medir hasta 254 caracteres; cifrado ocupa 28 bytes más.
ALTER TABLE contactos_candidato MODIFY valor_cifrado VARBINARY(512) NOT NULL;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (4, '004_solicitud_datos', UTC_TIMESTAMP(3));
