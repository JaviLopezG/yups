package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// CGO: Puente que permite a Go ejecutar código C/C++ (necesario para llama.cpp).
	// Esta librería es un wrapper de llama.cpp, el estándar de facto para inferencia local.
	llama "github.com/go-skynet/go-llama.cpp"
)

// Interpretation: Datos estructurados devueltos por la IA tras analizar un comando.
type Interpretation struct {
	Action   string   `json:"action"`
	Packages []string `json:"packages"`
}

type Engine struct {
	model *llama.Model
}

// NewEngine: Carga el modelo GGUF desde el directorio de YUPS.
// Nota: 'safetensors' debe convertirse a 'GGUF' usando scripts como 'convert_hf_to_gguf.py'
// o descargando directamente la versión GGUF desde Hugging Face.
func NewEngine() (*Engine, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".yups/models/gemma-3-270m.gguf")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("modelo no encontrado en %s. Asegúrate de que sea formato GGUF", path)
	}

	// Cargamos el modelo en memoria.
	// La inferencia es el proceso de generar una respuesta usando los pesos del modelo.
	model, err := llama.NewModel(path)
	if err != nil {
		return nil, fmt.Errorf("fallo al cargar el modelo: %w", err)
	}

	return &Engine{model: model}, nil
}

// InterpretCommand: Usa Function Calling para extraer la acción y paquetes de un string.
func (e *Engine) InterpretCommand(ctx context.Context, input string) (*Interpretation, error) {
	// Prompt: Texto de instrucción que guía al modelo.
	// Usamos el formato de turnos de Gemma (<start_of_turn>).
	prompt := fmt.Sprintf(`<start_of_turn>user
Analyze the command and extract the action and packages: "%s"
<end_of_turn>
<start_of_turn>model
<start_function_call>extract_command(action="`, input)

	// Inferencia local: Predecimos los tokens (unidades de texto) de la respuesta.
	out, err := e.model.Predict(ctx, prompt,
		llama.WithStopWords("<end_function_call>"),
		llama.WithTemperature(0.1), // Baja temperatura = respuesta más determinista (menos "creativa").
	)
	if err != nil {
		return nil, err
	}

	return parseRawOutput(out)
}

func parseRawOutput(raw string) (*Interpretation, error) {
	// Parser para limpiar la respuesta del modelo.
	action := strings.Split(raw, `"`)[0]
	pkgs := []string{}

	if strings.Contains(raw, "[") {
		pkgPart := strings.Split(strings.Split(raw, "[")[1], "]")[0]
		for _, p := range strings.Split(pkgPart, ",") {
			pkgs = append(pkgs, strings.Trim(p, ` "`))
		}
	}

	return &Interpretation{Action: action, Packages: pkgs}, nil
}
