package ai

/*
// #cgo: Configuración de compilación para CGO.

// CPPFLAGS: -I (Include).
// Añadimos tanto la carpeta de cabeceras de llama como la de ggml.
#cgo CPPFLAGS: -I${SRCDIR}/../../llama.cpp/include -I${SRCDIR}/../../llama.cpp/ggml/include

// LDFLAGS: -L (Library path) y -Wl,-rpath (Runtime search path).
#cgo LDFLAGS: -L${SRCDIR}/../../llama.cpp/build/bin -lllama -Wl,-rpath,${SRCDIR}/../../llama.cpp/build/bin

#include <stdlib.h>
#include "llama_bridge.h"
*/
import "C"
import (
	"context"
	"fmt"
	"strings"
	"unsafe"
)

// Conceptos técnicos:
// - rpath: Ruta incrustada en el binario para que el cargador del sistema operativo encuentre
//   las librerías dinámicas (.so) sin configurar variables de entorno globales.

type Interpretation struct {
	Action   string
	Packages []string
}

type Engine struct {
	instance *C.llama_instance
}

// NewEngine inicializa el motor cargando el modelo GGUF.
func NewEngine(modelPath string) (*Engine, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	inst := C.load_model(cPath)
	if inst == nil {
		return nil, fmt.Errorf("no se pudo cargar el modelo GGUF desde %s", modelPath)
	}

	return &Engine{instance: inst}, nil
}

// Close libera la memoria reservada en el lado de C++.
func (e *Engine) Close() {
	if e.instance != nil {
		C.free_model(e.instance)
	}
}

func (e *Engine) InterpretCommand(ctx context.Context, input string) (*Interpretation, error) {
	prompt := fmt.Sprintf("<start_of_turn>user\nAnalyze: %s\n<end_of_turn>\n<start_of_turn>model\n<start_function_call>extract_command(action=\"", input)

	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	cResult := C.infer(e.instance, cPrompt)
	defer C.free(unsafe.Pointer(cResult))

	res := C.GoString(cResult)
	return parseRawOutput(res), nil
}

func (e *Engine) InterpretProvides(ctx context.Context, output string) (string, error) {
	prompt := fmt.Sprintf("<start_of_turn>user\nIdentify package in: %s\n<end_of_turn>\n<start_of_turn>model\n<start_function_call>extract_provides(package=\"", output)

	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	cResult := C.infer(e.instance, cPrompt)
	defer C.free(unsafe.Pointer(cResult))

	res := C.GoString(cResult)
	return strings.Split(res, `"`)[0], nil
}

func parseRawOutput(raw string) *Interpretation {
	action := strings.Split(raw, `"`)[0]
	pkgs := []string{}
	if strings.Contains(raw, "[") {
		pkgPart := strings.Split(strings.Split(raw, "[")[1], "]")[0]
		for _, p := range strings.Split(pkgPart, ",") {
			pkgs = append(pkgs, strings.Trim(p, ` "`))
		}
	}
	return &Interpretation{Action: action, Packages: pkgs}
}
