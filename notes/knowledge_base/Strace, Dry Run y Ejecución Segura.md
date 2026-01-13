# **Diagnóstico Avanzado de Sistemas: Análisis Exhaustivo de la Traza de Sistemas, Inyección de Fallos y Entornos de Ejecución Efímera en Linux**

## **1\. Resumen Ejecutivo y Arquitectura del Problema**

La consulta planteada aborda una problemática fundamental en la administración de sistemas Unix y Linux: la distinción crítica entre la observabilidad (monitoreo) y la simulación (ejecución en seco o *dry run*). El usuario postula una hipótesis operativa basada en la herramienta strace, sugiriendo que su interceptación de llamadas al sistema podría constituir un mecanismo de ejecución simulada inofensiva. Específicamente, se cuestiona si la ejecución de strace chmod \+x archivo altera efectivamente los permisos del sistema de archivos y si el análisis forense de los registros de strace permite determinar la seguridad de re-ejecutar comandos fallidos para recuperar flujos de salida estándar (stdout) y de error (stderr) perdidos, una limitación operativa identificada en el uso de multiplexores de terminal como tmux.

Este reporte técnico establece de manera definitiva que strace, en su configuración estándar, no es una herramienta de simulación. Es un mecanismo de transparencia operativa que utiliza la llamada al sistema ptrace para interceptar y registrar la actividad entre el espacio de usuario y el espacio del núcleo (kernel space). En consecuencia, cualquier comando ejecutado bajo strace tiene efectos reales, inmediatos y persistentes sobre el estado del sistema. La ejecución de strace chmod \+x archivo modifica los bits de permisos en el inodo del sistema de archivos con la misma eficacia que si se ejecutara sin supervisión. La herramienta actúa como un observador pasivo, documentando la mutación del estado del sistema, no previniéndola.

Sin embargo, la intuición del usuario roza una capacidad avanzada y menos documentada del kernel de Linux: la inyección de fallos en llamadas al sistema (*System Call Fault Injection*). Mediante configuraciones avanzadas de strace (específicamente la directiva \-e inject), es técnicamente viable interceptar llamadas destructivas (como write, unlink, chmod) y forzar al núcleo a omitir su ejecución real mientras se devuelve un código de éxito falso al proceso. Esto permite construir, teóricamente, un entorno de "dry run" sintético. No obstante, este informe analiza por qué dicha práctica es frágil, propensa a generar divergencias lógicas ("alucinaciones" de proceso) y peligrosa para la integridad de datos en comandos complejos.

Para resolver la necesidad operativa del usuario —la captura segura de salida y error sin efectos colaterales—, este documento propone un cambio de paradigma desde la "intercepción" hacia el "aislamiento". Se presentan soluciones basadas en sistemas de archivos de capas (*OverlayFS*) y contención efímera (*Bubblewrap*, *Docker/Podman*), que permiten ejecutar comandos en un entorno de copia en escritura (*Copy-on-Write*). Estas tecnologías satisfacen el requisito de "ejecución segura" al redirigir todas las modificaciones de estado a una capa volátil en memoria que se descarta al finalizar, permitiendo capturar logs reales de una ejecución que el sistema percibe como exitosa, pero que permanece inocua para el sistema anfitrión.

## **2\. Fundamentos Mecánicos de la Traza de Sistemas en Linux**

Para comprender por qué strace no es un simulador por defecto, es imperativo diseccionar la arquitectura de llamadas al sistema y el mecanismo de ptrace sobre el cual se construye la herramienta. La distinción entre lo que el usuario percibe (el comando en la terminal) y lo que el núcleo ejecuta es la clave para desmitificar la naturaleza de la traza.

### **2.1 La Frontera Usuario-Núcleo y las Llamadas al Sistema**

En la arquitectura de sistemas operativos modernos tipo Unix, existe una separación estricta de privilegios. Las aplicaciones de usuario, como el comando chmod, los scripts de shell o los compiladores, se ejecutan en el "Espacio de Usuario" (*User Space*), un modo de ejecución no privilegiado (Anillo 3 en arquitecturas x86). En este modo, el proceso no tiene acceso directo al hardware, a la memoria física ni a las estructuras de datos del sistema de archivos.1

Para realizar cualquier operación útil —leer un archivo, escribir en la pantalla, asignar memoria o cambiar permisos— el proceso debe solicitar servicios al "Espacio del Núcleo" (*Kernel Space*, Anillo 0). Esta solicitud se realiza mediante una **Llamada al Sistema** (*syscall*). Cuando un programa ejecuta una instrucción de syscall, la CPU cambia de modo, salta a una rutina de manejo de interrupciones en el kernel, y este último valida y ejecuta la acción solicitada.2

### **2.2 La Mecánica de ptrace: Intercepción, No Emulación**

La herramienta strace (System Trace) se basa fundamentalmente en la llamada al sistema ptrace() (*process trace*), una potente interfaz de depuración proporcionada por el kernel de Linux.4 Entender el flujo de ejecución de ptrace es esencial para responder a la pregunta del usuario sobre la efectividad de los cambios.

Cuando el usuario ejecuta strace chmod \+x archivo, ocurre la siguiente secuencia de eventos a nivel de sistema operativo:

1. **Bifurcación (Forking):** strace inicia y se bifurca (fork), creando un proceso hijo.  
2. **Vinculación (Tracing):** El proceso hijo invoca ptrace(PTRACE\_TRACEME,...) antes de ejecutar cualquier otra cosa. Esto marca al proceso como "rastreable" por su padre (strace).  
3. **Ejecución (Execve):** El hijo ejecuta execve("/bin/chmod",...) para cargar el binario de chmod en memoria.  
4. **Interrupción Inicial:** El kernel detiene al hijo inmediatamente antes de que comience a ejecutarse y notifica al padre (strace).  
5. **Ciclo de Inspección:** A partir de este momento, cada vez que el proceso chmod intenta realizar una llamada al sistema (por ejemplo, fchmodat para cambiar permisos), la ejecución del proceso se detiene en el punto de entrada de la syscall (*syscall-enter-stop*).  
6. **Observación:** El control pasa a strace. strace inspecciona los registros de la CPU para determinar qué syscall se solicitó (el número de syscall) y qué argumentos se pasaron (nombre del archivo, modo octal). Esta información se decodifica y se imprime en stderr.  
7. **Reanudación y Ejecución Real:** Aquí reside el punto crítico. Una vez que strace ha registrado la intención, invoca ptrace(PTRACE\_SYSCALL,...) para indicar al kernel que continúe. **El kernel entonces ejecuta la llamada al sistema real.** Escribe en el disco, cambia los permisos, abre el socket, o borra el archivo. La acción no es simulada; es ejecutada con todos sus efectos secundarios.6  
8. **Interrupción de Salida:** Una vez que el kernel termina la operación, detiene nuevamente el proceso antes de devolver el control al espacio de usuario (*syscall-exit-stop*).  
9. **Registro de Resultado:** strace lee el valor de retorno (ej. 0 para éxito, \-1 para error) y lo imprime.

### **2.3 El Mito del "Dry Run"**

La confusión del usuario proviene de la similitud visual entre la salida de strace (que muestra lo que el sistema "está haciendo") y la salida de herramientas con modos de simulación integrados (como rsync \--dry-run o apt-get install \--dry-run).

La diferencia ontológica es profunda:

* **Simulación de Aplicación (rsync \--dry-run):** La lógica de "no hacer nada" está programada *dentro* del binario de la aplicación. El programa calcula las operaciones necesarias e imprime lo que haría, pero *decide* no invocar las llamadas al sistema destructivas (como write o unlink).  
* **Traza de Sistema (strace):** La herramienta es externa y agnóstica a la lógica del programa. No sabe "qué" quiere hacer el programa a alto nivel, solo ve las instrucciones de bajo nivel. Por defecto, su mandato es observar la verdad, y la verdad requiere ejecución.6

Por lo tanto, ante la pregunta directa: *"Si yo hago strace chmod \+x archivo ¿Se cambian efectivamente los permisos del archivo?"*, la respuesta técnica e irrefutable es **Sí**. El inodo del sistema de archivos es actualizado inmediatamente. Si el usuario verificara los permisos con ls \-l después de la ejecución bajo strace, vería el bit de ejecución activo. strace actúa como un notario que certifica que el cambio ocurrió, no como un guardia que lo impide.

## **3\. Inyección de Fallos: La Simulación Sintética mediante strace**

Aunque se ha establecido que strace ejecuta comandos por defecto, el usuario pregunta: *"¿No hace una especie de dry run?"*. Curiosamente, las versiones modernas de strace (4.15 en adelante) poseen capacidades de **Inyección de Fallos** (*Fault Injection*) que permiten, bajo una configuración experta y deliberada, aproximar este comportamiento.

### **3.1 Mecanismo de Supresión de Syscalls**

La interfaz ptrace permite al trazador modificar los registros de la CPU antes de que el kernel ejecute la llamada al sistema. Esto habilita una técnica conocida como "anulación de syscall" o "tampering".6

Mediante la opción \-e inject, un administrador puede instruir a strace para que intercepte una llamada específica y, en lugar de permitir que el kernel la ejecute, la cancele y devuelva un valor de retorno falso al proceso. Esto crea una realidad simulada para el proceso rastreado.

La sintaxis general es:

Bash

strace \-e inject=conjunto\_syscalls:retval=valor

### **3.2 Construcción de un "Dry Run" para chmod**

Para satisfacer la hipótesis del usuario de ejecutar chmod sin cambiar los permisos, se debe inyectar un éxito falso en las llamadas al sistema responsables de la modificación de metadatos de archivos.

El comando exacto sería:

Bash

strace \-e trace=fchmodat,fchmod,chmod \-e inject=fchmodat,fchmod,chmod:retval=0 chmod \+x archivo

**Análisis Técnico de la Inyección:**

1. **Detección:** strace detecta que chmod invoca fchmodat.  
2. **Intervención:** En el punto de entrada (*User-Enter Stop*), strace anula la llamada. El kernel **no** recibe la instrucción de modificar el inodo.  
3. **Simulación:** strace modifica el registro de retorno (generalmente RAX en x86\_64) para contener el valor 0, que por convención en C/Linux significa "Éxito" (Success).  
4. **Engaño:** El proceso chmod reanuda su ejecución. Verifica el valor de retorno, ve un 0, asume que la operación fue exitosa y termina silenciosamente sin reportar error.  
5. **Resultado:** El archivo archivo permanece intacto, pero el comando se ejecutó y reportó éxito.

Esto constituye técnicamente un "dry run" forzado externamente.6

### **3.3 Riesgos Críticos y Limitaciones de la Inyección**

Si bien esto demuestra que es *posible*, es extremadamente peligroso generalizar esta técnica para *"cualquier comando"* como sugiere el usuario, debido a tres factores de riesgo sistémico:

#### **3.3.1 Divergencia Lógica y "Alucinaciones" de Proceso**

Los programas complejos no son lineales; son árboles de decisión. Si simulamos el éxito de una operación crítica (como crear un directorio), las operaciones subsiguientes fallarán de maneras impredecibles.13

*Ejemplo:* Un script de instalación.

1. mkdir /tmp/instalacion (Inyectamos éxito falso: directorio no se crea).  
2. cp binario /tmp/instalacion/binario.  
3. El comando cp intentará abrir /tmp/instalacion/binario. Como el directorio no existe realmente, el kernel devolverá ENOENT (No such file or directory).  
4. El script fallará con un error confuso, o peor, intentará rutas alternativas no deseadas.

El "dry run" sintético provoca que el programa entre en un estado de "alucinación" donde cree que el entorno tiene un estado que no posee, invalidando la utilidad de la prueba para verificar la lógica del script.

#### **3.3.2 Complejidad de la Superficie de Ataque de Syscalls**

Para garantizar un "dry run" seguro de *cualquier* comando, se necesitaría bloquear exhaustivamente todas las llamadas al sistema que modifican estado. Esta lista es enorme y varía según la arquitectura y versión del kernel 14:

* **Escritura:** write, writev, pwrite64, sendfile.  
* **Metadatos:** chmod, fchmod, chown, lchown, utimensat, setxattr.  
* **Sistema de Archivos:** mkdir, rmdir, unlink, unlinkat, rename, link, symlink, mknod.  
* **Red:** connect, bind, sendto, sendmsg.  
* **Control:** ioctl, fcntl.

Omitir una sola de estas llamadas (por ejemplo, bloquear write pero olvidar pwrite64) resultaría en una corrupción parcial de datos: el programa creería estar en modo simulación pero algunas escrituras pasarían al disco.

#### **3.3.3 Conclusión sobre la Inyección**

Aunque strace \-e inject valida técnicamente la intuición del usuario, es una herramienta de **depuración y prueba de resiliencia ante errores** (Chaos Engineering), no una herramienta de seguridad operativa para re-ejecución segura. Su uso requiere un conocimiento enciclopédico de las llamadas al sistema que utiliza el binario específico.6

## **4\. Análisis de Idempotencia: Determinando la Seguridad de la Re-ejecución**

La segunda parte de la consulta del usuario es crucial: *"Mirando sus resultados ¿no se podría saber si un comando es seguro para volver a ejecutar?"*. Esta pregunta invoca el concepto de **Idempotencia** y análisis forense de trazas.

### **4.1 Definición de Idempotencia Operativa**

En ingeniería de sistemas, una operación es **idempotente** si ejecutarla múltiples veces produce el mismo resultado sistémico que ejecutarla una sola vez ($f(f(x)) \= f(x)$). Si un comando es idempotente, es seguro re-ejecutarlo para capturar su salida perdida.

### **4.2 Taxonomía de Seguridad basada en Logs de strace**

Al analizar un log de strace de una ejecución previa (o parcial), podemos clasificar la seguridad de re-ejecución basándonos en las syscalls observadas.2

| Categoría de Syscall | Comportamiento | Seguridad de Re-ejecución | Ejemplos en strace |
| :---- | :---- | :---- | :---- |
| **Lectura Pura** | Consulta estado sin modificarlo. | **Segura** (Infinita) | read, stat, access, open(O\_RDONLY), mmap (lectura). |
| **Escritura Declarativa** | Establece un valor absoluto (Idempotente). | **Segura** | chmod 755 (fchmodat), chown user (fchownat), mkdir \-p (si maneja EEXIST). |
| **Escritura Aditiva** | Añade datos al final de un recurso. | **Segura pero Contaminante** | open(O\_APPEND), write (logs). Genera datos duplicados pero no rompe lógica. |
| **Modificación Destructiva** | Cambia estado relativo o borra. | **Insegura** | unlink (borrar), rmdir, mv (rename), open(O\_TRUNC) (sobrescribir), write (en offset específico). |

#### **Análisis del Caso chmod \+x**

El comando chmod es típicamente idempotente.

* **Ejecución 1:** fchmodat(..., 0755). El archivo pasa a 755\.  
* **Ejecución 2:** fchmodat(..., 0755). El archivo se establece a 755 (sin cambios netos).  
* **Veredicto:** Es seguro re-ejecutar chmod tantas veces como sea necesario para capturar la salida, siempre que el archivo siga existiendo.

#### **Análisis del Caso Peligroso (No Idempotente)**

Supongamos un comando de rotación de logs: mv log.txt log.bak.

* **Ejecución 1:** rename("log.txt", "log.bak"). Éxito.  
* **Re-ejecución:** rename("log.txt", "log.bak"). Fallo crítico: ENOENT (log.txt ya no existe).  
* **Riesgo:** Si el usuario re-ejecuta esto esperando capturar la salida de éxito, obtendrá un error, confundiendo el diagnóstico. Peor aún, si el script tenía lógica rm log.bak && mv log.txt log.bak, la re-ejecución podría borrar el backup recién creado.

### **4.3 Heurística para el Usuario**

Para responder "¿No se podría saber si es seguro?": Sí, pero requiere un análisis experto.  
El usuario debe buscar en el log de strace la presencia de syscalls destructivas (unlink, rename) o modificaciones no idempotentes. Si el log muestra una actividad compleja de archivos temporales o sockets de red, la re-ejecución ciega es desaconsejada.

## **5\. Soluciones Arquitectónicas: Entornos de Ejecución Efímera**

Dado que strace modifica el sistema y la inyección de fallos es compleja, y dado que el análisis de idempotencia es propenso a errores humanos, la solución de ingeniería correcta para el problema del usuario ("re-ejecutar para capturar salida sin efectos secundarios") es el uso de **Entornos de Ejecución Efímera** o *Sandboxing*.

Estas tecnologías permiten ejecutar el comando *realmente* (evitando la divergencia lógica), pero atrapando todos los efectos secundarios (escrituras, borrados) en una capa desechable.

### **5.1 OverlayFS: El Estándar de Oro en Aislamiento de Sistema de Archivos**

OverlayFS es un sistema de archivos de unión (*union mount*) que permite superponer una capa de escritura volátil sobre un sistema de archivos base de solo lectura.18 Esta es la tecnología base de Docker y los contenedores modernos.

Implementación del Concepto "Dry Run Real":  
El usuario puede crear un entorno donde su directorio actual es la capa "Inferior" (LowerDir, Solo Lectura) y una carpeta temporal en RAM (tmpfs) es la capa "Superior" (UpperDir, Escritura).

1. El comando se ejecuta viendo todos los archivos originales.  
2. Si el comando hace chmod \+x archivo, OverlayFS copia el archivo a la capa Superior (copy-up) y modifica los permisos *allí*.  
3. El archivo original en el disco duro permanece intacto.  
4. El comando termina exitosamente (código 0), generando toda la salida stdout/stderr deseada.  
5. Al finalizar, se desmonta el Overlay y se borra la capa Superior.

Esto satisface perfectamente el requisito: ejecución lógica completa, captura de salida, cero impacto permanente en el sistema.

### **5.2 Bubblewrap (bwrap): La Herramienta de Aislamiento sin Privilegios**

Para un usuario estándar (sin acceso root para montar OverlayFS manualmente), la herramienta **Bubblewrap** (bwrap) es la solución idónea. Utiliza espacios de nombres de usuario (*User Namespaces*) del kernel para crear entornos aislados sin necesidad de privilegios de superusuario.20

Bubblewrap permite construir un "Dry Run" defensivo configurando el sistema de archivos como de solo lectura (*Read-Only Bind*).

**Ejemplo Práctico de "Safe Run":**

Bash

bwrap \--ro-bind / / \--dev /dev \--proc /proc \--tmpfs /tmp \--new-session bash \-c "comando\_sospechoso"

* \--ro-bind / /: Monta todo el sistema raíz como solo lectura dentro de la jaula.  
* \--tmpfs /tmp: Permite escritura solo en /tmp (para archivos temporales del proceso).

Si el usuario ejecuta chmod \+x archivo dentro de este entorno:

1. El comando intentará modificar el archivo.  
2. El kernel, dentro del namespace, bloqueará la escritura devolviendo EROFS (Read-only file system).  
3. El comando fallará, pero imprimirá los errores exactos de qué intentó hacer.  
4. El sistema real está protegido.

Esta es la forma más segura de "probar" un comando desconocido o recuperar errores de un comando que se sabe que fallará o intentará escribir donde no debe.24

### **5.3 Contenedores Efímeros (Docker/Podman)**

Si el entorno dispone de Docker o Podman, la ejecución efímera es trivial y robusta. El uso de la bandera \--rm junto con volúmenes de solo lectura proporciona un entorno de prueba perfecto.26

**Comando de Protección:**

Bash

docker run \--rm \-v "$(pwd):/work:ro" \-w /work alpine chmod \+x archivo

Aquí, :ro fuerza que el directorio de trabajo sea de solo lectura. El comando fallará de forma segura, permitiendo capturar el error. Para permitir que el comando *tenga éxito* (simulación completa), se podría copiar el contenido a un directorio interno del contenedor antes de ejecutar, sacrificando rendimiento por fidelidad de simulación.

## **6\. Estrategias de Captura de Salida y Persistencia**

El usuario menciona explícitamente: *"lo de tmux no me resuelve la papeleta"*. Esto indica un problema de gestión de buffers de terminal. Tmux y Screen tienen límites de historial (scrollback). Si un comando genera millones de líneas de error, las primeras se pierden.

Para evitar la necesidad de re-ejecución (ya sea segura o insegura), la ingeniería de sistemas dicta que la captura debe ser proactiva y persistente a disco, no dependiente de la memoria de la terminal.

### **6.1 La Utilidad script**

La herramienta estándar POSIX para este propósito es script. A diferencia de una simple redirección (\> file), script crea una pseudo-terminal (PTY) completa. Esto significa que captura todo lo que se ve en la pantalla, incluyendo códigos de control, colores, y la interacción de programas que detectan si están en una terminal (isatty).29

**Uso Preventivo:**

Bash

script \-a sesion\_debug.log

Al ejecutar esto antes de una sesión crítica, **todo** lo que ocurra (input y output) se guarda en disco. Si ocurre un error masivo que desborda el buffer de tmux, el archivo sesion\_debug.log contendrá la evidencia completa, eliminando la necesidad de re-ejecutar el comando peligroso.

### **6.2 Redirección Avanzada y tee**

Para comandos individuales, el uso de tuberías (pipes) es superior a confiar en la terminal.

Bash

comando 2\>&1 | tee salida.log

* 2\>&1: Redirige stderr (descriptor 2\) a stdout (descriptor 1), capturando los errores junto con la salida normal.  
* tee: Bifurca el flujo, mostrándolo en pantalla (para monitoreo en tiempo real) y escribiéndolo simultáneamente a disco.

Esta práctica asegura que, independientemente de la volatilidad de la ventana de tmux, los datos persisten.

## **7\. Conclusiones y Recomendaciones Técnicas**

El análisis exhaustivo de la mecánica de strace, la gestión de memoria del kernel y las técnicas de virtualización ligera permite emitir las siguientes conclusiones definitivas para el usuario:

1. **Refutación del Dry Run:** strace **no** es un mecanismo de ejecución simulada ("dry run"). Es un mecanismo de instrumentación transparente. Ejecutar strace chmod altera los permisos del archivo de forma real y permanente. Confiar en strace como salvaguarda de seguridad es un error operativo crítico.  
2. **Viabilidad Técnica (pero Inoperancia Práctica) de la Simulación:** Es técnicamente posible forzar a strace a actuar como simulador usando \-e inject, bloqueando llamadas como write y chmod. Sin embargo, esta práctica es desaconsejada para "cualquier comando" debido a la alta probabilidad de inducir estados de error artificiales ("alucinaciones") que invalidan la utilidad de la prueba.  
3. **Análisis de Seguridad Post-Facto:** Es posible determinar la seguridad de la re-ejecución analizando los logs de strace en busca de llamadas no idempotentes (unlink, rename, open(O\_TRUNC)). Si el log muestra solo operaciones de lectura o modificaciones de estado idempotentes (chmod), la re-ejecución es segura.  
4. **Solución Definitiva:** Para resolver el problema de fondo —capturar salida sin riesgos— no se debe "interceptar" el comando, sino "aislarlo".

### **Recomendaciones de Ingeniería**

Para el escenario del usuario ("Recuperar salida de error sin romper el sistema"):

1. **Nivel 1 (Prevención):** Implementar script \-a logfile.txt o | tee log.txt en los flujos de trabajo habituales. La persistencia en disco es la única garantía contra la pérdida de buffers en tmux.  
2. **Nivel 2 (Investigación Segura \- Sin Root):** Utilizar **Bubblewrap (bwrap)** para re-ejecutar el comando fallido en un entorno de solo lectura.  
   * *Comando:* bwrap \--ro-bind / / \--dev /dev \--tmpfs /tmp bash \-c "comando"  
   * Esto capturará la intención del comando y sus errores iniciales sin tocar el disco.  
3. **Nivel 3 (Simulación de Escritura \- Requiere Docker):** Si se necesita ver si el comando tiene éxito completo (incluyendo escrituras), ejecutarlo en un contenedor efímero con el directorio montado como volumen.  
   * *Comando:* docker run \--rm \-v $(pwd):/data alpine sh \-c "cp \-r /data /sandbox && cd /sandbox && comando"  
   * Esto permite que el comando destruya la copia en el sandbox sin afectar los datos reales.

En resumen, la seguridad en la re-ejecución no se logra observando el comando (strace), sino conteniéndolo (bwrap/contenedores).

#### **Obras citadas**

1. Understanding system calls on Linux with strace | Opensource.com, fecha de acceso: diciembre 2, 2025, [https://opensource.com/article/19/10/strace](https://opensource.com/article/19/10/strace)  
2. Decoding information from the strace output \- GeeksforGeeks, fecha de acceso: diciembre 2, 2025, [https://www.geeksforgeeks.org/linux-unix/decoding-information-from-the-strace-output/](https://www.geeksforgeeks.org/linux-unix/decoding-information-from-the-strace-output/)  
3. syscall(2) \- Linux manual page \- man7.org, fecha de acceso: diciembre 2, 2025, [https://man7.org/linux/man-pages/man2/syscall.2.html](https://man7.org/linux/man-pages/man2/syscall.2.html)  
4. strace \- Wikipedia, fecha de acceso: diciembre 2, 2025, [https://en.wikipedia.org/wiki/Strace](https://en.wikipedia.org/wiki/Strace)  
5. How can strace monitor itself? \- Unix & Linux Stack Exchange, fecha de acceso: diciembre 2, 2025, [https://unix.stackexchange.com/questions/266431/how-can-strace-monitor-itself](https://unix.stackexchange.com/questions/266431/how-can-strace-monitor-itself)  
6. strace(1) \- Linux manual page \- man7.org, fecha de acceso: diciembre 2, 2025, [https://man7.org/linux/man-pages/man1/strace.1.html](https://man7.org/linux/man-pages/man1/strace.1.html)  
7. How does strace work? \- system calls \- Stack Overflow, fecha de acceso: diciembre 2, 2025, [https://stackoverflow.com/questions/5494316/how-does-strace-work](https://stackoverflow.com/questions/5494316/how-does-strace-work)  
8. strace Wow Much Syscall \- Brendan Gregg, fecha de acceso: diciembre 2, 2025, [https://www.brendangregg.com/blog/2014-05-11/strace-wow-much-syscall.html](https://www.brendangregg.com/blog/2014-05-11/strace-wow-much-syscall.html)  
9. linux \- Dry-run a potentially dangerous script? \- Stack Overflow, fecha de acceso: diciembre 2, 2025, [https://stackoverflow.com/questions/22952959/dry-run-a-potentially-dangerous-script](https://stackoverflow.com/questions/22952959/dry-run-a-potentially-dangerous-script)  
10. How To Dry Run Or Simulate Linux Commands Without Changing Anything In The System, fecha de acceso: diciembre 2, 2025, [https://ostechnix.com/how-to-simulate-linux-commands-without-changing-anything-in-the-system/](https://ostechnix.com/how-to-simulate-linux-commands-without-changing-anything-in-the-system/)  
11. Using Strace for performing fault injection in system calls. | by buffer0x7cd \- Medium, fecha de acceso: diciembre 2, 2025, [https://medium.com/@manav503/using-strace-to-perform-fault-injection-in-system-calls-fcb859940895](https://medium.com/@manav503/using-strace-to-perform-fault-injection-in-system-calls-fcb859940895)  
12. Can strace make you fail? \- strace syscall fault injection, fecha de acceso: diciembre 2, 2025, [https://archive.fosdem.org/2017/schedule/event/failing\_strace/attachments/slides/1630/export/events/attachments/failing\_strace/slides/1630/strace\_fosdem2017\_ta\_slides.pdf](https://archive.fosdem.org/2017/schedule/event/failing_strace/attachments/slides/1630/export/events/attachments/failing_strace/slides/1630/strace_fosdem2017_ta_slides.pdf)  
13. Injecting syscall faults in Python and Ruby \- Matt Stuchlik, fecha de acceso: diciembre 2, 2025, [https://blog.mattstuchlik.com/2024/09/08/injecting-syscall-faults.html](https://blog.mattstuchlik.com/2024/09/08/injecting-syscall-faults.html)  
14. File system calls \- Linux Assembly, fecha de acceso: diciembre 2, 2025, [https://linasm.sourceforge.net/docs/syscalls/filesystem.php](https://linasm.sourceforge.net/docs/syscalls/filesystem.php)  
15. Linux System Call Table, fecha de acceso: diciembre 2, 2025, [https://thevivekpandey.github.io/posts/2017-09-25-linux-system-calls.html](https://thevivekpandey.github.io/posts/2017-09-25-linux-system-calls.html)  
16. README.md \- Error provenance in the Linux kernel \- GitHub, fecha de acceso: diciembre 2, 2025, [https://github.com/nviennot/linux-trace-error/blob/master/README.md](https://github.com/nviennot/linux-trace-error/blob/master/README.md)  
17. Frictions and Complexities of "Simple" Scripts \- Lloyd Atkinson, fecha de acceso: diciembre 2, 2025, [https://www.lloydatkinson.net/posts/2024/frictions-and-complexities-of-simple-bash-scripts/](https://www.lloydatkinson.net/posts/2024/frictions-and-complexities-of-simple-bash-scripts/)  
18. a practical look into overlayfs \- ops.tips, fecha de acceso: diciembre 2, 2025, [https://ops.tips/notes/practical-look-into-overlayfs/](https://ops.tips/notes/practical-look-into-overlayfs/)  
19. How to use OverlayFS to protect the root filesystem? \- Unix & Linux Stack Exchange, fecha de acceso: diciembre 2, 2025, [https://unix.stackexchange.com/questions/316018/how-to-use-overlayfs-to-protect-the-root-filesystem](https://unix.stackexchange.com/questions/316018/how-to-use-overlayfs-to-protect-the-root-filesystem)  
20. Bubblewrap \- ArchWiki, fecha de acceso: diciembre 2, 2025, [https://wiki.archlinux.org/title/Bubblewrap](https://wiki.archlinux.org/title/Bubblewrap)  
21. Firejail: Light, featureful and zero-dependency security sandbox for Linux | Hacker News, fecha de acceso: diciembre 2, 2025, [https://news.ycombinator.com/item?id=36681912](https://news.ycombinator.com/item?id=36681912)  
22. bwrap: container setup utility | Man Page | Commands | bubblewrap \- ManKier, fecha de acceso: diciembre 2, 2025, [https://www.mankier.com/1/bwrap](https://www.mankier.com/1/bwrap)  
23. Bubblewrap/Examples \- ArchWiki, fecha de acceso: diciembre 2, 2025, [https://wiki.archlinux.org/title/Bubblewrap/Examples](https://wiki.archlinux.org/title/Bubblewrap/Examples)  
24. Can't mount more filesystems within a read-only mount · Issue \#413 · containers/bubblewrap \- GitHub, fecha de acceso: diciembre 2, 2025, [https://github.com/containers/bubblewrap/issues/413](https://github.com/containers/bubblewrap/issues/413)  
25. bwrap(1) — bubblewrap — Debian testing \- Debian Manpages, fecha de acceso: diciembre 2, 2025, [https://manpages.debian.org/testing/bubblewrap/bwrap.1.en.html](https://manpages.debian.org/testing/bubblewrap/bwrap.1.en.html)  
26. Using Docker Compose for Python Development \- CloudBees, fecha de acceso: diciembre 2, 2025, [https://www.cloudbees.com/blog/using-docker-compose-for-python-development](https://www.cloudbees.com/blog/using-docker-compose-for-python-development)  
27. Building best practices \- Docker Docs, fecha de acceso: diciembre 2, 2025, [https://docs.docker.com/build/building/best-practices/](https://docs.docker.com/build/building/best-practices/)  
28. How to run python program (or other cmd) with docker dependencies? \- Stack Overflow, fecha de acceso: diciembre 2, 2025, [https://stackoverflow.com/questions/57457596/how-to-run-python-program-or-other-cmd-with-docker-dependencies](https://stackoverflow.com/questions/57457596/how-to-run-python-program-or-other-cmd-with-docker-dependencies)  
29. bash \- How to automatically record all your terminal sessions with script utility, fecha de acceso: diciembre 2, 2025, [https://unix.stackexchange.com/questions/25639/how-to-automatically-record-all-your-terminal-sessions-with-script-utility](https://unix.stackexchange.com/questions/25639/how-to-automatically-record-all-your-terminal-sessions-with-script-utility)  
30. Record your terminal with script and scriptreplay \- Red Hat, fecha de acceso: diciembre 2, 2025, [https://www.redhat.com/en/blog/record-terminal-script-scriptreplay](https://www.redhat.com/en/blog/record-terminal-script-scriptreplay)  
31. How to capture terminal sessions and output with the Linux script command \- Red Hat, fecha de acceso: diciembre 2, 2025, [https://www.redhat.com/en/blog/linux-script-command](https://www.redhat.com/en/blog/linux-script-command)