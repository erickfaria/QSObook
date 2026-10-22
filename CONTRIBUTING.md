# Como contribuir

O QSObook é um projeto pessoal, gratuito e de software livre para radioamadores. Contribuições de documentação, relatos de problemas, sugestões de interface e código são bem-vindas. O mantenedor decide o escopo e a integração das alterações.

## Antes de começar

Leia o [README](README.md), a [arquitetura](docs/ARCHITECTURE.md), o [roadmap](docs/ROADMAP.md) e as [instruções para agentes](AGENTS.md), quando aplicáveis. A primeira versão já possui servidor, SQLite, cadastro, caderno e Modo QSO. Não implemente funcionalidades só por estarem no roadmap: discuta mudanças importantes, novas dependências, esquema de dados e alterações arquiteturais antes de começar.

As contribuições devem preservar Go, SQLite, frontend sem frameworks pesados, funcionamento local/offline e compatibilidade Linux ARM64. Telemetria e rastreamento não são aceitos. Ao contribuir, disponibilize sua contribuição sob a AGPL v3 adotada pelo projeto e respeite as licenças de material de terceiros.

## Issues

Use as [issues do repositório](https://github.com/erickfaria/QSObook/issues) para bugs e propostas. Verifique primeiro se já existe uma discussão.

- Para bugs: descreva resultado esperado e observado, passos para reproduzir, versão ou commit e ambiente relevante.
- Para propostas: explique a necessidade, o fluxo de uso e a menor solução possível.
- Use dados fictícios. Não anexe bancos pessoais, credenciais ou logs com informações privadas.
- Para vulnerabilidades, siga [SECURITY.md](SECURITY.md) antes de abrir detalhes publicamente.

## Alterações e pull requests

1. Crie um fork se não tiver acesso de escrita e trabalhe em uma branch descritiva, por exemplo `docs/guia-inicial` ou `feat/cadastro-contatos`.
2. Faça mudanças pequenas e focadas, preservando trabalho existente. Evite refatorações sem relação com o objetivo.
3. Atualize a documentação afetada e execute as verificações cabíveis.
4. Envie um pull request para `main`, explicando problema, mudança, verificações e limitações. Relacione a issue quando houver; screenshots devem usar dados fictícios.
5. Aguarde a revisão e integração pelo mantenedor. Alterações importantes precisam de revisão antes da integração.

Mensagens de commit devem descrever a intenção com clareza. Não há necessidade de ferramentas extras para contribuir.

## Código e verificações

Prefira a biblioteca padrão, código Go idiomático, `gofmt`, erros explícitos, SQL parametrizado e testes de comportamento. Justifique qualquer dependência e seu impacto em ARM64 e CGO. Não altere o nome do módulo nem atualize a versão Go incidentalmente.

Use `go test ./...` e `go vet ./...` em ambiente de desenvolvimento apropriado. Use bancos temporários e dados fictícios. Testes não podem depender de serviços externos ou tocar bancos reais. Mudanças que afetem portabilidade devem validar Linux ARM64 e documentar limitações do ambiente. Compile com `CGO_ENABLED=0 go build -o bin/qsobook .`. Veja no README como manter caches e temporários dentro do repositório. Os testes HTTP usam `httptest` sem iniciar serviços persistentes.

Para documentação, revise clareza, links relativos e execute `git diff --check`. Preserve `go.mod`, `go.sum`, arquivos SQL, recursos estáticos e fixtures sintéticas necessárias ao projeto. As exceções para SQLite em `testdata/` não autorizam incluir dados pessoais.

Não instale ferramentas nem altere serviços ou configurações do host sem autorização. No Raspberry Pi que hospeda outros serviços, mantenha todo o trabalho restrito ao repositório.

## Configuração manual da branch main no GitHub

Esta seção orienta o proprietário; não afirma que a configuração remota já exista. A branch local inspecionada é `main`. A branch padrão remota e suas proteções devem ser conferidas manualmente.

Para um repositório público, proteção de branches está disponível no GitHub Free. Em **Settings → General → Default branch**, confirme `main` como padrão. Depois acesse **Settings → Branches → Add classic branch protection rule**, com padrão exato `main`.

Configuração inicial sugerida para um único mantenedor:

- Ativar **Require a pull request before merging**.
- Deixar **Require approvals** desmarcado enquanto não houver outro revisor com permissão adequada. Isso mantém a exigência de PR sem bloquear PRs do próprio mantenedor.
- Ativar **Require conversation resolution before merging**.
- Ativar **Do not allow bypassing the above settings**, para aplicar as exigências também ao proprietário/administrador.
- Manter **Allow force pushes** e **Allow deletions** desativados.
- Adicionar checks obrigatórios somente depois que existir CI real com os respectivos checks; não exigir checks inexistentes.

Enquanto apenas o proprietário tiver escrita, contribuições por fork não dão ao autor poder de integração; o proprietário revisa e faz o merge manualmente. Sem aprovação obrigatória, essa revisão é uma regra de manutenção, não um bloqueio técnico de aprovação. Evite conceder escrita a terceiros sem reavaliar esse modelo.

**O autor de um PR não pode aprovar o próprio PR.** Exigir uma aprovação e bloquear bypass, sendo o único revisor elegível, impede integrar seus próprios PRs. Quando houver outro revisor confiável com escrita, pode-se exigir uma aprovação e descartar aprovações antigas após novos commits. Não usar bypass administrativo como fluxo cotidiano.

Se for necessário exigir especificamente aprovação do proprietário nos PRs de terceiros, avaliar posteriormente um `.github/CODEOWNERS` com `* @erickfaria`, junto de **Require review from Code Owners**. Isso também bloqueia PRs do próprio proprietário se ele for o único code owner e bypass estiver proibido. É necessário decidir o fluxo de revisão dos PRs dele antes de ativar; não foi criado um CODEOWNERS nesta etapa.

Em repositórios pessoais, a opção clássica **Restrict who can push to matching branches** é reservada a repositórios de organizações, e listas de atores autorizados a bypass nessa proteção também são limitadas a organizações. Não presumir controles granulares de papéis iguais aos de uma organização. Nenhuma proteção impede o proprietário de editar posteriormente as próprias regras; o objetivo é prevenir alterações acidentais e reforçar o fluxo de revisão.

Referências oficiais:

- [Disponibilidade, revisões, bypass e restrições de branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches).
- [Criar uma regra de proteção](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/managing-a-branch-protection-rule).
- [Aprovações e impossibilidade de aprovar o próprio PR](https://docs.github.com/en/pull-requests/how-tos/review-pull-requests/approving-a-pull-request-with-required-reviews).
- [Code owners e revisões obrigatórias](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).
