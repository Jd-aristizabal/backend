# Backend en EC2

Servidor Ubuntu x86_64: `18.225.234.22`. Archivos operativos: `/opt/johan-backend`. Imagen compilada localmente: `johan-backend:ec2`. El frontend permanece en Vercel.

Esta instalación utiliza PostgreSQL en Docker en el mismo servidor. No es la arquitectura ECS/RDS/ALB de las plantillas originales. No se crearon recursos AWS adicionales.

## Configuración

`deploy/ec2/compose.yaml` inicia PostgreSQL, migración, provisionamiento de rol limitado, API en modo producción y Caddy. Solo el proxy publica puertos 80 y 443. La base de datos y la API no publican 5432 ni 8080. La conexión PostgreSQL utiliza TLS y verifica el certificado para el nombre `postgres`.

La contraseña administrativa, la contraseña del rol `backend_api` y la clave Gemini se conservan en `/opt/johan-backend/.env`, con permisos 600. No copiar este archivo a Git ni imprimir `docker compose config` sin `--quiet`.

La API usa Gemini mediante el adaptador compatible; los logs omiten claves y cuerpos. CORS permite únicamente `https://jd-aristizabal.vercel.app`. El proxy tiene una IP interna fija y la API solo confía en esa IP para identificar clientes.

## HTTPS provisional

Dirección configurada: `https://api.18-225-234-22.sslip.io`. El DNS compartido resuelve la IP del nombre y depende de un servicio externo. Cambiar una IP pública no elástica requiere actualizar el nombre y Caddyfile. Para operación permanente, usar un dominio propio y una IP estable.

El grupo de seguridad debe permitir TCP 80 y 443 desde Internet. SSH debe conservarse restringido al operador. No abrir PostgreSQL ni 8080. Caddy obtiene y renueva su certificado público cuando las validaciones externas pueden llegar al servidor.

## Comprobaciones

Se verificaron en EC2 `/health`, `/ready`, creación/listado/eliminación de tarea, respuesta real de Gemini y persistencia de dos mensajes. Las migraciones y el provisionamiento finalizaron con código 0. PostgreSQL confirmó TLS en conexiones de `backend_api`.

Al instalar, la verificación externa HTTPS estaba pendiente por bloqueo de los puertos públicos. La UI de Vercel no fue modificada para consumir esta API.

## Operación y respaldos

Desde `/opt/johan-backend`:

```sh
sudo docker compose ps --all
sudo docker compose logs --tail=100 backend proxy
sudo docker compose restart backend
sudo ./backup.sh
```

`/etc/cron.d/johan-backend` programa un respaldo a las 03:15 según la zona horaria del servidor. Los dumps tienen permisos privados y retención de siete días. Residen en el mismo disco del servidor: para recuperación ante pérdida de instancia/disco se necesita una copia externa o snapshots. No ejecutar `docker compose down -v`.

El certificado interno de PostgreSQL se generó con vigencia de 365 días; renovarlo antes de vencer conservando el nombre `postgres`, permisos y propietario de la clave, y reiniciar PostgreSQL y API.

La instancia tiene aproximadamente 1 GB de RAM y un disco de 7 GB. Los contenedores tienen límites de memoria y rotación de logs. Supervisar espacio y memoria; esta instalación tiene un único punto de fallo y no proporciona alta disponibilidad.
