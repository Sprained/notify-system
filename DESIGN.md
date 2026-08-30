# Design — notify-system (painel web)

Referência visual do dashboard, extraída do mockup aprovado em [Artifact](https://claude.ai/code/artifact/3d538bb4-8f40-4cfd-a7e8-48ddfd115301)
(salvo também em PDF fora do repo). Serve de guia pra implementação real do front —
qualquer mudança de paleta/tipografia deveria vir aqui antes de virar CSS.

## Paleta

Tema claro é o padrão do produto; escuro segue via `prefers-color-scheme`.

### Claro (padrão)

| Token | Hex | Uso |
| --- | --- | --- |
| `--bg` | `#F1F4F5` | fundo da página |
| `--surface` | `#FFFFFF` | cards, sidebar, tabelas |
| `--surface-sunken` | `#E9EDEF` | cabeçalho de tabela, hover de linha |
| `--border` | `#D8E0E3` | divisórias |
| `--border-strong` | `#BFC9CD` | bordas de botão/input |
| `--ink` | `#16222A` | texto principal |
| `--ink-muted` | `#56666E` | texto secundário |
| `--ink-faint` | `#8B9AA1` | labels, legendas |
| `--accent` | `#0E7C7B` | ação primária, nav ativa (teal) |
| `--accent-strong` | `#0A6362` | hover de ação primária |
| `--success` | `#1D8A57` | status `sent` |
| `--warning` | `#A9711A` | status `pending` |
| `--danger` | `#BF3327` | status `failed` |

### Escuro

| Token | Hex |
| --- | --- |
| `--bg` | `#0F161B` |
| `--surface` | `#161F25` |
| `--ink` | `#E6EDEF` |
| `--accent` | `#35C9C0` |
| `--success` | `#3FDC94` |
| `--warning` | `#E5AC4C` |
| `--danger` | `#F0776A` |

**Racional:** acento (teal) fica isolado das cores de status — nunca usar `--accent`
pra indicar pending/sent/failed, senão perde o contraste visual entre "ação" e "estado".

## Tipografia

- **IBM Plex Sans** — UI geral, texto corrido, títulos (400/500/600/700)
- **IBM Plex Mono** — tudo que é dado técnico de verdade: ULID, `chat_id` mascarado,
  timestamps, mensagens de erro, contadores. Não é decoração — é o que aparece na tela.
- Fonte via Google Fonts (`fonts.googleapis.com`), com fallback `ui-sans-serif`/`ui-monospace`.
- Escala: `12px` (legendas) → `13px` (tabela) → `14px` (base) → `16px` (seção) → `19px`
  (título de card) → `28px` (título de página/número de destaque).

## Layout

- Desktop-first: sidebar fixa (232px) + conteúdo. Sem menu hambúrguer — não é prioridade
  mobile pra essa v1.
- Uma página só (`single page`), navegação troca seção via JS, sem reload — bate com a
  decisão do `NOTIFY.md` de servir tudo pelo próprio binário Go.
- Status sempre como *chip* (pílula colorida), nunca só texto — precisa dar pra escanear
  a tela e notar o que está `failed` sem ler linha por linha.
- Tabela larga tem `overflow-x: auto` no próprio container, nunca o body inteiro rolando
  de lado.

## Ícones

SVG inline, geométricos, sem biblioteca externa (sem emoji nem ícone de fonte) — mesmo
espírito "operacional" do resto do painel.
