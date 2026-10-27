-- 006_edad_ficha.sql
-- Guarda la edad que declara la ficha de Computrabajo (años, tal como aparece) para mostrarla y
-- aplicar el rango de contratación sin volver a abrir la ficha. Es NULL cuando no se conoce: el
-- sistema no inventa edades. La que sale de la fecha de nacimiento del formulario sigue mandando.
--
-- Es SEGURA DE REPETIR (sintaxis de MariaDB 10.1.4 o superior).

ALTER TABLE perfiles
  ADD COLUMN IF NOT EXISTS edad TINYINT UNSIGNED NULL AFTER genero_inferido;

INSERT IGNORE INTO schema_migraciones (version, nombre, aplicada_en)
VALUES (6, '006_edad_ficha', UTC_TIMESTAMP(3));
