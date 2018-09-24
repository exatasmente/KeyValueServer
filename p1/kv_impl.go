// Funções básicas de acesso ao K/V a serem utilizadas em server_impl.

package p1

var kvstore = make(map[string][]byte)

// Esta função instancia o banco de dados.
func init_db() {
    kvstore = make(map[string][]byte)
}

// put insere um novo par chave/valor ou atualiza o valor para uma chave no banco.
func put(key string, value []byte) {
    kvstore[key] = value
}

// get obtém o valor associado a uma chave.
func get(key string) []byte {
    v, _ := kvstore[key]
    return v
}
