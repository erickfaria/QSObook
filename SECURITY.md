# Política de segurança

## Estado e suporte

O QSObook possui uma primeira implementação funcional para uso pessoal local. Ainda não há linha de releases com suporte ou prazo garantido para correções. Não há autenticação ou isolamento entre usuários locais. Não exponha diretamente à internet. A opção de endereço aceita somente loopback; não contorne isso com proxy antes de discutir autenticação, TLS e controle de acesso.

## Comunicação de vulnerabilidades

Não publique dados pessoais, credenciais, bancos reais ou detalhes de exploração em issues ou pull requests públicos.

Na [aba Security do repositório](https://github.com/erickfaria/QSObook/security), procure **Advisories → Report a vulnerability**. Use esse formulário privado **se estiver disponível**. A disponibilidade depende de o proprietário ativar o recurso; esta documentação não confirma que ele esteja habilitado.

Se o formulário privado não estiver disponível, não há outro canal privado confirmado por este projeto. Abra uma issue apenas solicitando um canal privado para relato de segurança, sem detalhes técnicos sensíveis, e aguarde orientação do mantenedor antes de enviar o relato completo. Nenhum endereço de e-mail é presumido.

No canal privado disponibilizado, informe versão ou commit afetado, ambiente, passos mínimos para reprodução com dados fictícios, impacto esperado e eventual mitigação. Combine a divulgação pública com o mantenedor após avaliação e correção. Por se tratar de um projeto pessoal, não há prazo de resposta garantido nem programa de recompensa anunciado.

## Diretrizes para o projeto

Manter dados locais, evitar exposição desnecessária à rede e nunca introduzir telemetria. Corrigir falhas com testes proporcionais ao risco e documentar eventuais ações necessárias para atualização, sem expor dados dos usuários.

O mantenedor pode habilitar relatos privados manualmente em **Settings → Advanced Security → Private vulnerability reporting**. Consulte as [instruções oficiais do GitHub](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository). Nenhuma configuração remota foi alterada nesta etapa.


## Controles implementados e limites

- SQL parametrizado, validação no servidor e templates com escape automático.
- Operações mutáveis somente por POST, token CSRF por execução, verificação de origem e rejeição de Host externo.
- Requisições de formulário limitadas a 32 KiB, cabeçalhos e tempos HTTP limitados; erros sem detalhes internos.
- CSP para recursos locais, bloqueio de frames, `nosniff`, `no-referrer` e respostas sem cache.
- Banco local `0600`, novos diretórios `0700`; banco, configurações e backups nunca são recursos estáticos.
- Nenhum envio automático a terceiros, consulta externa de indicativo ou telemetria.

O token CSRF não autentica usuários. Processos e pessoas com acesso à conta do sistema ou à porta local podem acessar os contatos. Após reiniciar, recarregue formulários antigos. Use diretório privado: diretórios preexistentes não têm permissões alteradas. Não há criptografia do banco ou backup automatizado. Encerre os processos que usam o banco antes de cópias manuais e teste restauração separadamente. O link para o código só é acessado se o usuário o abrir.

As dependências são fixadas em `go.mod`/`go.sum`; isso não substitui revisão de avisos e atualizações de segurança. Esta entrega não inclui auditoria externa nem scanner de vulnerabilidades. Consulte a arquitetura para a versão SQLite incorporada e seus limites.
