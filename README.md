# QSObook

Caderno digital de contatos de radioamador, gratuito, self-hosted e livre sob [AGPL-3.0](LICENSE). Go, SQLite, HTML/CSS e JavaScript puro. Sem telemetria, analytics, CDN ou consultas externas de indicativos.

## Primeira versão funcional

- **Modo QSO:** tabela de participantes com indicativo, nome, cidade, estado, país, modo e observações. Adicionar, editar, remover e registrar no caderno permanente.
- **Estações:** cadastro e edição; indicativo normalizado em maiúsculas e único.
- **Caderno:** registro manual, listagem, busca por indicativo/nome e detalhes com edição dos dados e observações.
- **Preenchimento local:** ao digitar um indicativo conhecido, recupera o cadastro e informa quantos QSOs anteriores existem. Os dados continuam editáveis. O indicativo na rodada também abre o histórico.
- **Persistência:** estações, contatos e rodada sobrevivem ao reinício. Horários automáticos em UTC, apresentados no fuso configurado.

A rodada é única e compartilhada pelas abas. Adicionar novamente um participante existente atualiza seus dados sem duplicá-lo. Cada participante pode registrar um QSO; cliques repetidos não duplicam esse registro. Remover da rodada mantém a estação e os contatos. Para outro QSO com a mesma estação, remova e adicione novamente.

Salvar um indicativo já cadastrado atualiza seu cadastro com os dados do formulário; revise os campos antes de salvar, especialmente sem JavaScript. Cada QSO guarda uma cópia histórica: editar a estação não altera contatos anteriores. Editar um QSO altera somente esse registro. Indicativo e instante de um registro existente não são editáveis nesta versão.

As buscas retornam até 200 registros, com os QSOs mais recentes primeiro. Refine o termo para localizar registros antigos. Busca por nome ignora maiúsculas/minúsculas ASCII; acentos são diferenciados.

## Preparar e compilar

É necessário Go **1.24.4 ou compatível**. Não é necessário instalar SQLite, compilador C ou pacotes do sistema. O módulo e a versão Go original foram preservados.

A primeira compilação baixa os módulos Go pela internet; depois de compilado, o executável funciona offline e inclui templates, CSS, JavaScript e dados de fuso horário.

```sh
go mod download
CGO_ENABLED=0 go build -o bin/qsobook .
```

Em Linux ARM64, esses comandos compilam nativamente para Raspberry Pi. Sem CGO, também é possível gerar um binário ARM64 em outro computador:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/qsobook-linux-arm64 .
```

Cross-compilation não substitui testar no hardware de destino. Build e testes desta implementação foram executados nativamente em Linux ARM64, com Go 1.24.4 e CGO desativado.

## Executar localmente

```sh
./bin/qsobook
```

Abra **http://127.0.0.1:8080** no mesmo computador. Encerre com `Ctrl+C`. O servidor roda em primeiro plano e não instala serviço algum.

O banco padrão fica em `$XDG_DATA_HOME/qsobook/qsobook.db`; sem essa variável, em `$HOME/.local/share/qsobook/qsobook.db`. O diretório é criado quando necessário. Não guarde dados reais no diretório versionado ou entre recursos estáticos.

Para escolher caminho, porta local e fuso:

```sh
./bin/qsobook -db "$HOME/.local/share/qsobook/meu-caderno.db" -addr 127.0.0.1:8081 -timezone America/Sao_Paulo
```

O fuso padrão é `America/Sao_Paulo`. `-timezone UTC` apresenta os horários em UTC. O registro usa o relógio do host, sem consultar serviços de hora. A opção `-addr` aceita somente IPs de loopback. Não há autenticação: o modelo é uso pessoal por usuários locais confiáveis. **Não exponha diretamente à internet** nem configure proxy ou acesso de rede sem definir autenticação, TLS e o modelo de acesso. Veja [SECURITY.md](SECURITY.md).

## Testes e verificações

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
git diff --check
```

Os testes usam bancos temporários e dados fictícios. Cobrem persistência após reabertura, cadastro, consulta, busca, edição, validação, integridade, migrações e rotas HTTP, inclusive CSRF e escape de conteúdo. Nenhum banco pessoal é utilizado.

Para manter caches e temporários de desenvolvimento dentro do projeto, como nesta implementação:

```sh
mkdir -p .cache/go-build .cache/go-mod .cache/tmp
export GOCACHE="$PWD/.cache/go-build"
export GOMODCACHE="$PWD/.cache/go-mod"
export GOTMPDIR="$PWD/.cache/tmp"
export TMPDIR="$PWD/.cache/tmp"
```

Não há dependências de frontend. A interface usa tabela com rolagem horizontal em telas pequenas, formulários adaptáveis, rótulos, foco visível e navegação por teclado. A validação visual em navegadores reais ainda deve ser feita pelo mantenedor; os testes automatizados verificam HTTP e HTML, não layout gráfico ou execução do JavaScript.

## Estrutura e manutenção

```text
main.go                 configuração, composição e ciclo HTTP
store.go                validação e operações SQLite
web.go                  rotas, templates e proteção HTTP
store_test.go           persistência e integridade
web_test.go             fluxos HTTP e segurança
migrations/             esquema SQL versionado
templates/             interface HTML
static/                 CSS e JavaScript locais
docs/                   arquitetura e roadmap
```

O driver é `modernc.org/sqlite v1.36.3`, sem CGO e sob BSD-3-Clause. É a única dependência direta; há dependências transitivas registradas em `go.mod`/`go.sum`. A [arquitetura](docs/ARCHITECTURE.md) documenta a escolha, limites e manutenção.

Não há exportação, importação, ADIF, mapas ou backup automatizado nesta versão; consulte o [roadmap](docs/ROADMAP.md). Para uma cópia manual, **pare a aplicação e qualquer outro processo que abra o banco antes de copiar o banco**; preserve eventuais arquivos auxiliares SQLite. Não copie apenas o `.db` enquanto estiver em uso. Teste restauração em caminho separado antes de depender da cópia. A aplicação não exclui estações nem QSOs permanentemente.

Leia [CONTRIBUTING.md](CONTRIBUTING.md) e [AGENTS.md](AGENTS.md). A integração segue revisão pelo mantenedor. Nenhum commit, push, PR ou configuração remota é feito pela aplicação.

## Licença

GNU Affero General Public License, versão 3: texto integral em [LICENSE](LICENSE). A distribuição oficial é gratuita; a licença permite redistribuição comercial sem restrições adicionais. O rodapé oferece acesso à licença e ao repositório do código. Quem disponibilizar uma versão modificada pela rede deve oferecer o código correspondente conforme a AGPL.
