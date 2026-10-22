# QSObook

Caderno digital de contatos de radioamador (QSOs), gratuito e de código aberto, com foco em simplicidade, privacidade e funcionamento local.

## Estado atual

O projeto está na etapa de preparação do repositório e definição da arquitetura. Existem o módulo Go e a documentação inicial; **a aplicação, o banco de dados e as funcionalidades abaixo ainda não estão implementados**. Ainda não há versão utilizável nem comando de instalação ou execução.

## Objetivos e princípios

- Permanecer software livre, com código-fonte público e distribuição gratuita pelo projeto.
- Funcionar localmente e offline, sem serviços externos para as funções essenciais.
- Armazenar os dados localmente em SQLite, sob controle de quem hospeda a aplicação.
- Não incluir telemetria, rastreamento ou coleta de dados de uso.
- Oferecer uma interface extremamente minimalista, limpa, moderna e responsiva.
- Facilitar instalação, manutenção, atualização e, futuramente, exportação e backup.
- Manter poucas dependências e evitar complexidade prematura.

Offline significa que o núcleo funcionará sem internet enquanto o servidor local estiver acessível ao navegador. Não implica, nesta etapa, um aplicativo independente do servidor.

## Funcionalidades planejadas

A primeira versão priorizará cadastro de contatos com indicativo, nome, cidade, estado e país; data e horário automáticos; modo de comunicação (AM, FM, DMR, EchoLink e outros); observações; modo QSO com tabela de participantes; busca rápida e preenchimento automático a partir de dados já registrados localmente.

Para etapas posteriores: exportação e importação, backup local, compatibilidade ADIF e mapas/geocodificação opcionais com OpenStreetMap. Um aplicativo Android é uma possibilidade a avaliar, sem compromisso de implementação. Veja o [roadmap](docs/ROADMAP.md).

O recurso opcional de mapas poderá exigir internet e comunicar dados a terceiros. Deverá vir desativado por padrão, explicar quais dados serão enviados e nunca impedir o uso do caderno local.

## Tecnologias e ambiente

- Go, com preferência pela biblioteca padrão: HTTP com `net/http` e páginas com `html/template`.
- SQLite para persistência local; driver ainda não escolhido.
- HTML, CSS e JavaScript puro, sem frameworks pesados nem CDN obrigatória.
- Hospedagem própria (self-hosted), inicialmente em Raspberry Pi com Debian Linux ARM64.

O módulo é `github.com/erickfaria/QSObook`. O `go.mod` existente declara Go `1.24.4`; mudanças de versão devem ser propostas e justificadas separadamente.

## Primeiros passos

Para obter uma cópia de desenvolvimento, com Git já disponível:

```sh
git clone https://github.com/erickfaria/QSObook.git
cd QSObook
```

Leia a [arquitetura](docs/ARCHITECTURE.md), o [roadmap](docs/ROADMAP.md) e o [guia de contribuição](CONTRIBUTING.md). Não há código Go executável ou testes ainda; comandos de build, execução e instalação serão documentados quando houver implementação. Não é necessário instalar banco, servidor ou dependências para revisar esta documentação.

## Contribuições e segurança

Sugestões e correções são bem-vindas por issues e pull requests. Mudanças importantes devem ser discutidas e revisadas pelo mantenedor antes da integração à `main`. Consulte [CONTRIBUTING.md](CONTRIBUTING.md), incluindo a orientação para proteger a branch, e [SECURITY.md](SECURITY.md) para vulnerabilidades.

## Licença

GNU Affero General Public License, versão 3 (AGPL-3.0), conforme o texto integral em [LICENSE](LICENSE). A distribuição oficial do projeto será gratuita. A licença preserva as liberdades dos usuários e também permite redistribuição comercial; não há restrição adicional de uso não comercial.
