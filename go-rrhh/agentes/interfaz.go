package agentes

import (
	"context"
	"go-rrhh/modelos"
)

// AgenteValidador define el contrato que todo agente de validacion externa debe cumplir (Principio Open/Closed y Liskov).
type AgenteValidador interface {
	Nombre() string
	Validar(ctx context.Context, candidato *modelos.Candidato) error
}
