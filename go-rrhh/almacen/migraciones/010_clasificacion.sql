-- 010_clasificacion.sql
-- Clasificación de cada persona evaluada:
--   reclutado_p1  cumple todas las reglas
--   semillero     no cumple la edad mínima, la EPS, el RUNT o el SIMIT (deuda hasta $2.000.000), sin nada que la descarte
--   descartado    género no aceptado, mayor a la edad tope, reintegro no aceptado, incidentes
--                 (Procuraduría, ONU, OFAC) o deuda en SIMIT mayor a $2.000.000
-- NULL = sin clasificación (pendiente, en pausa o evaluada antes de esta migración).
--
--   evaluaciones_idoneidad.clasificacion  la de esa evaluación (queda en el historial)
--   evaluaciones_idoneidad.semillero      JSON con las reglas que la dejaron para semillero, ej. ["edad","runt"]
--   postulaciones.clasificacion           la de la evaluación vigente, para filtrar sin unir tablas
--
-- OBLIGATORIA: sin estas columnas el programa no arranca (no podría guardar a nadie).
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

ALTER TABLE evaluaciones_idoneidad
  ADD COLUMN IF NOT EXISTS clasificacion VARCHAR(20) NULL,
  ADD COLUMN IF NOT EXISTS semillero     JSON        NULL;

ALTER TABLE postulaciones
  ADD COLUMN IF NOT EXISTS clasificacion VARCHAR(20) NULL;

CREATE INDEX IF NOT EXISTS ix_postulacion_oferta_clasificacion ON postulaciones (oferta_id, clasificacion);

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (10, '010_clasificacion', UTC_TIMESTAMP(3));
