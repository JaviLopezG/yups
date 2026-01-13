# **Informe de Investigación: Especialización de LLMs de Peso Abierto para Análisis y Corrección de Comandos Bash en Infraestructuras Locales (16GB VRAM)**

## **1\. Resumen Ejecutivo y Definición del Alcance**

La administración de sistemas Linux y la ingeniería de fiabilidad del sitio (SRE) han experimentado una transformación radical con la llegada de la inteligencia artificial generativa. Sin embargo, la naturaleza crítica de la infraestructura de TI —donde un solo error de sintaxis en un script Bash puede provocar interrupciones masivas del servicio o pérdidas irreversibles de datos— exige un nivel de precisión y seguridad que los modelos de lenguaje generalistas a menudo no logran garantizar. Además, las estrictas políticas de privacidad de datos en entornos corporativos y gubernamentales restringen el uso de APIs de modelos propietarios (como GPT-4 o Claude 3.5 Sonnet) cuando se trata de registros de servidores, configuraciones de red o código propietario.1

Este informe aborda la viabilidad técnica y las metodologías óptimas para desplegar Grandes Modelos de Lenguaje (LLMs) de peso abierto en entornos locales, restringidos por un presupuesto de hardware de **16GB de VRAM** —el estándar de facto para estaciones de trabajo de desarrollo de gama alta en 2025 (e.g., NVIDIA GeForce RTX 4080, Apple Silicon M3 Pro/Max).2 El objetivo central es determinar la arquitectura más efectiva para el análisis, generación y corrección de comandos Linux Bash, contrastando dos paradigmas predominantes: el uso de modelos técnicos intrínsecamente especializados en código (Code LLMs) frente a la adaptación de modelos generalistas mediante técnicas de Generación Aumentada por Recuperación (RAG) y refinamiento de prompts.

El análisis revela que, si bien los modelos generalistas potenciados con RAG ofrecen una precisión fáctica superior en la documentación de banderas oscuras, los modelos especializados de rango medio (14B-22B parámetros), ejecutados mediante cuantización optimizada, proporcionan una capacidad de razonamiento sintáctico y lógico superior para la estructuración de scripts complejos. La convergencia de estas técnicas en arquitecturas híbridas, orquestadas localmente mediante herramientas como Ollama, emerge como la solución definitiva para superar las limitaciones de memoria sin comprometer la capacidad de inferencia.3

## **2\. Restricciones de Hardware y Dinámica de Memoria en Entornos Locales**

La limitación de 16GB de VRAM no es simplemente un tope numérico; define la frontera termodinámica de lo que es posible en la inferencia local de baja latencia. Para comprender por qué ciertos modelos son viables y otros no, es imperativo diseccionar cómo los LLMs consumen memoria durante la inferencia y cómo las técnicas de cuantización modernas alteran esta ecuación.

### **2.1 Anatomía del Consumo de VRAM en Inferencia**

El consumo de memoria de video (VRAM) durante la ejecución de un LLM se compone de dos factores principales: el almacenamiento de los **pesos del modelo** y el **caché KV (Key-Value)** dinámico generado durante la conversación.

1. **Pesos del Modelo (Estático):** Es el espacio base requerido simplemente para cargar el modelo. En precisión media estándar (FP16), cada parámetro ocupa 2 bytes. Por ende, un modelo de 7 billones de parámetros (7B) requiere aproximadamente 14GB, dejando apenas 2GB libres en una tarjeta de 16GB, lo cual es insuficiente para el sistema operativo y el contexto de la ventana.2  
2. **Caché KV (Dinámico):** A medida que el modelo procesa la entrada (prompt) y genera la salida, debe almacenar los estados de atención de los tokens anteriores para mantener la coherencia. El tamaño de este caché crece linealmente con la longitud de la ventana de contexto y el tamaño del modelo. En tareas de análisis de scripts Bash, donde se pueden inyectar manuales técnicos enteros o logs de errores extensos, el caché KV puede consumir fácilmente varios gigabytes adicionales.

Esta dinámica obliga al uso de **cuantización**, una técnica que reduce la precisión de los pesos de 16 bits a 8, 6, 5, o 4 bits. En 2025, el formato GGUF y las cuantizaciones "K-quants" (como Q4\_K\_M) han alcanzado una eficiencia tal que la degradación en la perplejidad (una medida de "confusión" del modelo) es casi imperceptible para tareas de codificación, mientras que el ahorro de memoria es masivo.5

### **2.2 Viabilidad de Arquitecturas en 16GB VRAM**

A continuación se presenta un análisis detallado de la viabilidad de las arquitecturas de modelos predominantes en 2025 bajo la restricción de 16GB, considerando una cuantización agresiva pero funcional (4-bit) para maximizar el tamaño del modelo utilizable.

| Arquitectura del Modelo | Parámetros (Densos/Activos) | Cuantización (Recomendada) | VRAM Requerida (Modelo) | Margen para Contexto (16GB Total) | Idoneidad para Bash |
| :---- | :---- | :---- | :---- | :---- | :---- |
| **Llama 3.1 / Qwen2.5** | 8B / 7B | Q8\_0 (8-bit) | \~8.5 GB | \~7.5 GB (Excelente) | **Alta**. Permite contextos masivos (\>32k tokens) ideales para RAG. |
| **Mistral-Nemo / Phi-4** | 12B / 14B | Q6\_K (6-bit) | \~10-11 GB | \~4-5 GB (Moderada) | **Muy Alta**. Equilibrio perfecto entre razonamiento y memoria. |
| **Qwen2.5-Coder** | 14B | Q4\_K\_M (4-bit) | \~9.0 GB | \~6.5 GB (Buena) | **Óptima**. El estándar de oro actual para codificación local. |
| **DeepSeek-Coder-V2-Lite** | 16B (MoE, 2.4B Activos) | Q4\_K\_M | \~10.5 GB | \~5 GB (Aceptable) | **Alta**. Arquitectura MoE eficiente en cómputo pero exigente en VRAM. |
| **Gemma 3** | 27B | Q3\_K\_S (3-bit) | \~14.5 GB | \< 1.5 GB (Crítica) | **Baja**. Requiere cuantización destructiva; deja poco espacio para RAG. |
| **Qwen2.5-Coder** | 32B | IQ2\_XS (2-bit) | \~13 GB | \~2-3 GB (Limitada) | **Media**. La cuantización de 2 bits degrada severamente la sintaxis técnica. |

Implicaciones del Hardware:  
El análisis de 2 y 5 sugiere que intentar ejecutar modelos de 30B+ parámetros en 16GB VRAM mediante descarga parcial a la RAM del sistema (CPU offloading) resulta en una caída precipitada del rendimiento, pasando de \~40-50 tokens/segundo (lectura humana rápida) a \~2-5 tokens/segundo. Para la corrección interactiva de comandos en una terminal, esta latencia es inaceptable, rompiendo el flujo cognitivo del ingeniero. Por tanto, la investigación se centra en optimizar modelos en el rango de 7B a 14B o arquitecturas MoE (Mezcla de Expertos) ligeras.

## **3\. Panorama de Modelos de Peso Abierto en 2025**

El ecosistema de modelos abiertos ha evolucionado desde simples imitadores de GPT-3 hacia familias especializadas con capacidades distintas. Para el dominio de Bash, dos linajes destacan: los modelos de propósito general altamente capaces y los modelos técnicos diseñados específicamente para lenguajes de programación y sistemas.

### **3.1 Modelos Generalistas Adaptables**

Estos modelos sobresalen en la comprensión de instrucciones en lenguaje natural y razonamiento de sentido común, pero su entrenamiento no prioriza la sintaxis estricta de lenguajes de scripting oscuros.

* **Llama 3.1 8B (Meta):** Continúa siendo una referencia por su robustez en instrucciones complejas. Su capacidad para explicar conceptos ("¿Por qué este comando falló?") es superior a menudo a modelos de código más "robóticos". Sin embargo, tiende a ser conservador y a rechazar comandos administrativos legítimos por falsos positivos de seguridad si no se ajusta el *System Prompt*.6  
* **Gemma 2 9B (Google):** Destaca por una ventana de contexto efectiva y capacidades multilingües, lo cual es útil si la documentación o los comentarios del script están en idiomas distintos al inglés. Su arquitectura densa ofrece un rendimiento predecible.8  
* **Mistral-Nemo 12B (Mistral AI/NVIDIA):** Un modelo de tamaño intermedio diseñado para caber exactamente en la memoria de GPUs de gama media (como la RTX 4070/3060 de 12GB, y por ende holgadamente en 16GB). Ofrece una capacidad de razonamiento lógico superior a los modelos de 7B-9B.10

### **3.2 Modelos Técnicos Especializados (Code LLMs)**

Esta categoría ha visto el avance más significativo en 2025, con modelos que rivalizan con GPT-4 en tareas de programación pura.

* **Qwen2.5-Coder (Alibaba Cloud):** En sus variantes de 7B y 14B, este modelo se ha establecido como el líder indiscutible en benchmarks de código abierto.4 Entrenado con un corpus masivo de 5.5 billones de tokens enfocados en código, documentación técnica y matemáticas, posee una comprensión "nativa" de la sintaxis Bash que los modelos generalistas solo emulan. Su capacidad para predecir no solo el comando, sino los argumentos correctos basados en el contexto del sistema de archivos, es notable. Además, soporta nativamente *Tool Calling*, permitiendo integraciones avanzadas.4  
* **DeepSeek-Coder-V2-Lite (DeepSeek AI):** Este modelo utiliza una arquitectura *Mixture-of-Experts (MoE)*. A diferencia de un modelo denso que usa todos sus parámetros para cada palabra, el MoE activa solo un subconjunto de "expertos" (redes neuronales especializadas) para cada token.13 Con \~16B parámetros totales pero solo \~2.4B activos, ofrece la velocidad de un modelo pequeño con la base de conocimiento de uno grande. Es excepcionalmente fuerte en lenguajes de scripting y configuración (YAML, JSON, Bash).14  
* **Codestral 22B (Mistral AI):** Específicamente diseñado para "Fill-in-the-Middle" (FIM), ideal para autocompletar scripts a mitad de escritura. Sin embargo, su tamaño de 22B lo coloca en el límite absoluto de los 16GB VRAM, requiriendo cuantizaciones más agresivas que pueden afectar la precisión sintáctica fina necesaria para comandos Bash complejos.16

Análisis Comparativo de Rendimiento en Benchmarks:  
Datos recientes de benchmarks como HumanEval y MBPP (modificados para Bash) muestran que Qwen2.5-Coder-14B supera consistentemente a Llama 3.1 8B y se acerca peligrosamente a modelos propietarios mucho mayores.4 En tareas de scripting Bash, la capacidad de Qwen para manejar tuberías (pipes), redirecciones y sustituciones de procesos es superior, probablemente debido a una mayor proporción de código de sistemas en su pre-entrenamiento.

## **4\. Metodología 1: Adaptación de Modelos Generalistas mediante RAG**

La adaptación de modelos generalistas (como Llama 3.1) para tareas técnicas específicas mediante *Retrieval-Augmented Generation* (RAG) es una estrategia poderosa para superar el desconocimiento de banderas específicas o herramientas de línea de comandos oscuras. A diferencia del entrenamiento, RAG proporciona acceso a una "memoria externa" actualizable.

### **4.1 Desafíos de RAG con Documentación de Linux (Man Pages)**

Las páginas de manual (man pages) de Linux presentan desafíos únicos de ingestión de datos que difieren del procesamiento estándar de PDFs o HTMLs corporativos.

1. **Formato Arcaico:** Las man pages utilizan el sistema de composición troff/groff. Intentar leer el archivo fuente directamente resulta en una sopa de caracteres de control ininteligible para el modelo de embeddings.  
2. **Estructura No Lineal:** La información crítica (ej. qué hace la bandera \-r) está aislada en la sección de opciones, mientras que ejemplos de uso pueden estar al final del documento. Un chunking (fragmentación) lineal ciego rompería esta relación semántica.

### **4.2 Arquitectura del Pipeline RAG Local con Ollama**

Para implementar un sistema RAG efectivo en una máquina local con Ollama, se debe construir una tubería de datos robusta.

#### **Fase 1: Extracción y Limpieza**

Es necesario renderizar la página man a texto plano eliminando los caracteres de formato heredados (como el retroceso ^H usado para negritas en terminales antiguas).

* **Comando de Extracción:** man \-Tutf8 \--html=cat comando | col \-b o utilizando librerías de Python como subprocess para capturar la salida limpia.18  
* **Limpieza Python:** Scripts personalizados deben eliminar encabezados y pies de página recurrentes que añaden ruido a los embeddings.

#### **Fase 2: Fragmentación Semántica (Semantic Chunking)**

En lugar de dividir el texto cada 500 caracteres, el fragmentador debe respetar la estructura lógica de la documentación técnica.

* **Estrategia:** Detectar patrones de inicio de línea como \-f, \--force para identificar el comienzo de la definición de una bandera.  
* **Agrupación:** Cada fragmento debe contener la bandera y su descripción completa. Si la descripción es muy larga, debe subdividirse pero manteniendo la referencia a la bandera en el metadato del fragmento. Esto asegura que cuando el usuario pregunte "¿Qué hace \--force?", el sistema recupere exactamente ese bloque.20

#### **Fase 3: Embeddings y Almacenamiento Vectorial**

Dado el límite de VRAM, el modelo de embeddings no debe competir agresivamente por recursos con el LLM principal.

* **Modelo Recomendado:** nomic-embed-text-v1.5 (disponible en Ollama). Es un modelo de alta calidad con una ventana de contexto de 8192 tokens (matryoshka embeddings), lo que permite indexar secciones enteras de manuales sin truncamiento, ocupando menos de 300MB de VRAM.21  
* **Base de Datos:** **ChromaDB** ejecutándose localmente es ideal por su integración simple con Python y su ligereza. Alternativamente, **FAISS** ofrece mayor velocidad pura pero menos flexibilidad de gestión de metadatos.22

#### **Fase 4: Recuperación y Orquestación**

Cuando el usuario introduce una consulta sobre un comando:

1. El sistema busca en ChromaDB los fragmentos más relevantes.  
2. Se construye un "Meta-Prompt" que incluye estos fragmentos bajo una sección \#\#\# CONTEXTO.  
3. Ollama recibe este prompt enriquecido.

Impacto en Modelos Generalistas:  
La aplicación de RAG transforma un modelo como Llama 3.1 8B. Sin RAG, el modelo podría alucinar que tar \-z usa algoritmo bzip2 (incorrecto, es gzip). Con RAG, el modelo "lee" el fragmento recuperado que dice explícitamente "filter the archive through gzip", corrigiendo su respuesta al instante. Esta técnica iguala el terreno de juego fáctico entre modelos pequeños y grandes.3

## **5\. Metodología 2: Modelos Especializados y Prompt Engineering Avanzado**

Si bien RAG soluciona la falta de conocimiento, no mejora la capacidad intrínseca de razonamiento lógico del modelo. Aquí es donde los modelos especializados como **Qwen2.5-Coder** brillan, y donde el refinamiento del *System Prompt* es crucial.

### **5.1 Configuración del Modelfile en Ollama**

El Modelfile es el archivo de configuración que define la personalidad y parámetros operativos del modelo en Ollama. Para tareas de Bash, la configuración por defecto de los modelos de chat (diseñados para ser conversacionales y creativos) es subóptima.

**Optimización de Parámetros:**

* **Temperatura (temperature):** Debe reducirse drásticamente, al rango de **0.1 \- 0.2**. Bash es un lenguaje determinista; no queremos creatividad en la sintaxis de un comando.  
* **Penalización de Frecuencia (repeat\_penalty):** Aumentar ligeramente (ej. 1.1) para evitar bucles repetitivos en la generación de código.  
* **Ventana de Contexto (num\_ctx):** Maximizar según la VRAM disponible. Para Qwen2.5-14B en 16GB, un contexto de **8192 tokens** es seguro y suficiente para la mayoría de scripts.25

### **5.2 Diseño del System Prompt Técnico**

Un System Prompt eficaz para un experto en Bash debe establecer reglas claras de seguridad y estilo.

**Ejemplo de System Prompt Refinado:**

"Eres un Ingeniero DevOps Senior especializado en scripting Bash y seguridad en Linux.  
Tu objetivo es generar, analizar y corregir comandos de terminal con precisión quirúrgica.  
REGLAS OPERATIVAS:

1. **Seguridad Ante Todo:** Analiza cada solicitud buscando potencial destructivo (rm \-rf, dd, mkfs). Si detectas riesgo, emite una ADVERTENCIA EN MAYÚSCULAS antes del código.  
2. **Sintaxis Moderna:** Utiliza prácticas modernas de Bash 4.0+. Prefiere \[\[ \]\] sobre \[ \], y $(...) sobre backticks.  
3. **Robustez:** Cita siempre las variables ("$VAR") para evitar división de palabras. Maneja errores con set \-e o comprobaciones condicionales || exit 1\.  
4. **Explicación Estructurada:** Proporciona primero el bloque de código corregido, seguido de una explicación concisa de los cambios. No charles innecesariamente.  
5. **Pensamiento Cadena (CoT):** Antes de responder, piensa paso a paso en la tubería de datos del comando para asegurar que la salida de uno es compatible con la entrada del siguiente."

Este prompt aprovecha la capacidad de seguimiento de instrucciones de modelos como Qwen2.5 y Llama 3.1 para imponer un estándar de calidad.27

## **6\. Integración Avanzada: Agentes y Tool Calling (Uso de Herramientas)**

La frontera de la IA en 2025 no es solo chatear, sino actuar. Los modelos compatibles con *Tool Calling* pueden interactuar con el entorno Linux para verificar sus propias hipótesis, actuando como agentes autónomos supervisados.

### **6.1 Capacidades de Tool Calling en Qwen2.5 y Ollama**

Ollama ha implementado soporte para que modelos como Qwen2.5-Coder y Llama 3.1 soliciten la ejecución de funciones externas.4 Esto permite un flujo bidireccional:

1. El Usuario pregunta.  
2. El Modelo razona que necesita información externa y emite una llamada a función (JSON estructurado).  
3. El sistema anfitrión (script Python) ejecuta la función y devuelve el resultado.  
4. El Modelo incorpora el resultado y genera la respuesta final.

### **6.2 Herramientas Esenciales para un Agente Bash**

Para un asistente de corrección de comandos, se pueden definir herramientas específicas:

1. **check\_syntax(script\_content):**  
   * *Implementación:* Ejecuta bash \-n sobre el contenido.  
   * *Uso:* Permite al modelo verificar si su código generado tiene errores de sintaxis antes de mostrárselo al usuario.  
2. **lookup\_man\_page(command):**  
   * *Implementación:* Ejecuta la extracción de man page descrita en la sección RAG.  
   * *Uso:* El modelo decide autónomamente cuándo necesita leer el manual, en lugar de depender de una recuperación RAG pasiva. Esto es más eficiente en tokens.  
3. **list\_directory(path):**  
   * *Implementación:* Ejecuta ls \-F.  
   * *Uso:* Permite al modelo ver los archivos reales para corregir errores de "File not found" o sugerir nombres de archivo correctos en el script.30

### **6.3 Seguridad en Agentes**

La capacidad de ejecutar comandos (incluso de lectura como ls) introduce riesgos.

* **Sandboxing:** Las herramientas deben ejecutarse con privilegios mínimos, idealmente dentro de un contenedor o un usuario restringido.  
* **Human-in-the-loop:** Para cualquier herramienta que modifique el estado (write\_file, execute\_command), el sistema debe pausar y solicitar confirmación explícita del usuario (Y/n), mostrando exactamente qué comando se va a ejecutar.27

## **7\. Análisis Comparativo y Benchmarks**

Para validar las estrategias, contrastamos el rendimiento esperado de las distintas configuraciones.

### **7.1 Comparativa de Modelos y Estrategias**

| Criterio | Modelo Generalista (Llama 3.1 8B) | Modelo Generalista \+ RAG | Modelo Especializado (Qwen2.5-Coder 14B) | Híbrido (Qwen 14B \+ RAG) |
| :---- | :---- | :---- | :---- | :---- |
| **Precisión Sintaxis Bash** | Media (falla en edge cases) | Media-Alta | **Alta** (Nativa) | **Muy Alta** |
| **Conocimiento de Banderas** | Bajo (Alucinaciones frecuentes) | **Muy Alta** (Recupera Doc) | Alta (Entrenamiento denso) | **Excelente** |
| **Razonamiento Lógico** | Alto (Lenguaje Natural) | Alto | **Muy Alto** (Lógica de Código) | **Muy Alto** |
| **Uso de Recursos (16GB)** | Bajo (\~6GB) | Medio (\~7GB) | Medio-Alto (\~10GB) | **Alto (\~11GB)** |
| **Latencia Inferencia** | Muy Rápida | Lenta (Retrieval overhead) | Rápida | Media |
| **Seguridad (Detección)** | Media (Tiende a ser paranoico) | Alta (Contextualizada) | Alta (Entiende el riesgo real) | **Muy Alta** |

Insight Crítico de Segundo Orden:  
La tabla revela que la Arquitectura Híbrida (Modelo Especializado \+ RAG) ofrece el rendimiento máximo. Aunque Qwen2.5-Coder tiene un conocimiento interno vasto, la adición de RAG actúa como un mecanismo de verificación de hechos en tiempo real para banderas que pueden variar entre versiones de Linux (ej. netstat vs ss, ifconfig vs ip). Dado que Qwen 14B cuantizado a 4 bits ocupa unos 9GB, deja un margen de \~7GB en una tarjeta de 16GB, espacio más que suficiente para cargar el modelo de embeddings y manejar el contexto adicional del RAG sin incurrir en Out Of Memory (OOM).

### **7.2 Resultados en Benchmarks Sintéticos vs. Reales**

Mientras que benchmarks como HumanEval muestran puntuaciones altas (\>70%) para estos modelos, la realidad operativa es más matizada.

* **InterCode-Bash:** Un benchmark interactivo que mide la capacidad de resolver tareas reales en un entorno Bash. Qwen2.5-Coder demuestra una tasa de éxito significativamente mayor que Llama 3.1 debido a su capacidad para encadenar múltiples comandos lógicamente (pipe reasoning).33  
* **Falsos Positivos de Seguridad:** Los modelos generalistas a menudo se niegan a generar comandos de administración de discos (dd, fdisk) alegando seguridad, incluso cuando el usuario es administrador y el contexto es legítimo. Los modelos especializados, con un System Prompt adecuado, entienden mejor la diferencia entre "destructivo malicioso" y "administración necesaria", proporcionando el comando con las advertencias adecuadas.35

## **8\. Recomendación de Arquitectura Final y Despliegue**

Basado en la investigación exhaustiva, se propone la siguiente arquitectura de referencia para desplegar un asistente de Bash en hardware de 16GB VRAM en 2025\.

### **8.1 Stack Tecnológico Recomendado**

1. **Hardware Base:** GPU NVIDIA RTX 4080 (16GB) o Mac M2/M3 Pro (16GB Unificados).  
2. **Motor de Inferencia:** **Ollama** (v0.5.x o superior) por su gestión eficiente de memoria y soporte GGUF.37  
3. **Modelo LLM:** **Qwen2.5-Coder-14B-Instruct** cuantizado a **Q4\_K\_M**.  
   * *Justificación:* Ocupa \~9GB VRAM. Es el modelo más inteligente que cabe cómodamente sin sacrificar velocidad. Supera a modelos de 7B y a versiones más cuantizadas de modelos grandes.  
4. **Sistema RAG:**  
   * **Embeddings:** nomic-embed-text-v1.5 (Ollama).  
   * **Base Vectorial:** ChromaDB (Local).  
   * **Corpus:** Páginas man de las 500 herramientas más usadas en el sistema, procesadas y fragmentadas por opción.  
5. **Interfaz de Usuario:** **Open WebUI**.  
   * *Justificación:* Permite conectar Ollama, gestionar el RAG (subida de documentos arrastrar-soltar), configurar System Prompts personalizados y ofrece una experiencia tipo ChatGPT local.38

### **8.2 Guía de Implementación Paso a Paso**

1. **Instalación del Modelo:**  
   Bash  
   ollama pull qwen2.5-coder:14b  
   ollama pull nomic-embed-text

2. Configuración del Modelfile (Optimización):  
   Crear un archivo Modelfile.bash\_expert:  
   Dockerfile  
   FROM qwen2.5\-coder:14b  
   PARAMETER temperature 0.15  
   PARAMETER num\_ctx 8192  
   PARAMETER repeat\_penalty 1.1  
   SYSTEM "Eres un experto en Bash y seguridad Linux. Analiza comandos por seguridad, sintaxis y eficiencia. Advierte sobre operaciones destructivas. Usa sintaxis Bash 4.0+."

   Crear el modelo personalizado: ollama create bash-expert \-f Modelfile.bash\_expert.  
3. Despliegue de Open WebUI con RAG:  
   Ejecutar Open WebUI vía Docker conectando a la instancia local de Ollama.  
   En la interfaz, crear una "Knowledge Base" llamada "Linux Man Pages".  
   Ingestar las páginas manual procesadas (texto limpio) en esta base.  
   Activar la base de conocimiento en el chat con el modelo bash-expert.

## **9\. Conclusión y Perspectivas Futuras**

La investigación confirma que, en 2025, la barrera de entrada para tener un asistente de ingeniería de sistemas de nivel experto ejecutándose localmente ha desaparecido. La combinación de hardware de consumo de gama alta (16GB VRAM) con modelos eficientes como **Qwen2.5-Coder-14B** y técnicas de **RAG local**, permite superar las limitaciones tradicionales de alucinación y falta de conocimiento específico.

Esta arquitectura no solo iguala la utilidad de los asistentes en la nube para tareas de Bash, sino que la supera en privacidad, latencia y personalización. El futuro inmediato apunta hacia una mayor integración de agentes ("Agentic Workflows"), donde el LLM no solo sugiere el comando, sino que, bajo supervisión humana estricta, lo verifica, prueba y ejecuta, cerrando el ciclo de automatización DevOps en el escritorio del ingeniero. La adopción de estas herramientas representa una ventaja competitiva significativa para los profesionales que buscan maximizar su eficiencia operativa manteniendo la soberanía total sobre sus datos e infraestructura.

#### **Obras citadas**

1. Open Weight AI Models in 2025: Complete Rankings, Cost Analysis & Revolutionary Impact on Software Development \- Lunabase.ai, fecha de acceso: diciembre 2, 2025, [https://lunabase.ai/blog/open-weight-ai-models-in-2025-complete-rankings-cost-analysis-and-revolutionary-impact-on-software-development](https://lunabase.ai/blog/open-weight-ai-models-in-2025-complete-rankings-cost-analysis-and-revolutionary-impact-on-software-development)  
2. 5 Open-Source Coding LLMs You Can Run Locally in 2025 \- Labellerr, fecha de acceso: diciembre 2, 2025, [https://www.labellerr.com/blog/best-coding-llms/](https://www.labellerr.com/blog/best-coding-llms/)  
3. RAG vs. Fine-Tuning: How to Choose \- Oracle, fecha de acceso: diciembre 2, 2025, [https://www.oracle.com/artificial-intelligence/generative-ai/retrieval-augmented-generation-rag/rag-fine-tuning/](https://www.oracle.com/artificial-intelligence/generative-ai/retrieval-augmented-generation-rag/rag-fine-tuning/)  
4. qwen2.5-coder \- Ollama, fecha de acceso: diciembre 2, 2025, [https://ollama.com/library/qwen2.5-coder](https://ollama.com/library/qwen2.5-coder)  
5. 7 Fastest Open Source LLMs You Can Run Locally in 2025 \- Medium, fecha de acceso: diciembre 2, 2025, [https://medium.com/@namansharma\_13002/7-fastest-open-source-llms-you-can-run-locally-in-2025-524be87c2064](https://medium.com/@namansharma_13002/7-fastest-open-source-llms-you-can-run-locally-in-2025-524be87c2064)  
6. Best Small LLMs for Real-World Use: Your Recommendations? : r/LocalLLaMA \- Reddit, fecha de acceso: diciembre 2, 2025, [https://www.reddit.com/r/LocalLLaMA/comments/1hj50f5/best\_small\_llms\_for\_realworld\_use\_your/](https://www.reddit.com/r/LocalLLaMA/comments/1hj50f5/best_small_llms_for_realworld_use_your/)  
7. ollama/ollama: Get up and running with OpenAI gpt-oss, DeepSeek-R1, Gemma 3 and other models. \- GitHub, fecha de acceso: diciembre 2, 2025, [https://github.com/ollama/ollama](https://github.com/ollama/ollama)  
8. qwen2 5 coder 7b benchmark: Performance & Evaluation 2025 \- BytePlus, fecha de acceso: diciembre 2, 2025, [https://www.byteplus.com/en/topic/417636](https://www.byteplus.com/en/topic/417636)  
9. Gemma 2 9B vs Qwen2.5 7B Instruct (Comparative Analysis) \- Galaxy.ai Blog, fecha de acceso: diciembre 2, 2025, [https://blog.galaxy.ai/compare/gemma-2-9b-it-vs-qwen-2-5-7b-instruct](https://blog.galaxy.ai/compare/gemma-2-9b-it-vs-qwen-2-5-7b-instruct)  
10. Coding with Llama 3.1, new DeepSeek Coder & Mistral Large : r/LocalLLaMA \- Reddit, fecha de acceso: diciembre 2, 2025, [https://www.reddit.com/r/LocalLLaMA/comments/1ebqga2/coding\_with\_llama\_31\_new\_deepseek\_coder\_mistral/](https://www.reddit.com/r/LocalLLaMA/comments/1ebqga2/coding_with_llama_31_new_deepseek_coder_mistral/)  
11. Top 10 open source LLMs for 2025 \- NetApp Instaclustr, fecha de acceso: diciembre 2, 2025, [https://www.instaclustr.com/education/open-source-ai/top-10-open-source-llms-for-2025/](https://www.instaclustr.com/education/open-source-ai/top-10-open-source-llms-for-2025/)  
12. qwen2.5 \- Ollama, fecha de acceso: diciembre 2, 2025, [https://ollama.com/library/qwen2.5](https://ollama.com/library/qwen2.5)  
13. DeepSeek-Coder-V2: Breaking the Barrier of Closed-Source Models in Code Intelligence \- arXiv, fecha de acceso: diciembre 2, 2025, [https://arxiv.org/pdf/2406.11931](https://arxiv.org/pdf/2406.11931)  
14. Coding with Llama 3.1, new DeepSeek Coder & Mistral Large : r/ChatGPTCoding \- Reddit, fecha de acceso: diciembre 2, 2025, [https://www.reddit.com/r/ChatGPTCoding/comments/1ebqfs9/coding\_with\_llama\_31\_new\_deepseek\_coder\_mistral/](https://www.reddit.com/r/ChatGPTCoding/comments/1ebqfs9/coding_with_llama_31_new_deepseek_coder_mistral/)  
15. DeepSeek-Coder-V2 vs. Llama 3.1 Comparison \- SourceForge, fecha de acceso: diciembre 2, 2025, [https://sourceforge.net/software/compare/DeepSeek-Coder-V2-vs-Llama-3.1/](https://sourceforge.net/software/compare/DeepSeek-Coder-V2-vs-Llama-3.1/)  
16. Codestral \- Mistral AI, fecha de acceso: diciembre 2, 2025, [https://mistral.ai/news/codestral](https://mistral.ai/news/codestral)  
17. DeepSeek-Coder-V2: Breaking the Barrier of Closed-Source Models in Code Intelligence, fecha de acceso: diciembre 2, 2025, [https://arxiv.org/html/2406.11931v1](https://arxiv.org/html/2406.11931v1)  
18. How to obtain man page contents in Python? \- Stack Overflow, fecha de acceso: diciembre 2, 2025, [https://stackoverflow.com/questions/12768894/how-to-obtain-man-page-contents-in-python](https://stackoverflow.com/questions/12768894/how-to-obtain-man-page-contents-in-python)  
19. Formatting a command in python subprocess popen \- Stack Overflow, fecha de acceso: diciembre 2, 2025, [https://stackoverflow.com/questions/27985888/formatting-a-command-in-python-subprocess-popen](https://stackoverflow.com/questions/27985888/formatting-a-command-in-python-subprocess-popen)  
20. Part 2: Getting Started with Local AI \- Open WebUI Documents and Tools | by John Wong, fecha de acceso: diciembre 2, 2025, [https://medium.com/@able\_wong/getting-started-with-local-ai-open-webui-documents-and-tools-part-2-5f8f9c67a414](https://medium.com/@able_wong/getting-started-with-local-ai-open-webui-documents-and-tools-part-2-5f8f9c67a414)  
21. library \- Ollama, fecha de acceso: diciembre 2, 2025, [https://ollama.com/library](https://ollama.com/library)  
22. Constructing a RAG system using LlamaIndex and Ollama \- AMD ROCm documentation, fecha de acceso: diciembre 2, 2025, [https://rocm.docs.amd.com/projects/ai-developer-hub/en/v2.0/notebooks/inference/rag\_ollama\_llamaindex.html](https://rocm.docs.amd.com/projects/ai-developer-hub/en/v2.0/notebooks/inference/rag_ollama_llamaindex.html)  
23. Simple wonders of RAG using Ollama, Langchain and ChromaDB \- DEV Community, fecha de acceso: diciembre 2, 2025, [https://dev.to/arjunrao87/simple-wonders-of-rag-using-ollama-langchain-and-chromadb-2hhj](https://dev.to/arjunrao87/simple-wonders-of-rag-using-ollama-langchain-and-chromadb-2hhj)  
24. RAG vs. fine-tuning \- Red Hat, fecha de acceso: diciembre 2, 2025, [https://www.redhat.com/en/topics/ai/rag-vs-fine-tuning](https://www.redhat.com/en/topics/ai/rag-vs-fine-tuning)  
25. Modelfile Reference \- Ollama English Documentation, fecha de acceso: diciembre 2, 2025, [https://ollama.readthedocs.io/en/modelfile/](https://ollama.readthedocs.io/en/modelfile/)  
26. Ollama CLI tutorial: Running Ollama via the terminal \- Hostinger, fecha de acceso: diciembre 2, 2025, [https://www.hostinger.com/tutorials/ollama-cli-tutorial](https://www.hostinger.com/tutorials/ollama-cli-tutorial)  
27. Create Your Own Bash Computer Use Agent with NVIDIA Nemotron in One Hour, fecha de acceso: diciembre 2, 2025, [https://developer.nvidia.com/blog/create-your-own-bash-computer-use-agent-with-nvidia-nemotron-in-one-hour/](https://developer.nvidia.com/blog/create-your-own-bash-computer-use-agent-with-nvidia-nemotron-in-one-hour/)  
28. f/awesome-chatgpt-prompts: This repo includes ChatGPT prompt curation to use ChatGPT and other LLM tools better. \- GitHub, fecha de acceso: diciembre 2, 2025, [https://github.com/f/awesome-chatgpt-prompts](https://github.com/f/awesome-chatgpt-prompts)  
29. Ollama's new engine for multimodal models, fecha de acceso: diciembre 2, 2025, [https://ollama.com/blog/multimodal-models](https://ollama.com/blog/multimodal-models)  
30. Ollama tool calling | IBM, fecha de acceso: diciembre 2, 2025, [https://www.ibm.com/think/tutorials/local-tool-calling-ollama-granite](https://www.ibm.com/think/tutorials/local-tool-calling-ollama-granite)  
31. Function Calling \- Qwen, fecha de acceso: diciembre 2, 2025, [https://qwen.readthedocs.io/en/latest/framework/function\_call.html](https://qwen.readthedocs.io/en/latest/framework/function_call.html)  
32. Taming your shell for LLMs \- rand\[om\], fecha de acceso: diciembre 2, 2025, [https://ricardoanderegg.com/posts/control-shell-permissions-llm-codex/](https://ricardoanderegg.com/posts/control-shell-permissions-llm-codex/)  
33. LLM Leaderboard 2025 \- Vellum AI, fecha de acceso: diciembre 2, 2025, [https://www.vellum.ai/llm-leaderboard](https://www.vellum.ai/llm-leaderboard)  
34. InterCode, fecha de acceso: diciembre 2, 2025, [https://intercode-benchmark.github.io/](https://intercode-benchmark.github.io/)  
35. aounon/certified-llm-safety \- GitHub, fecha de acceso: diciembre 2, 2025, [https://github.com/aounon/certified-llm-safety](https://github.com/aounon/certified-llm-safety)  
36. From Commands to Prompts: LLM-based Semantic File System \- arXiv, fecha de acceso: diciembre 2, 2025, [https://arxiv.org/html/2410.11843v5](https://arxiv.org/html/2410.11843v5)  
37. Open WebUI \+ Ollama: Local AI Chat with RAG on Ubuntu Guide \- iTecs, fecha de acceso: diciembre 2, 2025, [https://itecsonline.com/post/openwebui-ollama-rag-guide](https://itecsonline.com/post/openwebui-ollama-rag-guide)  
38. Open WebUI RAG Tutorial, fecha de acceso: diciembre 2, 2025, [https://docs.openwebui.com/tutorials/tips/rag-tutorial/](https://docs.openwebui.com/tutorials/tips/rag-tutorial/)