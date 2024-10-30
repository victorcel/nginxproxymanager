import re
import subprocess
from collections import defaultdict

# Configuración de parámetros
PUERTO = 81  # Puerto a monitorear
UMBRAL_FALLIDOS = 3  # Intentos fallidos antes de bloquear la IP
INTERVALO_BLOQUEO = 60  # Segundos antes de desbloquear (opcional)
ips_fallidas = defaultdict(int)  # Registro de intentos fallidos por IP

def bloquear_ip(ip):
    print(f"Bloqueando IP: {ip}")
    subprocess.run(["sudo", "ufw", "deny", "from", ip])

def monitorear_puerto():
    tcpdump_proc = subprocess.Popen(
        f"sudo tcpdump -nn -A -s 0 -i any 'tcp port {PUERTO} or tcp port 443' | grep -B 5 'HTTP/1.1 401'",
        shell=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    buffer = ""
    for linea in tcpdump_proc.stdout:
        buffer += linea
        if "HTTP/1.1 401 Unauthorized" in buffer:
            match_ip = re.search(r"IP\s(\d+\.\d+\.\d+\.\d+)\.\d+\s>\s(\d+\.\d+\.\d+\.\d+)\.\d+", buffer)

            if match_ip:
                print(match_ip.group(2))
                ip_origen = match_ip.group(2)
                ips_fallidas[ip_origen] += 1
                print(f"Intento fallido desde IP: {ip_origen} (Total: {ips_fallidas[ip_origen]})")

                # Bloquear IP si alcanza el umbral
                if ips_fallidas[ip_origen] >= UMBRAL_FALLIDOS:
                    # bloquear_ip(ip_origen)
                    ips_fallidas[ip_origen] = 0

            buffer = ""



if __name__ == "__main__":
    try:
        print("Iniciando monitoreo en el puerto", PUERTO)
        monitorear_puerto()
    except KeyboardInterrupt:
        print("Monitoreo detenido.")
