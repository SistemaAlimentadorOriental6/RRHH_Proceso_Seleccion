-- 002_datos_para_leer.sql
-- Agrega lo que faltaba para que la base pueda reemplazar a los JSON: con la 001 se podía guardar,
-- pero no reconstruir lo que muestra la API (nombre del candidato, su slug, foto, etapa de cada
-- captura y los textos de la evaluación).
--
-- Requiere que 001_inicial.sql haya terminado. Es SEGURA DE REPETIR: usa IF NOT EXISTS, así que si
-- una ejecución anterior quedó a medias se puede volver a correr completa. Esa sintaxis es de
-- MariaDB (10.1.4 o superior); MySQL no la acepta.

ALTER TABLE perfiles
  ADD COLUMN IF NOT EXISTS indice       INT UNSIGNED NULL AFTER postulacion_id,
  ADD COLUMN IF NOT EXISTS nombre       VARCHAR(200) NULL AFTER indice,
  ADD COLUMN IF NOT EXISTS slug_persona VARCHAR(200) NULL AFTER nombre,
  ADD COLUMN IF NOT EXISTS url_foto     VARCHAR(500) NULL AFTER url_cv,
  ADD KEY IF NOT EXISTS ix_perfil_slug (slug_persona);

ALTER TABLE archivos
  ADD COLUMN IF NOT EXISTS etapa VARCHAR(30) NULL AFTER fuente_codigo;

ALTER TABLE evaluaciones_idoneidad
  ADD COLUMN IF NOT EXISTS genero        VARCHAR(30) NULL AFTER es_apto,
  ADD COLUMN IF NOT EXISTS detalle_eps   TEXT NULL,
  ADD COLUMN IF NOT EXISTS detalle_runt  TEXT NULL,
  ADD COLUMN IF NOT EXISTS detalle_simit TEXT NULL;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (2, '002_datos_para_leer', UTC_TIMESTAMP(3));
