package permissions

import "testing"

func TestConversoesOctais(t *testing.T) {
	tests := []struct {
		value, symbolic    string
		suid, sgid, sticky bool
	}{
		{"755", "-rwxr-xr-x", false, false, false}, {"770", "-rwxrwx---", false, false, false},
		{"640", "-rw-r-----", false, false, false}, {"1777", "-rwxrwxrwt", false, false, true},
		{"2755", "-rwxr-sr-x", false, true, false}, {"4755", "-rwsr-xr-x", true, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			mode, err := Parse(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			info := FromFileMode(mode)
			if info.SymbolicMode != tc.symbolic {
				t.Fatalf("modo simbólico: %q", info.SymbolicMode)
			}
			if info.SUID != tc.suid || info.SGID != tc.sgid || info.Sticky != tc.sticky {
				t.Fatalf("bits especiais incorretos: %+v", info)
			}
		})
	}
}

func TestModoInvalido(t *testing.T) {
	for _, value := range []string{"", "88", "999", "10000", "7a5", "-755"} {
		if _, err := Parse(value); err == nil {
			t.Errorf("%q deveria falhar", value)
		}
	}
}

func TestExplicacaoArquivoEDiretorio(t *testing.T) {
	mode, _ := Parse("770")
	info := FromFileMode(mode)
	file := Explain(info, false)
	dir := Explain(info, true)
	if file[0].Codes[0] != "file.read.allowed" {
		t.Fatalf("arquivo: %+v", file)
	}
	if dir[0].Codes[2] != "dir.execute.allowed" {
		t.Fatalf("diretório: %+v", dir)
	}
	if dir[2].Codes[0] != "dir.read.denied" {
		t.Fatalf("outros: %+v", dir)
	}
}
