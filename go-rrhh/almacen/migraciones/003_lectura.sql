-- 003_lectura.sql
-- Lo necesario para LEER desde la base lo mismo que antes se leía de los JSON:
--   * validaciones.datos: el resultado original que entregó la fuente (el mismo documento que viajaba
--     en el JSON). Las columnas tipadas de val_* sirven para buscar y reportar; este documento
--     permite devolver al candidato exactamente como lo produjo el análisis.
--   * postulaciones.activa: 0 cuando el postulante ya no aparece en la vacante (antes se borraba
--     del JSON; ahora se conserva su historial).
--
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

ALTER TABLE validaciones
  ADD COLUMN IF NOT EXISTS datos JSON NULL AFTER detalle;

ALTER TABLE postulaciones
  ADD COLUMN IF NOT EXISTS activa TINYINT(1) NOT NULL DEFAULT 1 AFTER dictamen;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (3, '003_lectura', UTC_TIMESTAMP(3));
