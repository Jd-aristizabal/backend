package transport

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictJSON(t *testing.T) {
	for _, body := range []string{`{"title":"hola","unknown":1}`, `{"title":"hola"} {}`, `{`, strings.Repeat("x", 17000)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var v struct {
			Title string `json:"title"`
		}
		if Decode(httptest.NewRecorder(), r, &v) == nil {
			t.Fatal("JSON inválido aceptado")
		}
	}
}
func TestUUID(t *testing.T) {
	for _, v := range []string{"", "00000000-0000-0000-0000-000000000000", "abc", "c76b8c45bad44b24b8d6cd7df5440b61"} {
		if _, err := UUID(v); err == nil {
			t.Fatal("UUID inválido aceptado")
		}
	}
}
