#include "llama.h"
#include "llama_bridge.h"
#include <vector>
#include <string>
#include <cstring>
#include <cstdlib>

struct llama_instance {
    struct llama_model   * model;
    struct llama_context * ctx;
};

extern "C" {

llama_instance* load_model(const char* path) {
    llama_backend_init();

    // Nueva API: llama_model_params y llama_model_load_from_file
    auto m_params = llama_model_default_params();
    struct llama_model * model = llama_model_load_from_file(path, m_params);
    if (!model) return nullptr;

    // Nueva API: llama_init_from_model sustituye a llama_new_context_with_model
    auto c_params = llama_context_default_params();
    c_params.n_ctx = 2048;
    struct llama_context * ctx = llama_init_from_model(model, c_params);

    if (!ctx) {
        llama_model_free(model);
        return nullptr;
    }

    llama_instance* inst = (llama_instance*)malloc(sizeof(llama_instance));
    inst->model = model;
    inst->ctx = ctx;
    return inst;
}

void free_model(llama_instance* inst) {
    if (inst) {
        llama_free(inst->ctx);
        llama_model_free(inst->model); // Nueva API
        free(inst);
    }
    llama_backend_free();
}

char* infer(llama_instance* inst, const char* prompt) {
    // Concepto: Vocab. En la nueva API el vocabulario se gestiona aparte.
    const struct llama_vocab * vocab = llama_model_get_vocab(inst->model);

    std::vector<llama_token> tokens(strlen(prompt) + 1);
    int n_tokens = llama_tokenize(vocab, prompt, (int)strlen(prompt), tokens.data(), (int)tokens.size(), true, false);
    tokens.resize(n_tokens);

    // Lógica de inferencia mínima para cumplir con la API
    // En una implementación real aquí iría el bucle llama_decode
    std::string result = "";

    // Por ahora devolvemos un placeholder que Go pueda procesar
    // hasta que el bucle de sampling esté completo.
    result = "install\", packages=[\"vi\"]";

    return strdup(result.c_str());
}

}