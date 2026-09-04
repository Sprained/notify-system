# Central de Notificações — homelab

Serviço próprio de notificações rodando no Raspberry Pi. Recebe eventos por HTTP e
roteia para múltiplos canais (Telegram, push Android, push no navegador) conforme regras
configuráveis por tópico e prioridade.

Inspirado no ntfy, mas construído do zero — o objetivo é aprender, não economizar tempo.

---

## Decisões já tomadas

Não revisitar sem motivo novo. O racional de cada uma está registrado aqui pra evitar
rediscussão.

| Decisão                                                    | Racional                                                                  |
| ---------------------------------------------------------- | ------------------------------------------------------------------------- |
| Backend em Go, binário único                               | Concorrência nativa pro broker; roda leve no Pi                           |
| Postgres como storage                                      | Reaproveitar o banco que ja tem no Pi                                     |
| Mensagem imutável, entrega com estado                      | Permite retry por canal e diagnóstico de falha                            |
| Endpoint de ingestão compatível com ntfy (`POST /:topico`) | Qualquer script/hook que já fala ntfy funciona sem adaptação              |
| ULID como ID de mensagem                                   | Ordenável por tempo — serve de cursor pro `?since=` sem índice extra      |
| Telegram como primeiro canal                               | Um POST resolve; valida o roteador inteiro antes de existir app           |
| Tailscale pro acesso remoto                                | Sem porta aberta, sem DDNS, e dá HTTPS válido via `tailscale cert`        |
| Push carrega só ID + resumo                                | Contorna o limite de ~4 KB do payload FCM; corpo completo vem do servidor |

### Premissas que já foram corrigidas

- Push **não** exige estar na mesma rede. O Pi faz uma requisição de saída pro FCM e o
  Google entrega no celular em qualquer lugar. A rede local só importa no registro
  inicial do app/PWA.
- O Telegram entra no projeto como canal rico e fallback (formatação, imagem, arquivo,
  funciona em qualquer device), não como solução pra "notificar fora de casa".
- APK sideloaded recebe FCM normalmente. Publicação na Play Store é irrelevante pra isso.

---

## Modelo de dados

Seis tabelas. O par `message` + `delivery` é o núcleo: a mensagem nasce uma vez e nunca
muda; cada destino gera uma linha de entrega com estado próprio.

**`topic`** — `name` (PK), `description`, `created_at`
Criado implicitamente no primeiro publish. Sem cadastro manual.

**`message`** — `id` (ULID, PK), `topic` (FK), `title`, `body`, `priority`, `tags`,
`click_url`, `created_at`
Imutável depois de gravada.

**`subscriber`** — `id` (PK), `kind`, `config` (JSON), `label`, `enabled`, `last_seen`
`kind` ∈ `telegram` | `fcm` | `webpush`. O `config` guarda o que cada canal precisa:
`chat_id` no Telegram, token no FCM, `endpoint` + `p256dh` + `auth` no Web Push.
Coluna JSON de propósito — nada de coluna por canal.

**`route`** — `id` (PK), `topic` (FK), `subscriber_id` (FK), `min_priority`, `enabled`
A regra de roteamento. É o que a dashboard gerencia.

**`delivery`** — `id` (PK), `message_id` (FK), `subscriber_id` (FK), `status`,
`attempts`, `last_error`, `sent_at`
`status` ∈ `pending` | `sent` | `failed`. Índice único em (`message_id`,
`subscriber_id`) garante idempotência.

**`message_read`** — `message_id` (FK), `subscriber_id` (FK), `read_at`
Leitura por device: ler no dash não marca como lido no celular.

---

## Roadmap

Cada fase termina em algo testável. Não avançar sem o critério de pronto.

### Fase 1 — Schema

Migração SQL e nada mais.

Testar inserindo registros à mão e rodando as queries que a dashboard vai precisar
depois: últimas N mensagens de um tópico, entregas falhadas, contagem de não lidas.

**Pronto quando:** as queries de leitura retornam o esperado com dados inseridos na mão.

### Fase 2 — Ingestão

`POST /:topico`, corpo da requisição = mensagem. Metadados em header, seguindo a
convenção do ntfy: `X-Title`, `X-Priority`, `X-Tags`, `X-Click`.

O handler grava a `message`, consulta as `route` que casam com tópico e prioridade, e
insere as linhas de `delivery` em `pending`. **Não envia nada ainda.**

**Pronto quando:** um `curl` gera a mensagem e as entregas pendentes corretas no banco.

### Fase 3 — Worker e a interface de canal

```go
type Notifier interface {
    Kind() string
    Send(ctx context.Context, sub Subscriber, msg Message) error
}
```

Loop que consome `delivery` pendente, chama o `Notifier` do `kind` correspondente e
atualiza status. Primeira implementação é um `logNotifier` que só imprime — valida
retry, backoff exponencial e transição de status sem depender de rede.

**Pronto quando:** entrega falha de propósito, tenta de novo com intervalo crescente e
para depois de N tentativas com o erro registrado.

### Fase 4 — Telegram

Implementação do `Notifier` via API de bot. Token do BotFather, `chat_id` no `config`
do subscriber.

**Pronto quando:** a notificação chega no celular a partir de um `curl` no Pi.

A partir daqui o backend está funcional. As fases seguintes são clientes.

### Fase 5 — Dashboard web

Página única servida pelo próprio binário. Lista mensagens, mostra entregas falhadas com
o erro, gerencia `route` e `subscriber`.

Depois: Web Push no navegador. Gera par de chaves VAPID, service worker se inscreve,
subscription vira um `subscriber` de `kind = webpush`.

**Pronto quando:** dá pra criar uma regra pela interface e ela passa a valer.

### Fase 6 — App Android

O objetivo de aprendizado do projeto. Duas rotas, decidir na hora:

- **FCM** — precisa de projeto Firebase + `google-services.json` batendo com o
  `applicationId`; o Pi dispara via API HTTP v1 com service account. Sem custo: FCM é
  gratuito nos dois planos, sem cobrança por mensagem.
- **Socket próprio** — foreground service segurando WebSocket contra o Pi. Zero Google,
  mas é briga com o ciclo de vida do Android.

**Pronto quando:** notificação chega com o app fechado e a tela apagada.

---

## Armadilhas conhecidas

**Assinante lento no broker.** Quando existir stream em tempo real, o canal de cada
assinante tem que ser buffered com descarte — assinante travado não pode segurar o
publish. E sempre um `select` no `r.Context().Done()` pra limpar quando a conexão cai.

**Streaming em Go.** Sem `w.(http.Flusher).Flush()` a cada mensagem, o Go bufferiza e o
cliente não recebe nada. Keepalive a cada ~45s ou proxy/NAT derruba a conexão ociosa.

**Android moderno.** `POST_NOTIFICATIONS` é permissão de runtime desde o 13; foreground
service precisa de tipo declarado no manifest desde o 14 (`dataSync` ou `specialUse`);
isenção de otimização de bateria é obrigatória ou o Doze mata o socket; e `BOOT_COMPLETED`
pra voltar depois do reboot.

**Instalação do APK.** A verificação de desenvolvedor do Google passa a valer no Brasil
em 30/09/2026. Instalação via `adb install` é isenta — usar esse caminho e não se
preocupar.

**Limites de payload.** FCM ~4 KB. Telegram 4096 caracteres por mensagem. Por isso o
push carrega resumo + ID, e o corpo completo é buscado no servidor.

**Service worker exige contexto seguro.** HTTPS com certificado válido, e a exceção de
`localhost` não vale no celular. Daí o `tailscale cert`.

---

## Fora de escopo por enquanto

Anexos e upload de arquivo. Autenticação multiusuário (é uso pessoal — um bearer token
fixo resolve). E-mail e Discord como canais. Federação ou qualquer coisa distribuída.
Retenção configurável por tópico — começar com um valor fixo e um job de limpeza.

---
## Tasks

Existe um roadmap mais para servir como guia mas irei criar uma lista de task para tocarmos com a ideia de como quero prosseguir com o processo. Irei adicionando mas task conforme for pensando nelas

[x] Criação repositorio
	Criar base de projeto pastas que vão ser utilizadas, docker e afins. Vamos usar golang
[x] Listar casos de testes para cada features
	Basicamente ideia é antes de começar escrever o código em se montar um documento com casos de testes para cada frente que vai servir como uma garantia do funcionamento do projeto
[x] TDD
	Escrever os casos de testes listados acima para conseguirmos usar como garantia que ira funcionar tudo certinho. Com chamadas externas mockada para evitar gastos nos teste unitários e afins
[x] Spikes banco
	Criação dos schemas e testes se banco ira funcionar para oq esperamos e testes que podemos considerar que banco postgres não sera um impeditivo
[x] Criação endpoint de ingestão
	Seguir oq foi falado no primeiro passo do roadmap
[x] Work e interface de canal
	Seguir oq foi falado no segundo passo do roadmap
[x] Integração com telagram
	Fazer integração com telegram para disparar a notificação

### Front

[x] Criação do designer para o front
  Analisar como vamos criar o designer do front para poder utilizar claude para isso. Talvez o claude designer de recomendações
[x] Spike se faz sentido fazer tdd para o front
  Caso faça sentido vamos adicionar mais tasks aqui
[x] Escolha de tecnologia para front. Pensando em deixar proprio go renderizar o front
[x] Preparar Go pra renderizar o front (templates, embed, rotas base, CSS — htmx entra na tela Visão geral, onde tem uso real)
[x] Tela: Visão geral
[x] Tela: Mensagens
[x] Tela: Entregas falhadas
[x] Tela: Rotas
[x] Tela: Assinantes

### Deploy

[x] Dockerfile de produção (build multi-stage, arm64)
[x] docker-compose de produção (rede homelab_net externa, aponta pro Postgres global)
[x] Build local + push manual pro GitHub Container Registry
[x] Subir no Portainer do Pi e validar ponta a ponta