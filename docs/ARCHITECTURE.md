# Arquitetura planejada

## Estado atual

Existe apenas o módulo Go e a documentação inicial. Não há servidor, interface, driver, esquema SQLite ou funcionalidades implementados. As decisões abaixo orientam a implementação; propostas não são compromissos de implementação.

## Decisões confirmadas

- Go como linguagem principal, com preferência pela biblioteca padrão.
- HTTP com `net/http` e renderização de páginas com `html/template`.
- HTML, CSS e JavaScript puro para uma interface minimalista, moderna, responsiva e acessível.
- SQLite local como armazenamento persistente, sem banco remoto obrigatório.
- Execução self-hosted, inicialmente em Raspberry Pi com Debian Linux ARM64.
- Núcleo utilizável sem internet, sem telemetria, rastreamento ou coleta de dados de uso.
- Código público sob AGPL v3, com distribuição gratuita pelo projeto.

## Organização inicial proposta

Começar com um único executável e fluxo direto:

```text
Navegador → HTTP / templates em Go → operações de persistência → SQLite local
```

Quando a implementação começar, um `main.go` na raiz pode compor servidor e banco. Arquivos Go próximos podem separar handlers, contatos e persistência conforme a necessidade. Criar `templates/` e `static/` somente ao adicionar os respectivos arquivos. Se o volume de código justificar, extrair pacotes concretos para `internal/`; `cmd/qsobook/` só será necessário se a organização dos executáveis justificar. Não criar essas pastas vazias agora.

Evitar uma API separada, SPA, microsserviços, ORM ou interfaces genéricas sem necessidade demonstrada. Templates fornecem as páginas; JavaScript melhora interações pontuais. A estrutura física e eventual uso de `embed` permanecem propostas.

## Persistência local

A intenção é usar `database/sql` com um driver SQLite compatível com ARM64. A biblioteca padrão Go não fornece esse driver. Sua escolha permanece pendente: avaliar manutenção, licença, custo de dependências, CGO, compilação e distribuição antes de adicioná-lo.

Manter banco e arquivos auxiliares fora dos recursos públicos e do controle de versão. Prever consultas parametrizadas, transações e evolução explícita do esquema. Caminho do banco, estratégia de migrações, modo de journal e configuração de concorrência serão definidos na implementação, com atenção às limitações do Raspberry Pi.

O modelo deverá distinguir cadastro de radioamadores, registros de contatos e participantes de uma sessão QSO. O esquema exato, regras de duplicidade e atualização de dados históricos ainda precisam ser discutidos. A busca e o preenchimento automático usarão exclusivamente registros locais.

Propõe-se armazenar instantes em UTC e apresentá-los com fuso explícito. A política de fuso, edição de horário e origem do relógio deve ser definida antes da implementação; não depender de serviço externo para registrar a hora.

## HTTP, interface e segurança

Validar dados no servidor, escapar conteúdo com `html/template`, impedir SQL construído por concatenação de entradas e limitar tamanhos de requisição. Planejar proteção contra CSRF, mudanças de estado somente em métodos apropriados, timeouts e erros sem exposição de detalhes internos. Manter logs mínimos, sem conteúdo pessoal desnecessário.

Servir CSS, scripts e recursos essenciais localmente, sem CDN ou fontes externas obrigatórias. O navegador ainda precisa alcançar o servidor local; funcionamento offline não pressupõe sincronização ou armazenamento independente no navegador.

Proposta de implantação inicial: escuta apenas em loopback como padrão seguro. Endereço, porta, acesso por LAN, autenticação, sessões e eventual TLS precisam ser definidos conforme o modelo de uso antes de qualquer exposição. Self-hosted não implica acesso público. Não presumir que uma rede local elimina riscos.

Banco e backups devem ter acesso restrito. Criptografia em repouso não está definida; SQLite local, por si só, não oferece essa garantia. Nunca testar com dados pessoais ou bancos em produção.

## Exportação, importação e backup futuros

São recursos planejados, ainda sem formato inicial escolhido. Buscar formatos documentados, portabilidade e exportação sem serviços externos; avaliar ADIF em etapa própria. Importação deverá validar dados e tratar duplicidades sem sobrescrever registros silenciosamente.

Backup deve ser consistente com o estado do SQLite, inclusive quando houver journal/WAL. Não orientar simples cópia de um arquivo em uso como garantia de backup. Definir e testar também a restauração em um banco separado antes de anunciar suporte.

## Integrações opcionais

Mapas e geocodificação com dados OpenStreetMap são possibilidades futuras, desativadas por padrão. Provedor, biblioteca, atribuição, limites de uso e política de privacidade ainda precisam de avaliação. Consultas podem revelar IP, localização ou termos pesquisados; ativação deve informar os dados enviados. A ausência de internet ou falha de provedor não poderá bloquear cadastro, busca ou consulta local.

Um aplicativo Android é apenas uma possibilidade. API pública, sincronização, contas e arquitetura móvel não estão decididas.

## Distribuição e manutenção

Pretende-se distribuir um executável simples com instruções de execução e atualização para Linux ARM64. Driver e eventual CGO determinam detalhes do build; não prometer binário estático ou cross-compilation sem validar. Documentar dependências reais, preservação dos dados e migrações antes da primeira versão. Docker, serviços de sistema e automação de implantação não são necessários nesta preparação.
