# Roadmap incremental

Sem datas prometidas. Apenas a preparação documental foi realizada; os itens de aplicação abaixo estão pendentes. Cada etapa deve caber em alterações pequenas e revisáveis. Mudanças de escopo devem ser discutidas antes de implementar.

## Etapa 0 — Preparação do projeto (esta etapa)

- Documentar objetivos, arquitetura, contribuição, segurança e instruções para agentes.
- Adotar AGPL v3 e regras locais para não versionar dados privados.
- Preservar o módulo Go existente, sem criar camadas ou pastas sem uso.
- Orientar proteção da `main`; aplicação das regras no GitHub fica a cargo do proprietário.

## Etapa 1 — Primeira versão utilizável (planejada)

Entregar primeiro uma base mínima Go/HTTP/templates e SQLite, escolhendo o driver e o modelo de acesso antes da implementação. Em incrementos:

1. Cadastro local de contatos de radioamador.
2. Campos de indicativo, nome, cidade, estado e país.
3. Data e horário automáticos com política de fuso definida.
4. Modo de comunicação: AM, FM, DMR, EchoLink e outros, com possibilidade de evolução sem depender de catálogo remoto.
5. Campo de observações.
6. Modo QSO com tabela de participantes de uma sessão; esclarecer o fluxo e a relação com o cadastro antes de modelar as tabelas.
7. Busca rápida de contatos anteriores.
8. Preenchimento automático com dados já registrados localmente, sem consultas externas.

Critérios para considerar a primeira versão utilizável: persistência após reinício, validação de entradas, busca e reutilização dos dados locais, interface responsiva acessível e uso do núcleo sem internet. Testar os fluxos principais com dados fictícios, incluindo erros, e validar execução em Linux ARM64. Documentar execução e atualização reais. Não expor o servidor à rede antes de definir controles de acesso.

## Etapa 2 — Portabilidade e preservação dos dados (planejada)

- Exportação e importação em formato simples e documentado, a escolher.
- Backup local consistente e restauração validada.
- Tratamento de duplicidades e proteção contra sobrescrita acidental na importação.
- Documentação de migrações e preservação de dados durante atualizações.

Priorizar a recuperação dos dados antes de ampliar integrações externas.

## Etapa 3 — Interoperabilidade (planejada)

- Avaliar compatibilidade ADIF, campos suportados e limitações.
- Implementar e testar conversão sem perda silenciosa de dados, após definir o escopo.

## Etapa 4 — Mapas opcionais (planejada)

- Mapa de contatos com OpenStreetMap e geocodificação opcional.
- Escolher provedor e atender atribuição e condições de uso.
- Manter recurso desativado por padrão e explicar comunicação externa antes de ativar.
- Garantir que falhas de rede não afetem o caderno local.

## Possibilidades posteriores, sem compromisso

- Avaliar aplicativo Android a partir das necessidades reais.
- Discutir instalação e atualização mais convenientes após estabilizar a aplicação.

Contas, sincronização em nuvem e serviços externos obrigatórios não fazem parte do escopo definido. Não antecipar infraestrutura para possibilidades ainda não aprovadas.
