# IMDTravel

Desenvolvido por:
* Antonio Walter Araújo Filho
* André Luiz de Sena Liberato
* Pedro de Andrade Cursino

Este projeto é a implementação da versão "Baseline" do sistema IMDTravel, um sistema de microsserviços para compra de passagens aéreas.
O objetivo é implementar a versão básica do sistema, com foco na comunicação entre os serviços via API REST e na execução de cada serviço em contêineres Docker distintos.

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

## 🛠️ Tecnologias Utilizadas

* **Linguagem:** Go (versão 1.25)
* **Comunicação:** API REST 
* **Contêineres:** Docker e Docker Compose 

## 🚀 Como Executar o Sistema

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
## Endpoints
- GET /health  
  Retorna 200 com `{"status":"healthy"}`

- POST /buyTicket  
  Recebe JSON:
  ```json
  {
    "flight": "FL123",
    "day": "2025-11-01",
    "user": "user-id"
  }
  ```
  Fluxo:
  1. Consulta voo em AIRLINESHUB (`/flight?flight=...&day=...`)
  2. Consulta taxa de câmbio em EXCHANGE (`/convert`) com timeout 1s
  3. Registra venda em AIRLINESHUB (`/sell`)
  4. Registra bônus em FIDELITY (`/bonus`)

  Resposta de sucesso (200):
  ```json
  {
    "success": true,
    "message": "Ticket purchased successfully",
    "transaction_id": "tx-id",
    "flight": "FL123",
    "day": "2025-11-01",
    "value_usd": 200.0,
    "value_brl": 1000.0,
    "exchange_rate": 5.0,
    "bonus_points": 200
  }
  ```
  Em erro retorna `success: false` e campo `error`.

## Exemplos curl
Health:
```bash
curl http://localhost:8080/health
```

Comprar passagem:
```bash
curl -X POST http://localhost:8080/buyTicket \
  -H "Content-Type: application/json" \
  -d '{"flight":"FL123","day":"2025-11-01","user":"user-id"}'
```