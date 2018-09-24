// Implementação de KeyValueServer. Alunos devem escrever seus códigos neste arquivo.

package p1

import (
	"bufio"
	"net"
	"strconv"
	"strings"
)

type keyValueServer struct {
	atualId int
	conns   map[int]*client
	ln      net.Listener
	ch      chan *mapData
}

// Estruturas de Auxilio
type client struct {
	id      int
	conn    net.Conn
	writeCh chan *mapData
	readCh  chan string
	errorCh chan error
}

type mapData struct {
	key   string
	value []byte
}

// New cria e retorna (mas não inicia) um KeyValueServer.
func New() KeyValueServer {
	return new(keyValueServer)
}

func (kvs *keyValueServer) Start(port int) error {
	var err error
	kvs.atualId = 0
	// Uso da biblioteca strconv pois a biblioteca sitrings causa um erro
	kvs.ln, err = net.Listen("tcp", ":"+strconv.Itoa(port))

	if err == nil {
		kvs.conns = make(map[int]*client)
		kvs.ch = make(chan *mapData)
		// GO routine para contole de acesso ao mapa
		go DbAccess(kvs)
		// GO routine para estabelecimento de conexões
		go func() {
			defer kvs.Close()
			for {
				conn, _ := kvs.ln.Accept()
				client := NewClient(conn, kvs.atualId)
				kvs.conns[kvs.atualId] = client
				kvs.atualId++
				// Go routine para contole de conexão
				go ConnHanddle(client, kvs)
			}
		}()
		return nil
	}
	return err
}

func (kvs *keyValueServer) Close() {
	
	for _, v := range kvs.conns {
		v.conn.Close()
	}

}

func (kvs *keyValueServer) Count() int {
	return len(kvs.conns)
}

// TODO: adicione métodos/funções abaixo!

func NewClient(conn net.Conn, id int) *client {
	c := &client{
		conn:    conn,
		id:      id,
		readCh:  make(chan string),
		writeCh: make(chan *mapData),
		errorCh: make(chan error),
	}
	return c
}

func broadcast(kvs *keyValueServer, data *mapData) {
	for _, v := range kvs.conns {
		v.writeCh <- data
	}

}

func ReadClientEvent(c *client) {
	recv := bufio.NewReader(c.conn)
	for {
		messageByte, err := recv.ReadBytes('\n')
		if err != nil {
			c.errorCh <- err
			break
		}
		message := string(messageByte)
		c.readCh <- message
	}
}

func ConnHanddle(c *client, kvs *keyValueServer) {
	defer delete(kvs.conns, c.id)
	go ReadClientEvent(c)
	broadcastCh := kvs.ch
	exit := false
	for {
		if exit {
			break
		}

		select {
		case readData := <-c.readCh:
			message := readData
			if strings.HasPrefix(message, "put") {
				putData := strings.Split(message, ",")
				putData = strings.Split(putData[1], ".")
				broadcastCh <- &mapData{key: putData[0], value: []byte(putData[1])}
			} else if strings.HasPrefix(message, "get") {
				getData := strings.Split(message, ",")
				getData[1] = strings.TrimSpace(getData[1])

				broadcastCh <- &mapData{key: getData[1], value: nil}
			}

		case writeData := <-c.writeCh:
			_, err := c.conn.Write([]byte(writeData.key + "," + string(writeData.value)))
			if err != nil {
				c.errorCh <- err

			}
		case _ = <-c.errorCh:
			defer c.conn.Close()
			exit = true
		}
	}

}

func DbAccess(kvs *keyValueServer) {

	for {
		data := <-kvs.ch
		if data.value != nil {
			put(data.key, data.value)
		} else {
			r := get(data.key)
			data.value = r
			go broadcast(kvs, data)

		}
	}
}
