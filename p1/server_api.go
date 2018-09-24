// Define a interface para KeyValueServer. NÃO MODIFIQUE ESTE ARQUIVO!

package p1

// KeyValueServer implementa um banco de dados centralizado simples chave/valor.
type KeyValueServer interface {

    // Start inicia o servidor em uma porta distinta e instancia o banco,
    // além de escutar por conexões dos clientes. Start retorna um erro se há
    // problema em escutar a porta especificada.
    //
    // Este método NÃO deveria bloquear. Ao invés, ele deve gerar uma ou mais
    // goroutines (para manipular coisas como aceitar as conexões de clientes,
    // fazer o broadcast de mensagens aos clientes, sincronizar acesso ao servidor,
    // etc.) e então retorna.
    //
    // Este método deve retornar um erro se o servidor já estiver fechado.
    Start(port int) error

    // Count retorna o número de clientes conectados atualmente ao servidor.
    // Este método não deve ser chamado em um servidor não iniciado ou fechado.
    Count() int

    // Close desliga o servidor. Todas conexões dos clientes devem ser encerradas imediatamente
    // e qualquer goroutines executando em background devem ser avisadas para encerrar.
    Close()
}
