package main

import (
	"testing"

	"github.com/xcreativs/terios/api/internal/config"
)

func TestBuildPaymentServiceWithoutCredentialsIsNilInterface(t *testing.T) {
	var svc any = buildPaymentService(config.Config{}, nil, nil)
	if svc != nil {
		t.Fatalf("missing gateway credentials returned a typed nil service: %#v", svc)
	}
}
