# Backend del portafolio de Johan Aristizabal

Monolito modular en Go, Chi y pgx. PostgreSQL guarda Focus Flow, Pixel Sprint y conversaciones. El frontend permanece en [Vercel](https://jd-aristizabal.vercel.app); este repositorio no contiene ni modifica su interfaz.

## Arranque local

Requisitos: Docker con contenedores Linux y Docker Compose v2.

```powershell
Copy-Item .env.example .env
docker compose build
docker compose up -d
Invoke-RestMethod http://localhost:8080/health
Invoke-RestMethod http://localhost:8080/ready
```

Compose inicia PostgreSQL, ejecuta una tarea de migración y arranca el binario Go. La API nunca modifica automáticamente el esquema. El volumen conserva datos entre reinicios. El servicio `migrate` es una tarea independiente de inicio local.

Por defecto `AI_PROVIDER=disabled`: el chat devuelve 503 hasta configurar un proveedor real. No se genera una respuesta simulada. Configura `AI_PROVIDER=openai`, `AI_API_KEY` y `AI_MODEL`, o `openai-compatible` con `AI_BASE_URL` HTTPS que implemente Chat Completions y `max_completion_tokens`. Reinicia el backend después de cambiar variables:

```powershell
docker compose up -d --force-recreate backend
```

Para ejecutar Go sin Docker, exporta `DATABASE_URL` y las variables necesarias en la terminal. La API no carga `.env` por sí sola.

Se requiere Go 1.27.1 en PATH. En este equipo se descargó una distribución portable para validación en `.tools/go/bin`, excluida de Git; se puede agregar temporalmente esa carpeta al PATH de la terminal.

```powershell
go run ./cmd/migrate up
go run ./cmd/api
```

## Contratos

Todas las rutas de datos privados requieren `X-Client-ID`, UUID canónico no nulo. No requiere usuario ni contraseña. El identificador permite aislar datos; quien conoce ese UUID puede acceder a ellos. Más adelante se puede reemplazar la identidad del middleware por un usuario autenticado.

```powershell
$clientId = [guid]::NewGuid().ToString()
$headers = @{ 'X-Client-ID' = $clientId }
Invoke-RestMethod -Method Post -Uri http://localhost:8080/api/v1/focus/tasks `
  -Headers $headers -ContentType application/json `
  -Body '{"title":"Organizar una idea","priority":"alta"}'
```

La API usa `{ "data": ... }` y errores `{ "error": { "code": "...", "message": "..." } }`. `/health` y `/ready` responden `{ "status": "ok" }`. Consultar [OpenAPI](openapi/openapi.yaml) y [contratos para el frontend](docs/contratos.md).

## Módulos

- Focus: tareas, sesiones, historial y estadísticas por zona horaria. Una sesión activa por cliente; completar requiere tiempo transcurrido en el servidor. Finalizaciones repetidas son idempotentes.
- Games: partidas validadas y leaderboard del mejor resultado por cliente. Los UUID privados nunca se publican en el leaderboard.
- Portfolio: una fuente JSON embebida y tipada, compartida por la API pública y Knowledge Service. Actualizaciones del conocimiento se revisan en Git.
- Chatbot: abstracción de proveedor, contexto acotado, claves en servidor, historial PostgreSQL y borrado de conversaciones. Se conservan hasta 100 mensajes por conversación y se envían solo los últimos 12, recortados a 1000 caracteres cada uno.

`internal/http` conecta los módulos y define los contratos HTTP. Las consultas parametrizadas viven junto al módulo que las utiliza; no se agrega sqlc para esta cantidad de consultas.

## Validación

Go 1.27.1. PostgreSQL 17. Docker utiliza imágenes de Go y PostgreSQL con versiones explícitas.

```powershell
go fmt ./...
go vet ./...
go test ./...
go build ./...
$env:TEST_DATABASE_URL = 'postgres://usuario:clave@localhost:5432/portfolio_test?sslmode=disable'
go test ./... -count=1
```

Las pruebas de integración crean y eliminan un esquema aleatorio dentro de la base de pruebas. Sin `TEST_DATABASE_URL`, se omiten explícitamente. No apuntar esa variable a producción. CI ejecuta integración, detector de carreras, OpenAPI, CloudFormation y arranque Docker.

Para revertir **una** migración, ejecuta `go run ./cmd/migrate down` o `docker compose run --rm migrate down`. La reversión inicial elimina las tablas y sus datos; se usa solo con respaldo y contexto apropiado. No se fuerza una migración dirty automáticamente.

## Operación y AWS

La [guía AWS](docs/aws.md), las [plantillas CloudFormation](deploy/aws/backend.template.json) y el [script de instalación](scripts/aws/deploy.ps1) preparan ECR, ALB, ECS Fargate, RDS privado, Secrets Manager y CloudWatch.

El despliegue requiere una cuenta AWS configurada, región, certificado ACM emitido para el dominio de la API y acceso DNS. Las credenciales se configuran fuera del repositorio. No se presume que `api.jd-aristizabal.com` exista.

Ver [seguridad y operación](docs/operacion.md) y [validaciones ejecutadas](docs/validacion.md).
