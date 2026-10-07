# Despliegue del backend en AWS

El frontend sigue en Vercel. AWS recibe únicamente el backend y PostgreSQL.

```text
Vercel → HTTPS / ALB → ECS Fargate (Go) → RDS PostgreSQL privado
                              └──────→ proveedor de IA por HTTPS / NAT
```

Las plantillas crean dos zonas de disponibilidad, subredes públicas para ALB/NAT, subredes privadas para ECS y subredes aisladas para RDS. ECS no recibe IP pública. PostgreSQL solo acepta conexiones desde el security group de ECS. ALB solo envía tráfico al puerto 8080. El HTTP público redirige a HTTPS con ACM.

La salida de ECS usa NAT para ECR, Secrets Manager, CloudWatch y el proveedor IA, conforme a la [documentación de networking de ECS](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/networking-outbound.html). Esta infraestructura crea recursos facturables: ALB, dos NAT gateways, ECS, RDS Multi-AZ, almacenamiento y logs. Revisar AWS Pricing Calculator con la región y carga antes de ejecutarla.

## Requisitos reales

1. Cuenta AWS, región elegida y AWS CLI v2 configurado con un perfil autorizado. Verificar `aws sts get-caller-identity`.
2. Docker con motor Linux y buildx. Validación local completada antes de subir imagen.
3. Dominio de API bajo tu control y certificado ACM **emitido en la misma región del ALB**. No se presupone un dominio ya existente. Solicitar certificado y completar su validación DNS.
4. Versión PostgreSQL 17 disponible en Amazon RDS regional. La plantilla propone 17.11; confirmar con `describe-db-engine-versions` o pasar otra versión 17 soportada.
5. Si habilitas IA, credencial del proveedor configurada en `AI_API_KEY` del entorno de la terminal y modelo soportado. No pegarla en comandos guardados, Git ni mensajes.

El script tiene permisos efectivos del perfil: CloudFormation necesita crear VPC, rutas, EIP/NAT, security groups, IAM, ECS, ALB, RDS, Secrets Manager y CloudWatch. El operador también necesita ECR push, ECS run-task y actualización del secreto IA. Las roles de ejecución del contenedor solo leen los secretos asignados. La aplicación no necesita permisos AWS adicionales.

## Instalación inicial ejecutable

```powershell
./scripts/aws/deploy.ps1 `
  -Region REGION_ELEGIDA `
  -Profile PERFIL_CONFIGURADO `
  -CertificateArn ARN_ACM_EMITIDO `
  -ImageTag VERSION_INMUTABLE
```

Para IA real, agregar `-AIProvider openai -AIModel MODELO_HABILITADO`, con `AI_API_KEY` previamente configurada en el entorno. El script escribe la clave a Secrets Manager mediante un archivo temporal que elimina al terminar; nunca la imprime ni la deja en el repositorio.

El procedimiento:

1. Verifica Docker, identidad AWS, versión RDS, certificado y plantilla.
2. Crea ECR y sube la imagen `linux/amd64` con tag inmutable.
3. Crea la infraestructura con `DesiredCount=0`.
4. Guarda la clave IA cuando corresponde.
5. Ejecuta `/app/migrate up` como tarea ECS administrativa y espera exit code 0.
6. Ejecuta `/app/provision` para crear el rol limitado `backend_api` y asignar permisos sobre las tablas.
7. Activa una réplica del backend y espera un servicio estable.

No es un script de actualización de un servicio activo: se detiene si ya existe una réplica. Puede retomar una instalación inicial con DesiredCount=0 usando el mismo stack. Un fallo administrativo mantiene el backend sin arrancar; revisar logs antes de continuar.

## ECR manual

```powershell
aws cloudformation deploy --stack-name johan-backend-ecr `
  --template-file deploy/aws/ecr.template.json --region REGION_ELEGIDA
aws ecr get-login-password --region REGION_ELEGIDA | `
  docker login --username AWS --password-stdin CUENTA.dkr.ecr.REGION_ELEGIDA.amazonaws.com
docker build --platform linux/amd64 -t johan-backend:VERSION .
docker tag johan-backend:VERSION CUENTA.dkr.ecr.REGION_ELEGIDA.amazonaws.com/johan-backend:VERSION
docker push CUENTA.dkr.ecr.REGION_ELEGIDA.amazonaws.com/johan-backend:VERSION
```

Usar un tag nuevo para cada versión. La política de ECR solo elimina imágenes sin tag; conservar versiones desplegadas para rollback. Revisar resultados del escaneo de vulnerabilidades antes de producción.

## Secrets y acceso PostgreSQL

CloudFormation genera contraseñas aleatorias administrativas y de aplicación. RDS usa `portfolio_admin`. El backend usa `backend_api`, con permisos DML y secuencias, sin CREATE ni acceso a `schema_migrations`. La tarea administrativa recibe otro `DATABASE_URL`; sus permisos y su definición no se asignan al servicio API.

`DATABASE_URL` y `AI_API_KEY` se inyectan desde Secrets Manager en el arranque de ECS. No se almacenan en variables visibles de la task definition. Después de rotar un secreto, iniciar un nuevo deployment para que ECS vuelva a leerlo.

La rotación de PostgreSQL requiere coordinar la contraseña del rol con su `DATABASE_URL`: los secretos URL construidos por CloudFormation no se regeneran automáticamente al cambiar el secreto fuente. Actualizar el rol y ambos secretos afectados mediante el procedimiento administrativo antes de iniciar tareas nuevas. No habilitar rotación automática de contraseñas sin implementar esa coordinación.

TLS de PostgreSQL usa `sslmode=verify-full`. La imagen incluye el bundle oficial de certificados RDS desde [AWS RDS Trust Store](https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem), configurado con `PGSSLROOTCERT`; verifica cadena y hostname. La configuración de producción rechaza `sslmode=disable` y `require`. Renovar la imagen si cambia el bundle.

No abrir RDS al público para ejecutar migraciones. Usar la tarea ECS administrativa dentro de la red privada. `migrate down` revierte una versión y puede destruir datos: no es el procedimiento habitual de rollback de aplicación.

## Dominio y validación HTTPS

Los outputs `ALBHost` y `ALBHostedZone` sirven para un alias Route 53 o un CNAME en tu proveedor DNS. El registro debe corresponder al dominio cubierto por el certificado ACM. El hostname generado del ALB no sustituye el dominio del certificado.

Después de propagar DNS:

```powershell
Invoke-RestMethod https://DOMINIO_API/health
Invoke-RestMethod https://DOMINIO_API/ready
Invoke-RestMethod https://DOMINIO_API/api/v1/portfolio/profile
```

Comprobar redirección HTTP → HTTPS, preflight desde `https://jd-aristizabal.vercel.app`, rechazo de otros orígenes, creación de tarea, persistencia tras redeploy y una respuesta IA real. CORS no autentica llamadas de clientes que no sean navegadores.

El CORS de producción solo acepta el frontend especificado. `TRUSTED_PROXY_CIDRS` contiene exclusivamente las subredes públicas de este ALB; el parser toma la primera IP no confiable de derecha a izquierda en `X-Forwarded-For`, sin confiar en headers arbitrarios.

## Logs y salud

CloudWatch recibe logs JSON de arranque, request_id, método, path, status y duración. Retención de contenedor: 30 días. No se imprimen cuerpos del chat, headers, claves ni URL de conexión.

```powershell
aws logs tail /portfolio/johan-backend --follow --region REGION_ELEGIDA
```

El contenedor verifica `/health`; el target group del ALB verifica `/ready`. Ante caída de RDS la liveness continúa y readiness devuelve 503. ECS dispone de deployment circuit breaker con rollback habilitado y apagado gradual de hasta 30 segundos.

## Actualizaciones

1. Compilar, probar y construir una imagen con tag nuevo. No reemplazar un tag existente.
2. Registrar una revisión de la task definition administrativa con esa imagen.
3. Ejecutar `migrate up` y confirmar éxito dentro de ECS antes de actualizar el servicio. Las futuras migraciones deben ser compatibles con la versión anterior para permitir rolling deployment.
4. Ejecutar provision cuando cambien tablas o permisos del rol.
5. Actualizar la plantilla con la nueva `ImageURI`, conservar los parámetros y establecer `DesiredCount` en el valor actual. No usar el script de instalación sobre el servicio activo.
6. Verificar targets saludables, `/ready`, persistencia, costos IA y logs. El circuit breaker vuelve a la tarea anterior si falla el arranque; no revierte datos ni migraciones.

Un backup RDS diario y su retención de siete días ya están configurados. Probar restauración antes de depender de ella. La base tiene deletion protection y política de snapshot; eliminar el stack exige gestionar RDS conscientemente y conservar secretos necesarios para restaurar.

No se han creado recursos AWS desde este repositorio hasta que se ejecute el despliegue con un perfil autorizado y los parámetros reales.
