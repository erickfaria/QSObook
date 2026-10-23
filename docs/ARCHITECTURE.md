# Arquitetura implementada

## Organização

Monólito Go: navegador → `net/http` → `html/template` / operações SQLite → arquivo local. Um pacote `main`, sem ORM, SPA ou frameworks. `main.go` compõe banco, configuração e servidor; `web.go` concentra HTTP; `store.go` mantém persistência e validação. Os recursos de `templates/`, `static/`, `migrations/` e a licença são incorporados com `embed`. O navegador busca somente dados locais para o preenchimento automático.

## Driver e dependências

Escolhido `modernc.org/sqlite v1.36.3`, driver `database/sql`, BSD-3-Clause, port Go de SQLite 3.49.0 sem CGO, com suporte documentado a Linux ARM64. Requer Go 1.23 e permite preservar o Go 1.24.4 do projeto. A biblioteca padrão não fornece driver SQLite. Dispensa compilador C e biblioteca SQLite instalada no host; o custo é uma árvore de módulos e compilação inicial maiores.

A [documentação da versão](https://pkg.go.dev/modernc.org/sqlite@v1.36.3) inclui plataformas, licença, changelog e referência ao projeto mantido em GitLab. A versão foi fixada para reprodutibilidade e não representa a versão mais recente. Atualizações exigem revisão de compatibilidade Go, avisos de segurança, testes e migrações antes da distribuição; não foi executado um scanner de vulnerabilidades nesta entrega.

Dependências transitivas de runtime estão em `go.mod`: `modernc.org/libc`, `mathutil`, `memory`, `golang.org/x/sys`, `x/exp`, `github.com/google/uuid`, `dustin/go-humanize`, `mattn/go-isatty`, `ncruces/go-strftime` e `remyoudompheng/bigfft`. Usam licenças permissivas BSD/MIT; os avisos foram preservados em `THIRD_PARTY_NOTICES.txt` e incorporados ao executável, acessíveis pelo rodapé, além da AGPL do QSObook. O SQLite original é de domínio público. `go.sum` também inclui módulos usados pelos testes/ferramentas das dependências. `libc` permanece na versão exigida pelo driver; não a atualize isoladamente.

Build, testes de persistência e fluxos HTTP são executáveis nativamente em ARM64 com `CGO_ENABLED=0`. Não foi necessário instalar pacote do sistema.

## Modelo de dados

- `stations`: cadastro atual, indicativo único e normalizado, nome, cidade, estado, país e observações. O indicativo identifica a estação; não há validação de licenças de radioamador ou consulta externa.
- `qsos`: contato permanente ligado a uma estação, instante UTC, modo e cópia dos dados no momento do registro. Edições posteriores do cadastro não alteram esse histórico. Edição de um contato modifica apenas sua cópia; indicativo e instante permanecem fixos.
- `participants`: participantes da única rodada atual. Referência única à estação, modo e observações próprios da rodada, referência opcional ao QSO registrado. Persistem após reinício e são compartilhados entre abas.

Inclusão/edição de participação e registro manual de QSO completam ou corrigem nome/localidade por valores não vazios, preservando os demais campos. As observações permanentes nunca são escritas por esses fluxos. Edições na página de cadastro permitem apagar campos intencionalmente. Indicativos continuam únicos e normalizados.

Adicionar à rodada não confirma contato. Uma nova tentativa para alguém já presente preserva modo, observações e referência ao QSO; a edição explícita permite alterar modo e apagar observações da conversa. A inclusão ou edição e a atualização do cadastro acontecem na mesma transação. Uma participação removida não é recriada por um formulário de edição antigo. Remover participante exclui apenas sua participação. Para novo contato, remova e adicione novamente.

O registro de um participante usa transação: verifica se já foi registrado, insere a cópia com as observações da conversa e liga o QSO à participação atomicamente. Isso evita duplicação por reenvios e confirmações simultâneas. Edições posteriores na participação ou no operador não alteram essa cópia. O cadastro manual de QSO também atualiza a estação e insere o contato na mesma transação; continua intencionalmente repetível para contatos sucessivos.

SQLite usa uma conexão por processo, `foreign_keys=ON`, `busy_timeout=5000` e journal padrão de rollback. Não há limpeza automática de dados. A migração inicial é `migrations/001_initial.sql`, aplicada em transação e versionada em `PRAGMA user_version`. Versões de esquema futuras são recusadas. Não há downgrade automático; copie o banco com o programa parado antes de atualizar.

Arquivo principal criado com permissão `0600`, diretórios novos com `0700`. Permissões de diretórios preexistentes não são alteradas. Arquivos principais que sejam links simbólicos são recusados; use um diretório privado confiável. Não há criptografia em repouso. O caminho padrão usa o diretório de dados do usuário; `-db` permite escolher outro local. Dados nunca são servidos como arquivos estáticos.

## Tempo, consulta e validação

Instantes vêm do relógio do host, são gravados em UTC com precisão de nanossegundos e largura fixa, permitindo ordenação textual consistente. `time/tzdata` incorpora dados IANA. A apresentação usa `-timezone` (padrão `America/Sao_Paulo`). Não há edição manual de instante.

Indicativos: 1–32 caracteres ASCII, letras/números, `/`, `.` e `-`, começando com letra/número. Nome/localidade: até 120 caracteres por campo; observações: 4000; modo: texto livre obrigatório de até 40 caracteres. Modos sugeridos são apenas um `datalist`. Consultas parametrizadas usam `instr`, sem interpretar curingas SQL. Busca por nome tem case folding ASCII do SQLite e distingue acentos. Listagens retornam até 200 registros por consulta; paginação fica para evolução posterior.

## HTTP e interface

Somente loopback, padrão `127.0.0.1:8080`; hosts externos são rejeitados para reduzir risco de DNS rebinding. Operações de escrita usam POST, token criptográfico CSRF por execução, comparação em tempo constante e verificação de Origin quando presente. O token não é autenticação; após reinício, formulários antigos precisam ser recarregados. POSTs têm limite de 32 KiB; cabeçalhos e tempos HTTP são limitados. O corpo do formulário é a fonte dos campos mutáveis.

`html/template` escapa valores. CSP permite recursos locais e bloqueia frames, scripts inline e submissão externa. Há `nosniff`, proteção de frames, política de referência e `Cache-Control: no-store`. Erros internos não aparecem ao usuário; logs não registram formulários ou contatos. O servidor pode produzir logs operacionais padrão de conexão, sem registrar conteúdo de aplicação.

Tabela como elemento principal, HTML semântico, foco visível e rótulos. O formulário rápido e os editores na tabela compartilham um template compacto; país e observações ficam em `details`. Modo e país são informações secundárias das células. As observações permanentes e da conversa têm identificação própria. Os links de edição mantêm a página de edição como alternativa sem JavaScript. Erros na inclusão conservam o formulário preenchido e a tabela.

CSS mantém a rolagem horizontal dentro da tabela. JavaScript consulta `/lookup` após uma pausa na digitação, cancela a requisição anterior e verifica um contador de versão após a resposta e após o JSON. Isso rejeita respostas e erros antigos, inclusive na sequência A → B → A. Campos editados não são sobrescritos e valores vazios da consulta não apagam campos. Rascunhos de dados e observações ficam em memória por indicativo normalizado, sem persistência no navegador. O lookup nunca copia observações permanentes para uma conversa.

O JavaScript bloqueia novos envios de um formulário em processamento e restaura controles na navegação de retorno. A garantia de um QSO por participação permanece no SQLite; o bloqueio visual é complementar. Sem JavaScript, cadastro, edição, consulta e confirmação continuam disponíveis, com preenchimento manual. `tests/app.test.cjs` usa somente bibliotecas nativas do Node.js e DOM simulado para verificar concorrência das consultas e controles de formulário.

Não há autenticação, isolamento entre usuários locais nem acesso LAN nesta versão. O relógio, a segurança do host e o armazenamento privado são responsabilidades de implantação. TLS, proxy e publicação exigem discutir outro modelo de acesso. Mapas, geocodificação e serviços externos não foram implementados.
