// Testes para a implementação de KeyValueServer.

package p1

import (
	"bufio"
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"testing"
	"time"
)

const (
	defaultStartDelay = 500
	startServerTries  = 5
	largeMsgSize      = 4096
	updateFreq        = 3
	keyFactor         = 10
)

// largeByteSlice é utilizado para inflar o tamanho das mensagens enviadas a clientes lentos
var largeByteSlice = make([]byte, largeMsgSize)

type testSystem struct {
	test     *testing.T     // Aponta para o caso de teste sendo executado.
	server   KeyValueServer // A implementação do estudante.
	hostport string         // O endereço do servidor.
}

type testClient struct {
	id   int      // Identificador único do cliente.
	conn net.Conn // Conexão TCP do cliente.
	slow bool     // True se e somente se o cliente lê lentamente.
}

type networkEvent struct {
	cli      *testClient // O cliente que recebeu um evento da rede.
	readMsg  string      // A mensagem lida da rede.
	writeMsg string      // A mensagem a ser escrita na rede.
	err      error       // Notifica que um erro ocorreu (se diferente de nil).
}

// countEvent é utilizado para descrever um round em testes Count.
type countEvent struct {
	start int // # de conexões a iniciar em um round.
	kill  int // # de conexões a morrer em um round.
	delay int // Tempo de espera em ms antes do próximo round iniciar.
}

func newTestSystem(t *testing.T) *testSystem {
	return &testSystem{test: t}
}

func newTestClients(num int, slow bool) []*testClient {
	clients := make([]*testClient, num)
	for i := range clients {
		clients[i] = &testClient{id: i, slow: slow}
	}
	return clients
}

// startServer tenta iniciar um servidor em uma porta aleatória, retentando até numTries
// vezes se necessário. Um error diferente de nil é retornado se o servidor falhou ao iniciar.
func (ts *testSystem) startServer(numTries int) error {
	randGen := rand.New(rand.NewSource(time.Now().Unix()))
	var err error
	for i := 0; i < numTries; i++ {
		ts.server = New()
		if ts.server == nil {
			return errors.New("servidor retornado por New() não deve ser nil")
		}
		port := 2000 + randGen.Intn(10000)
		if err := ts.server.Start(port); err == nil {
			ts.hostport = net.JoinHostPort("localhost", strconv.Itoa(port))
			return nil
		}
		fmt.Printf("Warning! Falha ao iniciar o servidor na porta %d: %s.\n", port, err)
		time.Sleep(time.Duration(50) * time.Millisecond)

	}
	if err != nil {
		return fmt.Errorf("Error! Falha ao tentar iniciar o servidor depois de %d tentativas", numTries)
	}
	return nil
}

// startClients conecta os clientes especificados com o servidor, e retorna um error
// não nil se um ou mais dos clientes falharem ao estabelecer conexão.
func (ts *testSystem) startClients(clients ...*testClient) error {
	for _, cli := range clients {
		if addr, err := net.ResolveTCPAddr("tcp", ts.hostport); err != nil {
			return err
		} else if conn, err := net.DialTCP("tcp", nil, addr); err != nil {
			return err
		} else {
			cli.conn = conn
		}
	}
	return nil
}

// killClients mata os clientes especificados. (i.e. fecham as conexões).
func (ts *testSystem) killClients(clients ...*testClient) {
	for i := range clients {
		cli := clients[i]
		if cli != nil && cli.conn != nil {
			cli.conn.Close()
		}
	}
}

// startReading sinaliza os clientes especificados para iniciar a leitura da rede.
// Mensagens lidas da rede são enviadas de voltar ao testador via readChan channel.
func (ts *testSystem) startReading(readChan chan<- *networkEvent,
	clients ...*testClient) {
	for _, cli := range clients {
		// Cria uma nova instância da cli para a goroutine
		// (veja http://golang.org/doc/effective_go.html#channels).

		cli := cli
		go func() {
			reader := bufio.NewReader(cli.conn)
			for {
				// Leia tudo até o primeiro caractere '\n'.
				msgBytes, err := reader.ReadBytes('\n')
				if err != nil {
					readChan <- &networkEvent{err: err}
					return
				}
				// Notifique o testador que uma mensagem foi lida da rede.
				readChan <- &networkEvent{
					cli:     cli,
					readMsg: string(msgBytes),
				}
			}
		}()
	}
}

// startWriting sinaliza os clientes especificados para começarem a escreve na rede. Para garantir que as leituras/
// escritas sejam sincronizadas, as mensagens são enviadas para e escritas no loop principal de eventos do testador.
func (ts *testSystem) startWriting(writeChan chan<- *networkEvent, numMsgs int,
	clients ...*testClient) {
	for _, cli := range clients {
		// Cria uma nova instância da cli para a goroutine
		// (veja http://golang.org/doc/effective_go.html#channels).
		cli := cli
		go func() {
			// Client e message IDs garantem que as mensagens enviadas são únicas.
			for msgID := 0; msgID < numMsgs; msgID++ {

				// Notifica o testador que a mensagem deve ser escrita na rede.
				writeChan <- &networkEvent{
					cli: cli,
				}

				if msgID%100 == 0 {
					// Dá aos leitores algum tempo para consumir as mensagens antes de escrever o próximo
					// conjunto de mensagens.
					time.Sleep(time.Duration(100) * time.Millisecond)
				}
			}
		}()
	}
}

func (ts *testSystem) runTest(numMsgs, timeout int, normalClients, slowClients []*testClient, slowDelay int) error {
	numNormalClients := len(normalClients)
	numSlowClients := len(slowClients)
	numClients := numNormalClients + numSlowClients
	hasSlowClients := numSlowClients != 0

	totalGetWrites := numMsgs * numClients
	totalReads := totalGetWrites * numClients
	normalReads, normalGetWrites := 0, 0
	slowReads, slowGetWrites := 0, 0

	numKeys := numMsgs / keyFactor

	// inicia um gerador de números aleatórios
	randGen := rand.New(rand.NewSource(time.Now().Unix()))

	keyMap := make(map[string]int)
	keyValueTrackMap := make(map[string]int)
	readChan, writeChan := make(chan *networkEvent), make(chan *networkEvent)

	// Começa a ler em background (uma goroutine por cliente).
	fmt.Println("All clients beginning to write...")
	ts.startWriting(writeChan, numMsgs, normalClients...)
	if hasSlowClients {
		ts.startWriting(writeChan, numMsgs, slowClients...)
	}

	// Começa a ler em background (uma goroutine por cliente).
	ts.startReading(readChan, normalClients...)
	if hasSlowClients {
		fmt.Println("Clientes normais iniciam a leitura...")
		time.AfterFunc(time.Duration(slowDelay)*time.Millisecond, func() {
			fmt.Println("Clientes lentos iniciam a leitura...")
			ts.startReading(readChan, slowClients...)
		})
	} else {
		fmt.Println("Todos os clientes iniciam a leitura...")
	}

	// Returna um canal que enviará uma notificação se ocorrer estouro de tempo.
	timeoutChan := time.After(time.Duration(timeout) * time.Millisecond)

	for slowReads+normalReads < totalReads ||
		slowGetWrites+normalGetWrites < totalGetWrites {

		select {
		case cmd := <-readChan:
			cli, msg := cmd.cli, cmd.readMsg
			if cmd.err != nil {
				return cmd.err
			}

			// remove bytes inflados para clientes lentos
			if hasSlowClients {
				byteIndex := bytes.IndexByte([]byte(msg), 0)
				msg = string(append([]byte(msg)[0:byteIndex], "\n"...))
			}

			if v, ok := keyValueTrackMap[msg]; !ok {
				// Aborta! Cliente recebeu uma mensagem que nunca foi enviada.
				return fmt.Errorf("cliente leu mensagem inesperada: %s", msg)
			} else if v > 1 {
				// Esperamos que a mensagem seja lida v - 1 mais vezes.
				keyValueTrackMap[msg] = v - 1
			} else {
				// A mensagem foi lida pela última vez.
				// Apague-a da memória.
				delete(keyValueTrackMap, msg)
			}
			if hasSlowClients && cli.slow {
				slowReads++
			} else {
				normalReads++
			}

		case cmd := <-writeChan:
			var key, value string
			// Sincronizar escritas é necessário porque as mensagens são lidas conconrrentemente
			// por uma goroutine em background.
			cli := cmd.cli

			// gera uma chave única para um cliente, manipulação especial para
			// clientes lentos
			if !cli.slow {
				key = fmt.Sprintf("key_%d_%d", cli.id, randGen.Intn(numKeys))
			} else {
				key = fmt.Sprintf("key_slow_%d_%d", cli.id,
					randGen.Intn(numKeys))
			}

			keySendCount, ok := keyMap[key]
			if !ok {
				keyMap[key] = 0
				keySendCount = 0
			}
			keyMap[key]++

			value_id := keySendCount / updateFreq
			if keySendCount%updateFreq == 0 {
				if !hasSlowClients {
					value = fmt.Sprintf("put,%s.value_%d\n", key, value_id)
				} else {
					// Infla o valor a aumentar a probabilidade de mensagens serem descartadas por
					// clientes lentos.
					value = fmt.Sprintf("put,%s.value_%d%s\n", key, value_id,
						largeByteSlice)
				}

				// envia uma mensagem put em intervalo updateFreq e retorna.
				if _, err := cli.conn.Write([]byte(value)); err != nil {
					return err
				}
			}

			request := fmt.Sprintf("get,%s\n", key)
			keyValue := fmt.Sprintf("%s,value_%d\n", key, value_id)

			if _, ok := keyValueTrackMap[keyValue]; !ok {
				keyValueTrackMap[keyValue] = numClients
			} else {
				keyValueTrackMap[keyValue] += numClients
			}

			if _, err := cli.conn.Write([]byte(request)); err != nil {
				// Aborta! Error ao escrever na rede.
				return err
			}
			if hasSlowClients && cli.slow {
				slowGetWrites++
			} else {
				normalGetWrites++
			}

		case <-timeoutChan:
			if hasSlowClients {
				if normalReads != totalGetWrites*numNormalClients {
					// Garante que clientes não lentos receberam todas as mensagens escritas.
					return fmt.Errorf("clientes não lentos receberam %d mensagens, esperadas %d",
						normalReads, totalGetWrites*numNormalClients)
				}
				if slowReads < 75 {
					// Garante que o servidor armazenou 75 mensagens para
					// clientes lentos.
					return errors.New("clientes lentos leram menos que 75 mensagens")
				}
				return nil
			}
			// Caso contrário, se não há clientes lentos, mensagens não devem ser descartadas
			// e o teste não deve estourar.
			return errors.New("teste expirou")
		}
	}

	if hasSlowClients {
		// Se há clientes lentos, então ao menos algumas mensagens deveriam ter sido descartadas.
		return errors.New("sem mensagens descartadas por clientes lentos")
	}

	return nil
}

func (ts *testSystem) checkCount(expected int) error {
	if count := ts.server.Count(); count != expected {
		return fmt.Errorf("Count retornou um valor incorreto (retornado %d, esperado %d)",
			count, expected)
	}
	return nil
}

func testBasic(t *testing.T, name string, numClients, numMessages, timeout int) {
	//syscall.Getrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: 64000, Max: 64000})
	fmt.Printf("========== %s: %d cliente(s), %d requisições get cada ==========\n", name, numClients, numMessages)

	ts := newTestSystem(t)
	if err := ts.startServer(startServerTries); err != nil {
		t.Errorf("Falhou em iniciar o servidor: %s\n", err)
		return
	}
	defer ts.server.Close()

	if err := ts.checkCount(0); err != nil {
		t.Error(err)
		return
	}

	allClients := newTestClients(numClients, false)
	if err := ts.startClients(allClients...); err != nil {
		t.Errorf("Falou em iniciar os clientes: %s\n", err)
		return
	}
	defer ts.killClients(allClients...)

	// Dá ao servidor algum tempo para registrar os clientes antes de executar o teste.
	time.Sleep(time.Duration(defaultStartDelay) * time.Millisecond)

	if err := ts.checkCount(numClients); err != nil {
		t.Error(err)
		return
	}

	if err := ts.runTest(numMessages, timeout, allClients, []*testClient{}, 0); err != nil {
		t.Error(err)
		return
	}

}

func testSlowClient(t *testing.T, name string, numMessages, numSlowClients, numNormalClients, slowDelay, timeout int) {
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: 64000, Max: 64000})
	fmt.Printf("========== %s: %d total de clientes, %d cliente(s) lentos, %d requisições get cada ==========\n",
		name, numSlowClients+numNormalClients, numSlowClients, numMessages)

	ts := newTestSystem(t)
	if err := ts.startServer(startServerTries); err != nil {
		t.Errorf("Falhou ao iniciar o servidor: %s\n", err)
		return
	}
	defer ts.server.Close()

	slowClients := newTestClients(numSlowClients, true)
	if err := ts.startClients(slowClients...); err != nil {
		t.Errorf("Teste falhou: %s\n", err)
		return
	}
	defer ts.killClients(slowClients...)
	normalClients := newTestClients(numNormalClients, false)
	if err := ts.startClients(normalClients...); err != nil {
		t.Errorf("Teste falhou: %s\n", err)
		return
	}
	defer ts.killClients(normalClients...)

	// Dá ao servidor algum tempo para registrar os clientes antes de executar o teste.
	time.Sleep(time.Duration(defaultStartDelay) * time.Millisecond)

	if err := ts.runTest(numMessages, timeout, normalClients, slowClients, slowDelay); err != nil {
		t.Error(err)
		return
	}
}

func (ts *testSystem) runCountTest(events []*countEvent, timeout int) {
	t := ts.test

	// Usa uma lista encadeada para armazenar uma fila de clientes.
	clients := list.New()
	defer func() {
		for clients.Front() != nil {
			ts.killClients(clients.Remove(clients.Front()).(*testClient))
		}
	}()

	// Count() deve retornar 0 no início de um teste count.
	if err := ts.checkCount(0); err != nil {
		t.Error(err)
		return
	}

	timeoutChan := time.After(time.Duration(timeout) * time.Millisecond)

	for i, e := range events {
		fmt.Printf("Round %d: iniciando %d, matando %d\n", i+1, e.start, e.kill)
		for i := 0; i < e.start; i++ {
			// Criar e iniciar um novo cliente, e adiciona-o à fila de clientes.
			cli := new(testClient)
			if err := ts.startClients(cli); err != nil {
				t.Errorf("Falhou ao iniciar os clientes: %s\n", err)
				return
			}
			clients.PushBack(cli)
		}
		for i := 0; i < e.kill; i++ {
			// Fecha a conexão TCP do cliente com o servidor e remove-o da fila.
			cli := clients.Remove(clients.Front()).(*testClient)
			ts.killClients(cli)
		}
		select {
		case <-time.After(time.Duration(e.delay) * time.Millisecond):
			// Continua para o próximo round...
		case <-timeoutChan:
			t.Errorf("Teste expirou.")
			return
		}
		if err := ts.checkCount(clients.Len()); err != nil {
			t.Errorf("Teste falhou durante o evento #%d: %s\n", i+1, err)
			return
		}
	}
}

func testCount(t *testing.T, name string, timeout int, max int, events ...*countEvent) {
	//syscall.Getrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: 64000, Max: 64000})
	fmt.Printf("========== %s: %d rounds, até %d clientes iniciaram/morrerar por round ==========\n",
		name, len(events), max)

	ts := newTestSystem(t)
	if err := ts.startServer(startServerTries); err != nil {
		t.Errorf("Falhou em iniciar o servidor: %s\n", err)
		return
	}
	defer ts.server.Close()

	// Dá ao servidor algum tempo para registrar os clientes antes de executar o teste.
	time.Sleep(time.Duration(defaultStartDelay) * time.Millisecond)

	ts.runCountTest(events, timeout)
}

func TestBasic1(t *testing.T) {
	testBasic(t, "TestBasic1", 1, 100, 5000)
}

func TestBasic2(t *testing.T) {
	testBasic(t, "TestBasic2", 1, 1500, 10000)
}

func TestBasic3(t *testing.T) {
	testBasic(t, "TestBasic3", 2, 20, 5000)
}

func TestBasic4(t *testing.T) {
	testBasic(t, "TestBasic4", 2, 1500, 10000)
}

func TestBasic5(t *testing.T) {
	testBasic(t, "TestBasic5", 5, 750, 10000)
}

func TestBasic6(t *testing.T) {
	testBasic(t, "TestBasic6", 10, 1500, 10000)
}

func TestCount1(t *testing.T) {
	testCount(t, "TestCount1", 5000, 20,
		&countEvent{start: 20, kill: 0, delay: 1000},
		&countEvent{start: 20, kill: 20, delay: 1000},
		&countEvent{start: 0, kill: 20, delay: 1000},
	)
}

func TestCount2(t *testing.T) {
	testCount(t, "TestCount3", 15000, 50,
		&countEvent{start: 15, kill: 0, delay: 1000},
		&countEvent{start: 25, kill: 5, delay: 1000},
		&countEvent{start: 45, kill: 25, delay: 1000},
		&countEvent{start: 50, kill: 35, delay: 1000},
		&countEvent{start: 30, kill: 1, delay: 1000},
		&countEvent{start: 0, kill: 15, delay: 1000},
		&countEvent{start: 0, kill: 50, delay: 1000},
	)
}

// func TestSlowClient1(t *testing.T) {
//     testSlowClient(t, "TestSlowClient1", 1000, 1, 1, 4000, 8000)
// }

// func TestSlowClient2(t *testing.T) {
//     testSlowClient(t, "TestSlowClient2", 1000, 4, 2, 5000, 10000)
// }
