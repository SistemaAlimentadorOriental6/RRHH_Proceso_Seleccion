package agentes

import (
	"testing"

	"go-rrhh/modelos"
)

func TestEsDocumentoDeSoporte(t *testing.T) {
	casos := map[string]bool{
		"licencia ATM.pdf":                 true,
		"Cedula ATM.pdf":                   true,
		"CC Alejandro Tobar.pdf":           true,
		"Certificado laboral.pdf":          true,
		"foto.jpg":                         true,
		"Hoja de vida Alejandro Tobar.pdf": false,
		"HV con foto y cedula.pdf":         false,
		"Alejandro Tobar.docx":             false,
		"ATM.pdf":                          false,
		"Documentos .pdf":                  true,
		"adjunto_1.pdf":                    false,
	}
	for nombre, quiere := range casos {
		if got := esDocumentoDeSoporte(nombre); got != quiere {
			t.Errorf("esDocumentoDeSoporte(%q) = %v, quería %v", nombre, got, quiere)
		}
	}
}

func TestFusionarLimpiaTextoMezclado(t *testing.T) {
	viejo := modelos.HojaVidaCorreo{TextoCV: "HOJA DE VIDA ... LICENCIA DE CONDUCCIÓN No. 1128435427 transcrita", LeidoConIA: true}
	nueva := modelos.HojaVidaCorreo{TextoCV: "HOJA DE VIDA limpia", ParecidoCV: true}
	fusionarHoja(&viejo, nueva)
	if viejo.TextoCV != nueva.TextoCV || viejo.LeidoConIA {
		t.Errorf("el texto mezclado no se reemplazó: %+v", viejo)
	}
}
