# REQUIREMENTS -- Proyecto YUPS
## ¿Qué es?
Es un CLI para ayudar al usuario de la terminal cuando lo necesita.

Ofrece ayuda rápida, precisa, y con la fricción mínima posible.

En ocasiones no recuerdas el nombre de un comando y los metodos rápidos no funcionan y los habituales proporcionan demasiada información; puede que no quieras leer toda una man page para verificar si has puesto las banderas adecuadas; quizá tienes tantas fuentes de información (apropos, whatis, help, tldr...) que no sabes por donde empezar; o símplemente no tienes claro lo que tienes que hacer pero te da una pereza terrible el cambio de contexto que supone ir a buscar información por Internet en un buscador, foro o LLM. Para todos esos momentos está YUPS.

YUPS, en una fracción de segundo recopila toda la información de contexto relevante, unifica la respuesta de múltiples fuentes y extrae quirúrgicamente la respuesta que puede serte de más ayuda.

Si es necesario, YUPS puede hacer uso de tu LLM local de confianza (en tu equipo o en tu red) para hacer un análisis más profundo de la situación y determinar el mejor camino a seguir.

Además es proactivo y no espera a que le pidas ayuda. Si cree que la necesitas, símplemente te la sugiere.

## Scope
- Sólo se busca ayudar a los usuarios de terminales habituales en Linux.
- Sólo se prevé apoyarse en modelos abiertos.
- El uso de modelos externos (Hugging Face) está por valorar, especialmente en términos de seguridad y velocidad.
- Sólo sirve para ofrecer ayuda interactiva, en ningún caso está previsto para ejecutarse de manera automatizada.

## Arquitectura core

Los componentes y modo de interaccionar de YUPS son:
 - YUPS es el punto central. Se puede ver como un ayudante. Un cliente o gestor de inferencia. Your Universal Prompt Straw boss (capataz), Solver (solucionador), Steward (administrador), Strategist (estratega), Shadow (sombra en el sentido de que está siempre siguiendo tus pasos y pendiente de lo que haces), Scout (explorador), Sentinel (centinela), Sidekick (compañero), Synthesizer (sintetizador -de sintetizar-), Safeguard (protector), Specialist (especialista), Substitute (sustituto), Supporter (partidario), Servant (sirviente).
 - Se engancha a bash [^1] mediante hooks para enterarse cuando surgen problemas.
 - Usa variables de entorno y otros comandos (history, pwd, ls...) para recopilar información.
 - En base a la situación determina cual es el mejor modo de encontrar ayuda.
 - Si la ayuda necesaria es básica la recopila de varias fuentes (man, apropos, which ...), la filtra y la muestra.
 - Si decide que la ayuda necesaria es avanzada la puede pedir al LLM conectando con un ollama [^2] local que esté expuesto por http (en la misma máquina o en otra de confianza).
 - Si tiene que preguntar a un LLM puede decidir qué modelo necesita en función de lo complejo que le parezca el problema o de si ya se le ha preguntado antes.
 - Un middleware en la máquina en la que esté el ollama pre-procesa la petición para determinar qué modelo usar en base a la lista proporcionada por YUPS y de si está alguno de esos modelos cargado en memoria. También puede priorizar peticiones interactivas y de procesamiento en segundo plano. Este middleware está por desarrollar y no forma parte de YUPS (es algo a instalar a parte) por lo que YUPS deberá poder configurarse para hablar con este middleware o directamente con ollama.
 - El LLM puede darle 4 tipos de respuesta:
    1. Un texto corto explicativo.
    2. Una propuesta de comando a ejecutar de una sola línea.
    3. Una propuesta de script a ejecutar.
    4. Una solicitud de más información mediante el uso de comandos autorizados en una lista blanca que se ejecutará automáticamente (siempre de manera que el usuario esté informado de lo que se está haciendo pero de un modo resumido que no lo sature por exceso de información ni lo frustre por no entender) y se mandará al modelo con el context previo. En este proceso, en cualquier momento YUPS puede decidir escalar y repetir la petición a un modelo superior.
- Una vez el LLM da una respuesta final se le presenta al usuario con una breve explicación que decide si ejecutarla (casos 2 y 3) respondiendo Sí, No o Editar.

[^1]: En el futuro se prevé incluir al menos zsh
[^2]: En el futuro se prevén incluir otros motores de inferencia locales como llama.cpp u openrouter.

```
# TODO: Diagrama que muestre la arquitectura de YUPS
```

## Información contextual valorada por yups
- Si se ha ejecutado a petición propia por detectar un problema o a petición del usuario mediante una pulsación de teclas (F1 o Ctrl+g) o por invocación directa del comando `yups`
- Lo que hay en este momento escrito en la línea de comandos
- La configuración que haya establecido el usuario
- Los flags que haya indicado el usuario si es una invocación directa
- El prompt que haya indicado el usuario si es una invocación directa
- Los comandos que se han lanzado anteriormente en la sesión actual
- El último error que se ha producido
- La distribución y la versión
- Los package managers disponibles
- El árbol del directorio padre hasta los directorios hijos
- La lista de detallada de archivos en el directorio actual
- Si está en un repositorio, el status
- La lista de comandos disponibles en el sistema

## Información adicional que recupera yups en función del contexto
- La man page del comando
- Si existe algún paquete que instale el comando inexistente

## Hooks y disparadores
- Cuando se produce un command not found (error 127) mediante el uso del handle `command_not_found_handle`.
```
# Example
command_not_found_handle() {
    if "/usr/local/bin/yups" --cnf-handle "$@"; then
        return $?
    else
        return 127
    fi
}
```
- Cuando se ejecuta un comando para checkear si se ha producido un error que haya que manejar de algún modo.
```
# Example
_yups_ce_handle() {
    local exit_code=$?
    # 0=OK; 127=Command not fund already managed; 130=SIGINT received (Ctrl+C for instance)
    if [[ $exit_code -eq 0 ]] || [[ $exit_code -eq 127 ]] || [[ $exit_code -eq 130]]; then
        return
    fi
    local last_command_text=$(history 1 | sed 's/^[ ]*[0-9]\+[ ]\+//')
    "/usr/local/bin/yups" --ce-handle "$exit_code" "$last_command_text"
}

if [[ -z "$PROMPT_COMMAND" ]]; then
    export PROMPT_COMMAND="_yups_ce_handle"
elif ! [[ "$PROMPT_COMMAND" == *"_yups_ce_handle"* ]]; then
    export PROMPT_COMMAND="_yups_ce_handle;${PROMPT_COMMAND}"
fi
```
- Cuando el usuario pulsa F1 o Ctrl+G (pulsaciones comunes para ayuda).
```
# Example
bind -x '"\eOP": explain_current_line'
bind -x '"\C-g": explain_current_line'
```
- Cuando el usuario invoca el comando yups.

## Formato de solicitudes a ollama o middleware
```
    {
            "model": "name-model:specific-flavour",  # default value to be used by ollama or to be used as preferred by the middleware
            "mw_models": ["name-model:specific-flavour", "name-model"], # With a list of accepted models
            "mw_election": "any", # To say how to select a model. Options: first (the first available), faster (the predicted to offer a shorter execution), loaded (the first loaded or model if no one is already loaded)
            "mw_type": "type", # If it is "interactive" or "background"
            "messages": [{"role": "system", "content": system_content}, {"role": "user", "content": user_query}],
            "max_tokens": 500,
            "temperature": 0.1, 
            "stream": False
            [, "context": # set response context if it is a follow up]
    }
```

# Lenguaje y prioridades
Se usará Go en todo lo que no requiera hacerse mediante bash

Las prioridades por orden son:
1. Velocidad de ejecución y respuesta.
1. Usabilidad.
1. Mantener informado al usuario sin saturar ni frustrar.
1. Ofrecer sensación de seguridad.


