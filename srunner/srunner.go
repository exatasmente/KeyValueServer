package main

import (
	"fmt"
	"sdcc/p1"
)

const defaultPort = 9999

func main() {
	// Inicializa o servidor.
	server := p1.New()
	if server == nil {
		fmt.Println("New() retornou um servidor nil. Saindo...")
		return
	}

	// Inicia o servidor e continua a escutar por conexões dos clientes em background.
	if err := server.Start(defaultPort); err != nil {
		fmt.Printf("KeyValueServer não pôde ser iniciado: \n %s\n", err)
		return
	}

	fmt.Printf("Iniciou KeyValueServer na porta %d...\n", defaultPort)

	// Bloqueia para sempre.
	for {
	}

}
