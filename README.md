# Sistema de Temperatura por CEP

Este projeto implementa dois serviços em Golang com tracing distribuído via OTEL e Zipkin.

## Serviços

- **Serviço A:** Recebe um CEP via POST (ex.: `{ "cep": "29902555" }`), valida e encaminha para o Serviço B.
- **Serviço B:** Consulta o viaCEP para obter a cidade e, em seguida, a WeatherAPI para obter a temperatura em Celsius. Realiza as conversões para Fahrenheit e Kelvin e retorna um JSON no seguinte formato:
  ```json
  {
    "city": "São Paulo",
    "temp_C": 28.5,
    "temp_F": 83.3,
    "temp_K": 301.5
  }

## Configuração
- Configure a variável de ambiente `WEATHER_API_KEY` no serviço B (em `docker-compose.yml`) com sua chave da
`WeatherAPI`.
- Se necessário, ajuste as variáveis `ZIPKIN_ENDPOINT` e `SERVICE_B_URL`.

## Como Executar
No diretório raiz do projeto, execute:
```bash
  docker-compose up --build
```

Os serviços serão iniciados:
- Serviço A: http://localhost:8080/cep
- Serviço B: http://localhost:8081/cep
- Zipkin UI: http://localhost:9411

## Testando a Aplicação
Envie uma requisição POST para o Serviço A:
```bash
  curl -X POST http://localhost:8080/cep \
  -H "Content-Type: application/json" \
  -d '{ "cep": "29902555" }'
```

- Se o CEP for válido, o serviço encaminhará a requisição para o Serviço B, que retornará a cidade e as temperaturas
em Celsius, Fahrenheit e Kelvin.
- Se o CEP for inválido, o serviço retornará um erro 422 com a mensagem `"invalid zipcode"`.

## Observações
- A aplicação utiliza OTEL e Zipkin para tracing distribuído entre os serviços.
- Cada etapa (consulta ao viaCEP e à WeatherAPI) é medida por spans para monitoramento do tempo de resposta.
