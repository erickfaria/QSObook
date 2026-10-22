# Instruções para agentes de IA

Estas instruções se aplicam a todo o repositório QSObook. Leia os arquivos existentes e examine o estado do Git antes de editar. Preserve alterações prévias do usuário, inclusive as já preparadas no índice.

## Escopo e arquitetura

- Mantenha Go, SQLite e HTML/CSS/JavaScript puro. Prefira `net/http`, `html/template` e outras bibliotecas padrão.
- Preserve o módulo `github.com/erickfaria/QSObook`. Não altere a versão Go sem necessidade discutida com o mantenedor.
- Priorize simplicidade, legibilidade, manutenção e funcionamento em Linux ARM64, especialmente Raspberry Pi com Debian.
- Crie pacotes e diretórios apenas quando contiverem código necessário. Evite camadas genéricas, frameworks pesados e abstrações especulativas.
- Justifique novas dependências, verificando licença, manutenção, necessidade, suporte ARM64 e implicações de CGO. A biblioteca padrão não inclui driver SQLite.
- Diferencie decisões confirmadas de propostas e funcionalidades planejadas de funcionalidades implementadas.

## Go e interface

- Use `gofmt`, nomes idiomáticos, pacotes pequenos e funções com responsabilidades claras.
- Trate e propague erros com contexto; não silencie falhas nem use panic para erros esperados.
- Use `context.Context` quando houver cancelamento ou limites de execução. Feche recursos e delimite transações.
- Evite estado global mutável e interfaces sem consumidor real.
- Mantenha interface minimalista, responsiva e acessível, com HTML semântico, rótulos e navegação por teclado.

## Privacidade e segurança

- Telemetria, analytics, rastreamento e envio de dados de uso são proibidos.
- O núcleo deve funcionar sem internet; sirva recursos essenciais localmente, sem dependências de CDN, fontes externas ou APIs remotas.
- Dados devem permanecer no SQLite local. Não registre contatos, observações, credenciais ou conteúdo de formulários em logs desnecessariamente.
- Mapas/geocodificação externos só poderão ser opcionais, desativados por padrão e com informação clara sobre dados enviados e ativação consciente.
- Use SQL parametrizado, validação no servidor, escape de templates e limites de entrada. Não transforme entrada de usuário em HTML confiável.
- Planeje proteção contra CSRF e alterações via GET, timeouts HTTP e tratamento de erros sem detalhes internos na interface.
- Não exponha banco, backups ou configurações como arquivos estáticos. Não inclua dados pessoais reais em exemplos ou testes.
- Não publique o servidor na rede nem decida autenticação ou exposição pública sem discutir o modelo de acesso.
- Preserve a AGPL v3 e os avisos de licença; não acrescente restrições incompatíveis.

## Testes e documentação

- Para código novo ou alterado, faça testes proporcionais ao risco usando preferencialmente `testing`, `httptest` e bancos temporários isolados.
- Cubra comportamentos relevantes: validação, persistência, busca, erros e integridade. Nunca use o banco real do usuário em testes.
- Quando houver pacotes Go, execute `go test ./...` e `go vet ./...` se o ambiente permitir; informe impedimentos e não anuncie checks não executados.
- Valide build e funcionamento ARM64 ao introduzir código ou dependências que possam afetá-los; não presuma que cross-compilation resolve exigências de CGO.
- Para mudanças só de documentação, revise links, coerência e `git diff --check`; não crie testes artificiais.
- Atualize documentos afetados, mantendo instruções reproduzíveis e sem promessas de recursos inexistentes.

## Limites operacionais

- Trabalhe exclusivamente no repositório. Não acesse credenciais nem arquivos pessoais fora do projeto.
- O host executa outros serviços: não instale pacotes nem altere sistema operacional, serviços, Docker, rede, firewall, DNS ou Tailscale.
- Não execute operações destrutivas, apague dados, reescreva histórico ou reverta trabalho alheio sem autorização explícita. Pergunte antes de decisões importantes indefinidas.
- Não faça commit, push, merge nem altere configurações remotas sem instrução explícita do usuário.
- Alterações devem seguir o fluxo de pull request e revisão do mantenedor; não contorne proteções da `main`.
- Ao terminar, apresente resumo das alterações, arquivos afetados, verificações executadas e pontos de atenção. Informe claramente o que ficou pendente.
