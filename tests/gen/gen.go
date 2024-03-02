package gen

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=types.cfg.yaml ../../internal/api/thea.openapi.yaml
//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=client.cfg.yaml ../../internal/api/thea.openapi.yaml
//go:generate go run github.com/vektra/mockery/v2@v2.42.0
