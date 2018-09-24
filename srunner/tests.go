package main

import "net"
import "fmt"
import "bufio"
import "strings" // only needed below for sample processing
import "os"

type Server struct {
	typeConn string
	port     string
	qt       int
}

var qt int = 0

var kvstore = make(map[string][]byte)

// Esta função instancia o banco de dados.
func init_db() {
	kvstore = make(map[string][]byte)
}


func NewServer(typec string, port string) Server {

	return Server{typec, port, 0}
}
func SocketHanddle(server Server) {
	ln, _ := net.Listen(server.typeConn, server.port)
	for {
		conn, _ := ln.Accept()
		qt += 1

		go Echo(conn)
	}

}


func Echo(conn net.Conn) {
	for {
		recvBuf := make([]byte, 1024)

		_, err := conn.Read(recvBuf[:]) // recv data
		if err != nil {
			qt -= 1
			break

		}
		message := string(recvBuf)
		if strings.HasPrefix(message, "put") {
			data := strings.Split(message, ",")
			data = strings.Split(data[1], ".")
			fmt.Printf("%q\n", data)

		}

		conn.Write([]byte("OK" + "\n"))
	}
}

func main() {

	fmt.Println("Launching server...")
	server := NewServer("tcp", ":9999")
	go SocketHanddle(server)
	for {
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		if text != " " {
			fmt.Printf("%d\n", qt)
		}

	}
}
