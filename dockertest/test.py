import socket

# Test file to just manually send TCP packets to a specific host/port

host = 'localhost'  # Docker host IP or hostname
port = 8081         # Host port mapped to server1's container port

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
    s.connect((host, port))
    s.sendall(b'Hello from python!')

print('Done. Sent to: ', (host, port))
