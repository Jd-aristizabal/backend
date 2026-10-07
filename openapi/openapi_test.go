package openapi

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestSpecification(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err = doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}
