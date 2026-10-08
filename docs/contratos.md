# Contrato de integración con el frontend existente

Fuente revisada el 7 de octubre de 2026: [portafolio](https://jd-aristizabal.vercel.app), [Focus Flow](https://jd-aristizabal.vercel.app/works/focus-flow/) y [Pixel Sprint](https://jd-aristizabal.vercel.app/works/pixel-sprint/), incluyendo módulos JavaScript publicados. No se modificó ningún archivo del frontend.

En la revisión inicial, el frontend guardaba Focus Flow en `focus-flow:v1`, el récord en `pixel-sprint:best` y responde el chat con acciones predefinidas. La integración se publicó el 8 de octubre de 2026 en el mismo dominio de Vercel: ahora usa esta API para tareas, sesiones, estadísticas, puntuaciones y chat. La integración posterior debe sustituir únicamente esas fuentes de datos, preservando sus componentes y estilos.

La URL base debe ser una variable pública que contenga exclusivamente la URL HTTPS del backend. Las claves de IA y de base de datos nunca son variables públicas ni deben quedar en Vercel del lado del navegador.

## Identidad y formato

Generar una vez `crypto.randomUUID()` y conservarlo en almacenamiento local. Enviar ese valor en `X-Client-ID` para datos privados. No usar fingerprinting. La identidad no ofrece recuperación ni sincronización entre dispositivos sin compartir el UUID.

Leer el objeto `data` en respuestas exitosas; en errores leer `error.code` y `error.message`. Un DELETE exitoso devuelve 204 sin cuerpo. Respetar 429 y `Retry-After`. No renderizar respuestas IA con HTML sin sanitizar; usar texto, como las burbujas actuales.

## Focus Flow

| Modelo del frontend | Modelo de API |
|---|---|
| `id`, `title`, `priority`, `completed` | mismos nombres; prioridades `alta`, `media`, `baja` |
| `createdAt` | `created_at` |
| fecha de edición/completado | `updated_at`, `completed_at` |
| sesión `minutes` | `duration_seconds / 60` |
| sesión `taskTitle` | `task_title` |
| sesión `completedAt` | `finished_at` |

Al comenzar un temporizador, crear sesión con `duration_seconds` y `task_id` opcional. Conservar el `id` devuelto. Al terminar llamar `PATCH .../{id}/complete`; al reiniciar o abandonar llamar `PATCH .../{id}/cancel`. Si hay una sesión activa tras recargar, obtenerla mediante GET sessions y recuperar su estado, o cancelarla antes de iniciar otra.

La UI tiene pausa local. El contrato actual no incorpora un estado `paused`: el tiempo del servidor transcurre desde `started_at`; una pausa local puede retrasar la finalización, pero no aumentará los minutos acreditados. Para una pausa sincronizada futura será necesario ampliar el modelo de sesiones, sin cambios visuales.

Duraciones actuales: 60, 900, 1500 y 3000 segundos. La API permite de 60 a 7200. El historial conserva el título al inicio aunque se borre la tarea. Las estadísticas agrupadas usan `America/Bogota` por defecto; pasar `time_zone` IANA si corresponde al usuario.

GET tasks/sessions usa `limit` de 1 a 100 y `offset` de 0 a 10000. Pedir páginas hasta recibir menos elementos que `limit`.

## Pixel Sprint

El juego publicado suma `100 × multiplicador` por acierto, aumenta multiplicador cada cinco aciertos consecutivos hasta ×5, y nivel cada siete aciertos totales hasta ocho. Dura 30 segundos. El resultado ya muestra la racha máxima de aciertos.

Enviar `score`, `max_combo` (racha máxima, **no** multiplicador), `level`, `duration_seconds` y `player_name` opcional. El adapter actual `record(score)` deberá recibir también esas métricas ya calculadas por el juego. El backend comprueba rangos y consistencia básica; no verifica cada interacción ni garantiza ausencia de trampas.

`GET .../leaderboard` devuelve un máximo de diez resultados por defecto, uno por cliente. `limit` permite hasta cien. `player_name=null` se puede representar como “Anónimo”. `GET .../me` devuelve `best_score`, `games_played` y una página de `scores`.

## Chat

Enviar `{ "message": "...", "conversation_id": "..." }` a POST `/api/v1/chat`. Omitir el UUID en el primer mensaje; guardar el `conversation_id` devuelto. Presentar `data.message` en la burbuja existente. Las acciones rápidas pueden enviar sus preguntas al mismo endpoint; los enlaces de contacto también están disponibles en Portfolio API.

El frontend limita actualmente el input a 200 caracteres; el backend acepta hasta 1000. El historial se consulta y elimina mediante `/chat/conversations/{id}`. El proveedor no está activo por defecto: 503 debe representarse como indisponibilidad, no como una respuesta inventada.

## Conocimiento

`internal/modules/portfolio/knowledge.json` centraliza los hechos publicados. GitHub se devuelve como `null` porque el sitio revisado no lo publica. No se inventan historial laboral, tarifas ni años de experiencia. El remoto Git no se usa para inferir el contacto público del portafolio.

## Ranking global de los tres juegos

Desde el 8 de octubre de 2026, `pixel-sprint`, `orbit-match` y `pulse-orbit` usan las rutas `POST /api/v1/games/{game}/scores`, `GET .../leaderboard` y `GET .../me`. Todos envían nombre y métricas. Orbit Match y Pulse Orbit requieren un nombre de 2 a 40 caracteres; Pixel Sprint conserva compatibilidad con clientes anteriores sin nombre.

`submission_id` es un UUID estable por partida. Reintentar el mismo contenido devuelve el mismo resultado; cambiar el contenido o cliente con ese UUID devuelve 409. No se duplica una partida cuando se pierde la respuesta HTTP.

Cada ranking publica la mejor puntuación por cliente con el último nombre enviado para ese juego. Las puntuaciones históricas conservan el apodo con el que se guardaron. El TOP 10 es público; el historial personal requiere `X-Client-ID`. PostgreSQL conserva todos los resultados aceptados. La migración `000002_arcade_scores` añade los juegos sin borrar Focus Flow, chat ni Pixel Sprint.

El frontend confirma guardado únicamente tras recibir éxito de la API. Las partidas antiguas con nombre en `portfolio:arcade-scores:v1` se importan de dos en dos, con un intervalo de 65 segundos mientras una página de juego permanece abierta. Solo se retiran de la cola local tras aceptación del servidor; las inválidas o pendientes conservan su copia local. La fecha registrada es la fecha de importación. Los datos privados siguen dependiendo del UUID de ese navegador, sin login ni sincronización automática entre dispositivos.

Color Studio forma parte de los cinco proyectos publicados en Portfolio API y en el contexto del chatbot. Su interfaz actual crea y exporta paletas; no contiene partidas, puntuaciones ni un formulario de guardar paletas.
