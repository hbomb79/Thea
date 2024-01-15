package gen

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=types.cfg.yaml ../thea.openapi.yaml
//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=server.cfg.yaml -templates=./templates ../thea.openapi.yaml
