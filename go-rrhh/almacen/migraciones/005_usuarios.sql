-- 005_usuarios.sql
-- Acceso de los reclutadores a la plataforma:
--
--   usuarios  quién puede entrar. La contraseña se guarda como hash PBKDF2-SHA256 con sal propia
--             (formato "pbkdf2-sha256$iteraciones$sal$hash"); nunca en claro. Tras 5 intentos fallidos
--             seguidos la cuenta se bloquea 15 minutos.
--   sesiones  una fila por sesión abierta. Se guarda el SHA-256 del token de la cookie, no el token.
--
-- Los usuarios NO se crean aquí (no hay contraseñas en el repositorio): se dan de alta con
--   RRHH_NUEVA_CLAVE=... go run . -crear-usuario -correo ... -nombre ... -cargo ...
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

CREATE TABLE IF NOT EXISTS usuarios (
  id                BINARY(16)   NOT NULL,
  correo            VARCHAR(254) NOT NULL,                 -- en minúsculas
  nombre            VARCHAR(150) NOT NULL,
  cargo             VARCHAR(150) NOT NULL,
  clave_hash        VARCHAR(255) NOT NULL,
  activo            TINYINT(1)   NOT NULL DEFAULT 1,
  intentos_fallidos TINYINT UNSIGNED NOT NULL DEFAULT 0,
  bloqueado_hasta   DATETIME(3)  NULL,
  ultimo_acceso     DATETIME(3)  NULL,
  creado_en         DATETIME(3)  NOT NULL,
  actualizado_en    DATETIME(3)  NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_usuarios_correo (correo)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sesiones (
  token_hash CHAR(64)    NOT NULL,                         -- SHA-256 del token de la cookie
  usuario_id BINARY(16)  NOT NULL,
  creada_en  DATETIME(3) NOT NULL,
  expira_en  DATETIME(3) NOT NULL,
  PRIMARY KEY (token_hash),
  KEY ix_sesiones_usuario (usuario_id),
  KEY ix_sesiones_expira (expira_en),
  CONSTRAINT fk_sesiones_usuario FOREIGN KEY (usuario_id) REFERENCES usuarios (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (5, '005_usuarios', UTC_TIMESTAMP(3));
