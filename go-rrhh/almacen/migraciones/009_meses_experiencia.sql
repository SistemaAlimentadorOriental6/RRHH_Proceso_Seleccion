-- 009_meses_experiencia.sql
-- La experiencia mínima y el tiempo inscrito en el RUNT se guardan en MESES enteros (ej. 1 año 6 meses = 18),
-- para poder elegir cualquier cantidad de meses. Las columnas de años de la 008 se siguen llenando como
-- espejo (meses / 12, un decimal) para que nada que las lea se rompa.
--
-- Las versiones que ya existen se pasan a meses multiplicando sus años.
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

ALTER TABLE reglas_idoneidad
  ADD COLUMN IF NOT EXISTS experiencia_min_meses INT UNSIGNED NULL,
  ADD COLUMN IF NOT EXISTS runt_antiguedad_meses INT UNSIGNED NULL;

UPDATE reglas_idoneidad
   SET experiencia_min_meses = ROUND(experiencia_min_anios * 12),
       runt_antiguedad_meses = ROUND(runt_antiguedad_anios * 12)
 WHERE experiencia_min_meses IS NULL OR runt_antiguedad_meses IS NULL;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (9, '009_meses_experiencia', UTC_TIMESTAMP(3));
