-- 008_parametros_aceptacion.sql
-- Parámetros de aceptación de hojas de vida que se editan desde la pantalla Configuración > Parámetros.
-- Viven en reglas_idoneidad (cada guardado crea una versión nueva y cierra la anterior, así los jobs y
-- las evaluaciones siguen apuntando a las reglas con las que corrieron). Las columnas que ya existían se
-- reutilizan: eps_autorizadas (EPS aceptadas), runt_categorias (licencias) y genero_requerido.
--
--   edad_min / edad_max     rango de edad aceptado, en años
--   acepta_reintegros       si se aceptan candidatos que ya trabajaron en la empresa
--   generos_aceptados       JSON, ej. ["masculino","femenino"]
--   experiencia_min_anios   experiencia mínima como conductor, en años
--
-- Es OPCIONAL para arrancar el programa (no sube la versión exigida): sin ella la pantalla de parámetros
-- responde que falta aplicar esta migración. Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

ALTER TABLE reglas_idoneidad
  ADD COLUMN IF NOT EXISTS edad_min              TINYINT UNSIGNED NULL,
  ADD COLUMN IF NOT EXISTS edad_max              TINYINT UNSIGNED NULL,
  ADD COLUMN IF NOT EXISTS acepta_reintegros     TINYINT(1)       NULL,
  ADD COLUMN IF NOT EXISTS generos_aceptados     JSON             NULL,
  ADD COLUMN IF NOT EXISTS experiencia_min_anios DECIMAL(4,1)     NULL;

-- La versión vigente de hoy refleja lo que ya aplica el código: edad 22-49, solo hombres, sin reintegros
-- y 2 años de experiencia. Solo toca filas que aún no tienen los valores nuevos.
UPDATE reglas_idoneidad
   SET edad_min = 22, edad_max = 49, acepta_reintegros = 0,
       generos_aceptados = IF(genero_requerido = 'Hombre', JSON_ARRAY('masculino'), JSON_ARRAY('masculino', 'femenino')),
       experiencia_min_anios = runt_antiguedad_anios
 WHERE edad_min IS NULL;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (8, '008_parametros_aceptacion', UTC_TIMESTAMP(3));
