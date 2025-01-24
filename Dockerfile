# Etapa de compilación
FROM golang:1.20-alpine AS builder

# Instalar git (si es necesario para dependencias)
RUN apk add --no-cache git

# Establecer el directorio de trabajo
WORKDIR /app

# Copiar go.mod y go.sum para gestionar dependencias
COPY go.mod go.sum ./

# Descargar las dependencias
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar el binario
RUN go build -o monitor main.go

# Etapa de ejecución
FROM alpine:latest

# Instalar dependencias necesarias
RUN apk --no-cache add ca-certificates

# Establecer el directorio de trabajo
WORKDIR /root/

# Copiar el binario compilado desde la etapa anterior
COPY --from=builder /app/monitor .

# Crear directorio para logs
RUN mkdir -p /var/log/monitor

# Definir variables de entorno predeterminadas
ENV SERVER_URL="http://192.168.1.180/monitor/monitor.php"
ENV LOG_FILE_PATH="/var/log/monitor/monitor.log"
ENV INTERVAL_SECONDS=20

# Exponer el puerto si tu aplicación lo requiere (opcional)
# EXPOSE 8080

# Ejecutar el binario
CMD ["./monitor"]
