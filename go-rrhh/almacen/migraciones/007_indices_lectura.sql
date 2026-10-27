-- 007_indices_lectura.sql
-- Índice para las lecturas de las pantallas: la lista de vacantes (agregado por oferta de las
-- postulaciones vigentes) y los candidatos vigentes de una vacante filtran por (oferta_id, activa).
-- Solo mejora el rendimiento: el programa funciona igual sin él (no sube la versión exigida).
--
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

ALTER TABLE postulaciones
  ADD KEY IF NOT EXISTS ix_postulacion_oferta_activa (oferta_id, activa, actualizado_en);

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (7, '007_indices_lectura', UTC_TIMESTAMP(3));
