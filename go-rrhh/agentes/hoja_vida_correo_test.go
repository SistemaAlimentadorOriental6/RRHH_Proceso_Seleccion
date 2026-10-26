package agentes

import (
	"strings"
	"testing"
)

func TestMencionaHojaDeVida(t *testing.T) {
	si := []string{
		"Hoja de vida", "HOJA DE VIDA - Juan Pérez", "hojas de vida", "Hoja de Bida", "ojas de vida",
		"hoja d vida", "HojaDeVida.pdf", "HOJA_DE_VIDA_JUAN.pdf", "hoja-de-vida", "Curriculum Vitae",
		"curiculum", "currículo", "CV conductor", "hv", "Envío mi H.V.", "hojadevida2025.pdf",
	}
	for _, s := range si {
		if _, ok := mencionaHojaDeVida(s); !ok {
			t.Errorf("%q debería reconocerse como hoja de vida", s)
		}
	}
	no := []string{"Factura electrónica", "Reunión de la hoja de ruta", "Promoción de verano", "cove", "vida sana", "Re: pago de nómina"}
	for _, s := range no {
		if t2, ok := mencionaHojaDeVida(s); ok {
			t.Errorf("%q no es hoja de vida (coincidió con %q)", s, t2)
		}
	}
}

func TestMencionaEmpleo(t *testing.T) {
	for _, s := range []string{"Postulación vacante conductor", "quiero aplicar al empleo", "solicito trabajo", "vacantes disponibles"} {
		if _, ok := mencionaEmpleo(s); !ok {
			t.Errorf("%q debería reconocerse como búsqueda de empleo", s)
		}
	}
	if _, ok := mencionaEmpleo("Factura de energía de octubre"); ok {
		t.Error("una factura no es búsqueda de empleo")
	}
}

func TestPareceHojaDeVida(t *testing.T) {
	cv := "JUAN PEREZ\nDatos personales\nCédula: 1035126774\nPerfil\nExperiencia laboral\nEmpresa X cargo conductor\nFormación académica\nReferencias"
	if !pareceHojaDeVida(cv) {
		t.Error("un CV con secciones típicas debe reconocerse")
	}
	if pareceHojaDeVida("Adjunto la factura del mes. Saludos.") {
		t.Error("una factura no parece hoja de vida")
	}
}

func TestEvaluarInteresConductor(t *testing.T) {
	casos := []struct {
		nombre, asunto, cuerpo, cv string
		quiero                     string
	}{
		{"lo dice el asunto", "Postulación conductor de bus", "", "", ConductorSi},
		{"lo dice el cuerpo", "Hoja de vida", "Buenas, quiero aplicar a la vacante de operador de bus", "", ConductorSi},
		{"dos señales en el CV", "Hoja de vida", "", "Experiencia: conductor de buses en Coonatra. Licencia C2 vigente", ConductorSi},
		{"solo licencia común", "Hoja de vida", "", "Auxiliar contable. Licencia de conducción B1", ConductorDudoso},
		{"sin nada de conducir", "Hoja de vida", "", "Auxiliar administrativa con experiencia en facturación y atención al cliente", ConductorNo},
		{"tildes y mayúsculas", "HOJA DE VIDA", "", "CONDUCTOR PROFESIONAL, CATEGORÍA C3", ConductorSi},
	}
	for _, c := range casos {
		got, motivo := EvaluarInteresConductor(c.asunto, c.cuerpo, c.cv)
		if got != c.quiero {
			t.Errorf("%s: got %q (%s); quería %q", c.nombre, got, motivo, c.quiero)
		}
		if motivo == "" {
			t.Errorf("%s: falta el motivo", c.nombre)
		}
	}
}

func TestCedulaEnTextoVariantes(t *testing.T) {
	for texto, esperada := range map[string]string{
		"C.C. No. 1.037.123.456":             "1037123456",
		"Cédula de ciudadanía N° 80766779":   "80766779",
		"Documento de identidad: 71.555.123": "71555123",
	} {
		if got := cedulaEnTexto(texto); got != esperada {
			t.Errorf("cedulaEnTexto(%q) = %q; quería %q", texto, got, esperada)
		}
	}
}

func TestTextoCrudoDoc(t *testing.T) {
	// Latin-1 con "ó" (0xF3) y el mismo texto en UTF-16 little endian.
	latin := []byte("\x00\x01Hoja de vida Mar\xeda L\xf3pez Cel 314 551 5276\x00\x02")
	if got := textoCrudoDoc(latin); !strings.Contains(got, "314 551 5276") {
		t.Errorf("no rescató el teléfono del .doc Latin-1: %q", got)
	}
	var u16 []byte
	for _, r := range "C.C. 1035126774 Conductor" {
		u16 = append(u16, byte(r), 0)
	}
	if got := textoCrudoDoc(append([]byte{0, 0}, u16...)); !strings.Contains(got, "Conductor") {
		t.Errorf("no rescató el texto UTF-16: %q", got)
	}
}

func TestCedulaSinEtiqueta(t *testing.T) {
	casos := []struct{ nombre, texto, quiero string }{
		{"bajo datos personales", "Datos personales 1037123456 Medellín", "1037123456"},
		{"con puntos", "Nací en Bello. Documento de la persona 1.037.123.456", "1037123456"},
		{"el celular no es cédula", "Datos personales 3145515276", ""},
		{"salario no es cédula", "Aspiración salarial $1.300.000.000", ""},
		{"nada", "Experiencia en ventas", ""},
	}
	for _, c := range casos {
		if got := cedulaSinEtiqueta(c.texto); got != c.quiero {
			t.Errorf("%s: cedulaSinEtiqueta(%q) = %q; quería %q", c.nombre, c.texto, got, c.quiero)
		}
	}
}

func TestBoletinNoEsHojaDeVida(t *testing.T) {
	boletin := "El perfil del líder de talento humano. Experiencia, formación y cargo: tendencias de la empresa en 2027. Estudios de caso y competencias."
	if pareceHojaDeVida(boletin) {
		t.Error("un boletín de recursos humanos no es una hoja de vida (no trae datos personales)")
	}
}

func TestSolicitaNoEsSolicito(t *testing.T) {
	if _, ok := mencionaEmpleo("SOFTWARES DE RECURSOS HUMANOS: la empresa solicita una demostración"); ok {
		t.Error("«solicita» en publicidad no es búsqueda de empleo")
	}
	if _, ok := mencionaEmpleo("Postulasion a la vacante"); !ok {
		t.Error("«postulasion» sí es una postulación mal escrita")
	}
}

func TestCamposFormulario(t *testing.T) {
	cuerpo := "Hello there!\r\n   nombre   Ana María Gómez  \r\n   email   ana@correo.com  \r\n   area   Conductores\r\n   mensaje   Quiero trabajar\r\n   attachment(s)\r\n   CV_ANA.pdf"
	c := camposFormulario(cuerpo)
	if c["nombre"] != "Ana María Gómez" || c["email"] != "ana@correo.com" || c["area"] != "Conductores" {
		t.Errorf("campos mal leídos: %v", c)
	}
}

func TestNecesitaLecturaConIA(t *testing.T) {
	if !necesitaLecturaConIA("") || !necesitaLecturaConIA("(Tj) (ET) BT /F1 12 Tf q Q endobj stream x y z endstream xref startxref %%EOF  obj  <<>>") {
		t.Error("un PDF sin texto útil debe leerse con IA")
	}
	cv := "Datos personales Cédula 1037123456 Cel 314 551 5276 Experiencia laboral conductor Formación académica bachiller Referencias personales"
	if necesitaLecturaConIA(cv) {
		t.Error("un PDF con texto de hoja de vida no necesita IA")
	}
}
