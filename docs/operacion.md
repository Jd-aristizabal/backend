# Seguridad y operación

## Costos IA y rate limiting

Límites persistentes en PostgreSQL, compartidos entre réplicas:

| Alcance | Ventana | Máximo |
|---|---|---|
| API por IP | 1 minuto | 120 |
| Rutas privadas por client_id | 1 minuto | 60 |
| Chat por IP | 1 minuto | 10 |
| Chat por client_id | 1 minuto | 6 |
| Chat global, incluyendo creación de conversaciones | 1 día UTC | `CHAT_DAILY_LIMIT`, 200 por defecto |
| Scores por IP | 1 minuto | 12 |
| Scores por client_id | 1 minuto | 6 |

El contador global incluye intentos fallidos y validaciones inválidas. No se reintenta automáticamente una llamada IA: un timeout puede haber generado consumo en el proveedor. Los límites de ventana fija permiten ráfagas en el cambio de minuto, pero mantienen un máximo diario global. Desactivar IA temporalmente con `AI_PROVIDER=disabled` sin afectar los demás módulos.

Máximo de cuerpo JSON: 16 KiB. Mensaje de usuario: 1000 caracteres Unicode. Contexto: instrucciones y conocimiento central, hasta doce mensajes históricos de 1000 caracteres cada uno y la pregunta actual. Salida: máximo 400 tokens incluyendo los que el proveedor contabilice como razonamiento, según [Chat Completions](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create). Timeout IA: 20 segundos. No se agregan herramientas ni acceso a red controlado por el usuario dentro del modelo.

Las IP se convierten en SHA-256 antes de formar las claves del limiter y las ventanas expiradas se eliminan. Cambiar client_id no evita el límite de IP ni el global. No hay fingerprinting ni almacenamiento de headers completos.

## Contexto del chatbot

El prompt contiene solo información pública; ninguna API key, variable de conexión o secreto participa en su construcción. Se rechazan localmente intentos evidentes de revelar instrucciones o claves. El historial mantiene solo roles user/assistant y se trata como contenido no confiable. El proveedor recibe `store=false`; revisar además su política de tratamiento de datos.

Las instrucciones orientan al modelo y los filtros reducen ataques evidentes, pero no constituyen una garantía matemática de que todas las respuestas permanezcan dentro del tema. Antes de habilitar un modelo en producción probar preguntas sobre datos inexistentes, instrucciones en varios idiomas y ataques indirectos. No dar al chatbot permisos para acciones externas. Evaluar nuevas reglas si el comportamiento real del modelo lo requiere.

## Datos y privacidad

Todas las consultas de recursos privados incluyen client_id y los IDs ajenos producen 404. Las tablas de sesiones verifican en la base que una tarea pertenece al mismo cliente. Esta identidad anónima no es autenticación; conservar el UUID privado. Para login posterior, resolver el usuario en el middleware y vincular los client_id existentes con un flujo explícito de vinculación.

El nombre del leaderboard es opcional y público. No se publica client_id. Las conversaciones pueden eliminarse junto a sus mensajes; guardar como máximo cien mensajes por conversación. No enviar secretos al chat. El backend no registra mensajes ni prompts en logs.

Los datos del portafolio viven en config versionada y embebida: el cambio es revisable y no necesita una tabla mutable ni un panel administrativo. La API y Knowledge Service consumen la misma fuente.

## Diagnóstico

- `/health`: proceso HTTP vivo.
- `/ready`: conexión PostgreSQL disponible, timeout de dos segundos.
- `X-Request-ID`: identificador generado por el servidor para cruzar respuesta y logs.
- Errores 500/503 sanitizados: revisar conectividad, estado de migración, disponibilidad y credenciales en su configuración; no imprimir secretos para diagnosticarlos.
- Chats simultáneos sobre una conversación: 409; no se duplican turnos por concurrencia. Los reintentos después de una respuesta perdida aún pueden generar un nuevo turno; el contrato no tiene clave de idempotencia para chat ni scores.

El pool usa máximo veinte conexiones por réplica. Cada chat mantiene una transacción bloqueada durante la llamada para serializar la conversación y persistir ambos mensajes juntos. El timeout acota esa ocupación. Con cuatro réplicas, prever al menos ochenta conexiones más tareas administrativas en la capacidad RDS; escalar pool e instancia conjuntamente si la carga lo requiere.

Las estadísticas de tareas completadas reflejan el estado actual: reabrir una tarea elimina su completed_at. No existe auditoría histórica de transiciones de tareas. Los minutos solo cuentan sesiones completadas; las canceladas no se acreditan. El sistema no puede impedir resultados falsificados que sigan las reglas básicas de puntuación.
