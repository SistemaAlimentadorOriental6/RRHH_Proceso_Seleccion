package modelos

import "time"

// EvaluacionIA contiene la calificación estructurada y accionable para selección.
type EvaluacionIA struct {
	ScoreTotal               int      `json:"score_total"`                // 0 a 100
	ScoreExperiencia         int      `json:"score_experiencia"`          // 0 a 30
	ScoreConduccionNormativo int      `json:"score_conduccion_normativo"` // 0 a 30
	ScoreEstabilidad         int      `json:"score_estabilidad"`          // 0 a 20
	ScoreAfinidad            int      `json:"score_afinidad"`             // 0 a 20
	Dictamen                 string   `json:"dictamen"`                   // Apto Prioritario, Apto Recomendado, etc.
	PorQueEsElHombre         string   `json:"por_que_es_el_hombre"`       // 2 oraciones accionables de decisión
	Fortalezas               []string `json:"fortalezas"`                 // 2 fortalezas accionables
	Alerta                   string   `json:"alerta"`                     // Alerta para decisión o 'Ninguna'
	NivelEducativo           string   `json:"nivel_educativo,omitempty"`  // Bachiller, Técnico, Tecnólogo, etc.
	AnosExperiencia          int      `json:"anos_experiencia,omitempty"` // Años de experiencia estimados
	Habilidades              []string `json:"habilidades,omitempty"`      // Habilidades detectadas
	Observaciones            string   `json:"observaciones,omitempty"`    // Observaciones adicionales

	// Justificación de cada puntaje, lo que no cuadra en la hoja de vida y quién calificó y cuándo.
	JustExperiencia string               `json:"just_experiencia,omitempty"`
	JustNormativo   string               `json:"just_normativo,omitempty"`
	JustEstabilidad string               `json:"just_estabilidad,omitempty"`
	JustAfinidad    string               `json:"just_afinidad,omitempty"`
	Inconsistencias []string             `json:"inconsistencias,omitempty"`
	Experiencia     []ExperienciaLaboral `json:"experiencia,omitempty"`
	Modelo          string               `json:"modelo,omitempty"`
	Cargo           string               `json:"cargo,omitempty"`
	PromptVersion   string               `json:"prompt_version,omitempty"`
	EvaluadoEn      time.Time            `json:"evaluado_en,omitempty"`
	DuracionMS      int64                `json:"duracion_ms,omitempty"`
}

// CandidatoCalificadoIA asocia a un candidato apto con su posición en el ranking y calificación de IA.
type CandidatoCalificadoIA struct {
	Puesto       int          `json:"puesto"`
	Candidato    RegistroApto `json:"candidato"`
	TextoCV      string       `json:"texto_cv,omitempty"`
	EvaluacionIA EvaluacionIA `json:"evaluacion_ia"`
}

// ExperienciaLaboral es un trabajo de la hoja de vida, tal como lo declara el candidato.
type ExperienciaLaboral struct {
	Empresa string `json:"empresa"`
	Cargo   string `json:"cargo"`
	Periodo string `json:"periodo,omitempty"` // fechas como figuran en la hoja de vida
	// Inicio y Fin (AAAA-MM; Fin "actual" si sigue vigente) son las fechas ya normalizadas: de ellas sale
	// la duración real, en vez de la cuenta que haga la IA.
	Inicio      string `json:"inicio,omitempty"`
	Fin         string `json:"fin,omitempty"`
	Duracion    string `json:"duracion,omitempty"` // p. ej. "5 años"
	Descripcion string `json:"descripcion,omitempty"`
}
