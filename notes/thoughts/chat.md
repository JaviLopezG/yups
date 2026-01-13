[Gemini](https://gemini.google.com/app)  
🧑‍🌾 YUPS  
Mis cosas  
![Imagen de][image1]  
![Imagen de][image2]  
Octo-Juggler Debugged Server

Conversación fijada

Conversación fijada

Conversación fijada

Conversación fijada

Bash prompt wrapping issue fix

Solucionar resaltados en Vim

Renombrar carpetas de repositorios Git

Bash Tab Completion: Proceso y Acceso

Bash Hooks y Mecanismos de Interceptación

Renombrar Chrome Nativo en Fedora

Gist vs. Pages en GitHub

Limitar caracteres en Mustache sin datos

Manejo de Pull Requests: Cambios Menores

Validación de Direcciones de Email

GitHub Actions: Usos Comunes y Peculiares

Apps para compartir música en grupo

Revisión de IA para Evolución ERP

Revisión de Texto y Estrategia de Proyecto

Revisión y Mejora de Perfil Profesional

Flecha 3D Giratoria en el Prompt

# **Conversación con Gemini**

Actúa como mi Ingeniero Senior de confianza y compañero de desarrollo. Estamos trabajando en un proyecto llamado 'YUPS' (Yet Unnamed Package Suggestion?).

Toma este contexto completo para continuar la sesión exactamente donde la dejamos:

\[CONTEXTO DEL PROYECTO: YUPS\]

1\. Objetivo: Un asistente CLI inteligente (wrapper/hook) que detecta errores en la terminal (command not found, errores de sintaxis) y sugiere correcciones o soluciones usando IA.

2\. Estado Actual (Fase 1/2):

\- Migrando cliente de Python a Go (single static binary).

\- Backend API en VPS ('Trillian') con fallback híbrido: Prioridad LLM Local (Ollama/Marvin en casa vía Tailscale) \-\> Fallback Hugging Face (Gemma 2).

\- Hooks implementados en .bashrc para capturar 'command\_not\_found' y errores de ejecución (código de salida).

\- Funcionalidad actual: Sugiere paquetes y traduce comandos entre gestores (apt/dnf/pacman).

\[ARQUITECTURA & DECISIONES TÉCNICAS\]

1\. Go Way: Decidimos usar un solo binario monolítico (patrón Cobra) en lugar de separar ejecutables, aprovechando el Demand Paging del OS.

2\. Estrategia de Errores:

\- "Semáforo de Riesgo": Verde (re-ejecutar para capturar stderr), Amarillo (Dry-run), Rojo (Inferencia de contexto sin re-ejecución).

\- Rechazamos Docker para sandboxing (overkill).

\- Futuro (Fase Deep Tech): Investigar eBPF/kprobes para capturar output real (tty\_write) sin re-ejecución, o un PTY Wrapper (Meta-shell).

3\. Integración IA:

\- Uso de RAG con man-pages/tldr/cheatsheets (Planeado: Proyecto Babel Fish).

\- Reconocimiento de IA: Uso de Git Trailers ("Co-authored-by" o "Assisted-by") y notas en README para transparencia ética.

\[PERFIL DEL USUARIO (JAVI)\]

\- Estudiante de Máster/Ingeniero de Sistemas.

\- Nivel: "Usuario medio de consola", prefiere soluciones de una sola instalación (sudo install once) frente a configuraciones complejas por sesión.

\- Filosofía: "Teoría de las Cucharas" (Spoon Theory) para el desarrollo. Roadmap ambicioso pero paso a paso.

\- Acaba de recibir una "masterclass" sobre Microarquitectura (Register Renaming, ROB, Out-of-Order execution) y cómo el hardware optimiza el código lineal, desmitificando registros físicos vs lógicos.

\[TAREA INMEDIATA\]

Continuar el desarrollo o la investigación profunda (Deep Research) sobre los siguientes pasos del Roadmap, manteniendo el tono técnico, educativo y motivador. Estamos listos para profundizar en la implementación de Go, eBPF o la arquitectura del Backend.

How can i say in a smart way this two things inside the readme.md:

\- this is a project in wich i only work on weekends so take it easy, tasks sre not been wudenly magically accomplished

\- that yups (your universal prompt straw-boss) is the one that is always ready to help you, to follow your orders, so let him work for you, let him get his hands dirty on the job. You are the manager and he only do what you really want, so don't worry \-anf carry a towell- and keep doing whatever you were doing. Relax.

Trafucelas a inglés

¿Cómo se traduce yet Universal prompt straw boss?

He reescrito un poco tu párrafo, quizá se me ha ido un poco largo y probablemente haya escrito algo mal. Corrígelo:

This project is developed during weekends under the "Spoon Theory" philosophy: we advance as energy permits, prioritizing "Quick Wins" and learning over immediate perfection. If you notice the roadmap is advancing at a cruising speed rather than warp speed, please bear with us. Feature merges are not accomplished by magic but be sure they are comming.

Esta es la web actual del proyecto. Me gustaría que la modificaras un poco para que sea más "graciosa". Reforzar el mensaje de que el capataz te ayuda, que se mancha las manos por ti y reflejar también que yups (aunque todavía no lo hace pero va en camino) te ayuda con más comandos que la simple instalación de paquetes actual. También que es seguro gracias al modo dry-run que tenemos en el roadmap. En definitiva: hacer la web como si estuviesemos al final de nuestro roadmap y no al principio :)

necesito desplegar la web que tengo en el repositorio https://github.com/javilopezg/yups/status-web

Es una web estática (index.html y un directorio img)

En mi máquina Marvin que no está expuesta a internet. En el Caddy de Trillian tendré que redirigir status.yups.io a la máquina Marvin donde esté escuchando el servidor web.

Para no guarrear lo quiero montar en un contenedor de docker. ¿Cuál es la mejor imagen para usar en un docker-compose.yml para este fin?

¿Cuál es el paquete que hay que instalar en ubuntu para poder usar docker compose?

snowball@Marvin:\~/yups-status$ docker compose up \-d \--build

unknown shorthand flag: 'd' in \-d

Usage:  docker \[OPTIONS\] COMMAND \[ARG...\]

Run 'docker \--help' for more information

snowball@Marvin:\~/yups-status$ docker-compose up \-d \--build

Traceback (most recent call last):

  File "/usr/bin/docker-compose", line 33, in \<module\>

    sys.exit(load\_entry\_point('docker-compose==1.29.2', 'console\_scripts', 'docker-compose')())

             ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

  File "/usr/bin/docker-compose", line 25, in importlib\_load\_entry\_point

    return next(matches).load()

           ^^^^^^^^^^^^^^^^^^^^

  File "/usr/lib/python3.12/importlib/metadata/\_\_init\_\_.py", line 205, in load

    module \= import\_module(match.group('module'))

             ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

  File "/usr/lib/python3.12/importlib/\_\_init\_\_.py", line 90, in import\_module

    return \_bootstrap.\_gcd\_import(name\[level:\], package, level)

           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^

  File "\<frozen importlib.\_bootstrap\>", line 1387, in \_gcd\_import

  File "\<frozen importlib.\_bootstrap\>", line 1360, in \_find\_and\_load

  File "\<frozen importlib.\_bootstrap\>", line 1331, in \_find\_and\_load\_unlocked

  File "\<frozen importlib.\_bootstrap\>", line 935, in \_load\_unlocked

  File "\<frozen importlib.\_bootstrap\_external\>", line 995, in exec\_module

  File "\<frozen importlib.\_bootstrap\>", line 488, in \_call\_with\_frames\_removed

  File "/usr/lib/python3/dist-packages/compose/cli/main.py", line 9, in \<module\>

    from distutils.spawn import find\_executable

ModuleNotFoundError: No module named 'distutils'

snowball@Marvin:\~/yups-status$ docker-compose version

\-bash: /usr/bin/docker-compose: No such file or directory

snowball@Marvin:\~/yups-status$ /usr/local/bin/docker-compose version

Docker Compose version v2.29.1

snowball@Marvin:\~/yups-status$ mv /usr/local/bin/docker-compose /usr/bin/docker-compose

mv: cannot move '/usr/local/bin/docker-compose' to '/usr/bin/docker-compose': Permission denied

snowball@Marvin:\~/yups-status$ sudo mv /usr/local/bin/docker-compose /usr/bin/docker-compose

snowball@Marvin:\~/yups-status$ docker-compose version

Docker Compose version v2.29.1

snowball@Marvin:\~/yups-status$ docker compose up \-d \--build

unknown shorthand flag: 'd' in \-d

Usage:  docker \[OPTIONS\] COMMAND \[ARG...\]

Run 'docker \--help' for more information

snowball@Marvin:\~/yups-status$ 

Todo bien. Antes de ir a trillian, vamos a dejar esto apañado. ¿Cómo se hacía para no tener que usar sudo con docker? Había que crear un grupo docker, meter el usuario en ese grupo ¿y dar permisos al daemon?

Tengo dos inquietudes. Por un lado cuando intento relanzar el caddy para que se actualice con la nueva configuración, se reinician los tres servicios y creo que eso no debería ser así ¿no?

Por otro lado, parece que está dando un error en el caddy y el log tiene tal sobredosis de información que resulta inútil ¿qué puedo hacer?

snowball@Trillian:\~/yups-server$ docker compose up \-d \--build caddy

WARN\[0000\] Docker Compose is configured to build using Bake, but buildx isn't installed 

\[+\] Building 0.1s (14/14) FINISHED                                                        docker:default

 \=\> \[yups-dev internal\] load build definition from Dockerfile                                       0.0s

 \=\> \=\> transferring dockerfile: 590B                                                                0.0s

 \=\> \[yups-dev internal\] load metadata for docker.io/library/python:3.9-slim                         0.0s

 \=\> \[yups-dev internal\] load .dockerignore                                                          0.0s

 \=\> \=\> transferring context: 2B                                                                     0.0s

 \=\> \[yups-dev 1/8\] FROM docker.io/library/python:3.9-slim                                           0.0s

 \=\> \[yups-dev internal\] load build context                                                          0.0s

 \=\> \=\> transferring context: 63B                                                                    0.0s

 \=\> CACHED \[yups-dev 2/8\] WORKDIR /app                                                              0.0s

 \=\> CACHED \[yups-dev 3/8\] COPY requirements.txt .                                                   0.0s

 \=\> CACHED \[yups-dev 4/8\] RUN pip install \--no-cache-dir \-r requirements.txt                        0.0s

 \=\> CACHED \[yups-dev 5/8\] COPY app.py .                                                             0.0s

 \=\> CACHED \[yups-dev 6/8\] RUN mkdir logs                                                            0.0s

 \=\> CACHED \[yups-dev 7/8\] RUN useradd \-m yupsuser                                                   0.0s

 \=\> CACHED \[yups-dev 8/8\] RUN chown \-R yupsuser:yupsuser /app                                       0.0s

 \=\> \[yups-dev\] exporting to image                                                                   0.0s

 \=\> \=\> exporting layers                                                                             0.0s

 \=\> \=\> writing image sha256:00196ee123d7d0d9c7c3fc7ded44d99ade189a7e9c6e3d95847309240fe586fb        0.0s

 \=\> \=\> naming to docker.io/library/yups-server:latest                                               0.0s

 \=\> \[yups-dev\] resolving provenance for metadata file                                               0.0s

\[+\] Running 4/4

 ✔ yups-dev              Built                                                                      0.0s 

 ✔ Container yups\_prod   Running                                                                    0.0s 

 ✔ Container yups\_dev    Running                                                                    0.0s 

 ✔ Container yups\_caddy  Running                                                                    0.0s 

snowball@Trillian:\~/yups-server$ cat Caddyfile 

api.yups.io {

    reverse\_proxy yups-prod:5000

}

dev.yups.io {

    reverse\_proxy yups-dev:5000

}

status.yups.io {

    reverse\_proxy 100.70.90.66:8080

}

yups.io {

    handle\_path /yups/\* {

        rewrite \* /yups{uri}

        reverse\_proxy yups-prod:5000

    }

    handle {

        redir https://www.yups.io{uri} permanent

    }

}

snowball@Trillian:\~/yups-server$ ping status.yups.io \-c 3

PING yups.io (213.109.161.164) 56(84) bytes of data.

64 bytes from yups.supersrv.de (213.109.161.164): icmp\_seq=1 ttl=64 time=0.038 ms

64 bytes from yups.supersrv.de (213.109.161.164): icmp\_seq=2 ttl=64 time=0.079 ms

64 bytes from yups.supersrv.de (213.109.161.164): icmp\_seq=3 ttl=64 time=0.065 ms

\--- yups.io ping statistics \---

3 packets transmitted, 3 received, 0% packet loss, time 2079ms

rtt min/avg/max/mdev \= 0.038/0.060/0.079/0.017 ms

snowball@Trillian:\~/yups-server$ wget status.yups.io

\--2025-12-06 23:12:53--  http://status.yups.io/

Resolving status.yups.io (status.yups.io)... 213.109.161.164

Connecting to status.yups.io (status.yups.io)|213.109.161.164|:80... connected.

HTTP request sent, awaiting response... 308 Permanent Redirect

Location: https://status.yups.io/ \[following\]

\--2025-12-06 23:12:53--  https://status.yups.io/

Connecting to status.yups.io (status.yups.io)|213.109.161.164|:443... connected.

OpenSSL: error:0A000438:SSL routines::tlsv1 alert internal error

Unable to establish SSL connection.

snowball@Trillian:\~/yups-server$ 

snowball@Trillian:\~/yups-server$ docker compose logs \--tail=50 caddy

yups\_caddy  | {"level":"info","ts":1764988057.8479507,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["yups.io"\],"cert\_hash":"0a5803900f72ac7abd1a46781c346a1cb722621f8b1d6c6329749d0a6a116edf","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BqCX1dJdy2bbJ4GEDHT7NfN5","cert\_expiry":1771873137,"selected\_time":1769206408,"next\_update":1765009657.845499,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1764988057.982252,"msg":"got renewal info","names":\["api.yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769206679,"recheck\_after":1765009657.9822385,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1764988057.9849257,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["api.yups.io"\],"cert\_hash":"2dc22ac993beb942262ded6668e0cc672920dab410731740fe24fe10a7f4edf6","ari\_unique\_id":"jw0TovYuftFQbDMYOF1ZjiNykco.BnoVaZ5eY1nB2z-bfDi6\_5ey","cert\_expiry":1771873137,"selected\_time":1769300987,"next\_update":1765009657.9822385,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1764988058.1195428,"msg":"got renewal info","names":\["dev.yups.io"\],"window\_start":1769204584,"window\_end":1769360033,"selected\_time":1769347984,"recheck\_after":1765009658.1195297,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1764988058.1220276,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["dev.yups.io"\],"cert\_hash":"f3349280a57f274fcc1d31833399a47af893e278433ab18b5f7d702aaf493ec5","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BmjBNC2\_xUJVl0fhOUwLxvZ6","cert\_expiry":1771873139,"selected\_time":1769336287,"next\_update":1765009658.1195297,"explanation\_url":""}

yups\_caddy  | {"level":"warn","ts":1765005755.8918145,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.217.238.109:38787","user\_agent":"Mozilla/5.0 (Linux; Android 12; V2134) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765005756.872,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.217.238.109:19260","user\_agent":"Mozilla/5.0 (Linux; Android 12; V2134) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765005758.1125338,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.217.238.109:16921","user\_agent":"Mozilla/5.0 (Linux; Android 12; SM-A525F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765005758.386899,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.217.238.109:19260","user\_agent":"Mozilla/5.0 (Linux; Android 12; SM-A525F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765006008.157851,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.217.238.109:44437","user\_agent":"Mozilla/5.0 (Linux; Android 13; SM-G991U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765006008.4274762,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.217.238.109:19260","user\_agent":"Mozilla/5.0 (Linux; Android 13; SM-G991U) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"info","ts":1765010257.974756,"msg":"got renewal info","names":\["api.yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769249773,"recheck\_after":1765031857.9747412,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765010257.977202,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["api.yups.io"\],"cert\_hash":"2dc22ac993beb942262ded6668e0cc672920dab410731740fe24fe10a7f4edf6","ari\_unique\_id":"jw0TovYuftFQbDMYOF1ZjiNykco.BnoVaZ5eY1nB2z-bfDi6\_5ey","cert\_expiry":1771873137,"selected\_time":1769300987,"next\_update":1765031857.9747412,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765010258.1248217,"msg":"got renewal info","names":\["dev.yups.io"\],"window\_start":1769204584,"window\_end":1769360033,"selected\_time":1769300159,"recheck\_after":1765031858.1248074,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765010258.126973,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["dev.yups.io"\],"cert\_hash":"f3349280a57f274fcc1d31833399a47af893e278433ab18b5f7d702aaf493ec5","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BmjBNC2\_xUJVl0fhOUwLxvZ6","cert\_expiry":1771873139,"selected\_time":1769336287,"next\_update":1765031858.1248074,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765010258.2619658,"msg":"got renewal info","names":\["yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769288371,"recheck\_after":1765031858.2619555,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765010258.2637956,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["yups.io"\],"cert\_hash":"0a5803900f72ac7abd1a46781c346a1cb722621f8b1d6c6329749d0a6a116edf","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BqCX1dJdy2bbJ4GEDHT7NfN5","cert\_expiry":1771873137,"selected\_time":1769206408,"next\_update":1765031858.2619555,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765021057.4333484,"logger":"tls","msg":"storage cleaning happened too recently; skipping for now","storage":"FileStorage:/data/caddy","instance":"b5cf4721-b438-49a9-8c33-e8b89fca487f","try\_again":1765107457.433344,"try\_again\_in":86399.999999249}

yups\_caddy  | {"level":"info","ts":1765021057.4334917,"logger":"tls","msg":"finished cleaning storage units"}

yups\_caddy  | {"level":"info","ts":1765032457.861113,"msg":"got renewal info","names":\["yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769329746,"recheck\_after":1765054057.861102,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765032457.8635535,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["yups.io"\],"cert\_hash":"0a5803900f72ac7abd1a46781c346a1cb722621f8b1d6c6329749d0a6a116edf","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BqCX1dJdy2bbJ4GEDHT7NfN5","cert\_expiry":1771873137,"selected\_time":1769206408,"next\_update":1765054057.861102,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765032457.9965675,"msg":"got renewal info","names":\["api.yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769327669,"recheck\_after":1765054057.996557,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765032457.9986398,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["api.yups.io"\],"cert\_hash":"2dc22ac993beb942262ded6668e0cc672920dab410731740fe24fe10a7f4edf6","ari\_unique\_id":"jw0TovYuftFQbDMYOF1ZjiNykco.BnoVaZ5eY1nB2z-bfDi6\_5ey","cert\_expiry":1771873137,"selected\_time":1769300987,"next\_update":1765054057.996557,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765032458.133241,"msg":"got renewal info","names":\["dev.yups.io"\],"window\_start":1769204584,"window\_end":1769360033,"selected\_time":1769297731,"recheck\_after":1765054058.1332276,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765032458.1357536,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["dev.yups.io"\],"cert\_hash":"f3349280a57f274fcc1d31833399a47af893e278433ab18b5f7d702aaf493ec5","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BmjBNC2\_xUJVl0fhOUwLxvZ6","cert\_expiry":1771873139,"selected\_time":1769336287,"next\_update":1765054058.1332276,"explanation\_url":""}

yups\_caddy  | {"level":"warn","ts":1765041470.5145073,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.194.87.87:46096","user\_agent":"Mozilla/5.0 (Linux; Android 13; SM-S908E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765041472.3475416,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.194.87.87:46112","user\_agent":"Mozilla/5.0 (Linux; Android 13; SM-S908E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765041488.6795497,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.194.87.87:46107","user\_agent":"Mozilla/5.0 (Linux; Android 12; SM-A525F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765041491.4637523,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.194.87.87:46112","user\_agent":"Mozilla/5.0 (Linux; Android 12; SM-A525F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765041982.5817473,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.194.87.87:46259","user\_agent":"Mozilla/5.0 (Linux; Android 12; V2134) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765041982.74874,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"4.194.87.87:46112","user\_agent":"Mozilla/5.0 (Linux; Android 12; V2134) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765048030.6964185,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"20.184.52.113:22402","user\_agent":"Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765048034.5205543,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"20.184.52.113:22407","user\_agent":"Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765048041.250461,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"20.184.52.113:22505","user\_agent":"Mozilla/5.0 (Linux; Android 11; 21081111RG) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765048041.4087732,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"20.184.52.113:22407","user\_agent":"Mozilla/5.0 (Linux; Android 11; 21081111RG) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765048748.0651326,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"20.184.52.113:22413","user\_agent":"Mozilla/5.0 (Linux; Android 12; V2134) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765048748.2230668,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"20.184.52.113:22407","user\_agent":"Mozilla/5.0 (Linux; Android 12; V2134) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"error","ts":1765053829.6167367,"logger":"http.log.error","msg":"dial tcp: lookup yups-dev on 127.0.0.11:53: server misbehaving","request":{"remote\_ip":"172.18.0.1","remote\_port":"42504","client\_ip":"172.18.0.1","proto":"HTTP/2.0","method":"POST","host":"dev.yups.io","uri":"/yups/v1/chat","headers":{"Accept":\["\*/\*"\],"Content-Type":\["application/json"\],"X-Yups-Client-Auth":\["yups-secret-v1-camaleon"\],"Content-Length":\["192"\],"User-Agent":\["curl/8.15.0"\]},"tls":{"resumed":false,"version":772,"cipher\_suite":4865,"proto":"h2","server\_name":"dev.yups.io"}},"duration":0.002076493,"status":502,"err\_id":"gngmgqc5a","err\_trace":"reverseproxy.statusError (reverseproxy.go:1390)"}

yups\_caddy  | {"level":"error","ts":1765053889.3042774,"logger":"http.log.error","msg":"dial tcp: lookup yups-dev on 127.0.0.11:53: server misbehaving","request":{"remote\_ip":"172.18.0.1","remote\_port":"37166","client\_ip":"172.18.0.1","proto":"HTTP/2.0","method":"POST","host":"dev.yups.io","uri":"/yups/v1/chat","headers":{"Content-Type":\["application/json"\],"X-Yups-Client-Auth":\["yups-secret-v1-camaleon"\],"Content-Length":\["192"\],"User-Agent":\["curl/8.15.0"\],"Accept":\["\*/\*"\]},"tls":{"resumed":false,"version":772,"cipher\_suite":4865,"proto":"h2","server\_name":"dev.yups.io"}},"duration":0.001557634,"status":502,"err\_id":"uk7i2zhri","err\_trace":"reverseproxy.statusError (reverseproxy.go:1390)"}

yups\_caddy  | {"level":"error","ts":1765054076.207693,"logger":"http.log.error","msg":"dial tcp: lookup yups-dev on 127.0.0.11:53: server misbehaving","request":{"remote\_ip":"172.18.0.1","remote\_port":"54368","client\_ip":"172.18.0.1","proto":"HTTP/2.0","method":"POST","host":"dev.yups.io","uri":"/yups/v1/chat","headers":{"Content-Type":\["application/json"\],"X-Yups-Client-Auth":\["yups-secret-v1-camaleon"\],"Content-Length":\["192"\],"User-Agent":\["curl/8.15.0"\],"Accept":\["\*/\*"\]},"tls":{"resumed":false,"version":772,"cipher\_suite":4865,"proto":"h2","server\_name":"dev.yups.io"}},"duration":0.002039312,"status":502,"err\_id":"02c9n9ypq","err\_trace":"reverseproxy.statusError (reverseproxy.go:1390)"}

yups\_caddy  | {"level":"info","ts":1765054657.9983616,"msg":"got renewal info","names":\["api.yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769228371,"recheck\_after":1765076257.9983482,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765054658.0023432,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["api.yups.io"\],"cert\_hash":"2dc22ac993beb942262ded6668e0cc672920dab410731740fe24fe10a7f4edf6","ari\_unique\_id":"jw0TovYuftFQbDMYOF1ZjiNykco.BnoVaZ5eY1nB2z-bfDi6\_5ey","cert\_expiry":1771873137,"selected\_time":1769300987,"next\_update":1765076257.9983482,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765054658.1600788,"msg":"got renewal info","names":\["dev.yups.io"\],"window\_start":1769204584,"window\_end":1769360033,"selected\_time":1769333772,"recheck\_after":1765076258.160063,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765054658.1628563,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["dev.yups.io"\],"cert\_hash":"f3349280a57f274fcc1d31833399a47af893e278433ab18b5f7d702aaf493ec5","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BmjBNC2\_xUJVl0fhOUwLxvZ6","cert\_expiry":1771873139,"selected\_time":1769336287,"next\_update":1765076258.160063,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765054658.3113444,"msg":"got renewal info","names":\["yups.io"\],"window\_start":1769204582,"window\_end":1769360031,"selected\_time":1769346013,"recheck\_after":1765076258.311334,"explanation\_url":""}

yups\_caddy  | {"level":"info","ts":1765054658.3136857,"logger":"tls.cache.maintenance","msg":"updated and stored ACME renewal information","identifiers":\["yups.io"\],"cert\_hash":"0a5803900f72ac7abd1a46781c346a1cb722621f8b1d6c6329749d0a6a116edf","ari\_unique\_id":"rkie3IcdRKBv2qLlYHQEeMKcAIA.BqCX1dJdy2bbJ4GEDHT7NfN5","cert\_expiry":1771873137,"selected\_time":1769206408,"next\_update":1765076258.311334,"explanation\_url":""}

yups\_caddy  | {"level":"warn","ts":1765057664.5602784,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"40.119.192.84:60673","user\_agent":"Mozilla/5.0 (iPhone; CPU iPhone OS 17\_0\_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0.1 Mobile/15E148 Safari/604.1","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765057665.1949215,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"40.119.192.84:60691","user\_agent":"Mozilla/5.0 (iPhone; CPU iPhone OS 17\_0\_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0.1 Mobile/15E148 Safari/604.1","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765057666.2583952,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"40.119.192.84:60784","user\_agent":"Mozilla/5.0 (Linux; Android 12; 2201116SG) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

yups\_caddy  | {"level":"warn","ts":1765057666.4692328,"logger":"http","msg":"looking up info for HTTP challenge","host":"yups.io","remote\_addr":"40.119.192.84:60691","user\_agent":"Mozilla/5.0 (Linux; Android 12; 2201116SG) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Mobile Safari/537.36","error":"no information found to solve challenge for identifier: yups.io"}

snowball@Trillian:\~/yups-server$ ping marvin \-c 3

PING marvin.tail255a84.ts.net (100.70.90.66) 56(84) bytes of data.

64 bytes from marvin.tail255a84.ts.net (100.70.90.66): icmp\_seq=1 ttl=64 time=91.9 ms

64 bytes from marvin.tail255a84.ts.net (100.70.90.66): icmp\_seq=2 ttl=64 time=44.8 ms

64 bytes from marvin.tail255a84.ts.net (100.70.90.66): icmp\_seq=3 ttl=64 time=43.5 ms

\--- marvin.tail255a84.ts.net ping statistics \---

3 packets transmitted, 3 received, 0% packet loss, time 2003ms

rtt min/avg/max/mdev \= 43.475/60.057/91.911/22.530 ms

snowball@Trillian:\~/yups-server$ 

snowball@Trillian:\~/yups-server$ docker compose exec caddy ping \-c 2 100.70.90.66

PING 100.70.90.66 (100.70.90.66): 56 data bytes

64 bytes from 100.70.90.66: seq=0 ttl=63 time=92.763 ms

64 bytes from 100.70.90.66: seq=1 ttl=63 time=45.385 ms

\--- 100.70.90.66 ping statistics \---

2 packets transmitted, 2 packets received, 0% packet loss

round-trip min/avg/max \= 45.385/69.074/92.763 ms

snowball@Trillian:\~/yups-server$ docker compose exec \-w /etc/caddy caddy caddy reload

2025/12/06 22:21:36.175 INFO using adjacent Caddyfile

2025/12/06 22:21:36.177 INFO adapted config to JSON {"adapter": "caddyfile"}

2025/12/06 22:21:36.177 WARN Caddyfile input is not formatted; run 'caddy fmt \--overwrite' to fix inconsistencies {"adapter": "caddyfile", "file": "Caddyfile", "line": 2}

snowball@Trillian:\~/yups-server$ vi Caddyfile 

snowball@Trillian:\~/yups-server$ docker compose exec \-w /etc/caddy caddy caddy fmt \--overwrite

snowball@Trillian:\~/yups-server$ cat Caddyfile 

api.yups.io {

    reverse\_proxy yups-prod:5000

}

dev.yups.io {

    reverse\_proxy yups-dev:5000

}

status.yups.io {

    reverse\_proxy 100.70.90.66:8080

}

yups.io {

    handle\_path /yups/\* {

        rewrite \* /yups{uri}

        reverse\_proxy yups-prod:5000

    }

    handle {

        redir https://www.yups.io{uri} permanent

    }

}

snowball@Trillian:\~/yups-server$ docker compose exec \-w /etc/caddy caddy caddy reload

2025/12/06 22:22:49.251 INFO using adjacent Caddyfile

2025/12/06 22:22:49.252 INFO adapted config to JSON {"adapter": "caddyfile"}

snowball@Trillian:\~/yups-server$ wget status.yups.io

\--2025-12-06 23:23:06--  http://status.yups.io/

Resolving status.yups.io (status.yups.io)... 213.109.161.164

Connecting to status.yups.io (status.yups.io)|213.109.161.164|:80... connected.

HTTP request sent, awaiting response... 308 Permanent Redirect

Location: https://status.yups.io/ \[following\]

\--2025-12-06 23:23:06--  https://status.yups.io/

Connecting to status.yups.io (status.yups.io)|213.109.161.164|:443... connected.

OpenSSL: error:0A000438:SSL routines::tlsv1 alert internal error

Unable to establish SSL connection.

snowball@Trillian:\~/yups-server$ 

¿Cómo habíamos dicho que podíamos hacer un dryrun? Era la combinación de un comando y un sistema de ficheros virtual o algo así

Dame una explicación breve de cómo usar cobra y viper en go

cómo pongo en goland un shortcut para poner o quitar comentarios? El que tiene por defecto es ctr+/ pero yo no tengo la tecla /, el slash está en el 7

Vamos a empezar con algo fácil. En el terminal de goland cada vez que lanzo un comando me da un error, parece como si ejecutase el código de bashrc pero no mantuviera el propio código ahí definido en memoria. En el terminal normal no pasa eso como puedes ver:

\[📦 com.jetbrains.GoLand cli\]$ ls

cmd  go.mod  go.sum  internal  main.go

sh: \_yups\_ce\_handle: orden no encontrada

\[📦 com.jetbrains.GoLand cli\]$ 

javi@Arthur:\~$ cat .bashrc

\# .bashrc

\# Source global definitions

if \[ \-f /etc/bashrc \]; then

    . /etc/bashrc

fi

\# User specific environment

if \! \[\[ "$PATH" \=\~ "$HOME/.local/bin:$HOME/bin:" \]\]; then

    PATH="$HOME/.local/bin:$HOME/bin:$PATH"

fi

export PATH

\# Uncomment the following line if you don't like systemctl's auto-paging feature:

\# export SYSTEMD\_PAGER=

\# User specific aliases and functions

if \[ \-d \~/.bashrc.d \]; then

    for rc in \~/.bashrc.d/\*; do

        if \[ \-f "$rc" \]; then

            . "$rc"

        fi

    done

fi

unset rc

\# \--- YUPS\_HOOK\_START \---

\# Hooks for the YUPS project

\# 1: Command Not Found Hook

command\_not\_found\_handle() {

    if "/usr/local/bin/yups" \--cnf-handle "$@"; then

        return $?

    else

        return 127

    fi

}

\# 2: Command Error Hook

\_yups\_ce\_handle() {

    local exit\_code=$?

    if \[\[ $exit\_code \-eq 0 \]\] || \[\[ $exit\_code \-eq 127 \]\]; then

        return

    fi

    local last\_command\_text=$(history 1 | sed 's/^\[ \]\*\[0-9\]\\+\[ \]\\+//')

    "/usr/local/bin/yups" \--ce-handle "$exit\_code" "$last\_command\_text"

}

if \[\[ \-z "$PROMPT\_COMMAND" \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle"

elif \! \[\[ "$PROMPT\_COMMAND" \== \*"\_yups\_ce\_handle"\* \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle;${PROMPT\_COMMAND}"

fi

\# \--- YUPS\_HOOK\_END \---

javi@Arthur:\~$ ls \-R yups

yups:

cli    dumb\_tests.sh  LICENSE    server          status-web           uninstall.sh  yups

CNAME  install.sh     README.md  smart\_tests.sh  testing\_environment  web

yups/cli:

cmd  go.mod  go.sum  internal  main.go

yups/cli/cmd:

install.go  root.go

yups/cli/internal:

parser  sys

yups/cli/internal/parser:

bash.go

yups/cli/internal/sys:

scanner.go  system.go  system\_test.go

yups/server:

app.py  Caddyfile  docker-compose.yml  Dockerfile  requirements.txt

yups/status-web:

img  index.html

yups/status-web/img:

apple-touch-icon.png  favicon.ico  logo.png        site.webmanifest              web-app-manifest-512x512.png

favicon-96x96.png     favicon.svg  logo\_trans.png  web-app-manifest-192x192.png

yups/testing\_environment:

docker-compose.yml  Dockerfile.template

yups/web:

img  index.html

yups/web/img:

apple-touch-icon.png  favicon.ico  logo.png        site.webmanifest              web-app-manifest-512x512.png

favicon-96x96.png     favicon.svg  logo\_trans.png  web-app-manifest-192x192.png

ninguno de los dos, quiero que me expliques esta salida al hacer el run, porque no sé de donde sale la mitad de lo que ahí dice:

Executing...

The YUPS CLI handles your command not found and other

prompt errors. It can solve any situation or requirement by

asking to an online LLM.

Usage:

  yups \[command\]

Available Commands:

  completion  Generate the autocompletion script for the specified shell

  help        Help about any command

  install     Instala paquetes usando IA para detectar dependencias

Flags:

      \--config string   configuration file (default: $HOME/.yups/config.toml)

  \-d, \--debug           set the log level to debug

  \-h, \--help            help for yups

Use "yups \[command\] \--help" for more information about a command.

Process finished with the exit code 0

Ahora mismo en cmd yo solo tengo definidos dos archivos:

root.go

package cmd

import (

"fmt"

"log/slog"

"os"

"github.com/spf13/cobra"

"github.com/spf13/viper"

)

var (

cfgFile string

debug bool

)

var rootCmd \= \&cobra.Command{

Use: "yups",

Short: "YUPS: Your Universal Prompt Straw-boss (AI Powered)",

Long: \`The YUPS CLI handles your command not found and other

prompt errors. It can solve any situation or requirement by

asking to an online LLM.\`,

PersistentPreRun: func(cmd \*cobra.Command, args \[\]string) {

setupLogger(debug)

},

}

func Execute() {

fmt.Println("Executing...")

if err := rootCmd.Execute(); err \!= nil {

fmt.Println(err)

os.Exit(1)

}

}

func init() {

cobra.OnInitialize(initConfig)

rootCmd.PersistentFlags().

StringVar(\&cfgFile, "config", "",

"configuration file (default: $HOME/.yups/config.toml)")

rootCmd.PersistentFlags().

BoolVarP(\&debug, "debug", "d",

false, "set the log level to debug")

viper.BindPFlag("debug",

rootCmd.PersistentFlags().Lookup("debug"))

}

func initConfig() {

if cfgFile \!= "" {

viper.SetConfigFile(cfgFile)

} else {

home, err := os.UserHomeDir()

if err \!= nil {

fmt.Println(err)

os.Exit(1)

}

viper.AddConfigPath(home \+ "/.yups")

viper.SetConfigType("toml")

viper.SetConfigName("config")

}

viper.AutomaticEnv()

if err := viper.ReadInConfig(); err \== nil && debug {

fmt.Println("📄 Setting config file:", viper.ConfigFileUsed())

}

}

func setupLogger(isDebug bool) {

opts := \&slog.HandlerOptions{

Level: slog.LevelInfo,

}

if isDebug {

opts.Level \= slog.LevelDebug

}

logger := slog.New(slog.NewTextHandler(os.Stderr, opts))

slog.SetDefault(logger)

}

install.go

package cmd

import (

//"fmt"

"time"

"log/slog"

"github.com/fatih/color"

"github.com/spf13/cobra"

"github.com/tu-usuario/yups/cli/internal/sys"

)

var installCmd \= \&cobra.Command{

Use: "install \[paquetes...\]",

Short: "Instala paquetes usando IA para detectar dependencias",

Args: cobra.MinimumNArgs(1),

Run: func(cmd \*cobra.Command, args \[\]string) {

// 1\. Detectar sistema

slog.Debug("Analizando sistema...")

info := sys.GetSystemInfo()

color.Cyan("🔍 Sistema detectado: %s (Root: %v) | Gestor: %s", info.OS, info.IsRoot, info.Manager)

// 2\. Aquí iría la llamada a tu API (futura implementación)

slog.Info("Consultando a YUPS Server...", "query", args)

// Simulación de espera (spinner visual mental)

time.Sleep(500 \* time.Millisecond)

if info.Manager \== "unknown" {

color.Red("❌ No se ha detectado un gestor de paquetes soportado.")

return

}

color.Green("✅ Listo para instalar: %v", args)

},

}

func init() {

rootCmd.AddCommand(installCmd)

}

¿De dónde sale el resto de la información? (completion, help, flags...)

¿Para qué es el directorio .idea?

vale. he creado el archivo command-not-found.go en el que pretendo meter la funcionalidad de lo que tiene que hacer el handler. Lo cierto es que he borrado el install.go sin leer porque ahora no tengo muy claro cual es la estructura de un archivo del package cmd. De momento lo que he puesto "rompe" la ejecución de cobra:

package cmd

func init() {

rootCmd.PersistentFlags().

StringVar(\&cfgFile, "command-not-found", "",

\`this should be executed by the system handler,

it just try to understand what the user wants

without querying the online LLM\`)

}

func Execute() {

//TODO get the arguments and parse them to identify the command

}

Y la ejecución es esta (aunque estrictamente hablado debería haber salido con 1 u otro valor distinto de cero xD):

The YUPS CLI handles your command not found and other

prompt errors. It can solve any situation or requirement 

by querying an online LLM.

Process finished with the exit code 0

Pero yo quiero crear un flag. ¿Por qué? Esta invocación no la hará nunca (o no debería hacerla) el usuario. El usuario sólo llamará a "yups lo\_que\_quiera\_que\_pase", y dado que eso puede ser cualquier cosa como "yups handle command errors with an explanation" quiero evitar usar los comandos sin guiones para evitar errores o confusión. Por esto la idea es crear 3 comandos como flags: \--command-not-found (para manejar el cnf), \--command-error (para manejar otros errores) y \--auto-config (que se tendrá que ejecutar la primera vez si yups no está instalado/configurado). Todo lo demás que escriba el usuario lo tengo que entender como una query y hacerle un análisis específico con ayuda del llm

Un problema menor pero que me resulta muy molesto. Goland no me está mostrando los emojis, supongo que es algo del encoding porque recuerdo haberlo usado en windows (ahora estoy en fedora) y sí los mostraba. En los settings he verificado que no está activa la opción de avoid emoticons and emojis. ¿Se te ocurre qué puede ser?

No ha funcionado ninguna de las opciones que me has dado, aunque la del jdk no la he podido poner en práctica porque no he encontrado nada que se llamase parecido. He pensado que podría ser por ser un flatpack pero lo he desinstalado y he utilizado la herramienta de jetbrains para instalar (toolbox) y siguen sin mostrarse. ¿Se te ocurre alguna solución o me olvido de los bonitos emojis en mi código?

Sigo con los emojis. Una curiosidad, si en lugar de poner el granjero pongo un smile sí que me imprime un caracter, pero como de un tipo de letra antigua tipo symbols en Windows. Eso si no tengo puesto ningún tipo de letra como fallback, si pongo el noto color emoji deja de mostrar ese caracter especial.

Ha desaparecido el emoji raro y ahora no muestra nada ponga o no ponga la letra de fallback

Bueno, no me queda más remedio que ignorar lo de los emojis. Recuerda que no puedo usar emojis en este proyecto.

He creado los tres comandos (flags) pero ahora cobra no muestra nada al ejecutarlo aunque tampoco se produce ningún error (he puesto en el main un defer para ver los panic).

javi@Arthur:\~/yups/cli/cmd$ tail \-n \+1 \*

\==\> auto-config.go \<==

package cmd

import (

"log/slog"

)

var acMode bool

func init() {

rootCmd.Flags().BoolVar(\&acMode, "auto-config",

false, "Set configuration to default values.")

}

func handleAC() {

slog.Info("Straw-boss (AC Mode).")

//TODO identify the command and make suggestions.

}

\==\> command-error.go \<==

package cmd

import (

"log/slog"

"strings"

)

var ceMode bool

func init() {

rootCmd.Flags().BoolVar(\&ceMode, "command-error",

false, "System hook for command error.")

}

func handleCE(args \[\]string) {

slog.Info("Straw-boss (CE Mode) analyzing: %s",

strings.Join(args, " "))

//TODO identify the command and make suggestions.

}

\==\> command-not-found.go \<==

package cmd

import (

"log/slog"

"strings"

)

var cnfMode bool

func init() {

rootCmd.Flags().BoolVar(\&cnfMode, "command-not-found",

false, "System hook for command not found.")

}

func handleCNF(args \[\]string) {

slog.Info("Straw-boss (CNF Mode) analyzing: %s",

strings.Join(args, " "))

//TODO identify the command and make suggestions.

}

\==\> root.go \<==

package cmd

import (

"fmt"

"log/slog"

"os"

"github.com/spf13/cobra"

"github.com/spf13/viper"

)

var (

cfgFile string

debug   bool

)

var rootCmd \= \&cobra.Command{

Use:   "yups \[query\]",

Short: "YUPS: Your Universal Prompt Straw-boss (AI Powered)",

Long: \`The YUPS CLI handles your command not found and other

prompt errors. It can solve any situation or requirement 

by querying an online LLM.\`,

Run: func(cmd \*cobra.Command, args \[\]string) {

if cnfMode {

handleCNF(args)

return

}

if ceMode {

handleCE(args)

return

}

if acMode {

handleAC()

return

}

processQuery(args)

},

}

func processQuery(args \[\]string) {

//TODO process user query

}

func Execute() {

slog.Debug("Executing yups")

if err := rootCmd.Execute(); err \!= nil {

fmt.Println(err)

os.Exit(1)

}

}

func init() {

cobra.OnInitialize(initConfig)

rootCmd.PersistentFlags().

StringVar(\&cfgFile, "config", "",

"configuration file (default: $HOME/.yups/config.toml)")

rootCmd.PersistentFlags().

BoolVarP(\&debug, "debug", "d",

false, "set the log level to debug")

viper.BindPFlag("debug",

rootCmd.PersistentFlags().Lookup("debug"))

}

func initConfig() {

if cfgFile \!= "" {

viper.SetConfigFile(cfgFile)

} else {

home, err := os.UserHomeDir()

if err \!= nil {

fmt.Println(err)

os.Exit(1)

}

viper.AddConfigPath(home \+ "/.yups")

viper.SetConfigType("toml")

viper.SetConfigName("config")

}

viper.AutomaticEnv()

if err := viper.ReadInConfig(); err \== nil && debug {

fmt.Println("📄 Setting config file:", viper.ConfigFileUsed())

}

}

func setupLogger(isDebug bool) {

opts := \&slog.HandlerOptions{

Level: slog.LevelInfo,

}

if isDebug {

opts.Level \= slog.LevelDebug

}

logger := slog.New(slog.NewTextHandler(os.Stderr, opts))

slog.SetDefault(logger)

}

javi@Arthur:\~/yups/cli/cmd$ 

Cobra sigue sin mostrar nada como ayuda o mensaje por defecto, si quito el Run sí que aparece la descripción larga:

javi@Arthur:\~/yups/cli/cmd$ tail \-n \+1 \*

\==\> auto-config.go \<==

package cmd

import (

"log/slog"

)

var acMode bool

func init() {

rootCmd.Flags().BoolVar(\&acMode, "auto-config",

false, "Set configuration to default values.")

}

func handleAC() {

slog.Info("Straw-boss (AC Mode).")

//TODO identify the command and make suggestions.

}

\==\> command-error.go \<==

package cmd

import (

"log/slog"

"strings"

)

var ceMode bool

func init() {

rootCmd.Flags().BoolVar(\&ceMode, "command-error",

false, "System hook for command error.")

}

func handleCE(args \[\]string) {

slog.Info("Straw-boss (CE Mode) analyzing: ",

"query", strings.Join(args, " "))

//TODO identify the command and make suggestions.

}

\==\> command-not-found.go \<==

package cmd

import (

"log/slog"

"strings"

)

var cnfMode bool

func init() {

rootCmd.Flags().BoolVar(\&cnfMode, "command-not-found",

false, "System hook for command not found.")

}

func handleCNF(args \[\]string) {

slog.Info("Straw-boss (CNF Mode) analyzing: ",

"query", strings.Join(args, " "))

//TODO identify the command and make suggestions.

}

\==\> root.go \<==

package cmd

import (

"fmt"

"log/slog"

"os"

"github.com/spf13/cobra"

"github.com/spf13/viper"

)

var (

cfgFile string

debug   bool

)

var rootCmd \= \&cobra.Command{

Use:   "yups \[query\]",

Short: "YUPS: Your Universal Prompt Straw-boss (AI Powered)",

Long: \`The YUPS CLI handles your command not found and other

prompt errors. It can solve any situation or requirement 

by querying an online LLM.\`,

PersistentPreRun: func(cmd \*cobra.Command, args \[\]string) {

setupLogger(debug)

},

Run: func(cmd \*cobra.Command, args \[\]string) {

if cnfMode {

handleCNF(args)

return

}

if ceMode {

handleCE(args)

return

}

if acMode {

handleAC()

return

}

processQuery(args)

},

}

func processQuery(args \[\]string) {

//TODO process user query

}

func Execute() {

slog.Debug("Executing yups")

if err := rootCmd.Execute(); err \!= nil {

fmt.Println(err)

os.Exit(1)

}

}

func init() {

cobra.OnInitialize(initConfig)

rootCmd.PersistentFlags().

StringVar(\&cfgFile, "config", "",

"configuration file (default: $HOME/.yups/config.toml)")

rootCmd.PersistentFlags().

BoolVarP(\&debug, "debug", "d",

false, "set the log level to debug")

viper.BindPFlag("debug",

rootCmd.PersistentFlags().Lookup("debug"))

}

func initConfig() {

if cfgFile \!= "" {

viper.SetConfigFile(cfgFile)

} else {

home, err := os.UserHomeDir()

if err \!= nil {

fmt.Println(err)

os.Exit(1)

}

viper.AddConfigPath(home \+ "/.yups")

viper.SetConfigType("toml")

viper.SetConfigName("config")

}

viper.AutomaticEnv()

if err := viper.ReadInConfig(); err \== nil && debug {

fmt.Println("📄 Setting config file:", viper.ConfigFileUsed())

}

}

func setupLogger(isDebug bool) {

opts := \&slog.HandlerOptions{

Level: slog.LevelInfo,

}

if isDebug {

opts.Level \= slog.LevelDebug

}

logger := slog.New(slog.NewTextHandler(os.Stderr, opts))

slog.SetDefault(logger)

}

javi@Arthur:\~/yups/cli/cmd$ 

¿Cuál es el go way de escribir esto?

err := viper.ReadInConfig()

if err \== nil && debug {

slog.Debug("Setting config file.", "ConfigFileUsed", viper.ConfigFileUsed())

}

if err \!= nil && (ok=err.(viper.ConfigFileNotFoundError); ok) {

handleAC()

}

¿Cuál es el go way de escribir esto?

err := viper.ReadInConfig()

if err \== nil && debug {

slog.Debug("Setting config file.", "ConfigFileUsed", viper.ConfigFileUsed())

}

if err \!= nil && (ok=err.(viper.ConfigFileNotFoundError); ok) {

handleAC()

}

me gustaría crear una batería de pruebas para los flags. No necesito probar cobra, por lo que me basta con ejecutar los handle. ¿Lo correcto es crear un archivo \*\_test.go por paquete o por archivo?

En el caso del auto-config tengo que verificar que crea el archivo de configuración con la configuración esperada ¿Cómo se puede hacer eso sin volverse loco inyectando objetos?

un par de cuestiones:

1\. ¿Cómo puedo lanzar los tests?

2\. ¿Cuales son las convenciones que tengo que tener en cuenta además de los nombres \_test?

3\. El ide tiene la opción de Run (Mayus+F10) o de Debug (Mayus+F9), pero en ambos casos hace un go build carpeta\_cli. ¿Cómo puedo decirle que para debug use el flag \-d al ejecutar el cli?

Pregunta¿En qué orden ae ejecutan los metodos de los distintos archivos?

Entonces cmd. Execute() nunca se llega a ejecutar realmente¿Correcto?

Sí, eso lo entiendo pero me refiero al código de Execute en root.go. ese código se ve eclipsado por el manejador run de cobra ¿No?

Ahí t ngo un slog.Debug("Executing yups") que no se imprime nunca.

Vale entiendo lo que me dices y cambiando el mensaje a info sí que sale, aunque sale en rojo lo que es un poco agresivo para un mensaje de información. Vamos a dar un repaso a lo que se puede hacer o no con slog.

1\. Ahora saca todo por stderr ¿se puede hacer que solo saque los errores por stderr y el resto de mensajes los saque por stdout?

2\. ¿Se puede añadir una salida a un archivo de log? para que se guarde todo lo que va saliendo por pantalla con su fecha y su hora.

3\. En los mensjaes que se muestran al usuario en la consola interactiva (stdout y stderr) no es necesario mostrar la fecha y la hora ¿se puede quitar?

4\. Los mensajes de tipo INFO no necesitan mostrar su tipo antes del mensaje.

5\. ¿Se puede establecer el color por tipo de mensaje para la salida interactiva (en el archivo no tiene sentido)? Por ejemplo los INFO podrían ser de color blanco estándar, los warning amarillos, los errores rojos, los debug verdes. Eso facilitaría la lectura de la salida ya que no tenemos emojis.

dale

vale, te sigo aunque tarde eh, que me gusta leer todo el código y entenderlo. Está guay el handler aunque no muestra los argumentos key, val que siguen al mensaje. ¿Podemos corregir eso?

Vale, ahora tengo que sacar la parte útil del install.sh y meterla en el auto-config (obviando todo lo de python). Además hay que aprovechar para cambiar lo que se mete en el bashrc ya que no exporta las fuciones (creo que dijiste que había que ponerles \-f). Además, habría que tener en cuenta las mejoras que he hecho en el bashrc: en LAST\_CMD se guarda el comando que dió error si la entrada es comando1 && comando\_error && comando2 cuando se invoca el handler se tiene en LAST\_CMD comando\_error lo cual nos facilitará el trabajo de análisis en yups, aunque probablemente habría que hacer alguna mejora como llamar a la variable YUPS\_XXXX o revisar el save\_last\_command para que sea a prueba de fallos. No he hecho pruebas de si esta funcionalidad la podríamos aprovechar también en el command not found handler ¿tú qué dices? Otra cosa que no estamos teniendo en cuenta es el exit code 130 (CTRL+C) que en ese caso el usuario no va a querer que le propongamos nada, ya que el comando acaba porque él quiere.

El TODO lo dejo para el futuro, de momento con que funcionemos para bash estoy más que contento ¿no te parece?

javi@Arthur:\~/yups$ cat \~/.bashrc

\# .bashrc

\# Source global definitions

if \[ \-f /etc/bashrc \]; then

    . /etc/bashrc

fi

\# User specific environment

if \! \[\[ "$PATH" \=\~ "$HOME/.local/bin:$HOME/bin:" \]\]; then

    PATH="$HOME/.local/bin:$HOME/bin:$PATH"

fi

export PATH

\# Uncomment the following line if you don't like systemctl's auto-paging feature:

\# export SYSTEMD\_PAGER=

\# User specific aliases and functions

if \[ \-d \~/.bashrc.d \]; then

    for rc in \~/.bashrc.d/\*; do

        if \[ \-f "$rc" \]; then

            . "$rc"

        fi

    done

fi

unset rc

\# \--- YUPS\_HOOK\_START \---

\# Hooks for the YUPS project

\# 1: Command Not Found Hook

command\_not\_found\_handle() {

    if "/usr/local/bin/yups" \--cnf-handle "$@"; then

        return $?

    else

        return 127

    fi

}

\# 2: Command Error Hook

\_yups\_ce\_handle() {

    local exit\_code=$?

    echo "Last command: $LAST\_CMD"

    if \[\[ $exit\_code \-eq 0 \]\] || \[\[ $exit\_code \-eq 127 \]\]; then

        return

    fi

    local last\_command\_text=$(history 1 | sed 's/^\[ \]\*\[0-9\]\\+\[ \]\\+//')

    "/usr/local/bin/yups" \--ce-handle "$exit\_code" "$last\_command\_text"

}

if \[\[ \-z "$PROMPT\_COMMAND" \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle"

elif \! \[\[ "$PROMPT\_COMMAND" \== \*"\_yups\_ce\_handle"\* \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle;${PROMPT\_COMMAND}"

fi

save\_last\_command() {

    \# Evitar recursión y captura de comandos internos del prompt

    if \[ "$BASH\_COMMAND" \!= "\_yups\_ce\_handle" \]; then

    LAST\_CMD="$BASH\_COMMAND"

    fi

}

trap 'save\_last\_command' DEBUG

\# \--- YUPS\_HOOK\_END \---

javi@Arthur:\~/yups$ cat cli/cmd/auto-config.go

package cmd

import (

"log/slog"

)

var acMode bool

func init() {

rootCmd.Flags().BoolVar(\&acMode, "auto-config",

false, "Set configuration to default values.")

}

func handleAC() {

slog.Info("Straw-boss (AC Mode).")

removeConfigFile()

createConfigFile()

cleanBashrc()

updateBashrc()

installProvidesHelper()

//TODO manage other shell different of bash

}

javi@Arthur:\~/yups$ cat cli/cmd/auto-config\_test.go 

package cmd

import (

"os"

"path/filepath"

"testing"

"github.com/spf13/viper"

"github.com/stretchr/testify/assert"

)

func TestHandleAC(t \*testing.T) {

tmpDir := t.TempDir()

configPath := filepath.Join(tmpDir, "config.toml")

viper.SetConfigFile(configPath)

handleAC()

\_, err := os.Stat(configPath)

if os.IsNotExist(err) {

t.Fatalf("File not found at %s", configPath)

}

err \= viper.ReadInConfig()

assert.NoError(t, err)

assert.Equal(t, "info", viper.GetString("log\_level"))

assert.Equal(t, "Linux", viper.GetString("os"))

assert.NotNil(t, viper.Get("pm"))

assert.NotNil(t, viper.Get("distro\_id"))

assert.NotNil(t, viper.Get("distro\_version"))

assert.NotNil(t, viper.Get("distro\_pretty"))

}

javi@Arthur:\~/yups$ cat install.sh uninstall.sh

\#\!/bin/bash

\# YUPS Installation Script (v6 \- Lean & Mean)

\# Changes:

\# \- Removed huggingface\_hub dependency (we use the API now).

\# \- Added auto-installation of 'apt-file' and 'pkgfile' for native 'provides' support.

\# \- Forces update of file databases (apt-file update / pkgfile \-u).

\# \--- Configuration \---

INSTALL\_PATH="/usr/local/bin/yups"

VENV\_PATH="/opt/yups/venv"

SOURCE\_FILE="yups"

BASHRC\_FILE=\~/.bashrc

\# Detect sudo requirement

SUDO="sudo"

if \[ "$EUID" \-eq 0 \]; then

  echo "WARNING: Do not run this script as root."

  echo "Run it as your normal user. It will ask for 'sudo' when needed."

  read \-p "Do you want to continue? (Y/n): " choice

  if \[\[ "$choice" \== "n" || "$choice" \== "N" \]\]; then

      exit 1

  fi

  SUDO=""

fi

\# \--- Bash Hooks Text Block \---

read \-r \-d '' BASH\_HOOKS \<\<'EOF'

\# \--- YUPS\_HOOK\_START \---

\# Hooks for the YUPS project

\# 1: Command Not Found Hook

command\_not\_found\_handle() {

    if "/usr/local/bin/yups" \--cnf-handle "$@"; then

        return $?

    else

        return 127

    fi

}

\# 2: Command Error Hook

\_yups\_ce\_handle() {

    local exit\_code=$?

    if \[\[ $exit\_code \-eq 0 \]\] || \[\[ $exit\_code \-eq 127 \]\] || \[\[ $exit\_code \-eq 130\]\]; then

        return

    fi

    local last\_command\_text=$(history 1 | sed 's/^\[ \]\*\[0-9\]\\+\[ \]\\+//')

    "/usr/local/bin/yups" \--ce-handle "$exit\_code" "$last\_command\_text"

}

if \[\[ \-z "$PROMPT\_COMMAND" \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle"

elif \! \[\[ "$PROMPT\_COMMAND" \== \*"\_yups\_ce\_handle"\* \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle;${PROMPT\_COMMAND}"

fi

\# \--- YUPS\_HOOK\_END \---

EOF

\# \--- Helper Functions \---

get\_python\_version\_str() {

    $1 \-c 'import sys; print(f"{sys.version\_info.major}.{sys.version\_info.minor}")' 2\>/dev/null

}

check\_python\_meets\_requirements() {

    \# YUPS v10 only needs requests, so 3.7+ is fine, even 3.6 might work but let's stick to 3.7 standard

    $1 \-c 'import sys; sys.exit(0 if sys.version\_info \>= (3, 7\) else 1)' 2\>/dev/null

}

\# \--- 1\. Pre-flight Checks \---

if \[ \! \-f "$SOURCE\_FILE" \]; then

    echo "ERROR: '$SOURCE\_FILE' script not found in this directory."

    exit 1

fi

echo "🚀 Starting YUPS Installation..."

\# \--- 2\. Environment Validation \---

echo "🐍 Validating Python Environment..."

CHOSEN\_PYTHON=""

CANDIDATES=("python3" "python3.13" "python3.12" "python3.11" "python3.10" "python3.9" "python3.8")

for candidate in "${CANDIDATES\[@\]}"; do

    if command \-v $candidate &\> /dev/null; then

        ver=$(get\_python\_version\_str $candidate)

        if check\_python\_meets\_requirements "$candidate"; then

            echo "   \-\> Found '$candidate' (v$ver) \- OK."

            CHOSEN\_PYTHON=$(command \-v $candidate)

            break

        fi

    fi

done

if \[\[ \-z "$CHOSEN\_PYTHON" \]\]; then

    echo "❌ ERROR: YUPS requires Python 3.7 or newer."

    \# Try to help Rocky Linux users

    if grep \-qi "rocky\\|rhel\\|centos" /etc/os-release; then

         echo "   \-\> Detected RHEL-based system. Trying to install python39..."

         if $SUDO dnf install \-y python39; then

             CHOSEN\_PYTHON=$(command \-v python3.9)

         fi

    fi

    

    if \[\[ \-z "$CHOSEN\_PYTHON" \]\]; then

        echo "   Please install a newer python manually."

        exit 1

    fi

fi

echo "✅ Selected Interpreter: $CHOSEN\_PYTHON"

\# \--- 3\. Install Helper Tools (apt-file / pkgfile) \---

echo "🛠️  Checking for 'provides' helper tools..."

if command \-v apt-get &\> /dev/null; then

    if \! command \-v apt-file &\> /dev/null; then

        echo "   \-\> installing 'apt-file' for advanced search..."

        $SUDO apt-get update && $SUDO apt-get install \-y apt-file

        echo "   \-\> updating apt-file cache (this may take a moment)..."

        $SUDO apt-file update

    else

        echo "   \-\> 'apt-file' is already installed."

    fi

elif command \-v pacman &\> /dev/null; then

    if \! command \-v pkgfile &\> /dev/null; then

        echo "   \-\> installing 'pkgfile' for advanced search..."

        $SUDO pacman \-S \--noconfirm pkgfile

        echo "   \-\> updating pkgfile database..."

        $SUDO pkgfile \--update

    else

        echo "   \-\> 'pkgfile' is already installed."

    fi

fi

\# \--- 4\. Create Isolated Environment \---

echo "📦 Setting up isolated Python environment in $VENV\_PATH..."

\# Security check for venv python version match

if \[ \-d "$VENV\_PATH" \] && \[ \-f "$VENV\_PATH/bin/python3" \]; then

    VENV\_VER=$("$VENV\_PATH/bin/python3" \-c 'import sys; print(f"{sys.version\_info.major}.{sys.version\_info.minor}")' 2\>/dev/null)

    CHOSEN\_VER=$($CHOSEN\_PYTHON \-c 'import sys; print(f"{sys.version\_info.major}.{sys.version\_info.minor}")')

    if \[\[ "$VENV\_VER" \!= "$CHOSEN\_VER" \]\]; then

        echo "   \-\> Version mismatch. Recreating venv..."

        $SUDO rm \-rf "$VENV\_PATH"

    fi

fi

$SUDO mkdir \-p "$(dirname "$VENV\_PATH")"

\# Ubuntu/Debian specific fix

if grep \-qi "ubuntu\\|debian" /etc/os-release; then

    if \! dpkg \-s python3-venv &\> /dev/null; then

        echo "   \-\> Installing python3-venv package..."

        $SUDO apt-get update && $SUDO apt-get install \-y python3-venv

    fi

fi

if \[ \! \-d "$VENV\_PATH" \]; then

    $SUDO "$CHOSEN\_PYTHON" \-m venv "$VENV\_PATH"

fi

\# \--- 5\. Install Dependencies \---

echo "📚 Installing dependencies (requests)..."

\# We ONLY need requests now, huggingface\_hub is gone\!

if \! $SUDO "$VENV\_PATH/bin/pip" install \--upgrade pip requests \> /dev/null; then

    echo "❌ ERROR: Failed to install Python dependencies."

    exit 1

fi

\# \--- 6\. Install Executable & Rewrite Shebang \---

echo "🔧 Installing executable to $INSTALL\_PATH..."

$SUDO cp "$SOURCE\_FILE" "$INSTALL\_PATH"

echo "   \-\> Linking executable to isolated environment..."

TMP\_FILE=$(mktemp)

sed "1s|.\*|\#\!$VENV\_PATH/bin/python3|" "$SOURCE\_FILE" \> "$TMP\_FILE"

$SUDO cp "$TMP\_FILE" "$INSTALL\_PATH"

rm "$TMP\_FILE"

$SUDO chmod \+x "$INSTALL\_PATH"

\# \--- 7\. User Configuration (Bashrc) \---

echo "🎣 Injecting hooks into $BASHRC\_FILE..."

if grep \-q "\# \--- YUPS\_HOOK\_START \---" "$BASHRC\_FILE"; then

    echo "   \-\> Hooks block already detected. Skipping."

else

    echo \-e "\\n$BASH\_HOOKS\\n" \>\> "$BASHRC\_FILE"

    echo "✓ Bash hooks installed."

fi

\# \--- 8\. HF Token Check (No longer needed strictly, but kept for legacy or custom use) \---

\# We skip the mandatory check since the server handles auth now.

\# \--- 9\. Initialize \---

echo "⚙️  Initializing YUPS config..."

"$INSTALL\_PATH" \--auto-config

echo \-e "\\n✅ YUPS installation complete\!"

echo "Please run: source $BASHRC\_FILE"

\#\!/bin/bash

\# YUPS Uninstallation Script

\# \--- Configuration \---

INSTALL\_PATH="/usr/local/bin/yups"

BASHRC\_FILE=\~/.bashrc

CONFIG\_DIR=\~/.yups

SUDO=sudo

\# \--- 1\. Root Check \---

if \[ "$EUID" \-eq 0 \]; then

  echo "WARNING: Do not run this script as root."

  echo "Run it as your normal user. It will ask for 'sudo' when needed."

  read \-p "Do you want to continue? (Y/n): " choice

  if \[\[ "$choice" \== "n" || "$choice" \== "N" \]\]; then

      exit 1

  fi

  SUDO=""

fi

echo "Uninstalling YUPS..."

\# \--- 2\. Remove Executable \---

echo "Removing executable ($INSTALL\_PATH)..."

if \[ \-f "$INSTALL\_PATH" \]; then

    if \! $SUDO rm \-f "$INSTALL\_PATH"; then

        echo "ERROR: Could not remove executable. Did 'sudo' fail?"

        \# Don't stop, still try to clean up bashrc

    else

        echo "✓ Executable removed."

    fi

else

    echo "✓ Executable not found (skipped)."

fi

\# \--- 3\. Remove Hooks from .bashrc \---

echo "Cleaning $BASHRC\_FILE..."

if grep \-q "\# \--- YUPS\_HOOK\_START \---" "$BASHRC\_FILE"; then

    \# Use sed to delete the block between our markers

    sed \-i '/\# \--- YUPS\_HOOK\_START \---/,/\# \--- YUPS\_HOOK\_END \---/d' "$BASHRC\_FILE"

    echo "✓ Bash hooks removed."

else

    echo "✓ No hooks found in $BASHRC\_FILE (skipped)."

fi

\# \--- 4\. Remove Config Directory (Optional) \---

if \[ \-d "$CONFIG\_DIR" \]; then

    echo "The configuration directory and logs are still at $CONFIG\_DIR"

    read \-p "Do you want to remove them? (y/N): " choice

    if \[\[ "$choice" \== "y" || "$choice" \== "Y" \]\]; then

        rm \-rf "$CONFIG\_DIR"

        echo "✓ Configuration directory removed."

    fi

fi

\# \--- 5\. Finish \---

echo \-e "\\nYUPS uninstallation complete\!"

echo "Please restart your terminal or run:"

echo "  source $BASHRC\_FILE"

javi@Arthur:\~/yups$ 

Lo que se hace en el yups actual para auto config es:

def detect\_os\_details():

    os\_name \= platform.system()

    distro\_info \= {

        "id": "unknown",

        "version\_id": "unknown",

        "pretty\_name": f"{os\_name} (Unknown Distro)"

    }

    if os\_name \== "Linux" and os.path.exists("/etc/os-release"):

        try:

            with open("/etc/os-release", 'r', encoding='utf-8') as f:

                for line in f:

                    line \= line.strip()

                    if '=' in line:

                        key, value \= line.split('=', 1\)

                        value \= value.strip('"')

                        if key \== "ID":

                            distro\_info\["id"\] \= value

                        elif key \== "VERSION\_ID":

                            distro\_info\["version\_id"\] \= value

                        elif key \== "PRETTY\_NAME":

                            distro\_info\["pretty\_name"\] \= value

        except Exception:

            pass

    return os\_name, distro\_info

def detect\_pm():

    for pm\_name in KNOWN\_PMS:

        if shutil.which(pm\_name):

            if pm\_name \== "apt-get":

                return "apt"

            if pm\_name \== "yum":

                if shutil.which("dnf"):

                    return "dnf"

                return "yum"

            return pm\_name

    return "unknown"

def run\_auto\_config():

    os.makedirs(YUPS\_DIR, exist\_ok=True)

    os\_name, distro\_info \= detect\_os\_details()

    pm\_name \= detect\_pm()

    config \= {

        "os": os\_name,

        "pm": pm\_name,

        "distro\_id": distro\_info.get("id"),

        "distro\_version": distro\_info.get("version\_id"),

        "distro\_pretty": distro\_info.get("pretty\_name"),

    }

    try:

        with open(CONFIG\_FILE, 'w', encoding='utf-8') as f:

            json.dump(config, f, indent=2)

        return config

    except Exception as e:

        print(f"YUPS\_ERROR: Could not write config file: {e}", file=sys.stderr)

        return config

¿Puedes coger toda esta información y montar el auto-config.go?

Nos quedan dos acciones que hacer:

installProvidesHelper()

copyExecutableToPath()

La primera instala apt-file si apt y no apt-file, o instala pkgfile si pacman y no pkgfile.

La seguda se asegura de que el ejecutable actual esté en el path y si no es así lo copia. De este modo, aunque esté en \~/Descargas/ejecutable\_descargado pasará a estar accesible.

Estoy intentando que el runSudo pida la contraseña (sea interactivo) pero me he atascado:

func runSudoCommand(name string, args ...string) error {

allArgs := append(\[\]string{"-c"}, append(\[\]string{"sudo"}, append(\[\]string{name}, args...)...)...)

cmd := exec.Command("/bin/sh", allArgs...)

cmd.Stdout \= os.Stdout

cmd.Stderr \= os.Stderr

return cmd.Run()

}

2025/12/20 17:53:57 INFO Straw-boss (AC Mode).

2025/12/20 17:53:57 INFO .bashrc hooks updated successfully

2025/12/20 17:53:57 INFO Ensuring yups is in /usr/local/bin... from=/home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_build\_github\_com\_tu\_usuario\_yups\_cli

usage: sudo \-h | \-K | \-k | \-V

usage: sudo \-v \[-ABkNnS\] \[-g group\] \[-h host\] \[-p prompt\] \[-u user\]

usage: sudo \-l \[-ABkNnS\] \[-g group\] \[-h host\] \[-p prompt\] \[-U user\]

            \[-u user\] \[command \[arg ...\]\]

usage: sudo \[-ABbEHkNnPS\] \[-r role\] \[-t type\] \[-C num\] \[-D directory\]

            \[-g group\] \[-h host\] \[-p prompt\] \[-R directory\] \[-T timeout\]

            \[-u user\] \[VAR=value\] \[-i | \-s\] \[command \[arg ...\]\]

usage: sudo \-e \[-ABkNnS\] \[-r role\] \[-t type\] \[-C num\] \[-D directory\]

            \[-g group\] \[-h host\] \[-p prompt\] \[-R directory\] \[-T timeout\]

            \[-u user\] file ...

2025/12/20 17:53:57 ERROR Failed to copy executable to path error="exit status 1"

ERROR: Error setting file log Error=open /home/javi/.yups/log: no such file or directory

Process finished with the exit code 0

Vale, eso funciona pero y no es pequeño el pero, como es lógico solo funciona cuando lo lanzo directamente en la terminal...

javi@Arthur:\~/yups/cli$ go run main.go \--auto-config

Straw-boss (AC Mode).

.bashrc hooks updated successfully                                                                                 

Ensuring yups is in /usr/local/bin... from=/home/javi/.cache/go-build/b3/b393ae68cc834c0fcb498c0dff9300a105e4502ff0624ed52a47cbb6290b4b91-d/main                                                                                      

\[sudo\] contraseña para javi:                                                                                       

javi@Arthur:\~/yups/cli$ go run main.go \--auto-remove

javi@Arthur:\~/yups/cli$ go test cmd/

ok      cmd/addr2line   1.202s

ok      cmd/api 7.164s

?       cmd/asm \[no test files\]

?       cmd/asm/internal/arch   \[no test files\]

ok      cmd/asm/internal/asm    0.888s

?       cmd/asm/internal/flags  \[n....

Si lo ejecuto desde el IDE (Mayus+F10) me dice que necesito una terminal o dar la contraseña aparte:

2025/12/20 18:04:52 INFO Straw-boss (AC Mode).

2025/12/20 18:04:52 INFO .bashrc hooks updated successfully

2025/12/20 18:04:52 INFO Ensuring yups is in /usr/local/bin... from=/home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_build\_github\_com\_tu\_usuario\_yups\_cli

sudo: a terminal is required to read the password; either use the \-S option to read from standard input or configure an askpass helper

sudo: a password is required

2025/12/20 18:04:54 ERROR Failed to copy executable to path error="exit status 1"

ERROR: Error setting file log Error=open /home/javi/.yups/log: no such file or directory

The YUPS CLI handles your command not found and other....

Y se ejecuto los tests, también me dice que me peine, como es normal:

\=== RUN   TestHandleAC

2025/12/20 18:07:28 INFO Straw-boss (AC Mode).

2025/12/20 18:07:28 INFO .bashrc hooks updated successfully

2025/12/20 18:07:28 INFO Ensuring yups is in /usr/local/bin... from=/home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_test\_github\_com\_tu\_usuario\_yups\_cli\_cmd.test

sudo: a terminal is required to read the password; either use the \-S option to read from standard input or configure an askpass helper

sudo: a password is required

2025/12/20 18:07:30 ERROR Failed to copy executable to path error="exit status 1"

\--- PASS: TestHandleAC (2.62s)

\=== RUN   TestHandleAR

2025/12/20 18:07:30 INFO Straw-boss (AC Mode).

2025/12/20 18:07:30 INFO .bashrc hooks updated successfully

2025/12/20 18:07:30 INFO Ensuring yups is in /usr/local/bin... from=/home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_test\_github\_com\_tu\_usuario\_yups\_cli\_cmd.test

sudo: a terminal is required to read the password; either use the \-S option to read from standard input or configure an askpass helper

sudo: a password is required

2025/12/20 18:07:32 ERROR Failed to copy executable to path error="exit status 1"

    auto-config\_test.go:39: File not found at /usr/local/bin/yups

\--- FAIL: TestHandleAR (1.94s)

FAIL

Process finished with the exit code 1

Así que hay que darle una vuelta al código para organizarlo de tal modo que pueda testear las partes que no requieren sudo y dar las que sí lo requieren por probadas. Quizá se podría detectar si es una sesión interactiva en cuyo caso hace lo suyo y si no lo es y no está corriendo con privilegios y los necesita, dar un error, aunque eso complicaría un poco el código ¿o no?

Last command: clear

javi@Arthur:\~/yups/cli/cmd$ tail \-n \+1 auto\*

\==\> auto-config.go \<==

package cmd

import (

"fmt"

"log/slog"

"os"

"os/exec"

"path/filepath"

"strings"

"github.com/spf13/viper"

"github.com/tu-usuario/yups/cli/internal/sys"

)

var acMode bool

var arMode bool

const (

hookStart \= "\# \--- YUPS\_HOOK\_START \---"

hookEnd   \= "\# \--- YUPS\_HOOK\_END \---"

yupsPath  \= "/usr/local/bin/yups"

)

func init() {

rootCmd.Flags().BoolVar(\&acMode, "auto-config",

false, "Set configuration to default values.")

rootCmd.Flags().BoolVar(\&arMode, "auto-remove",

false, "Remove configuration and binaries.")

}

func handleAR() {

home, \_ := os.UserHomeDir()

os.RemoveAll(filepath.Join(home, ".yups"))

updateBashrc(false)

runSudoCommand("rm", yupsPath)

}

func handleAC() {

slog.Info("Straw-boss (AC Mode).")

info := sys.GetSystemInfo()

viper.Set("os", info.OS)

viper.Set("pm", info.PM)

viper.Set("distro\_id", info.DistroID)

viper.Set("distro\_version", info.DistroVersion)

viper.Set("distro\_pretty", info.DistroPretty)

viper.Set("log\_level", "info")

if err := viper.WriteConfig(); err \!= nil {

os.MkdirAll(filepath.Dir(viper.ConfigFileUsed()), 0755\)

viper.SafeWriteConfig()

}

if err := updateBashrc(true); err \!= nil {

slog.Error("Failed to update .bashrc", "error", err)

} else {

slog.Info(".bashrc hooks updated successfully")

}

installProvidesHelper()

copyExecutableToPath()

//TODO manage other shell different of bash

}

func updateBashrc(insert bool) error {

home, \_ := os.UserHomeDir()

bashrcPath := filepath.Join(home, ".bashrc")

content, err := os.ReadFile(bashrcPath)

if err \!= nil {

return err

}

lines := strings.Split(string(content), "\\n")

var newLines \[\]string

skipping := false

for \_, line := range lines {

if strings.Contains(line, hookStart) {

skipping \= true

continue

}

if strings.Contains(line, hookEnd) {

skipping \= false

continue

}

if \!skipping {

newLines \= append(newLines, line)

}

}

bashHooks := fmt.Sprintf(\`

%s

\# Hooks for the YUPS project

command\_not\_found\_handle() {

    if "%s" \--command-not-found "$@"; then

        return $?

    else

        return 127

    fi

}

export \-f command\_not\_found\_handle

\_yups\_ce\_handle() {

    local exit\_code=$?

    \# 130 is Ctrl+C, 127 is CNF (handled above), 0 is success

    if \[\[ $exit\_code \-eq 0 \]\] || \[\[ $exit\_code \-eq 127 \]\] || \[\[ $exit\_code \-eq 130 \]\]; then

        return

    fi

    "%s" \--command-error "$exit\_code" "$YUPS\_LAST\_CMD"

}

export \-f \_yups\_ce\_handle

if \[\[ \-z "$PROMPT\_COMMAND" \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle"

elif \! \[\[ "$PROMPT\_COMMAND" \== \*"\_yups\_ce\_handle"\* \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle;${PROMPT\_COMMAND}"

fi

\_yups\_save\_last\_cmd() {

    if \[\[ "$BASH\_COMMAND" \!= "\_yups\_ce\_handle" \]\]; then

        export YUPS\_LAST\_CMD="$BASH\_COMMAND"

    fi

}

trap '\_yups\_save\_last\_cmd' DEBUG

%s\`, hookStart, yupsPath, yupsPath, hookEnd)

var finalContent string

if insert {

finalContent \= strings.TrimSpace(strings.Join(newLines, "\\n")) \+ "\\n" \+ bashHooks \+ "\\n"

} else {

finalContent \= strings.TrimSpace(strings.Join(newLines, "\\n"))

}

return os.WriteFile(bashrcPath, \[\]byte(finalContent), 0644\)

}

func installProvidesHelper() {

info := sys.GetSystemInfo()

switch info.PM {

case "apt":

if \_, err := exec.LookPath("apt-file"); err \!= nil {

slog.Info("Installing apt-file for advanced search...")

runSudoCommand("apt-get", "update")

runSudoCommand("apt-get", "install", "-y", "apt-file")

runSudoCommand("apt-file", "update")

}

case "pacman":

if \_, err := exec.LookPath("pkgfile"); err \!= nil {

slog.Info("Installing pkgfile for advanced search...")

runSudoCommand("pacman", "-S", "--noconfirm", "pkgfile")

runSudoCommand("pkgfile", "--update")

}

}

}

func runSudoCommand(name string, args ...string) error {

allArgs := append(\[\]string{name}, args...)

cmd := exec.Command("sudo", allArgs...)

cmd.Stdin \= os.Stdin

cmd.Stdout \= os.Stdout

cmd.Stderr \= os.Stderr

return cmd.Run()

}

func copyExecutableToPath() {

targetPath := yupsPath

currentPath, err := os.Executable()

if err \!= nil {

slog.Error("Could not determine current executable path", "error", err)

return

}

if currentPath \== targetPath {

return

}

slog.Info("Ensuring yups is in /usr/local/bin...", "from", currentPath)

if err := runSudoCommand("cp", currentPath, targetPath); err \!= nil {

slog.Error("Failed to copy executable to path", "error", err)

return

}

runSudoCommand("chmod", "+x", targetPath)

}

\==\> auto-config\_test.go \<==

package cmd

import (

"os"

"path/filepath"

"testing"

"github.com/spf13/viper"

"github.com/stretchr/testify/assert"

)

func TestHandleAC(t \*testing.T) {

tmpDir := t.TempDir()

configPath := filepath.Join(tmpDir, "config.toml")

viper.SetConfigFile(configPath)

handleAC()

\_, err := os.Stat(configPath)

if os.IsNotExist(err) {

t.Fatalf("File not found at %s", configPath)

}

err \= viper.ReadInConfig()

assert.NoError(t, err)

assert.Equal(t, "info", viper.GetString("log\_level"))

assert.Equal(t, "linux", viper.GetString("os"))

assert.NotNil(t, viper.Get("pm"))

assert.NotNil(t, viper.Get("distro\_id"))

assert.NotNil(t, viper.Get("distro\_version"))

assert.NotNil(t, viper.Get("distro\_pretty"))

}

func TestHandleAR(t \*testing.T) {

handleAC()

\_, err := os.Stat(yupsPath)

if os.IsNotExist(err) {

t.Fatalf("File not found at %s", yupsPath)

}

home, \_ := os.UserHomeDir()

folder := filepath.Join(home, ".yups")

\_, err \= os.Stat(folder)

if os.IsNotExist(err) {

t.Fatalf("File not found at %s", yupsPath)

}

handleAR()

\_, err \= os.Stat(yupsPath)

assert.True(t, os.IsNotExist(err))

\_, err \= os.Stat(folder)

assert.True(t, os.IsNotExist(err))

}

Last command: tail \-n \+1 auto\*

javi@Arthur:\~/yups/cli/cmd$ 

Entiendo que el testMain se ejecuta al arrancar los tests? es un nombre predefinido? ¿Hay otros?

ponme un ejemplo de test example

una ultima pregunta. Ahora tengo unos tests en cmd y otros en sys ¿Hay algún modo de correr todos los tests de todos los paquetes míos (no cómo he hecho antes que ejecuté go test cmd/ y se estaba ejecutando test de todos los paquetes)

y a través de goland? hay alguna combinación de teclas o algo para ejecutar todos los test?

Hazme un favor y convierte esto en una constante diccionario de go:

PM\_COMMANDS \= {

"install": {

"help": "Install one or more packages.",

"takes\_packages": True,

"commands": {

"apt": "sudo apt install {packages}",

"dnf": "sudo dnf install {packages}",

"pacman": "sudo pacman \-S {packages}",

"zypper": "sudo zypper install {packages}",

}

},

"remove": {

"help": "Remove one or more packages.",

"takes\_packages": True,

"commands": {

"apt": "sudo apt remove {packages}",

"dnf": "sudo dnf remove {packages}",

"pacman": "sudo pacman \-R {packages}",

"zypper": "sudo zypper remove {packages}",

}

},

"search": {

"help": "Search for available packages.",

"takes\_packages": True,

"commands": {

"apt": "apt search {packages}",

"dnf": "dnf search \-C {packages}",

"pacman": "pacman \-Ss {packages}",

"zypper": "zypper \--no-refresh search {packages}",

}

},

"autoremove": {

"help": "Remove unused packages (cleanup).",

"takes\_packages": False,

"commands": {

"apt": "sudo apt autoremove",

"dnf": "sudo dnf autoremove",

"pacman": "sudo pacman \-Rns $(pacman \-Qdtq)",

"zypper": "sudo zypper remove \--clean-deps",

}

},

"upgrade": {

"help": "Upgrade all installed packages.",

"takes\_packages": False,

"commands": {

"apt": "sudo apt upgrade",

"dnf": "sudo dnf upgrade",

"pacman": "sudo pacman \-Syu",

"zypper": "sudo zypper dup",

}

},

"update": {

"help": "Refresh package repository information.",

"takes\_packages": False,

"commands": {

"apt": "sudo apt update",

"dnf": "sudo dnf check-update",

"pacman": "sudo pacman \-Sy",

"zypper": "sudo zypper refresh",

}

},

"provides": {

"help": "Find which package provides a file or command.",

"takes\_packages": True,

"commands": {

"apt": "apt-file search {packages}",

"dnf": "dnf provides \-C {packages}",

"pacman": "pacman \-F {packages}",

"zypper": "zypper \--no-refresh what-provides {packages}",

}

}

}

Hazme un ejemplo de archivo de test para testear el command not found, no sé si el go way marcaría hacerlos como TestXXX como ExampleXXX o ambos. A continuación te pongo el código pero pruebas que se pueden hacer serían:

yups \--command-not-found nano

yups \--command-not-found sudo nano

yups \--command-not-found nan

yups \--command-not-found nano && echo "ok"

y las que se te ocurran. Todas deberían tener como resultado algo en plan:

yups: Do you want to install packeage "nano-8.5-2.fc43.x86\_64" that provides the command "nano"? YES/no

package cmd

import (

"bytes"

"log/slog"

"os/exec"

"strings"

"github.com/spf13/viper"

"github.com/tu-usuario/yups/cli/internal/parser"

"github.com/tu-usuario/yups/cli/internal/sys"

)

var cnfMode bool

func init() {

rootCmd.Flags().BoolVar(\&cnfMode, "command-not-found",

false, "System hook for command not found.")

rootCmd.Flags().MarkHidden("command-not-found")

}

func handleCNF(args \[\]string) {

query := strings.Join(args, " ")

slog.Info("Straw-boss (CNF Mode) analyzing: ",

"query", query)

lastCommand := viper.GetString("YUPS\_LAST\_CMD")

commands, \_ := parser.ExtractCommands(lastCommand)

replacer := strings.NewReplacer(sys.PackagesString, commands\[0\])

provides := replacer.Replace(

sys.PMCommands\["provides"\].Commands\[viper.GetString("pm")\])

cmd := exec.Command(provides)

var outb, errb bytes.Buffer

cmd.Stdout \= \&outb

cmd.Stderr \= \&errb

err := cmd.Run()

if err \== nil {

//TODO parse output

slog.Debug("Provides output", "output", string(outb.Bytes()))

}

}

¿Podrías echar un vistazo a esta documentación o buscar cosas sobre gemma functions que ha sacado google la semana pasada?

https://ai.google.dev/gemma/docs/functiongemma/formatting-and-best-practices

Me gustataría probarlo para interpretar la salida de los comandos en la máquina del usuario, en lugar de fabricar yo parsers frágiles que dependan del tipo de package manager.

Por ejemplo para buscar el paquete que provides un comando, en dnf es la tercera línea antes del espacio en blanco, si es que la hay. En apt la primera hasta los :. En Pacman la primera también y en zypper la ultima entre el primer par de |.

Un modelo pequeñito como este podría ser la solución perfecta.

Con tu explicación y lo que veo en github en

https://github.com/ollama/ollama/blob/main/api/client.go

Puede que no haya entendido pero parece que el usuario necesitaría tener instalado ollama y el modelo bajado, lo que inutiliza nuestra ventaja de "todo en uno" para facilitar la distribución que nos da go. ¿Se puede encapsular de algún modo?

Sí, eso estaba pensando, que el momento adecuado para descargarlo (si no existe ya) es al hacer el \--auto-config porque lo estamos forzando siempre en la primera ejecución de yups (si no hay autoconfig no funciona nada), y así lo podemos dar siempre por existente.

Quizá lo has entendido por los comentarios en el código pero tengo previsto un proceso yups \--update que se ejecute desde crontab cada X tiempo. Se encargaría de actualizar la base de datos de comandos o de recrescar los repositorios para que luego se pueda ejecutar el apt-file search (o el comando que toque) sin esperar. Así que en ese mismo proceso puedo verificar si hay una nueva versión del modelo para bajar y usar siempre la ultima versión en \~/.yups/models

¿Cómo lo ves?¿Algún agujero?

Sí, prepara todo lo que esté en tu mano para que pueda incorporar esta funcionalidad cuando vuelva al ordenador. Usaremos la librería que dijiste que tiene el motor de inferencia todo en go, no me importa que sea más lento ya que el usuario ya estará sobre aviso de que se están haciendo cosas.

Lo de la barra de progreso es vital, debería de revisar todo el proceso que se da en auto-config.go para asegurar que se informa de los pasos que se están dando para mantener la experiencia lo más amigable posible aunque sea con un "step 2/7" .

Es un poco desagradable para el usuario que la primera vez que quiere usar algo tenga que esperar varios minutos, pero informando y poniendo animaciones podemos suavizar el proceso.

Si todavía tienes el código en tu contexto revisa a ver si podemos paralelizar estas tareas que son más lentas (como descargar el modelo o instalar apt-file).

Muchas gracias

No, 200 MB en los sistemas actuales es algo que podemos asumir que hay disponible. Si se llena el disco con eso, el menor problema que va a tener el usuario va a ser que no le funcione yups. Lo mismo con la ram.

Lo que sí puedes, si quieres, es encapsular toda la funcionalidad de llamar al modelo en un wrapper por tener todo aislado en caso de que en el futuro lo queramos cambiar por otra cosa.

De momento las funcionalidades que tenemos que exponer son:

\- Interpretar la salida de un provides para identificar el paquete a instalar

\- Interpretar el comando de pm de un usuario para identificar la acción (el subcomando) y en caso de que lleve argumento (los paquetes) extraerlos. Ej.: el usuario en open suse escribe "apt install nano vim gedit" debería devolver install como acción y nano vim gedit como argumento para que podamos montar el comando adecuado: sudo zypper install nano vim gedit. Si alguno de los paquetes no existe ya nos tocará tirar del hermano mayor que tenemos online para que traduzca los paquetes.

Con esas dos cositas tendremos el 80% de la funcionalidad implementada y si las pruebas van bien quizá podamos aprovechar el motor para algo más.

No, de momento probaré esto sin fallbacks y en función de como vayan las pruebas determinaremos los siguientes pasos

He visto que en el wrapper de la IA lo has dejado sin implementar, supongo que a la espera de tener una librería que funcione. A pesar de que parece que ollama está implementado en go, no encuentro más librería que esta que lleva sin actualizarse desde que se creó

https://github.com/prathyushnallamothu/ollamago

¿Podrías ayudarme a buscar? No sé si estoy buscando bien. Google parece que sólo ofrede documentación de este gemma functions en Python. He visto que también hay un modelo qwen pequeño bastante efectivo, pero supera el giga

Está perfecto así, muchas gracias

Una pega. He bajado el modelo desde kaggle y es un tar gz que de los archivos que tiene el que parece el modelo no es gguf es safetensors que parece ser un formato de archivo creado por hugging face. Por lo que veo go llama cpp https://github.com/go-skynet/go-llama.cpp solo soporta gguf ¿Hay alguna otra librería que valorar o debería buscar el modo de convertir el formato?

Con lo de descargar el modelo de inferencia es dificil de mokear toda la configuración, pero haz lo que puedas. También he separado todo en funciones para que no haga falta testear todo de golpe y si hay algo muy dificil de mokear lo podemos aislar por lo menos y probar todo lo demás.

javi@Arthur:\~/yups/cli/cmd$ tail \-n \+1 auto\*

\==\> auto-config.go \<==

package cmd

import (

"context"

"crypto/sha256"

"encoding/hex"

"fmt"

"io"

"log/slog"

"net/http"

"os"

"os/exec"

"path/filepath"

"strings"

"time"

"github.com/spf13/viper"

"github.com/tu-usuario/yups/cli/internal/sys"

"golang.org/x/sync/errgroup"

)

var acMode bool

var arMode bool

var yupsPath \= "/usr/local/bin/yups"

var modelUri \= "https://huggingface.co/bartowski/google\_functiongemma-270m-it-GGUF/resolve/main/google\_functiongemma-270m-it-Q8\_0.gguf"

var modelHash \= "f50fbac8552d090863d5fefa983d24ac1ca37df23b1c77e3bbbd80aeb3b208c4"

const (

hookStart \= "\# \--- YUPS\_HOOK\_START \---"

hookEnd   \= "\# \--- YUPS\_HOOK\_END \---"

)

func init() {

rootCmd.Flags().BoolVar(\&acMode, "auto-config",

false, "Set configuration to default values.")

rootCmd.Flags().BoolVar(\&arMode, "auto-remove",

false, "Remove configuration and binaries.")

}

func handleAR() {

home, \_ := os.UserHomeDir()

os.RemoveAll(filepath.Join(home, ".yups"))

updateBashrc(false)

sys.RunSudoCommand("rm", yupsPath)

}

func handleAC() {

slog.Info("Straw-boss (AC Mode).")

start := time.Now()

const steps \= 6

sys.Step(1, steps, "Getting system info")

info := sys.GetSystemInfo()

sys.Step(2, steps, "Saving config file")

saveConfigFile(info)

sys.Step(3, steps, "Setting bash integration")

if err := updateBashrc(true); err \!= nil {

slog.Error("Failed to update .bashrc",

"error", err)

slog.Warn("Yups will work with limited functionality.")

} else {

slog.Info(".bashrc hooks updated successfully")

}

g, ctx := errgroup.WithContext(context.Background())

g.Go(func() error {

sys.Step(4, steps, "Installing 'provides' helper")

installProvidesHelper()

return nil

})

g.Go(func() error {

sys.Step(5, steps, "Installing yups")

copyExecutableToPath()

return nil

})

g.Go(func() error {

sys.Step(6, steps, "Downloading model")

return downloadModel(ctx)

})

if err := g.Wait(); err \!= nil {

slog.Error("Error config yups", "internal", err)

os.Exit(1)

}

slog.Info("Yups configuration completed in ", time.Since(start).Round(time.Second))

}

func saveConfigFile(info sys.Info) {

viper.Set("os", info.OS)

viper.Set("pm", info.PM)

viper.Set("distro\_id", info.DistroID)

viper.Set("distro\_version", info.DistroVersion)

viper.Set("distro\_pretty", info.DistroPretty)

viper.Set("log\_level", "info")

if err := viper.WriteConfig(); err \!= nil {

os.MkdirAll(filepath.Dir(viper.ConfigFileUsed()), 0755\)

viper.SafeWriteConfig()

}

}

func updateBashrc(insert bool) error {

home, \_ := os.UserHomeDir()

bashrcPath := filepath.Join(home, ".bashrc")

content, err := os.ReadFile(bashrcPath)

if err \!= nil {

return err

}

lines := strings.Split(string(content), "\\n")

var newLines \[\]string

skipping := false

for \_, line := range lines {

if strings.Contains(line, hookStart) {

skipping \= true

continue

}

if strings.Contains(line, hookEnd) {

skipping \= false

continue

}

if \!skipping {

newLines \= append(newLines, line)

}

}

bashHooks := fmt.Sprintf(\`

%s

\# Hooks for the YUPS project

command\_not\_found\_handle() {

    if "%s" \--command-not-found "$@"; then

        return $?

    else

        return 127

    fi

}

export \-f command\_not\_found\_handle

\_yups\_ce\_handle() {

    local exit\_code=$?

    \# 130 is Ctrl+C, 127 is CNF (handled above), 0 is success

    if \[\[ $exit\_code \-eq 0 \]\] || \[\[ $exit\_code \-eq 127 \]\] || \[\[ $exit\_code \-eq 130 \]\]; then

        return

    fi

    "%s" \--command-error "$exit\_code" "$YUPS\_LAST\_CMD"

}

export \-f \_yups\_ce\_handle

if \[\[ \-z "$PROMPT\_COMMAND" \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle"

elif \! \[\[ "$PROMPT\_COMMAND" \== \*"\_yups\_ce\_handle"\* \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle;${PROMPT\_COMMAND}"

fi

\_yups\_save\_last\_cmd() {

    if \[\[ "$BASH\_COMMAND" \!= "\_yups\_ce\_handle" \]\]; then

        export YUPS\_LAST\_CMD="$BASH\_COMMAND"

    fi

}

trap '\_yups\_save\_last\_cmd' DEBUG

%s\`, hookStart, yupsPath, yupsPath, hookEnd)

var finalContent string

if insert {

finalContent \= strings.TrimSpace(strings.Join(newLines, "\\n")) \+ "\\n" \+ bashHooks \+ "\\n"

} else {

finalContent \= strings.TrimSpace(strings.Join(newLines, "\\n"))

}

return os.WriteFile(bashrcPath, \[\]byte(finalContent), 0644\)

}

func installProvidesHelper() {

info := sys.GetSystemInfo()

switch info.PM {

case "apt":

if \_, err := exec.LookPath("apt-file"); err \!= nil {

slog.Info("Installing apt-file for advanced search...")

sys.RunSudoCommand("apt-get", "update")

sys.RunSudoCommand("apt-get", "install", "-y", "apt-file")

sys.RunSudoCommand("apt-file", "update")

}

case "pacman":

if \_, err := exec.LookPath("pkgfile"); err \!= nil {

slog.Info("Installing pkgfile for advanced search...")

sys.RunSudoCommand("pacman", "-S", "--noconfirm", "pkgfile")

sys.RunSudoCommand("pkgfile", "--update")

}

}

}

func copyExecutableToPath() {

targetPath := yupsPath

currentPath, err := os.Executable()

if err \!= nil {

slog.Error("Could not determine current executable path", "error", err)

return

}

if currentPath \== targetPath {

return

}

slog.Info("Ensuring yups is in /usr/local/bin...", "from", currentPath)

if err := sys.RunSudoCommand("cp", currentPath, targetPath); err \!= nil {

slog.Error("Failed to copy executable to path", "error", err)

return

}

sys.RunSudoCommand("chmod", "+x", targetPath)

}

func downloadModel(ctx context.Context) error {

home, \_ := os.UserHomeDir()

path := filepath.Join(home, ".yups/models/gemma-3-270m.gguf")

if \_, err := os.Stat(path); err \== nil {

return nil

}

os.MkdirAll(filepath.Dir(path), 0755\)

resp, err := http.Get(modelUri)

if err \!= nil {

return err

}

defer resp.Body.Close()

out, \_ := os.Create(path \+ ".tmp")

defer out.Close()

counter := \&sys.ProgressWriter{Total: uint64(resp.ContentLength), Message: "Downloading model"}

\_, \_ \= io.Copy(out, io.TeeReader(resp.Body, counter))

fmt.Print("\\n")

os.Rename(path+".tmp", path)

if \!verifyChecksum(path, modelHash) {

return sys.YupsError{Message: "Checksum verification failed"}

}

return nil

}

func verifyChecksum(path, expected string) bool {

f, \_ := os.Open(path)

defer f.Close()

h := sha256.New()

\_, \_ \= io.Copy(h, f)

return hex.EncodeToString(h.Sum(nil)) \== expected

}

\==\> auto-config\_test.go \<==

package cmd

import (

"fmt"

"os"

"path/filepath"

"testing"

"github.com/spf13/viper"

"github.com/stretchr/testify/assert"

"github.com/tu-usuario/yups/cli/internal/sys"

)

func TestMain(m \*testing.M) {

originalRunner := sys.SudoRunner

sys.SudoRunner \= func(name string, args ...string) error {

fmt.Printf("Mock Sudo: %s %v\\n", name, args)

return nil

}

code := m.Run()

sys.SudoRunner \= originalRunner

os.Exit(code)

}

func TestHandleAC(t \*testing.T) {

tmpDir := t.TempDir()

configPath := filepath.Join(tmpDir, "config.toml")

viper.SetConfigFile(configPath)

handleAC()

\_, err := os.Stat(configPath)

if os.IsNotExist(err) {

t.Fatalf("File not found at %s", configPath)

}

err \= viper.ReadInConfig()

assert.NoError(t, err)

assert.Equal(t, "info", viper.GetString("log\_level"))

assert.Equal(t, "linux", viper.GetString("os"))

assert.NotNil(t, viper.Get("pm"))

assert.NotNil(t, viper.Get("distro\_id"))

assert.NotNil(t, viper.Get("distro\_version"))

assert.NotNil(t, viper.Get("distro\_pretty"))

}

func TestHandleAR(t \*testing.T) {

tmpDir := t.TempDir()

viper.SetConfigFile(filepath.Join(tmpDir, "config.toml"))

home, \_ := os.UserHomeDir()

yupsDir := filepath.Join(home, ".yups")

os.MkdirAll(yupsDir, 0755\)

handleAR()

\_, err := os.Stat(yupsDir)

assert.True(t, os.IsNotExist(err), "La carpeta .yups debería haber sido eliminada")

}

javi@Arthur:\~/yups/cli/cmd$ 

Si se ponen dos defer ¿se ejecutan los dos o es como un puntero a función que se sobreescribe con cada llamada?

out, \_ := os.Create(path \+ ".tmp")

defer out.Close()

defer os.Remove(path \+ ".tmp")

cuestión sobre paquetes ¿Cómo se debería importar desde el paquete cmd el paquete sys (o cualquier otro paquete)?

Actualmente pone

"github.com/tu-usuario/yups/cli/internal/sys"

y claro, mi usuario es JaviLopezG no "tu-usuario", pero aunque eso estuviera bien no tiene sentido importarlo de github ya que puedo querer usar código recién incorporado que aún no esté en github o al menos no en la rama main. ¿Cual es el modo correcto de hacerlo?

No consigo que me funcione, a lo mejor el error es por otra cosa

./root\_test.go:15:3: use of package sys not in selector

./root\_test.go:15:3: cannot assign to sys.PromptConfirmReplacement (neither addressable nor a map index expression)

Vamos avanzando, pero tengo este error que no entiendo porque es como si no estuviera establecido el configPath pero sí que existe y está establecido como se ve al preguntar a viper por el archivo de configuración que usa:

2025/12/26 15:24:10 ERROR Error writing config file file=/tmp/TestHandleAC263832864/001/.yups/config.toml error="missing configuration for 'configPath'"

No doy con ello. En ningún archivo tengo escrita la palabra "missing", te pego el log completo y el código a ver si tú ves algo que yo no estoy viendo:

GOROOT=/home/javi/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.4.linux-amd64 \#gosetup

GOPATH=/home/javi/go \#gosetup

/home/javi/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.4.linux-amd64/bin/go test \-c \-o /home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_test\_github\_com\_javilopezg\_yups\_cli\_cmd.test github.com/javilopezg/yups/cli/cmd \#gosetup

/home/javi/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.4.linux-amd64/bin/go tool test2json \-t /home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_test\_github\_com\_javilopezg\_yups\_cli\_cmd.test \-test.v=test2json \-test.paniconexit0 \#gosetup

\=== RUN   TestHandleAC

Viper config file: /tmp/TestHandleAC2156958270/001/.yups/config.toml

2025/12/26 15:38:53 INFO Straw-boss (AC Mode).

\[1/1\] Getting system info...

\[2/2\] Saving config file...

2025/12/26 15:38:53 ERROR Error writing config file file=/tmp/TestHandleAC2156958270/001/.yups/config.toml error="missing configuration for 'configPath'"

\[3/3\] Setting bash integration...

2025/12/26 15:38:53 INFO .bashrc hooks updated successfully

\[6/6\] Downloading model...

\[4/4\] Installing 'provides' helper...

\[5/5\] Installing yups...

2025/12/26 15:38:53 INFO Ensuring yups is in /usr/local/bin... from=/home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_test\_github\_com\_javilopezg\_yups\_cli\_cmd.test

\[Test Mock\] Sudo: cp \[/home/javi/.cache/JetBrains/GoLand2025.3/tmp/GoLand/\_\_\_go\_test\_github\_com\_javilopezg\_yups\_cli\_cmd.test /tmp/TestHandleAC2156958270/001/fake\_yups\_bin\]

\[Test Mock\] Sudo: chmod \[+x /tmp/TestHandleAC2156958270/001/fake\_yups\_bin\]

📥 Downloading model: \[==================================================\] 100.0%

2025/12/26 15:38:53 INFO Yups configuration completed in  time=0s

Viper config file: /tmp/TestHandleAC2156958270/001/.yups/config.toml

    auto-config\_test.go:78: 

        Error Trace: /home/javi/yups/cli/cmd/auto-config\_test.go:78

        Error:      unable to find file "/tmp/TestHandleAC2156958270/001/.yups/config.toml"

        Test:        TestHandleAC

\--- FAIL: TestHandleAC (0.00s)

\=== RUN   TestHandleAR

\[Test Mock\] Sudo: rm \[/usr/local/bin/yups\]

\--- PASS: TestHandleAR (0.00s)

\=== RUN   TestHandleCNF

\=== RUN   TestHandleCNF/Simple\_command

2025/12/26 15:38:53 INFO Straw-boss (CNF Mode) analyzing:  query=nano

\--- PASS: TestHandleCNF/Simple\_command (0.00s)

\=== RUN   TestHandleCNF/Command\_with\_sudo

2025/12/26 15:38:53 INFO Straw-boss (CNF Mode) analyzing:  query="sudo nano"

\--- PASS: TestHandleCNF/Command\_with\_sudo (0.00s)

\=== RUN   TestHandleCNF/Complex\_chain\_with\_&&

2025/12/26 15:38:53 INFO Straw-boss (CNF Mode) analyzing:  query="nano && echo ok"

\--- PASS: TestHandleCNF/Complex\_chain\_with\_&& (0.00s)

\--- PASS: TestHandleCNF (0.00s)

\=== RUN   TestHandleCNF\_Mocked

2025/12/26 15:38:53 INFO Straw-boss (CNF Mode) analyzing:  query=nano

\--- PASS: TestHandleCNF\_Mocked (0.00s)

\=== RUN   TestCheckConfig\_UserResponse

\--- PASS: TestCheckConfig\_UserResponse (0.00s)

FAIL

Process finished with the exit code 1

javi@Arthur:\~/yups/cli/cmd$ tail \-n \+1 \*

\==\> auto-config.go \<==

package cmd

import (

"context"

"crypto/sha256"

"encoding/hex"

"fmt"

"io"

"log/slog"

"net/http"

"os"

"os/exec"

"path/filepath"

"strings"

"time"

"github.com/javilopezg/yups/cli/internal/sys"

"github.com/spf13/viper"

"golang.org/x/sync/errgroup"

)

var acMode bool

var arMode bool

var yupsPath \= "/usr/local/bin/yups"

var modelUri \= "https://huggingface.co/bartowski/google\_functiongemma-270m-it-GGUF/resolve/main/google\_functiongemma-270m-it-Q8\_0.gguf"

var modelHash \= "f50fbac8552d090863d5fefa983d24ac1ca37df23b1c77e3bbbd80aeb3b208c4"

const (

hookStart \= "\# \--- YUPS\_HOOK\_START \---"

hookEnd   \= "\# \--- YUPS\_HOOK\_END \---"

)

func init() {

rootCmd.Flags().BoolVar(\&acMode, "auto-config",

false, "Set configuration to default values.")

rootCmd.Flags().BoolVar(\&arMode, "auto-remove",

false, "Remove configuration and binaries.")

}

func handleAR() {

home, \_ := os.UserHomeDir()

os.RemoveAll(filepath.Join(home, ".yups"))

updateBashrc(false)

sys.RunSudoCommand("rm", yupsPath)

}

func handleAC() {

slog.Info("Straw-boss (AC Mode).")

start := time.Now()

const steps \= 6

sys.Step(1, steps, "Getting system info")

info := sys.GetSystemInfo()

sys.Step(2, steps, "Saving config file")

saveConfigFile(info)

sys.Step(3, steps, "Setting bash integration")

if err := updateBashrc(true); err \!= nil {

slog.Error("Failed to update .bashrc",

"error", err)

slog.Warn("Yups will work with limited functionality.")

} else {

slog.Info(".bashrc hooks updated successfully")

}

g, ctx := errgroup.WithContext(context.Background())

g.Go(func() error {

sys.Step(4, steps, "Installing 'provides' helper")

installProvidesHelper()

return nil

})

g.Go(func() error {

sys.Step(5, steps, "Installing yups")

copyExecutableToPath()

return nil

})

g.Go(func() error {

sys.Step(6, steps, "Downloading model")

return downloadModel(ctx)

})

if err := g.Wait(); err \!= nil {

slog.Error("Error config yups", "internal", err)

os.Exit(1)

}

slog.Info("Yups configuration completed in ", "time", time.Since(start).Round(time.Second))

}

func saveConfigFile(info sys.Info) {

viper.Set("os", info.OS)

viper.Set("pm", info.PM)

viper.Set("distro\_id", info.DistroID)

viper.Set("distro\_version", info.DistroVersion)

viper.Set("distro\_pretty", info.DistroPretty)

viper.Set("log\_level", "info")

if err := viper.WriteConfig(); err \!= nil {

os.MkdirAll(filepath.Dir(viper.ConfigFileUsed()), 0755\)

err \= viper.SafeWriteConfig()

if err \!= nil {

slog.Error("Error writing config file", "file", viper.ConfigFileUsed(), "error", err)

}

}

}

func updateBashrc(insert bool) error {

home, \_ := os.UserHomeDir()

bashrcPath := filepath.Join(home, ".bashrc")

content, err := os.ReadFile(bashrcPath)

if err \!= nil {

return err

}

lines := strings.Split(string(content), "\\n")

var newLines \[\]string

skipping := false

for \_, line := range lines {

if strings.Contains(line, hookStart) {

skipping \= true

continue

}

if strings.Contains(line, hookEnd) {

skipping \= false

continue

}

if \!skipping {

newLines \= append(newLines, line)

}

}

bashHooks := fmt.Sprintf(\`

%s

\# Hooks for the YUPS project

command\_not\_found\_handle() {

    if "%s" \--command-not-found "$@"; then

        return $?

    else

        return 127

    fi

}

export \-f command\_not\_found\_handle

\_yups\_ce\_handle() {

    local exit\_code=$?

    \# 130 is Ctrl+C, 127 is CNF (handled above), 0 is success

    if \[\[ $exit\_code \-eq 0 \]\] || \[\[ $exit\_code \-eq 127 \]\] || \[\[ $exit\_code \-eq 130 \]\]; then

        return

    fi

    "%s" \--command-error "$exit\_code" "$YUPS\_LAST\_CMD"

}

export \-f \_yups\_ce\_handle

if \[\[ \-z "$PROMPT\_COMMAND" \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle"

elif \! \[\[ "$PROMPT\_COMMAND" \== \*"\_yups\_ce\_handle"\* \]\]; then

    export PROMPT\_COMMAND="\_yups\_ce\_handle;${PROMPT\_COMMAND}"

fi

\_yups\_save\_last\_cmd() {

    if \[\[ "$BASH\_COMMAND" \!= "\_yups\_ce\_handle" \]\]; then

        export YUPS\_LAST\_CMD="$BASH\_COMMAND"

    fi

}

trap '\_yups\_save\_last\_cmd' DEBUG

%s\`, hookStart, yupsPath, yupsPath, hookEnd)

var finalContent string

if insert {

finalContent \= strings.TrimSpace(strings.Join(newLines, "\\n")) \+ "\\n" \+ bashHooks \+ "\\n"

} else {

finalContent \= strings.TrimSpace(strings.Join(newLines, "\\n"))

}

return os.WriteFile(bashrcPath, \[\]byte(finalContent), 0644\)

}

func installProvidesHelper() {

info := sys.GetSystemInfo()

switch info.PM {

case "apt":

if \_, err := exec.LookPath("apt-file"); err \!= nil {

slog.Info("Installing apt-file for advanced search...")

sys.RunSudoCommand("apt-get", "update")

sys.RunSudoCommand("apt-get", "install", "-y", "apt-file")

sys.RunSudoCommand("apt-file", "update")

}

case "pacman":

if \_, err := exec.LookPath("pkgfile"); err \!= nil {

slog.Info("Installing pkgfile for advanced search...")

sys.RunSudoCommand("pacman", "-S", "--noconfirm", "pkgfile")

sys.RunSudoCommand("pkgfile", "--update")

}

}

}

func copyExecutableToPath() {

targetPath := yupsPath

currentPath, err := os.Executable()

if err \!= nil {

slog.Error("Could not determine current executable path", "error", err)

return

}

if currentPath \== targetPath {

return

}

slog.Info("Ensuring yups is in /usr/local/bin...", "from", currentPath)

if err := sys.RunSudoCommand("cp", currentPath, targetPath); err \!= nil {

slog.Error("Failed to copy executable to path", "error", err)

return

}

sys.RunSudoCommand("chmod", "+x", targetPath)

}

func downloadModel(ctx context.Context) error {

home, \_ := os.UserHomeDir()

path := filepath.Join(home, ".yups/models/gemma-3-270m.gguf")

if \_, err := os.Stat(path); err \== nil {

return nil

}

os.MkdirAll(filepath.Dir(path), 0755\)

resp, err := http.Get(modelUri)

if err \!= nil {

return err

}

defer resp.Body.Close()

out, \_ := os.Create(path \+ ".tmp")

defer os.Remove(path \+ ".tmp")

defer out.Close()

counter := \&sys.ProgressWriter{Total: uint64(resp.ContentLength), Message: "Downloading model"}

\_, \_ \= io.Copy(out, io.TeeReader(resp.Body, counter))

fmt.Print("\\n")

os.Rename(path+".tmp", path)

if \!verifyChecksum(path, modelHash) {

return sys.YupsError{Message: "Checksum verification failed"}

}

return nil

}

func verifyChecksum(path, expected string) bool {

f, \_ := os.Open(path)

defer f.Close()

h := sha256.New()

\_, \_ \= io.Copy(h, f)

return hex.EncodeToString(h.Sum(nil)) \== expected

}

\==\> auto-config\_test.go \<==

package cmd

import (

"crypto/sha256"

"encoding/hex"

"fmt"

"net/http"

"net/http/httptest"

"os"

"path/filepath"

"testing"

"github.com/javilopezg/yups/cli/internal/sys"

"github.com/spf13/viper"

"github.com/stretchr/testify/assert"

)

func TestMain(m \*testing.M) {

originalRunner := sys.SudoRunner

sys.SudoRunner \= func(name string, args ...string) error {

fmt.Printf("\[Test Mock\] Sudo: %s %v\\n", name, args)

return nil

}

code := m.Run()

sys.SudoRunner \= originalRunner

os.Exit(code)

}

func TestHandleAC(t \*testing.T) {

// 1\. Aislamiento del Entorno

tmpDir := t.TempDir()

originalHome := os.Getenv("HOME")

os.Setenv("HOME", tmpDir)

defer os.Setenv("HOME", originalHome)

// Crear un .bashrc falso para que updateBashrc tenga algo que leer

fakeBashrc := filepath.Join(tmpDir, ".bashrc")

os.WriteFile(fakeBashrc, \[\]byte("\# Fake bashrc\\n"), 0644\)

// 2\. Mock del Servidor de Descarga para el Modelo

mockContent := "fake-model-content"

hash := sha256.Sum256(\[\]byte(mockContent))

expectedHash := hex.EncodeToString(hash\[:\])

server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r \*http.Request) {

w.Header().Set("Content-Length", fmt.Sprintf("%d", len(mockContent)))

w.WriteHeader(http.StatusOK)

w.Write(\[\]byte(mockContent))

}))

defer server.Close()

// 3\. Sobrescribir variables globales para el test

originalUri := modelUri

originalHash := modelHash

originalPath := yupsPath

modelUri \= server.URL

modelHash \= expectedHash

yupsPath \= filepath.Join(tmpDir, "fake\_yups\_bin")

defer func() {

modelUri \= originalUri

modelHash \= originalHash

yupsPath \= originalPath

}()

// 4\. Ejecución del Test

configPath := filepath.Join(tmpDir, ".yups", "config.toml")

viper.SetConfigFile(configPath)

fmt.Printf("Viper config file: %s\\n", viper.ConfigFileUsed())

handleAC()

// 5\. Aserciones

// Verificar Configuración

fmt.Printf("Viper config file: %s\\n", viper.ConfigFileUsed())

assert.FileExists(t, configPath)

viper.ReadInConfig()

assert.Equal(t, "linux", viper.GetString("os"))

// Verificar Bashrc (debe contener los hooks)

bashContent, \_ := os.ReadFile(fakeBashrc)

assert.Contains(t, string(bashContent), hookStart)

// Verificar Modelo (debe existir y ser el archivo correcto)

modelPath := filepath.Join(tmpDir, ".yups", "models", "gemma-3-270m.gguf")

assert.FileExists(t, modelPath)

downloadedContent, \_ := os.ReadFile(modelPath)

assert.Equal(t, mockContent, string(downloadedContent))

}

func TestHandleAR(t \*testing.T) {

tmpDir := t.TempDir()

originalHome := os.Getenv("HOME")

os.Setenv("HOME", tmpDir)

defer os.Setenv("HOME", originalHome)

// Simular instalación previa

yupsDir := filepath.Join(tmpDir, ".yups")

os.MkdirAll(yupsDir, 0755\)

os.WriteFile(filepath.Join(tmpDir, ".bashrc"), \[\]byte(hookStart+"\\n"+hookEnd), 0644\)

handleAR()

// Verificar que se eliminó la carpeta de configuración

\_, err := os.Stat(yupsDir)

assert.True(t, os.IsNotExist(err), "La carpeta .yups debería haber sido eliminada")

// Verificar que el bashrc ya no tiene los hooks

bashContent, \_ := os.ReadFile(filepath.Join(tmpDir, ".bashrc"))

assert.NotContains(t, string(bashContent), hookStart)

}

\==\> command-error.go \<==

package cmd

import (

"log/slog"

"strings"

)

var ceMode bool

func init() {

rootCmd.Flags().BoolVar(\&ceMode, "command-error",

false, "System hook for command error.")

rootCmd.Flags().MarkHidden("command-error")

}

func handleCE(args \[\]string) {

slog.Info("Straw-boss (CE Mode) analyzing: ",

"query", strings.Join(args, " "))

//TODO identify the command and make suggestions.

}

\==\> command-not-found.go \<==

package cmd

import (

"log/slog"

"slices"

"strings"

"github.com/javilopezg/yups/cli/internal/parser"

"github.com/javilopezg/yups/cli/internal/sys"

"github.com/spf13/viper"

)

var cnfMode bool

func init() {

rootCmd.Flags().BoolVar(\&cnfMode, "command-not-found",

false, "System hook for command not found.")

rootCmd.Flags().MarkHidden("command-not-found")

}

func handleCNF(args \[\]string) {

query := strings.Join(args, " ")

slog.Info("Straw-boss (CNF Mode) analyzing: ",

"query", query)

lastCommand := viper.GetString("YUPS\_LAST\_CMD")

slog.Debug("Last command: ", "command", lastCommand)

command, \_ := parser.ExtractEffectiveCommand(lastCommand)

//TODO if command is in sys.PMTypes analyze and replace

if slices.Contains(sys.PMTypes, command) {

//TODO identify the subcommand

//TODO identify the packages

//TODO suggest correct command Y/n

//TODO if not suggest yups query for deep analysis Y/n

return

}

//TODO if command is similar to one in scanned suggest

//TODO is command in currentCommands with fuzzy search

//TODO suggest correct command Y/n

//TODO if not suggest yups query for deep analysis Y/n

//TODO execute provides, parse output and suggest install

replacer := strings.NewReplacer(sys.ArgumentString, command)

provides := replacer.Replace(

sys.PMCommands\["provides"\].Commands\[viper.GetString("pm")\])

output, err := sys.RunCommand(provides)

if err \== nil {

//TODO parse output???

slog.Debug("Provides output", "output", output)

}

}

\==\> command-not-found\_test.go \<==

package cmd

import (

"strings"

"testing"

"github.com/javilopezg/yups/cli/internal/sys"

"github.com/spf13/viper"

"github.com/stretchr/testify/assert"

)

func TestHandleCNF(t \*testing.T) {

//TODO mock sys.Runner

tests := \[\]struct {

name          string

fullCmd       string

lastCmd       string

args          \[\]string

mockOutput    string

expectedInLog string

}{

{

name:          "Simple command",

fullCmd:       "nano",

lastCmd:       "nano",

args:          \[\]string{"nano"},

mockOutput:    "nano-8.5-2.fc43.x86\_64",

expectedInLog: "nano-8.5-2.fc43.x86\_64",

},

{ //FIXME this case is not real, sudo doesn't exit with 127

name:          "Command with sudo",

fullCmd:       "sudo nano",

lastCmd:       "sudo nano",

args:          \[\]string{"sudo", "nano"},

mockOutput:    "nano-8.5-2.fc43.x86\_64",

expectedInLog: "nano-8.5-2.fc43.x86\_64",

},

{

name:          "Complex chain with &&",

fullCmd:       "nano && echo 'ok'",

lastCmd:       "nano",

args:          \[\]string{"nano", "&&", "echo", "ok"},

mockOutput:    "nano-8.5-2.fc43.x86\_64",

expectedInLog: "nano-8.5-2.fc43.x86\_64",

},

}

for \_, tt := range tests {

t.Run(tt.name, func(t \*testing.T) {

viper.Set("pm", "dnf")

viper.Set("YUPS\_LAST\_CMD", tt.lastCmd)

handleCNF(tt.args)

//TODO Asserts

assert.Equal(t, "", "")

})

}

}

func TestHandleCNF\_Mocked(t \*testing.T) {

oldRunner := sys.Runner

defer func() { sys.Runner \= oldRunner }()

sys.Runner \= func(provides string, args ...string) (string, error) {

if strings.Contains(provides, "provides") {

return "nano-8.5-2.fc43.x86\_64", nil

}

return "", nil

}

viper.Set("pm", "dnf")

viper.Set("YUPS\_LAST\_CMD", "nano")

handleCNF(\[\]string{"nano"})

//TODO Agrega aserciones aquí

}

\==\> root.go \<==

package cmd

import (

"fmt"

"log/slog"

"os"

"path/filepath"

"github.com/javilopezg/yups/cli/internal/sys"

"github.com/spf13/cobra"

"github.com/spf13/viper"

)

var (

cfgFile string

debug   bool

)

var rootCmd \= \&cobra.Command{

Use:   "yups",

Short: "YUPS: Your Universal Prompt Straw-boss (AI Powered)",

Long: \`The YUPS CLI handles your command not found and other

prompt errors. It can solve any situation or requirement 

by querying an online LLM.\`,

PersistentPreRunE: func(cmd \*cobra.Command, args \[\]string) error {

setupLogger(debug)

return checkConfig()

},

Run: func(cmd \*cobra.Command, args \[\]string) {

if cnfMode {

handleCNF(args)

return

}

if ceMode {

handleCE(args)

return

}

if acMode {

handleAC()

return

}

if arMode {

handleAR()

return

}

if len(args) \== 0 {

cmd.Help()

return

}

processQuery(args)

return

},

}

func checkConfig() error {

err := viper.ReadInConfig()

if err \== nil {

return nil

}

if \_, ok := err.(viper.ConfigFileNotFoundError); ok {

slog.Warn("Configuration file not found.")

answer, \_ := sys.PromptConfirmReplacement("yups \--auto-config")

if answer {

return nil

}

}

return sys.YupsError{

"Yups needs to be configured before execution. Try 'yups \--auto-config'.",

sys.ExitUsage, err,

}

}

func processQuery(args \[\]string) {

//TODO process user query

}

func Execute() {

if err := rootCmd.Execute(); err \!= nil {

yErr, ok := err.(sys.YupsError)

if \!ok {

fmt.Println(err)

os.Exit(1)

}

fmt.Println(yErr.Message)

os.Exit(yErr.Code)

}

}

func init() {

cobra.OnInitialize(initConfig)

rootCmd.PersistentFlags().

StringVar(\&cfgFile, "config", "",

"Configuration file (default: $HOME/.yups/config.toml)")

rootCmd.PersistentFlags().

BoolVarP(\&debug, "debug", "d",

false, "set the log level to debug")

viper.BindPFlag("debug",

rootCmd.PersistentFlags().Lookup("debug"))

}

func initConfig() {

if cfgFile \!= "" {

viper.SetConfigFile(cfgFile)

} else {

home, err := os.UserHomeDir()

if err \!= nil {

slog.Error("Error getting home directory.", "Error", err)

os.Exit(1)

}

viper.AddConfigPath(home \+ "/.yups")

viper.SetConfigType("toml")

viper.SetConfigName("config")

}

viper.AutomaticEnv()

viper.ReadInConfig()

slog.Debug("Setting config file.", "ConfigFileUsed", viper.ConfigFileUsed())

}

func setupLogger(isDebug bool) {

home, err := os.UserHomeDir()

if err \!= nil {

slog.Error("Error getting home directory.", "Error", err)

os.Exit(1)

}

folder := filepath.Join(home, ".yups")

os.MkdirAll(folder, 0755\)

logFile, err := os.OpenFile(folder+"/log", os.O\_APPEND|os.O\_CREATE|os.O\_WRONLY, 0644\)

level := slog.LevelInfo

if isDebug {

level \= slog.LevelDebug

}

handler := sys.NewYupsHandler(logFile, level)

logger := slog.New(handler)

slog.SetDefault(logger)

if err \!= nil {

slog.Error("Error setting file log", "Error", err)

}

}

\==\> root\_test.go \<==

package cmd

import (

"testing"

"github.com/javilopezg/yups/cli/internal/sys"

"github.com/spf13/viper"

"github.com/stretchr/testify/assert"

)

func TestCheckConfig\_UserResponse(t \*testing.T) {

oldPrompt := sys.PromptConfirmReplacement

oldConfigFile := viper.ConfigFileUsed()

defer func() {

sys.PromptConfirmReplacement \= oldPrompt

viper.SetConfigFile(oldConfigFile)

}()

sys.PromptConfirmReplacement \= func(command string) (bool, error) {

return false, nil

}

viper.SetConfigFile("/non/existent/path")

err := checkConfig()

assert.Error(t, err)

assert.Contains(t, err.Error(), "needs to be configured")

}

\==\> update.go \<==

package cmd

//TODO auto-config \-\> \--update in crontab

//TODO PM.update

//TODO scanner.ListAllCommands \-\> .yups/commandsDB

javi@Arthur:\~/yups/cli/cmd$ 

Tenemos un error con el go llama que intuyo que es porque hay que compilarlo a parte antes de utilizarlo, o instalar algo en el equipo. ollama está instalado, pero no he instalado ninguna librería extra ni nada.

\# github.com/go-skynet/go-llama.cpp

binding.cpp:1:10: error fatal: common.h: No existe el fichero o el directorio

    1 | \#include "common.h"

      |          ^\~\~\~\~\~\~\~\~\~

compilación terminada.

estoy probando el ejemplo que viene en el repo de go-llama.cpp pero parece que espera tener el modelo en formato .bin no .gguf. Mirando la librería que importa https://github.com/ggml-org/llama.cpp/tree/master sí que soporta gguf pero no tengo claro que el wrapper de go lo soporte. Además tira de un commit de hace 2 años mientras que la librería está en desarrollo actualmente por lo que nos quedamos sin esos dos años de avances. ¿Es mucho lío usar directamente la librería de cpp y saltarnos el wrapper de go?

Dices "CGO buscará libllama.so en la ruta que pusimos en LDFLAGS."

Pero en ningún paso previo comentas nada sobre LDFLAGS ni veo referencia a ello por el código que has creado del wrapper C ni del motor de inferencia

Vale, entendido. Sí, por favor crea todos los archivos necesarios y las dependencias que haya que incluir.

Creo que no lo he hecho bien porque me dice que no encuentra llama.h

llama\_bridge.cpp:1:10: error fatal: llama.h: No existe el fichero o el directorio

    1 | \#include "llama.h"

      |          ^\~\~\~\~\~\~\~\~

compilación terminada.

la carpeta de llama.cpp debe estar el yups/cli ¿correcto?

Vale, ya veo el problema. llama.h no está en la raiz del repo:

javi@Arthur:\~/yups/cli/llama.cpp$ find . |grep llama.h

./include/llama.h

./src/llama-hparams.cpp

./src/llama-hparams.h

./build/src/CMakeFiles/llama.dir/llama-hparams.cpp.o.d

./build/src/CMakeFiles/llama.dir/llama-hparams.cpp.o

javi@Arthur:\~/yups/cli/llama.cpp$ find . |grep libllama.so

./build/bin/libllama.so.0.0.7549

./build/bin/libllama.so.0

./build/bin/libllama.so

hemos avanzado, porque ahora sí encuentra el llama.h, pero ahora es el ggml.h. Mi suposición puede que sea incorrecta, es que espera que pwd sea el directorio llama.cpp y al ser otro se lía ¿puede ser? Está en llama.cpp/ggml/include

\# github.com/javilopezg/yups/cli/internal/ai

📥 Dow

En el fichero incluido desde llama\_bridge.cpp:1:

internal/ai/../../llama.cpp/include/llama.h:4:10: error fatal: ggml.h: No existe el fichero o el directorio

    4 | \#include "ggml.h"

      |          ^\~\~\~\~\~\~\~

📥 Downloading model: \[============                   

compilación terminada.

Genial, ya hemos superado esa fase. Ahora en el bridge nos dice que estamos usando cosas viejas ¿puedes arreglarlo o reviso la documentación actual?

\# github.com/javilopezg/yups/cli/internal/ai

📥 Downloading model: \[===

llama\_bridge.cpp: In function ‘llama\_instance\* load\_model(const char\*)’:

llama\_bridge.cpp:18:44: aviso: ‘llama\_model\* llama\_load\_model\_from\_file(const char\*, llama\_model\_params)’ es obsoleto: use llama\_model\_load\_from\_file instead \[-Wdeprecated-declarations\]

📥 Downloading model: \[========================================       

   18 |     auto model \= llama\_load\_model\_from\_file(path, m\_params);

📥 Downloa

      |                  \~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~^\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~

En el fichero incluido desde llama\_bridge.cpp:1:

internal/ai/../../llama.cpp/include/llama.h:430:47: nota: se declara aquí

  430 |     DEPRECATED(LLAMA\_API struct llama\_model \* llama\_load\_model\_from\_file(

      |                                               ^\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:29:36: nota: en definición de macro ‘DEPRECATED’

   29 | \#    define DEPRECATED(func, hint) func \_\_attribute\_\_((deprecated(hint)))

      |                                    ^\~\~\~

llama\_bridge.cpp:23:44: aviso: ‘llama\_context\* llama\_new\_context\_with\_model(llama\_model\*, llama\_context\_params)’ es obsoleto: use llama\_init\_from\_model instead \[-Wdeprecated-declarations\]

   23 |     auto ctx \= llama\_new\_context\_with\_model(model, c\_params);

      |                \~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~^\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:462:49: nota: se declara aquí

  462 |     DEPRECATED(LLAMA\_API struct llama\_context \* llama\_new\_context\_with\_model(

      |                                                 ^\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:29:36: nota: en definición de macro ‘DEPRECATED’

   29 | \#    define DEPRECATED(func, hint) func \_\_attribute\_\_((deprecated(hint)))

      |                                    ^\~\~\~

llama\_bridge.cpp:25:25: aviso: ‘void llama\_free\_model(llama\_model\*)’ es obsoleto: use llama\_model\_free instead \[-Wdeprecated-declarations\]

   25 |         llama\_free\_model(model);

      |         \~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~^\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:453:31: nota: se declara aquí

  453 |     DEPRECATED(LLAMA\_API void llama\_free\_model(struct llama\_model \* model),

      |                               ^\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:29:36: nota: en definición de macro ‘DEPRECATED’

   29 | \#    define DEPRECATED(func, hint) func \_\_attribute\_\_((deprecated(hint)))

      |                                    ^\~\~\~

llama\_bridge.cpp: In function ‘void free\_model(llama\_instance\*)’:

llama\_bridge.cpp:38:25: aviso: ‘void llama\_free\_model(llama\_model\*)’ es obsoleto: use llama\_model\_free instead \[-Wdeprecated-declarations\]

   38 |         llama\_free\_model(inst-\>model);

      |         \~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~^\~\~\~\~\~\~\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:453:31: nota: se declara aquí

  453 |     DEPRECATED(LLAMA\_API void llama\_free\_model(struct llama\_model \* model),

      |                               ^\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:29:36: nota: en definición de macro ‘DEPRECATED’

   29 | \#    define DEPRECATED(func, hint) func \_\_attribute\_\_((deprecated(hint)))

      |                                    ^\~\~\~

llama\_bridge.cpp: In function ‘char\* infer(llama\_instance\*, const char\*)’:

llama\_bridge.cpp:47:41: error: cannot convert ‘llama\_model\*’ to ‘const llama\_vocab\*’

   47 |     int n\_tokens \= llama\_tokenize(inst-\>model, prompt, strlen(prompt), tokens.data(), tokens.size(), true, false);

      |                                   \~\~\~\~\~\~^\~\~\~\~

      |                                         |

      |                                         llama\_model\*

internal/ai/../../llama.cpp/include/llama.h:1062:36: nota: initializing argument 1 of ‘int32\_t llama\_tokenize(const llama\_vocab\*, const char\*, int32\_t, llama\_token\*, int32\_t, bool, bool)’

 1062 |         const struct llama\_vocab \* vocab,

      |         \~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~\~^\~\~\~\~

internal/ai/../../llama.cpp/include/llama.h:61:12: nota: class type ‘llama\_model’ is incomplete

   61 |     struct llama\_model;

      |            ^\~\~\~\~\~\~\~\~\~\~

Bueno, pues compilar ya compila, así que genial. De momento lo dejo así y en cuanto saque otro rato hago las pruebas pertinentes. Muchas gracias

Gemini puede cometer errores, incluso sobre personas, así que verifica sus respuestas. [Tu privacidad y Gemini](https://support.google.com/gemini?p=privacy_notice)

[Se abre en una ventana nueva](https://support.google.com/gemini?p=privacy_notice)

[image1]: <data:image/png;base64,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>

[image2]: <data:image/png;base64,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>