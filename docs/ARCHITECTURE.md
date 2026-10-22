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

Ao adicionar um indicativo conhecido, os campos revisados do formulário atualizam a estação. Adicionar novamente na rodada atualiza modo/observações sem duplicar nem reabrir um QSO já registrado. A edição de um participante também atualiza o cadastro da estação; QSOs permanecem inalterados. Remover participante exclui apenas sua participação. Para novo contato, remova e adicione novamente.

O registro de um participante usa transação: verifica se já foi registrado, insere a cópia e liga o QSO à participação atomicamente. Isso evita duplicação por reenvios. O cadastro manual de QSO salva a estação antes do contato; se a segunda operação falhar, o cadastro pode permanecer, e a interface retorna erro. O registro manual é intencionalmente repetível para contatos sucessivos.

SQLite usa uma conexão por processo, `foreign_keys=ON`, `busy_timeout=5000` e journal padrão de rollback. Não há limpeza automática de dados. A migração inicial é `migrations/001_initial.sql`, aplicada em transação e versionada em `PRAGMA user_version`. Versões de esquema futuras são recusadas. Não há downgrade automático; copie o banco com o programa parado antes de atualizar.

Arquivo principal criado com permissão `0600`, diretórios novos com `0700`. Permissões de diretórios preexistentes não são alteradas. Arquivos principais que sejam links simbólicos são recusados; use um diretório privado confiável. Não há criptografia em repouso. O caminho padrão usa o diretório de dados do usuário; `-db` permite escolher outro local. Dados nunca são servidos como arquivos estáticos.

## Tempo, consulta e validação

Instantes vêm do relógio do host, são gravados em UTC com precisão de nanossegundos e largura fixa, permitindo ordenação textual consistente. `time/tzdata` incorpora dados IANA. A apresentação usa `-timezone` (padrão `America/Sao_Paulo`). Não há edição manual de instante.

Indicativos: 1–32 caracteres ASCII, letras/números, `/`, `.` e `-`, começando com letra/número. Nome/localidade: até 120 caracteres por campo; observações: 4000; modo: texto livre obrigatório de até 40 caracteres. Modos sugeridos são apenas um `datalist`. Consultas parametrizadas usam `instr`, sem interpretar curingas SQL. Busca por nome tem case folding ASCII do SQLite e distingue acentos. Listagens retornam até 200 registros por consulta; paginação fica para evolução posterior.

## HTTP e interface

Somente loopback, padrão `127.0.0.1:8080`; hosts externos são rejeitados para reduzir risco de DNS rebinding. Operações de escrita usam POST, token criptográfico CSRF por execução, comparação em tempo constante e verificação de Origin quando presente. O token não é autenticação; após reinício, formulários antigos precisam ser recarregados. POSTs têm limite de 32 KiB; cabeçalhos e tempos HTTP são limitados. O corpo do formulário é a fonte dos campos mutáveis.

`html/template` escapa valores. CSP permite recursos locais e bloqueia frames, scripts inline e submissão externa. Há `nosniff`, proteção de frames, política de referência e `Cache-Control: no-store`. Erros internos não aparecem ao usuário; logs não registram formulários ou contatos. O servidor pode produzir logs operacionais padrão de conexão, sem registrar conteúdo de aplicação.

Tabela como elemento principal, HTML semântico, foco visível e rótulos. CSS adapta formulários e permite rolagem horizontal de tabelas. JavaScript consulta `/lookup` após uma pausa na digitação, cancela buscas antigas e não sobrescreve campos que o usuário já alterou. Ao trocar de indicativo, limpa valores anteriormente preenchidos automaticamente. Sem JavaScript, todos os fluxos de cadastro/edição/consulta funcionam; o preenchimento é manual.

Não há autenticação, isolamento entre usuários locais nem acesso LAN nesta versão. O relógio, a segurança do host e o armazenamento privado são responsabilidades de implantação. TLS, proxy e publicação exigem discutir outro modelo de acesso. Mapas, geocodificação e serviços externos não foram implementados.
