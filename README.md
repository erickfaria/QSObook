# QSObook

Caderno digital de contatos de radioamador, gratuito, self-hosted e livre sob [AGPL-3.0](LICENSE). Go, SQLite, HTML/CSS e JavaScript puro. Sem telemetria, analytics, CDN ou consultas externas de indicativos.

## Primeira versão funcional

- **Modo QSO:** tabela centrada em indicativo, nome, cidade/UF e observações; modo e país aparecem como informações secundárias. Inclusão rápida e edição na própria tabela, com página de edição como alternativa sem JavaScript.
- **Estações:** cadastro e edição; indicativo normalizado em maiúsculas e único.
- **Caderno:** registro manual, listagem, busca por indicativo/nome e detalhes com edição dos dados e observações.
- **Preenchimento local:** ao digitar um indicativo conhecido, recupera o cadastro e informa quantos QSOs anteriores existem. Os dados continuam editáveis. O indicativo na rodada também abre o histórico.
- **Persistência:** estações, contatos e rodada sobrevivem ao reinício. Horários automáticos em UTC, apresentados no fuso configurado.

A rodada é única e compartilhada pelas abas. **Adicionar à rodada não registra um QSO.** Use **Registrar QSO** para confirmar um contato com um clique. Adicionar novamente alguém já presente preserva o modo, as observações da conversa e o estado de confirmação; use **Editar** para modificá-los. Cada participante pode registrar um QSO; reenvios e cliques simultâneos não duplicam esse registro. Remover da rodada mantém a estação e os contatos. Para outro QSO com a mesma estação, remova e adicione novamente.

Na inclusão e na edição de uma conversa, valores não vazios podem completar ou corrigir nome e localidade do cadastro; campos vazios preservam os valores existentes, mesmo sem JavaScript ou antes de a consulta terminar. Para apagar intencionalmente um campo do cadastro, use **Estações → Editar**. A inclusão na rodada é transacional: uma falha não deixa um cadastro parcialmente atualizado.

**Observações do operador** são permanentes e editadas em **Estações**. **Observações da conversa** pertencem à participação e são copiadas para o QSO somente na confirmação. Nunca atualizam automaticamente as observações permanentes. A tabela identifica os dois tipos. O registro manual de QSO segue essa separação e também usa transação; cada envio manual continua sendo um contato independente.

Cada QSO guarda uma cópia histórica: editar a estação ou a participação não altera contatos anteriores. Editar um QSO altera somente esse registro. Indicativo e instante de um registro existente não são editáveis nesta versão.

O preenchimento automático mantém as edições feitas durante a consulta, ignora respostas antigas (inclusive ao trocar A → B → A) e não apaga campos com respostas vazias. Rascunhos de operadores diferentes ficam separados enquanto a página permanece aberta. Observações permanentes só são preenchidas automaticamente no formulário de cadastro, nunca no formulário de conversa.

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

Cross-compilation não substitui testar no hardware de destino. A versão inicial foi validada nativamente em Linux ARM64 com Go 1.24.4. Esta etapa foi validada em Linux AMD64 com Go 1.27.3, sem CGO, incluindo compilação cruzada para ARM64. A execução desta etapa na Raspberry Pi permanece pendente.

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

As regressões do Modo QSO incluem preservação de campos vazios, separação de observações, rollback por falha de escrita, confirmação concorrente e edição com IDs de cadastro e participação distintos.

Para verificar o JavaScript, opcionalmente use Node.js com o executor de testes nativo:

```sh
node --test tests/app.test.cjs
```

Não há pacotes npm nem instalação de dependências. Esses testes executam `static/app.js` em um DOM simulado, com consultas e temporizadores controlados; não substituem a verificação em navegador.

Para manter caches e temporários de desenvolvimento dentro do projeto, como nesta implementação:

```sh
mkdir -p .cache/go-build .cache/go-mod .cache/tmp
export GOCACHE="$PWD/.cache/go-build"
export GOMODCACHE="$PWD/.cache/go-mod"
export GOTMPDIR="$PWD/.cache/tmp"
export TMPDIR="$PWD/.cache/tmp"
```

Não há dependências de frontend. A interface usa tabela com rolagem horizontal em telas pequenas, formulários adaptáveis, rótulos, foco visível e navegação por teclado. Foram conferidos no navegador o layout desktop, as dimensões em largura de celular, o preenchimento local e a abertura da edição. O navegador integrado bloqueou POSTs por sua origem intermediada; os envios foram validados por testes HTTP e requisições locais com origem válida. A revisão visual completa em celular físico e a revisão com tecnologia assistiva permanecem pendentes.

## Testar o Modo QSO manualmente

Após compilar, inicie com um banco separado para dados fictícios:

```sh
./bin/qsobook -db .cache/teste-manual.db -addr 127.0.0.1:8081
```

1. Abra `http://127.0.0.1:8081` em um navegador local. Adicione `ZZ0TEST`, nome e cidade. Confira a tabela; o Caderno deve continuar sem esse QSO.
2. Clique em **Editar**, abra **País e observações da conversa**, escreva uma nota e salve. Confira que a edição ficou na própria rodada.
3. Em **Estações**, edite o operador e salve uma observação permanente diferente. Ela deve aparecer identificada como **Operador** na tabela.
4. Digite novamente `zz0test`. Aguarde a recuperação de nome e localidade. As observações da conversa devem continuar vazias. Envie imediatamente ou com nome/cidade vazios: o cadastro salvo deve ser preservado e a participação existente não deve se duplicar.
5. Clique em **Registrar QSO**. Confira um único contato no Caderno, com a nota da conversa. Atualizar a página não deve criar outro contato.
6. Altere o cadastro e a nota da rodada. Confira que o contato confirmado manteve seus dados e que a observação permanente não foi substituída pela nota da conversa.
7. Troque rapidamente entre indicativos conhecidos. Confira os dados do indicativo atual e a preservação de suas edições durante a consulta.
8. Encerre com `Ctrl+C` e abra novamente com o mesmo comando. A lista, os cadastros e o histórico devem continuar disponíveis. Verifique também a alternativa sem JavaScript e a navegação por teclado.

## Estrutura e manutenção

```text
main.go                 configuração, composição e ciclo HTTP
store.go                validação e operações SQLite
web.go                  rotas, templates e proteção HTTP
store_test.go           persistência e integridade
web_test.go             fluxos HTTP e segurança
tests/app.test.cjs       regressões JavaScript sem pacotes externos
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
