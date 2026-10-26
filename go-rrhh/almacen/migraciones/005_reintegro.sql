-- Fuente "reintegro": validación contra Administremos (servicio web de SIGMA) que mira si la cédula ya
-- figura como empleado. Es OPCIONAL y segura de repetir: sin ella el análisis funciona igual, solo que el
-- resultado de reintegro no se guarda en la base (se ve en vivo, pero no al recargar).
INSERT IGNORE INTO fuentes (codigo, nombre, tipo, es_nucleo, orden) VALUES
  ('reintegro', 'Reintegro (Administremos)', 'api', 0, 0);
