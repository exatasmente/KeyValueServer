package main

import "net"
import "fmt"
import "bufio"
import "os"
import "bytes"
func main() {

	// connect to this socket
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err == nil {
		for {
			// read in input from stdin
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Text to send: ")
			text, _ := reader.ReadString('\n')
			// send to socket
			conn.Write([]byte(text))
			// listen for reply
			go func() {
				for {
					buffer := make([]byte, 1024)
					conn.Read(buffer[:])
					fmt.Println(string(bytes.Trim(buffer, "\x00")))
				}
			}()
		}

	}

	fmt.Println(err)
}
