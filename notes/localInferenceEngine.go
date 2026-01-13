package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// CGO: Es el puente que permite a Go usar código de C/C++. Necesario aquí
	// para usar la potencia de cálculo de llama.cpp.
	llama "github.com/amenzhinsky/go-llama-cpp"
)

// Interpretation define la estructura de datos que esperamos de la IA.
type Interpretation struct {
	Action   string   `json:"action"`
	Packages []string `json:"packages"`
}

type Engine struct {
	model *llama.Model
}

// NewEngine carga el modelo en memoria. 
func NewEngine() (*Engine, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".yups/models/gemma-3-270m.gguf")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("modelo no encontrado")
	}

	// Cargamos el modelo con una configuración ligera para 270M
	model, err := llama.NewModel(path)
	if err != nil {
		return nil, fmt.Errorf("error cargando el modelo: %w", err)
	}

	return &Engine{model: model}, nil
}

// InterpretCommand analiza el comando del usuario.
func (e *Engine) InterpretCommand(ctx context.Context, input string) (*Interpretation, error) {
	// Prompt para FunctionGemma 3
	prompt := fmt.Sprintf(`<start_of_turn>user
Analyze the package manager command and extract the action (install, remove, search, upgrade, update, provides, autoremove) and packages.
Command: "%s"
<end_of_turn>
<start_of_turn>model
<start_function_call>extract_command(action="`, input)

	// Inferencia: El proceso de predecir la siguiente palabra basada en los pesos del modelo.
	out, err := e.model.Predict(ctx, prompt,
		llama.WithStopWords("<end_function_call>"),
		llama.WithMaxTokens(128),
		llama.WithTemperature(0.1), // Temperatura baja = Menos "creatividad", más precisión.
	)
	if err != nil {
		return nil, err
	}

	return parseRawOutput(out)
}

// InterpretProvides identifica el paquete específico.
func (e *Engine) InterpretProvides(ctx context.Context, output string) (string, error) {
	prompt := fmt.Sprintf(`<start_of_turn>user
From the following system output, identify the specific package name that provides the requested file.
Output:
"%s"
<end_of_turn>
<start_of_turn>model
<start_function_call>extract_provides(package="`, output)

	out, err := e.model.Predict(ctx, prompt,
		llama.WithStopWords("<end_function_call>"),
		llama.WithTemperature(0.1),
	)
	if err != nil {
		return "", err
	}

	// Extraemos el nombre del paquete de la cadena (ej: "nano\"")
	pkg := strings.Split(out, `"`)[0]
	return pkg, nil
}

func parseRawOutput(raw string) (*Interpretation, error) {
	// Lógica para limpiar la salida del modelo y convertirla en struct.
	// FunctionGemma suele responder: install", packages=["nano", "vim"])
	
	action := strings.Split(raw, `"`)[0]
	
	// Una forma rápida de extraer paquetes sin un parser de JSON pesado
	parts := strings.Split(raw, "[")
	if len(parts) < 2 {
		return &Interpretation{Action: action}, nil
	}
	
	pkgPart := strings.Split(parts[1], "]")[0]
	pkgs := strings.Split(pkgPart, ",")
	for i, p := range pkgs {
		pkgs[i] = strings.Trim(p, ` "`)
	}

	return &Interpretation{
		Action:   action,
		Packages: pkgs,
	}, nil
}
