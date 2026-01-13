#ifndef LLAMA_BRIDGE_H
#define LLAMA_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

// Concepto: Opaque Struct. Definimos la estructura aquí pero su contenido 
// es privado en el .cpp para no exponer dependencias de C++ a Go.
typedef struct llama_instance llama_instance;

llama_instance* load_model(const char* path);
void free_model(llama_instance* inst);
char* infer(llama_instance* inst, const char* prompt);

#ifdef __cplusplus
}
#endif

#endif
