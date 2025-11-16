# IMDTravel

Desenvolvido por:
* Antonio Walter Araújo Filho
* André Luiz de Sena Liberato
* Pedro de Andrade Cursino

Este projeto é a implementação da versão "Baseline" + "COMFALHAS" + TOLERANTE do sistema IMDTravel, um sistema de microsserviços para compra de passagens aéreas.
O objetivo é implementar a versão básica do sistema, com foco na comunicação entre os serviços via API REST e na execução de cada serviço em contêineres Docker distintos.

## Versões do Sistema

### BASELINE (Parte 1)
Sistema básico funcionando sem falhas

### COMFALHAS (Parte 2)
Sistema com simulação de falhas implementadas

### TOLERANTE (Parte 3)
Sistema com mecanismos de tolerância a falhas

## 🏛️ Arquitetura do Sistema

O sistema é composto por quatro microsserviços, orquestrados pelo `docker-compose.yml`, que seguem o fluxo de compra definido na especificação:

1.  **IMDTravel (`:8080`)**
    * **Função:** O serviço principal (orquestrador) que atua como a fachada (façade) do sistema.
    * **Fluxo:** Recebe a requisição de compra (`/buyTicket`) , consulta o voo no `AirlinesHub` , busca a taxa de câmbio no `Exchange` , registra a venda no `AirlinesHub`  e, por fim, registra os pontos de bônus no `Fidelity`.
    * **Arquivo:** `imdtravel/main.go`

2.  **AirlinesHub (`:8081`)**
    * **Função:** Simula o sistema de uma companhia aérea, gerenciando voos e vendas.
    * **Endpoints:** `/flight` (para consulta de voos)  e `/sell` (para registrar uma venda).
    * **Arquivo:** `airlineshub/main.go`

3.  **Exchange (`:8082`)**
    * **Função:** Fornece taxas de câmbio de Dólar (USD) para Real (BRL).
    * **Endpoint:** `/convert` , que retorna um valor aleatório para a conversão.
    * **Arquivo:** `exchange/main.go`

4.  **Fidelity (`:8083`)**
    * **Função:** Gerencia o programa de pontos de fidelidade dos usuários.
    * **Endpoint:** `/bonus` (para registrar novos bônus)  e `/points` (para consultar pontuação).
    * **Arquivo:** `fidelity/main.go`

## Tecnologias Utilizadas

* **Linguagem:** Go (versão 1.25)
* **Comunicação:** API REST 
* **Contêineres:** Docker e Docker Compose 

## Como Executar o Sistema

### Pré-requisitos

* Docker
* Docker Compose (preferencialmente V2, que usa o comando `docker compose` sem hífen)

### Instruções

1.  Clone este repositório (ou certifique-se de ter todos os arquivos nas pastas corretas).
2.  Navegue pelo terminal até a pasta raiz do projeto (o diretório que contém o arquivo `docker-compose.yml`).
3.  Execute o comando abaixo para construir as imagens (se ainda não existirem) e iniciar todos os serviços em modo "detached" (em segundo plano):

    ```bash
    docker compose up -d --build
    ```

4.  O sistema estará pronto. Os serviços estarão disponíveis nas portas `8080` (IMDTravel), `8081` (AirlinesHub), `8082` (Exchange) e `8083` (Fidelity).
## 🔌 Endpoints da API

### 1. Health Check
Verifica se o serviço IMDTravel está operante. Útil para o Docker Compose e healthchecks de infraestrutura.

* **URL:** `/health`
* **Método:** `GET`
* **Sucesso (200 OK):**
    ```json
    {
      "status": "healthy"
    }
    ```

### 2. Comprar Passagem (`/buyTicket`)
Endpoint principal que orquestra todo o fluxo de compra: consulta o voo, converte a moeda, efetua a venda e registra os pontos de fidelidade.

* **URL:** `/buyTicket`
* **Método:** `POST`
* **Corpo da Requisição (JSON):**

| Campo | Tipo | Obrigatório | Descrição |
| :--- | :--- | :--- | :--- |
| `flight` | `string` | Sim | Código do voo (ex: "AA123"). |
| `day` | `string` | Sim | Data do voo (ex: "2025-11-15"). |
| `user` | `string` | Sim | ID do usuário comprador. |
| `ft` | `boolean` | Não | **Flag de Tolerância a Falhas**. Se `true`, ativa as estratégias de tolerância a falhas. |

**Exemplo de Request:**
```json
{
  "flight": "AA123",
  "day": "2025-11-15",
  "user": "walter_filho",
  "ft": true
}
```

#### Respostas Possíveis

**✅ 200 OK - Compra Realizada com Sucesso**
Retornada quando todo o fluxo funciona. Se o sistema de fidelidade falhar mas `ft=true`, o `bonus_status` será "pending".

```json
{
  "success": true,
  "message": "Ticket purchased successfully",
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "flight": "AA123",
  "day": "2025-11-15",
  "value_usd": 500.00,
  "value_brl": 2650.50,
  "exchange_rate": 5.301,
  "bonus_points": 500,
  "bonus_status": "processed" 
}
```

**⚠️ 503 Service Unavailable - Falha Graciosa (Latência/Rede)**
Ocorre quando `ft=true` e o serviço de vendas (AirlinesHub) demora mais de 2 segundos para responder (Timeout Protection) ou está indisponível.

```json
{
  "success": false,
  "error": "o sistema de vendas está instável no momento devido à alta latência. Por favor, tente novamente em alguns instantes"
}
```

**❌ 500 Internal Server Error**
Ocorre em falhas críticas de dependências quando a tolerância a falhas está desligada (`ft=false`).

```json
{
  "success": false,
  "error": "Failed to get flight info: request failed: ..."
}
```

**❌ 400 Bad Request**
Ocorre quando campos obrigatórios estão faltando no JSON enviado.

```json
{
  "success": false,
  "error": "Missing required fields: flight, day, user"
}
```

## Simulação de Falhas (Tolerância a Falhas)

A especificação `Fail (Type, Probability, Duration)` foi implementada da seguinte maneira:

### Lógica de Implementação

Para falhas com `Duration` (Duração) maior que zero (como `Error` e `Time`), a implementação é *stateful* (com estado):

1.  **Probability (Probabilidade):** É a chance (ex: 10%) de uma requisição *ativar* o estado de falha do serviço.
2.  **Duration (Duração):** Uma vez ativado, o serviço permanece em "estado de falha" pelo tempo especificado (ex: 10 segundos).
3.  **Type (Tipo):** Representa o *efeito* que será aplicado a **todas** as requisições que chegarem ao serviço *enquanto* ele estiver no "estado de falha" (ex: atrasar 5s).

Para falhas com `Duration` zero ou não definida (como `Omission` e `Crash`), a implementação é *stateless* (sem estado), e o efeito é aplicado apenas na requisição que ativou a probabilidade.

### Detalhamento por Requisição

* **Request 1: `Fail (Omission, 0.2, 0s)`**
    * **Local:** `airlineshub/main.go` (no endpoint `/flight`).
    * **Implementação:** *Stateless*. Há 20% de chance de a requisição simplesmente não responder (um `return` sem escrita de resposta), simulando a omissão.

* **Request 2: `Fail (Error, 0.1, 5s)`**
    * **Local:** `exchange/main.go` (no endpoint `/convert`).
    * **Implementação:** *Stateful*. Há 10% de chance de ativar um estado de falha que dura **5 segundos**. Durante esse período, todas as requisições ao `/convert` retornam imediatamente um `HTTP 500` (Erro).

* **Request 3: `Fail (Time=5s, 0.1, 10s)`**
    * **Local:** `airlineshub/main.go` (no endpoint `/sell`).
    * **Implementação:** *Stateful*. Há 10% de chance de ativar um estado de falha que dura **10 segundos**. Durante esse período, todas as requisições ao `/sell` sofrem um atraso (efeito `Time`) de **5 segundos** antes de serem processadas.

* **Request 4: `Fail (Crash, 0.02, _)`**
    * **Local:** `fidelity/main.go` (no endpoint `/bonus`).
    * **Implementação:** *Stateless*. Há 2% de chance de o serviço forçar um `os.Exit(1)`, simulando um Crash. O `docker-compose.yml` está configurado com `restart: always` para que o contêiner reinicie automaticamente.

## Mecanismos de Tolerância Implementados

### Request 1: Consulta de Voo (Retry Pattern)
**Problema:** O serviço AirlinesHub pode sofrer de "Omissão" (não responder) ou falhas transientes de rede.

**Solução:** Implementação do padrão de **Retentativa (Retry)**.
1.  **Detecção:** O sistema detecta erros de rede ou timeouts na conexão.
2.  **Estratégia:** Caso a primeira tentativa falhe e a flag `FT` esteja ativa, o sistema realiza até **3 novas tentativas** automaticamente.
3.  **Backoff:** Entre cada tentativa, existe uma pausa fixa de **500ms** (backoff simples) para evitar sobrecarregar o serviço instável.
4.  **Resultado:** Aumenta a chance de sucesso em falhas temporárias sem intervenção do usuário.

### Request 2: Conversão de Moeda (Fallback & Caching)
**Problema:** O serviço Exchange pode entrar em estado de erro (HTTP 500) ou não responder.

**Solução:** Implementação do padrão de **Fallback com Histórico em Memória**.
1.  **Cache:** O sistema mantém em memória um histórico das últimas **10 taxas de câmbio** obtidas com sucesso.
2.  **Fallback:** Se o serviço externo falhar (retornar erro ou timeout) e a flag `FT` estiver ativa, o sistema calcula a **média aritmética** das taxas armazenadas.
3.  **Continuidade:** A operação de compra continua utilizando essa taxa média estimada, evitando que a queda de um serviço auxiliar impeça a venda principal.

### Request 3: Venda de Passagem (Timeout & Fail Gracefully)
**Problema:** O serviço AirlinesHub pode apresentar alta latência (>5s), o que travaria a thread do orquestrador e a experiência do usuário.

**Solução:** Implementação de **Timeout Rígido** e **Falha Graciosa**.
1.  **Proteção de Latência:** O cliente HTTP foi configurado com um **timeout rígido de 2 segundos**. Se o serviço demorar mais que isso, a conexão é abortada imediatamente para liberar recursos do servidor.
2.  **Tratamento de Erro:** Diferente de um erro genérico (500), o sistema captura o timeout.
3.  **Falha Graciosa:** Retorna ao usuário uma mensagem amigável e semântica (HTTP 503 - Service Unavailable), informando: *"o sistema de vendas está instável no momento devido à alta latência"*, instruindo-o a tentar novamente mais tarde.

### Request 4: Bonificação (Async Queue & Eventual Consistency)
**Problema:** O serviço Fidelity pode sofrer um Crash fatal (encerrar o processo).

**Solução:** Implementação de **Processamento Assíncrono** e **Consistência Eventual**.
1.  **Retry Imediato:** Tenta registrar o bônus 3 vezes com backoff exponencial curto.
2.  **Fila em Memória:** Se todas as tentativas falharem, o bônus não é perdido; ele é adicionado a uma fila segura (`pendingBonuses`) em memória.
3.  **Desacoplamento:** A falha no bônus **não impede a venda**. O cliente recebe a confirmação de sucesso da compra imediatamente, com o status do bônus marcado como `"pending"`.
4.  **Reconciliação:** Uma *Goroutine* em background verifica a fila a cada 10 segundos e reprocessa as bonificações pendentes assim que o serviço Fidelity volta a ficar online.