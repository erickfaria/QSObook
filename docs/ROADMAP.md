# Roadmap incremental

Sem datas prometidas. Funcionalidades planejadas não estão disponíveis até implementação e validação.

## Primeira versão — implementada

- A — Fundação: Go 1.24.4 preservado, HTTP com recursos incorporados, SQLite sem CGO, configuração local e testes básicos.
- B — Persistência: estações, QSOs com cópia histórica e participantes separados; migração transacional, integridade referencial, testes com reabertura de banco temporário.
- C — Caderno: cadastro, listagem, busca por indicativo/nome, detalhes e edição.
- D — Modo QSO: tabela de participantes, edição/remoção, preenchimento automático local e registro permanente sem duplicação por cliques repetidos.
- E — Refinamento: CSS responsivo, rótulos e foco, CSRF, limites HTTP, erros seguros, testes HTTP e documentação operacional.

Build, testes e vet realizados em Linux ARM64, sem CGO. Validação gráfica em navegadores desktop/celular e revisão de acessibilidade com tecnologia assistiva ainda pendentes. Testes automatizados não validam layout nem executam o JavaScript. Não há scanner de vulnerabilidades configurado.

## Preservação e portabilidade — planejadas

- Exportação/importação com formatos documentados e tratamento explícito de duplicidades.
- Backup consistente e restauração testada em banco separado.
- Paginação do histórico e melhoria da busca Unicode.
- Revisão periódica das versões do driver e dependências.

## Interoperabilidade — planejada

- Avaliar ADIF e conversão dos campos sem perda silenciosa.

## Integrações opcionais — planejadas

- Mapas e geocodificação OpenStreetMap somente com ativação consciente, desativados por padrão, informando provedor, atribuição e dados enviados.
- Falhas ou ausência de internet nunca devem bloquear o caderno local.

## Possibilidades sem compromisso

Aplicativo Android e facilidades de atualização poderão ser avaliados após estabilizar os dados. Contas em nuvem, telemetria e serviços externos obrigatórios não fazem parte do escopo.
