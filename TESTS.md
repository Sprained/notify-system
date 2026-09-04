# Casos de teste — por feature

Documento vivo. Serve de guia pro TDD: cada caso aqui vira um teste antes da
implementação da feature correspondente.

---

## Fase 1 — Schema (spike do banco)

**Pronto quando:** as queries de leitura retornam o esperado com dados inseridos na mão.

### Migração
- [x] Migration sobe limpo em banco vazio (todas as 6 tabelas, FKs, índices)
- [x] Down migration desfaz limpo

### Constraints
- [x] Inserir `delivery` com `message_id` inexistente falha (FK)
- [x] Inserir `delivery` duplicada (`message_id` + `subscriber_id`) falha — índice único de idempotência
- [x] `subscriber.kind` só aceita `telegram`, `fcm`, `webpush`
- [x] `delivery.status` só aceita `pending`, `sent`, `failed`

### Queries de leitura
- [x] Últimas N mensagens de um tópico, ordenadas por tempo (`?since=<ulid>` retorna só mensagens mais novas)
- [x] Entregas falhadas: `delivery.status = failed`, join trazendo `last_error` + dados de `message`/`subscriber`
- [x] Contagem de não lidas por subscriber (mensagens de tópicos roteados, menos as já lidas via `message_read`)

### Edge cases
- [x] Tópico sem `route` configurada → `message` gravada, zero `delivery` (não é erro)
- [x] `route.min_priority` maior que a prioridade da mensagem → não gera `delivery`
- [x] `route.enabled = false` → não gera `delivery`
- [x] `subscriber.enabled = false` → não gera `delivery` mesmo com `route` ativa

---

## Fase 2 — Endpoint de ingestão

> **Status:** ✅ implementado, todos os testes passando (`internal/message`,
> `internal/httpapi`, `internal/postgres`).

**Pronto quando:** um `curl` gera a mensagem e as entregas pendentes corretas no banco.

### Parsing da requisição
- [x] `POST /:topico` com corpo texto grava `message.body` igual ao enviado
- [x] `X-Title`, `X-Priority`, `X-Tags`, `X-Click` populam os campos correspondentes
- [x] Ausência de header usa default (prioridade default quando `X-Priority` não vem)
- [x] `X-Priority` numérico fora do range (1–5) → clampa pro valor válido mais próximo
- [x] `X-Priority` não-numérico (ex: "aaaa") → rejeita com `400`
- [x] `X-Tags` separada por vírgula vira slice
- [x] Corpo vazio → rejeita, não grava a `message`

### Efeitos no banco
- [x] Tópico inexistente é criado implicitamente no primeiro publish
- [x] ID da mensagem é ULID válido, retornado na resposta
- [x] `delivery` criada em `pending` só para `route` que casam tópico + `min_priority`
- [x] Nenhuma `route` casando → `message` gravada, zero `delivery`
- [x] `route`/`subscriber` desabilitados → não gera `delivery` (via HTTP) — coberto pela query de matching validada na Fase 1, reaproveitada aqui
- [x] Teste ponta a ponta (HTTP real → Postgres real via testcontainers) confirma o critério de pronto

### Contrato HTTP
- [x] Método diferente de `POST` no mesmo path → `405`
- [x] Resposta volta rápido, sem esperar envio (handler não depende de nenhum `Notifier`)

---

## Fase 3 — Worker e interface de canal

> **Status:** ✅ implementado, todos os testes passando (`internal/notifier/lognotifier`,
> `internal/worker`, `internal/postgres`).

**Pronto quando:** entrega falha de propósito, tenta de novo com intervalo crescente (1s, 2s, 4s), e para depois de 1 tentativa inicial + 3 retries (4 no total) com o erro registrado.

### Dispatch
- [x] Worker pega `delivery` em `pending` e chama o `Notifier` do `subscriber.kind` correspondente
- [x] `kind` sem `Notifier` registrado → `delivery` marcada `failed` direto, com `last_error` explicando a ausência (sem contar como tentativa de envio)

### Sucesso
- [x] `Notifier.Send` sem erro → `status = sent`, `sent_at` preenchido (o `sent_at` é setado pelo `postgres.Store.MarkSent` via `now()`, não testado por teste de integração dedicado ainda)

### Falha e retry
> As 4 tentativas (1 inicial + 3 retries) rodam em memória dentro de uma única
> execução do worker; só o resultado final é persistido no banco (um `MarkSent` ou
> `MarkFailed`), não uma escrita por tentativa. Ver `DECISIONS.md`.
- [x] Intervalo entre tentativas: 1s → 2s → 4s (backoff exponencial, base 1s, dobrando)
- [x] Depois de 4 tentativas (1 inicial + 3 retries) → `status = failed`, `attempts` registrado, `last_error` com a última mensagem de erro

### Isolamento
- [x] Falha numa `delivery` não trava o processamento de outras
- [x] `delivery` em `sent` ou `failed` nunca é reprocessada (`internal/postgres/store_test.go`, contra Postgres real)

### logNotifier (sem rede)
- [ ] Contador configurável ("falha N vezes, depois sucede") pra simular retry deterministicamente

---

## Fase 4 — Integração com Telegram

> **Status:** ✅ implementado e validado ponta a ponta com bot real — `curl` → `message`
> gravada → `delivery` pendente → worker pega → chegou no Telegram de verdade.

**Pronto quando:** a notificação chega no celular a partir de um `curl` no Pi.

### Envio
- [x] `Send` usa `chat_id` do `subscriber.config` e token do bot corretamente
- [x] Mensagem enviada em texto puro, sem `parse_mode`
- [x] `click_url`, quando presente, vira inline keyboard button (não link em texto)

### Erros da API
- [x] Resposta não-200 da API → `Send` retorna erro (alimenta `last_error` / retry da Fase 3)
- [x] `chat_id` ou token inválido → erro claro o suficiente pro `last_error`
- [x] Timeout de rede respeita o `context` passado

### Limites
- [x] Corpo passando de 4096 caracteres → trunca e envia mesmo assim

### Testabilidade
- [x] Testes unitários usam `httptest.Server` simulando a API do Telegram (sem chamada real)
- [x] Caso de sucesso (200) e caso de falha (400/403) cobertos separadamente

---

## Fase 5 — Dashboard web

> **Status:** ✅ as 5 telas do dashboard implementadas, testadas e validadas no
> navegador — Visão geral, Mensagens, Entregas falhadas, Rotas e Assinantes.

**Pronto quando:** dá pra criar uma regra pela interface e ela passa a valer (critério do roadmap completo — cada tela fecha sua fatia).

### Visão geral

#### Repository (Postgres real, via testcontainers)
- [x] `CountSentToday` conta só entregas enviadas hoje
- [x] `CountSentToday` zero quando nada foi enviado
- [x] `CountFailedDeliveries` conta só `status = failed`
- [x] `CountActive` (assinantes) ignora desabilitado
- [x] `Recent` traz últimas N mensagens de todos os tópicos, mais recente primeiro
- [x] `Recent` retorna vazio sem erro quando não há mensagem

#### Handler (`webui`, com fakes)
- [x] `GET /` mostra a contagem certa de pendentes, enviadas hoje, falhadas e assinantes ativos
- [x] `GET /` lista as últimas mensagens na tabela
- [x] Tudo zerado renderiza sem erro
- [x] `GET /partials/stats` devolve o fragmento com contadores atualizados (htmx polling a cada 10s)

### Infraestrutura (pré-requisito, compartilhada com as próximas telas)
- [x] `GET /` serve o shell completo (sidebar, topbar, CSS, fontes)
- [x] Estáticos (`/static/...`) servidos via `embed.FS`
- [x] `htmx` vendorizado, polling funcionando ponta a ponta (confirmado no navegador)

### Mensagens
- [x] `GET /mensagens` lista as mensagens (tópico, título, prioridade)
- [x] Sem mensagem nenhuma → renderiza vazio, sem erro
- [x] Reaproveita `message.Repository.Recent` — nenhum método novo de repository necessário

### Entregas falhadas
- [x] `ListFailed` traz só `status = failed`, com título da mensagem e label do assinante (Postgres real)
- [x] `ListFailed` vazio quando não há falha
- [x] `GET /entregas` lista mensagem, assinante, tentativas e erro de cada falha
- [x] Sem falha nenhuma → renderiza vazio, sem erro

### Rotas
- [x] `ListRoutes` traz o label do assinante junto (join, Postgres real)
- [x] `ListRoutes` vazio quando não há rota
- [x] `Create` persiste e a rota aparece no `List` em seguida
- [x] `SubscriberStore.List` retorna todos os assinantes (pro dropdown do formulário)
- [x] `GET /rotas` lista as rotas existentes
- [x] Sem rota nenhuma → renderiza vazio, sem erro
- [x] `POST /rotas` com formulário válido cria a rota e redireciona (303) de volta pro `GET /rotas`
- [x] Validado ponta a ponta no navegador — rota criada pela interface aparece na lista

### Assinantes
- [x] `Create` persiste o assinante e ele aparece no `List` em seguida (Postgres real)
- [x] `GET /assinantes` lista com config mascarado (chat_id nunca aparece cru na tela)
- [x] Sem assinante nenhum → renderiza vazio, sem erro
- [x] `POST /assinantes` com formulário válido cria e redireciona (303) pro `GET /assinantes`

---

## Integração — ingestão via JSON (webhook do Dozzle)

> **Status:** ✅ implementado, todos os testes passando (`internal/httpapi`).

**Motivação:** Dozzle (v10+) manda alertas de log via webhook com headers estáticos
(`X-Title`, `X-Priority` etc) + body JSON dinâmico — não dá pra mandar texto puro no
body. `POST /:topico` só aceitava corpo cru até aqui.

**Pronto quando:** um alerta do Dozzle (`Content-Type: application/json`,
`{"message": "..."}`) vira `message` com o corpo certo, sem quebrar o contrato
existente (`curl` com texto puro continua funcionando igual).

- [x] `Content-Type: application/json` com `{"message": "..."}` → `message.Body` recebe o valor do campo `message`
- [x] `Content-Type: application/json; charset=utf-8` (com parâmetro) → mesmo comportamento, charset não atrapalha a detecção
- [x] JSON malformado com `Content-Type: application/json` → rejeita `400`, não grava `message`
- [x] JSON válido sem o campo `message` → mesma regra de corpo vazio → rejeita `400`
- [x] `{"message": ""}` → mesma regra de corpo vazio → rejeita `400`
- [x] Sem `Content-Type: application/json` (mesmo se o corpo por acaso parecer JSON) → corpo tratado como texto puro, sem parsing — comportamento atual intocado
- [x] Headers (`X-Title`, `X-Priority`, `X-Tags`, `X-Click`) continuam vindo do header normalmente, mesmo com body JSON
