// main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

type Metrics struct {
	ClientIP    string  `json:"clientIP"`
	CPUUsage    float64 `json:"cpuUsage"`    // Uso de CPU en porcentaje
	MemoryUsage float64 `json:"memoryUsage"` // Uso de RAM en porcentaje
	DiskUsage   float64 `json:"diskUsage"`   // Uso de disco en porcentaje
}

func main() {
	// Obtener variables de entorno con valores por defecto
	serverURL := getEnv("SERVER_URL", "http://192.168.1.180/monitor/monitor.php")
	logFilePath := getEnv("LOG_FILE_PATH", "/var/log/monitor/monitor.log")
	intervalSecondsStr := getEnv("INTERVAL_SECONDS", "20")
	intervalSeconds, err := strconv.Atoi(intervalSecondsStr)
	if err != nil {
		log.Printf("Error al convertir INTERVAL_SECONDS: %v. Usando valor por defecto 20 segundos.", err)
		intervalSeconds = 20
	}

	// Configurar el registro de logs a un archivo.
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Error al abrir el archivo de log:", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Println("Servicio Go de monitorización iniciado.")

	// Obtener la IP local.
	clientIP, err := getLocalIP()
	if err != nil {
		log.Printf("Error al obtener la IP local: %v\n", err)
		clientIP = "Unknown"
	}
	log.Printf("IP local: %s\n", clientIP)

	// Ejecutar el bucle de monitoreo.
	for {
		metrics := Metrics{
			ClientIP: clientIP,
		}

		// Obtener uso de CPU.
		cpuPercents, err := cpu.Percent(0, false)
		if err != nil {
			log.Printf("Error al obtener uso de CPU: %v\n", err)
		} else if len(cpuPercents) > 0 {
			metrics.CPUUsage = cpuPercents[0]
		}

		// Obtener uso de RAM.
		vmStat, err := mem.VirtualMemory()
		if err != nil {
			log.Printf("Error al obtener uso de RAM: %v\n", err)
		} else {
			metrics.MemoryUsage = vmStat.UsedPercent
		}

		// Obtener uso de Disco.
		diskStat, err := disk.Usage("/")
		if err != nil {
			log.Printf("Error al obtener uso de Disco: %v\n", err)
		} else {
			metrics.DiskUsage = diskStat.UsedPercent
		}

		// Enviar métricas al servidor.
		err = sendMetricsToServer(metrics, serverURL)
		if err != nil {
			log.Printf("Error al enviar métricas al servidor: %v\n", err)
		} else {
			log.Printf("Métricas enviadas: %+v\n", metrics)
		}

		// Esperar el intervalo definido.
		time.Sleep(time.Duration(intervalSeconds) * time.Second)
	}
}

func sendMetricsToServer(metrics Metrics, serverURL string) error {
	// Convertir las métricas a JSON.
	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("error al convertir métricas a JSON: %v", err)
	}

	// Enviar la solicitud POST.
	resp, err := http.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error al enviar solicitud POST: %v", err)
	}
	defer resp.Body.Close()

	// Leer la respuesta del servidor.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error al leer respuesta del servidor: %v", err)
	}

	// Opcional: Registrar la respuesta del servidor.
	log.Printf("Respuesta del servidor: %s\n", string(body))
	return nil
}

func getLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("error al obtener interfaces de red: %v", err)
	}

	var validIPs []string

	for _, iface := range interfaces {
		// Ignorar interfaces inactivas o loopback.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			log.Printf("Error al obtener direcciones para la interfaz %s: %v\n", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// Filtrar solo IPv4 que no sean loopback ni link-local.
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				log.Printf("Encontrada IP válida: %s en la interfaz %s\n", ip.String(), iface.Name)
				validIPs = append(validIPs, ip.String())
			}
		}
	}

	if len(validIPs) == 0 {
		log.Println("No se encontró ninguna IP válida. Usando fallback 127.0.0.1")
		return "127.0.0.1", nil
	}

	// Preferir una IP en 192.168.1.x si existe.
	for _, ip := range validIPs {
		if strings.HasPrefix(ip, "192.168.1.") {
			return ip, nil
		}
	}

	// Si no hay 192.168.1.x, devolver la primera encontrada.
	return validIPs[0], nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
