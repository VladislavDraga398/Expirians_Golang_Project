//go:build tools

// Пакет tools предназначен для фиксации зависимостей инструментов.
// На текущем этапе генераторы protoc устанавливаются вручную:
//
//	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//
// поэтому импорты-заглушки не нужны.
package tools
