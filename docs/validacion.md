# Validaciones ejecutadas

Fecha: 7 de octubre de 2026. Workspace: `C:\backend`. Rama: `feature/implementacion-backend`.

## Resultado local

| Comprobación | Resultado |
|---|---|
| `go fmt ./...` | Correcto |
| `go vet ./...` | Correcto |
| `go test ./... -count=1` con `TEST_DATABASE_URL` | Correcto, integración incluida |
| `go build ./...` | Correcto |
| `govulncheck` después de actualizar x/text | Sin vulnerabilidades conocidas alcanzables |
| Compilación de api, migrate y provision para linux/amd64, CGO deshabilitado | Correcto |
| Especificación OpenAPI 3.0.3 validada con kin-openapi | Correcto |
| Plantillas AWS validadas con cfn-lint 1.47.0 | Correcto |
| Sintaxis de scripts PowerShell | Correcta |
| Migración inicial aplicada mediante `cmd/migrate` | Correcta |
| Migraciones up/down/up en esquemas aislados | Correctas |
| API ejecutada como binario local, `/health` y `/ready` | HTTP 200, `status=ok` |
| `/portfolio/contact` local | Datos de contacto confirmados |
| Usuario `backend_api` provisionado | Puede leer tablas; CREATE TABLE rechazado |
| Existencia de tags de imágenes Go, Alpine y PostgreSQL en registry oficial | Confirmada |

Go usado: 1.27.1 para Windows amd64, obtenido del catálogo oficial y verificado por SHA-256. PostgreSQL real de pruebas: 17.6, ejecutado en loopback y puerto 55432 con credencial aleatoria. Docker Compose y la configuración de producción usan PostgreSQL 17.11; la ejecución Docker y RDS con esa versión continúa pendiente.

## Cobertura funcional comprobada

- Tareas: creación, normalización, edición, completado y eliminación; rechazo de acceso desde otro client_id.
- Sesiones: propiedad de tarea, exclusión de múltiples sesiones activas, rechazo de completado prematuro, completado/cancelación e idempotencia. Historial conservado al borrar una tarea.
- Estadísticas: tareas, minutos, sesiones y siete días por zona horaria.
- Scores: validación de partidas reales generadas con las reglas públicas, rechazo de negativos, inconsistencias y campos ausentes; mejor puntuación por cliente y historial individual.
- Chat: validación, contexto y límites de historial, llamadas HTTP al adapter con servidor de pruebas, timeout, errores sanitizados, no devolución de clave configurada, persistencia de pares exitosos, aislamiento y bloqueo concurrente.
- Rate limiter PostgreSQL: límite de scores y presupuesto compartido por dos instancias concurrentes.
- Portfolio: conocimiento central, dos proyectos, ocho tecnologías, contacto y GitHub desconocido representado como null.
- Readiness: 503 al cerrar el pool, con liveness aún disponible.
- HTTP: CORS, confianza acotada en proxies, UUID y JSON estricto.

Las pruebas de IA utilizan un proveedor de pruebas y un servidor HTTP local. Eso valida el adapter y la persistencia, **no** una llamada con una cuenta real del proveedor.

Se detectó y corrigió la dependencia transitiva afectada por [GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970), actualizando `golang.org/x/text` a 0.39.0. El escaneo de vulnerabilidades forma parte de CI.

## Pendientes externos

1. `docker compose build` y `docker compose up -d`: no ejecutados porque el equipo no dispone de Docker ni de un motor Linux. La construcción, el arranque y smoke tests están incluidos en CI y en el script local, pero no se reportan como aprobados todavía.
2. Detector de carreras Go: configurado en CI Linux; no ejecutado localmente en este equipo.
3. AWS: no se creó ECR, RDS, ECS, ALB ni Secrets Manager. Falta un perfil AWS configurado, región y certificado ACM emitido para un dominio disponible. El script y las plantillas están preparados y validados estáticamente.
4. IA real: falta configurar la credencial y elegir un modelo del proveedor. Con la configuración predeterminada se devuelve 503 para consultas normales del chat.
5. Integración del frontend: sus adaptadores publicados siguen usando almacenamiento local y respuestas predefinidas. Solo se analizaron y documentaron sus contratos; no se modificó la UI ni su despliegue en Vercel.

La implementación Go y PostgreSQL funciona localmente. El flujo completo Vercel → AWS → RDS/IA no se declara validado ni desplegado hasta completar estos pasos.
