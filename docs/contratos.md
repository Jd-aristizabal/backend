# Contrato de integración con el frontend existente

Fuente revisada el 7 de octubre de 2026: [portafolio](https://jd-aristizabal.vercel.app), [Focus Flow](https://jd-aristizabal.vercel.app/works/focus-flow/) y [Pixel Sprint](https://jd-aristizabal.vercel.app/works/pixel-sprint/), incluyendo módulos JavaScript publicados. No se modificó ningún archivo del frontend.

Actualmente el frontend guarda Focus Flow en `focus-flow:v1`, el récord en `pixel-sprint:best` y responde el chat con acciones predefinidas. No llama a esta API todavía. La integración posterior debe sustituir únicamente esas fuentes de datos, preservando sus componentes y estilos.

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
