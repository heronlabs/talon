# vps-agent: agente que provisiona e mede uma VPS Easypanel a partir do CMS

Data: 2026-10-01. Estado: design aprovado em conversa; aguardando revisão desta spec. Destino: `docs/superpowers/specs/2026-10-01-vps-agent-design.md` do repositório novo `heronlabs/vps-agent`.

## Objetivo

Comprar uma VPS com Easypanel, instalar um binário por SSH uma única vez e, a partir daí, a VPS se monta e se reporta sozinha:

- o agente lê um manifesto do CMS (`cms.heronlabs.com.br`) e cria no Easypanel os projetos e serviços que faltam, com seus limites de memória e CPU;
- o agente mede cada serviço (memória, CPU, disco, tamanho do banco) e o host, e envia ao CMS em intervalo fixo;
- o admin mostra, por projeto e serviço, quanto do limite está em uso (ex.: `cms-heronlabs`: banco 80% da cota de disco, API 35% da memória, backup 1%).

Sucesso significa:

- uma VPS nova, com Easypanel e o agente instalados, recria todos os clientes cadastrados para ela no CMS sem cliques no painel, segredos inclusive: o agente os lê do SSM da conta AWS de cada cliente (ver _Segredos_);
- trocar de VPS é cadastrar o host novo, mover os clientes para ele no CMS, deixar o agente convergir, restaurar os dumps e virar o DNS;
- a cada 10 minutos o CMS recebe a medição de cada serviço de cada projeto e do host;
- uma versão do Easypanel diferente da suportada nunca recebe escrita do agente.

## Decisões tomadas

- **Projeto do zero**, repositório privado `heronlabs/vps-agent`. Nada do `terminal-cli`/`hcli`; ele fica só com o backup. Este agente substitui o "A2 PR 2" (`hcli agent`) do roadmap de `webapps`.
- **Go**, binário estático único, sem runtime no host. Roda no host como serviço systemd, não em container: lê `/proc`, o `docker.sock` e os volumes direto e fala com o Easypanel em `http://127.0.0.1:3000`.
- **Só conexões de saída.** O agente chama o CMS; nada chama o agente; nenhuma porta aberta.
- **Declarativo.** O manifesto é o estado desejado; o agente converge (reconcile). Ações pontuais (backup agora, restore) ficam fora, para um canal de comandos futuro.
- **O reconcile nunca apaga.** Cria projeto e serviço que faltam e ajusta limites. Remover é sempre manual. Serviço no Easypanel fora do manifesto é reportado como não gerenciado.
- **O reconcile não troca imagem nem env de serviço existente.** Isso é do CD de cada app (deploy hook). O agente só aplica imagem e env ao criar. Única exceção: chave de segredo ausente no env de um serviço existente é acrescentada, nunca sobrescrita.
- **Segredos ficam no SSM da conta AWS de cada cliente; o CMS guarda só referências.** O agente tem um usuário IAM próprio na conta heronlabs que só pode assumir o role `vps-agent-secrets-reader` de cada conta (a heronlabs inclusive), e lê os parâmetros com as credenciais temporárias do role.
- **Easypanel com versão fixada.** A API tRPC é interna e não documentada; o agente fala só com a versão que tem fixtures capturadas e testadas. Outra versão: o agente mede, mas não escreve.
- **Senha do banco vem do `DATABASE_URL` do SSM**, o mesmo que a app usa para conectar. Banco novo nasce com a credencial que a app já tem; nada muda na app ao recriar o banco numa VPS nova.
- **Tamanho do banco = disco do serviço de banco** (volume de dados medido no host). O agente nunca conecta no banco nem faz exec dentro de container.
- **Porcentagens no admin**, não no agente. O agente manda números; o CMS guarda; o admin divide pelo limite desejado.

## Arquitetura

```
VPS (host)                                               CMS (server-cms)
┌──────────────────────────────────────────┐            ┌────────────────────────────────┐
│ vps-agent (systemd, root)                │   GET      │ /api/infra-manifests/:host     │
│  ├─ reconcile (boot + a cada 5 min) ─────┼──────────▶ │   estado desejado do host      │
│  │    ├─ Easypanel tRPC 127.0.0.1:3000   │            │                                │
│  │    └─ AWS STS + SSM (por cliente) ─┐  │            │                                │
│  └─ report (boot + a cada 10 min) ────┼──┼──────────▶ │ /api/infra-reports             │
│       ├─ /proc, statfs(/)             │  │   POST     │   medições + estado do         │
│       └─ /var/run/docker.sock         │  │            │   reconcile por projeto        │
└───────────────────────────────────────┼──┘            └────────────────────────────────┘
                                        ▼                                ▲
┌──────────────────────────────────────────┐             client-admin lê o CMS (como hoje)
│ AWS de cada cliente (heronlabs inclusive)│
│  sts:AssumeRole vps-agent-secrets-reader │
│  ssm:GetParameters (lista do role)       │
└──────────────────────────────────────────┘
```

Os dois jobs rodam no mesmo processo e nunca ao mesmo tempo (um mutex). Um job que falha é logado, entra no próximo report e não derruba o processo. Só erro de configuração na partida sai com código 1.

## Contrato com o CMS

O CMS é dono dos dois contratos. Este repositório guarda uma cópia dos tipos (DTOs do adapter) em `internal/infrastructure/cms/` e fixtures JSON em `testdata/cms/`. A implementação no `webapps` é um PR separado (ver _Dependências no webapps_).

Autenticação: `Authorization: Bearer <AGENT_TOKEN>`, um API token custom do Strapi com só duas ações: ler manifesto e enviar report.

### `GET /api/infra-manifests/:host`

`:host` é o `name` do `infra-host`. Devolve os clientes do host com `lifecycle` `provisioning`, `active` ou `failed`; `suspended` não vem. `failed` vem de propósito: o agente tenta de novo a cada ciclo, e um `converged` devolve o cliente a `active` sem reset manual. Se `failed` saísse do manifesto, um erro passageiro do Easypanel tiraria o cliente do reconcile para sempre.

```json
{
  "data": {
    "host": "vps-heron-02",
    "clients": [
      {
        "slug": "cms-heronlabs",
        "lifecycle": "provisioning",
        "secretsRoleArn": "arn:aws:iam::088132334646:role/vps-agent-secrets-reader",
        "services": [
          {
            "name": "cms-database",
            "type": "postgres",
            "image": "postgres:17.6",
            "env": "",
            "secrets": [],
            "databaseUrl": {"ssm": "/server-cms/DATABASE_URL"},
            "domains": [],
            "resources": {"memoryReservationMiB": 256, "memoryLimitMiB": 512, "cpuReservation": 0.1, "cpuLimit": 0.5},
            "diskQuotaGB": 10
          },
          {
            "name": "cms-api",
            "type": "app",
            "image": "ghcr.io/heronlabs/server-cms:3.20.0",
            "env": "NODE_ENV=production\nAWS_REGION=us-east-1\nAWS_ENV_PATH=/server-cms/",
            "secrets": [
              {"key": "AWS_ACCESS_KEY_ID", "ssm": "/server-cms-deployer/CMS_DEPLOY_AWS_ACCESS_KEY_ID"},
              {"key": "AWS_SECRET_ACCESS_KEY", "ssm": "/server-cms-deployer/CMS_DEPLOY_AWS_SECRET_ACCESS_KEY"}
            ],
            "domains": [{"host": "cms.heronlabs.com.br", "port": 80}],
            "resources": {"memoryReservationMiB": 512, "memoryLimitMiB": 1024, "cpuReservation": 0.1, "cpuLimit": 1},
            "diskQuotaGB": 2
          }
        ]
      }
    ]
  }
}
```

- `type`: `app`, `postgres` ou `mysql`.
- `env`: linhas `KEY=VALUE` **sem segredo**. Marcador `$(...)` é do Easypanel e passa intacto (ex.: `$(PROJECT_NAME)`). `AWS_ENV_PATH` e `AWS_REGION` não são segredo e vão aqui. A app não recebe `DATABASE_URL` pelo env do Easypanel: lê do SSM no boot, como hoje.
- `secretsRoleArn`: role `vps-agent-secrets-reader` da conta AWS do cliente (a própria heronlabs quando o cliente vive nela). Obrigatório quando algum serviço tem `secrets`. O agente só aceita o formato `arn:aws:iam::<12 dígitos>:role/vps-agent-secrets-reader`. Campo explícito no CMS, não derivado de `awsAccounts`.
- `secrets`: pares `key` (nome da variável no env do serviço) e `ssm` (nome do parâmetro `SecureString` na conta do `secretsRoleArn`). Só referência, nunca valor. O nome da variável e o do parâmetro podem diferir (ex.: `AWS_ACCESS_KEY_ID` ← `/server-cms-deployer/CMS_DEPLOY_AWS_ACCESS_KEY_ID`). O agente valida só o formato do nome; quem decide o que pode ser lido é a policy do role (ver _Segredos_).
- `databaseUrl`: obrigatório em `postgres` e `mysql`, proibido em `app`. Referência (`ssm`) ao parâmetro com a connection string do banco: o próprio `DATABASE_URL` que a app do cliente lê, sem cópia (ex.: `/server-cms/DATABASE_URL`). Na criação, o agente extrai dele usuário, senha e nome do banco e exige:
  - esquema compatível com `type` (`postgres://` ou `postgresql://` para `postgres`, `mysql://` para `mysql`);
  - host igual a `<slug>_<name>`, o nome do serviço Swarm que a app resolve na rede do Easypanel;
  - porta ausente ou a padrão do motor (5432 ou 3306).
  Fora disso, o projeto fica `failed` com `invalid-database-url`, sem o valor na mensagem.
- `diskQuotaGB`: cota lógica, só para a porcentagem; o Docker não a impõe.
- Respostas: `200`; `403` sem token ou token sem a ação; `404` `unknown-infra-host`.

### `POST /api/infra-reports`

O contrato do A2 (`webapps/docs/superpowers/specs/2026-09-23-infra-host-report-design.md`, seção _Contrato_) com três acréscimos: o bloco `agent`, o bloco `reconcile` por projeto e `cpuLimit`/`managed` por serviço. O A2 PR 1 ainda não foi entregue, então o contrato muda antes de existir.

```json
{
  "host": "vps-heron-02",
  "measuredAt": "2026-10-01T14:10:00.000Z",
  "agent": {
    "version": "1.0.0",
    "easypanelVersion": "2.26.3",
    "easypanelSupported": true,
    "lastReconcile": {"at": "2026-10-01T14:05:00.000Z", "outcome": "ok"}
  },
  "system": {
    "ramTotalGiB": 7.75, "ramUsedGiB": 5.1,
    "swapTotalGiB": 4, "swapUsedGiB": 0.3,
    "diskTotalGB": 97, "diskUsedGB": 41.2,
    "load1": 0.42, "load5": 0.38, "load15": 0.35,
    "uptimeSeconds": 1209600,
    "dockerImagesGB": 6.1, "dockerBuildCacheGB": 2.4
  },
  "projects": [
    {
      "name": "cms-heronlabs",
      "reconcile": {"state": "converged", "missingSecrets": [], "error": null},
      "services": [
        {
          "name": "cms-database",
          "managed": true,
          "state": "running",
          "replicasRunning": 1, "replicasDesired": 1,
          "recentTaskFailures": 0,
          "memoryUsedMiB": 310, "memoryLimitMiB": 512,
          "cpuPercent": 3.2, "cpuLimit": 0.5,
          "diskGB": 8.1,
          "netRxMB": 120.5, "netTxMB": 88.1,
          "blockReadMB": 50.2, "blockWriteMB": 12.9
        }
      ]
    }
  ]
}
```

- `agent.lastReconcile.outcome`: `ok`, `skipped` (versão do Easypanel não suportada ou manifesto inacessível) ou `failed`; com `error` (mensagem curta, sem segredo) quando não é `ok`.
- `projects[].reconcile.state`:
  - `converged`: todos os serviços do manifesto existem, com os limites do manifesto, e toda chave de `secrets` está no env do serviço;
  - `pending`: falta segredo (`missingSecrets` lista as chaves cujo parâmetro não existe no SSM; `DATABASE_URL` quando é o do banco, que então não é criado) ou um serviço ainda não está `running`;
  - `failed`: uma chamada ao Easypanel falhou para esse projeto, o `AssumeRole` foi negado, o SSM negou acesso ou o manifesto trouxe `secretsRoleArn`/`ssm` fora do formato aceito ou um `DATABASE_URL` incompatível com o serviço (`error`);
  - `unmanaged`: projeto no Easypanel sem cliente no manifesto (só medido).
- `agent.lastReconcile` é `null` no primeiro report, se ele sair antes do primeiro reconcile terminar. `agent.easypanelVersion`, `agent.easypanelSupported` e `agent.lastReconcile` são `null` enquanto o agente não fala com o Easypanel (fases 1 e 2); o CMS aceita `null` nos três.
- O CMS deriva o `lifecycle`: `provisioning` ou `failed` + `converged` vira `active`; qualquer `lifecycle` + `failed` vira `failed` com o `error` como motivo. O agente nunca escreve `lifecycle` direto; a única escrita dele é este report.
- `cpuPercent` segue o `docker stats`: 100 é um core inteiro. A porcentagem de CPU do limite é `cpuPercent / 100 / cpuLimit`.
- `diskGB` do serviço: volumes nomeados montados (tamanho de `/system/df`) mais bind mounts sob `/etc/easypanel/projects/<projeto>/<serviço>/`. No serviço de banco, é o tamanho do banco.
- O report nunca carrega env, valores de segredo, labels, nomes de imagem, nomes de volume nem caminhos.

## Reconcile

A cada execução (boot, depois a cada `RECONCILE_INTERVAL`, padrão 5 min):

1. Lê a versão do Easypanel. Diferente de `SupportedEasypanelVersion` (constante em `internal/infrastructure/easypanel/version.go`): `skipped`, nada é escrito, o report segue.
2. `GET` do manifesto. Falha de rede ou 5xx: `skipped`, o agente não faz nada com estado velho.
3. Lê o estado atual: `projects.listProjectsAndServices` e o `inspectService` de cada serviço.
4. Calcula o plano, função pura em `internal/core/domain/`, a partir de manifesto mais estado atual:
   - `CreateProject(slug)` se o projeto não existe;
   - `CreateService(slug, service)` se o serviço não existe, com todas as chaves de `secrets` e, no banco, a referência `databaseUrl`; bancos antes de apps, para a app subir com o banco de pé;
   - `UpdateResources(slug, service, resources)` se os quatro números de `resources` diferem;
   - `AddSecrets(slug, service, keys)` se o serviço existe e alguma chave de `secrets` não está no env dele;
   - nada para imagem, env ou valor de segredo diferentes de serviço existente;
   - nunca `Delete`.
   O plano carrega só nomes de chave e de parâmetro, nunca valor; por isso `plan` não precisa de AWS.
5. Aplica o plano na ordem, por projeto. Se o plano do projeto tem `CreateService` ou `AddSecrets` com segredos, ou `CreateService` de banco, antes de escrever no Easypanel:
   1. valida o formato de `secretsRoleArn`, de cada `ssm` e do `databaseUrl.ssm`; fora do formato, o projeto fica `failed`;
   2. chama `sts:AssumeRole` no `secretsRoleArn` com `ExternalId` igual ao slug, sessão `vps-agent-<host>` e duração de 15 min;
   3. busca só os parâmetros do plano com `ssm:GetParameters` (`WithDecryption`) e descarta as credenciais temporárias;
   4. converte cada `DATABASE_URL` em usuário, senha e nome do banco, com as regras de `databaseUrl` (ver _Contrato_);
   5. parâmetro de app inexistente vai para `missingSecrets` e o serviço é criado sem aquela chave; `DATABASE_URL` inexistente vai para `missingSecrets` e o banco não é criado; `AccessDenied` no STS ou no SSM marca o projeto `failed`.
   Um erro marca aquele projeto `failed` e passa para o próximo. Os valores vivem só na memória do ciclo: nunca vão para log, report, plano ou disco.
6. Depois de aplicar, relê o env de cada serviço com `secrets` e confere a presença das chaves (só a presença, nunca o valor). Chave ausente deixa o projeto `pending` com `missingSecrets`.

Mapeamento para a API do Easypanel (nomes de procedure confirmados na captura da _Fase 0_):

| Ação | Procedure |
|---|---|
| criar projeto | `projects.createProject` |
| criar serviços (primário) | `templates.createFromSchema` com `{projectName, schema: {services: [...]}}` contendo só os serviços que faltam. É o único caminho verificado que aceita o `data` completo do `templateSchema` (imagem, env, domínios, `resources`, senha do banco), o mesmo do botão "Create from Schema" |
| criar serviços (fallback) | `services.<tipo>.createService` com `{projectName, serviceName}` mais `services.app.updateSourceImage`, `services.app.updateEnv`, `domains.*` e `services.<tipo>.updateResources`, se a _Fase 0_ mostrar que `createFromSchema` não serve para adicionar serviço a projeto existente |
| usuário, senha e nome do banco novo | extraídos do `DATABASE_URL` do SSM e postos no `data` do serviço de banco (nomes dos campos confirmados na _Fase 0_) |
| ajustar limites | `services.<tipo>.updateResources` |
| acrescentar segredo a serviço existente | `services.app.updateEnv`: lê o env atual pelo `inspectService`, acrescenta só as linhas `KEY=VALUE` ausentes, mantém o resto intacto e regrava; se a _Fase 0_ mostrar que o env só vale depois de deploy, chama o deploy do serviço |

`vps-agent plan` imprime o plano sem aplicar. É a ferramenta para validar uma versão nova do Easypanel ou um manifesto novo.

## Segredos

- O manifesto não carrega segredo, só referências.
- Senha do banco: o agente não gera senha. O serviço de banco nasce com usuário, senha e nome do banco do `DATABASE_URL` que já está no SSM, o mesmo que a app lê no boot. Banco recriado numa VPS nova e restaurado de dump aceita a app sem mudança nenhuma nela. A senha só vale na criação: as imagens do Postgres e do MySQL só a aplicam ao iniciar um volume vazio, então trocar o parâmetro depois não muda a senha de um banco existente. Rotação de senha de banco é manual (`ALTER USER` mais o parâmetro) e fica fora da v1.
- Os parâmetros ficam como estão hoje no SSM Parameter Store (`SecureString`) da conta AWS de cada cliente; nada é copiado nem movido para o agente. O agente lê três tipos:
  - o id e o secret do usuário IAM que a API usa para carregar o próprio env (ex.: `/server-cms-deployer/*`, que só lê `parameter/server-cms/*`);
  - o id e o secret do usuário IAM do backup do `terminal-cli` (ex.: `/server-cms-backup/*`);
  - o `DATABASE_URL` da app, só para criar o banco (ex.: `/server-cms/DATABASE_URL`).
  Esses usuários e parâmetros já são criados pelo `UserModule` e pelo `ParameterModule` do `cloud-iac`. O resto do env sensível da app continua onde já está: a app carrega do SSM no boot com `env-ssm` e `AWS_ENV_PATH`.
- Identidade do agente: um usuário IAM `vps-agent-<host>` por VPS, na conta heronlabs (`088132334646`). A policy dele tem uma permissão só: `sts:AssumeRole` em `arn:aws:iam::*:role/vps-agent-secrets-reader`. Ele não lê SSM direto, nem na própria heronlabs.
- Em cada conta que hospeda cliente, a heronlabs inclusive, existe o role `vps-agent-secrets-reader`:
  - permissão: `ssm:GetParameters` numa lista explícita de ARNs por cliente, nunca um prefixo amplo. Para o `cms-heronlabs`:

    ```yaml
    resources:
      - 'arn:aws:ssm:us-east-1:088132334646:parameter/server-cms/DATABASE_URL'
      - 'arn:aws:ssm:us-east-1:088132334646:parameter/server-cms-deployer/*'
      - 'arn:aws:ssm:us-east-1:088132334646:parameter/server-cms-backup/*'
    ```

    Do `/server-cms/` só o `DATABASE_URL`: o resto do env da API fica fora do alcance do agente. Todos os parâmetros usam a chave gerenciada `aws/ssm`, cuja key policy já libera o decrypt via SSM; o role não tem permissão de KMS. Chave KMS própria fica fora da v1;
  - trust policy: `Principal: {"AWS": "arn:aws:iam::088132334646:root"}` com `ArnLike aws:PrincipalArn = arn:aws:iam::088132334646:user/vps-agent-*` (a AWS não aceita wildcard direto em `Principal`) e `StringEquals sts:ExternalId` igual ao slug do cliente daquela conta (lista, se a conta tem mais de um cliente);
  - duração máxima de sessão de 1 h; o agente pede 15 min.
- A autorização exige os dois lados: a policy do usuário libera o assume e a trust do role aceita. Mesma regra dentro da heronlabs e entre contas.
- `ExternalId` é uma string que quem chama `AssumeRole` é obrigado a mandar e que a trust policy confere. Aqui ele é o slug do cliente no CMS (ex.: `cms-heronlabs`): a trust do role exige esse valor e o agente manda o `slug` do cliente que está reconciliando, lido do manifesto. Não é segredo; impede que o manifesto de um cliente aponte para o role de outro, porque a trust da outra conta exige outro slug. Slug renomeado no CMS sem atualizar `externalIds` no tenant faz o `AssumeRole` ser negado: o projeto fica `failed` e o `doctor` acusa.
- Entre contas, o `ExternalId` isola os clientes. Dentro de uma mesma conta com mais de um cliente, o role é um só e a lista dele soma os parâmetros de todos; o isolamento entre esses clientes depende só de quem edita o manifesto no CMS. Hoje cada conta tem um cliente; se uma conta passar a ter vários e isso importar, o caminho é um role por cliente (`vps-agent-secrets-reader-<slug>`), cada um com a sua lista.
- O que vaza em cada caso:
  - `AGENT_TOKEN` (CMS) vazado: só nomes de parâmetro e ARNs de role, nenhum valor;
  - access key do `vps-agent-<host>` vazada: os parâmetros listados nos roles das contas que confiam no padrão `vps-agent-*`. Equivale a perder o host, cujo Easypanel já guarda esses valores em texto puro. Revogar é desativar a key na heronlabs, sem tocar nas contas dos clientes;
  - edição maliciosa no CMS: pode criar, num cliente, um serviço com imagem própria e apontar para segredo do mesmo cliente. É o mesmo risco que a imagem vinda do manifesto já tem (ver _Fora de escopo_).
- Rotação: o agente nunca sobrescreve valor presente. Para rotacionar na v1, o operador atualiza o parâmetro no SSM e apaga a chave no painel; o reconcile seguinte grava o valor novo.
- Configuração local em `/etc/vps-agent/config.env`, `root:root 0600`: `CMS_URL`, `AGENT_TOKEN`, `AGENT_HOST_NAME`, `EASYPANEL_URL` (padrão `http://127.0.0.1:3000`), `EASYPANEL_TOKEN`, `AWS_ACCESS_KEY_ID` e `AWS_SECRET_ACCESS_KEY` (do `vps-agent-<host>`), `AWS_REGION` (padrão `us-east-1`), `RECONCILE_INTERVAL` (padrão `5m`), `REPORT_INTERVAL` (padrão `10m`). O agente lê as chaves AWS só desse arquivo, nunca de `~/.aws` nem de metadata de instância. Formato `KEY=VALUE`, uma por linha; linhas vazias e `#` ignoradas; espaços em volta da chave e do valor, `\r` de fim de linha e um par de aspas duplas em volta do valor são descartados; chave desconhecida ou repetida recusa o arquivo. Cada subcomando valida só o que usa: `report --once` não exige nada do CMS, do Easypanel nem da AWS; `plan` exige CMS e Easypanel; `run` exige tudo.

## Medição

Igual ao A2, sem dependência nova:

- host: `/proc/meminfo` (swap ausente vale 0, nunca `NaN`), `/proc/loadavg`, `/proc/uptime`, `statfs("/")`;
- Docker Engine API pelo socket, com `net/http` e `DialContext` unix, sem SDK: `/services`, `/tasks`, `/containers/json`, `/containers/{id}/stats?stream=false` (em paralelo, com timeout por container), `/system/df`;
- agrupamento pelo label `com.docker.swarm.service.name` = `<projeto>_<serviço>`. A regra foi confirmada pelo template Umami do Easypanel (`$(PROJECT_NAME)_<serviço>`) e é revalidada na _Fase 0_;
- container sem esse label é ignorado; serviço sem container rodando sai como `missing`, com as falhas contadas; um `stats` que estoura o timeout zera aquele serviço e o report segue;
- o estado do serviço é o mais vivo dos seus containers (`running` > `restarting` > `paused` > `created` > `exited` > `dead` > `missing`); o estado `removing` do Docker conta como `exited` e qualquer estado desconhecido como `dead`, para o CMS nunca receber valor fora do contrato;
- memória e CPU seguem as fórmulas do `docker stats` (cache inativo descontado, chave do cgroup v1 antes da v2; 100% é um core);
- bind mount de arquivo único sob a pasta do serviço conta o tamanho do arquivo;
- o host precisa ser manager do Swarm: fora dele, `/services` responde 503 e o report falha com o status no erro.

## Layout do repositório

```
cmd/vps-agent/main.go                 app: flags, sinal, monta o grafo e despacha o subcomando
internal/core/domain/                 entidades, value objects, ações do plano, planner e montagem do report (puros)
internal/core/port/                   interfaces: Easypanel, Docker, HostFS, CMS, SecretStore, Clock, Ticker
internal/core/usecase/reconcile/      versão, manifesto, estado, plano, segredos, aplicação
internal/core/usecase/plan/           o mesmo até o plano, sem aplicar
internal/core/usecase/report/         coleta host e Docker, monta e envia
internal/core/usecase/doctor/         checagens em ordem
internal/core/usecase/agent/          loop do run: intervalos, mutex, isolamento de falha
internal/infrastructure/easypanel/    cliente tRPC, versão suportada, DTOs
internal/infrastructure/docker/       Engine API pelo unix socket
internal/infrastructure/hostfs/       /proc, statfs, tamanho de diretório
internal/infrastructure/cms/          GET manifesto, POST report, DTOs do contrato
internal/infrastructure/aws/          STS AssumeRole e SSM GetParameters (aws-sdk-go-v2, só sts e ssm)
internal/infrastructure/config/       leitura e validação do config.env
internal/testkit/                     asserções (Equal, ErrorIs), builders e must...
internal/testkit/fake/                fakes escritos à mão, um por port
internal/tools/testshape/             analisador do lint: um cenário e uma asserção por teste
test/integration/                     testes de integração (build tag integration)
testdata/easypanel/<versão>/          fixtures capturadas da versão suportada
testdata/cms/                         fixtures do contrato
scripts/install.sh                    instalador/atualizador
deploy/vps-agent.service              unit systemd
.githooks/pre-commit
Makefile
```

- Direção dos imports, imposta pelo `depguard` do `golangci-lint` (o papel do `dep:cruise` dos repositórios Node):

  ```
  cmd ──▶ usecase ──▶ port ──▶ domain
    └───▶ infrastructure ──────┘ (domain + port)
  ```

  `domain` não importa nada interno; `port` só `domain`; `usecase` só `port` e `domain`; `infrastructure` só `domain` e `port`, nunca `usecase`; só o `cmd` enxerga tudo. A `infrastructure` importa `port` apenas para a checagem em tempo de compilação `var _ port.Easypanel = (*Client)(nil)`.
- Um módulo Go só (`github.com/heronlabs/vps-agent`), com um `go.mod` na raiz.
- CLI com o `flag` da biblioteca padrão, um `FlagSet` por subcomando; sem `cobra`.
- Subcomandos: `vps-agent run` (o serviço), `vps-agent plan`, `vps-agent report --once` (coleta e imprime, sem enviar), `vps-agent doctor`, `vps-agent version`.
- `doctor` verifica, em ordem e parando no primeiro erro: config válida, `docker.sock` acessível, Easypanel alcançável e na versão suportada, token do Easypanel aceito, CMS alcançável, token aceito e host conhecido (manifesto `200`), chaves AWS aceitas (`sts:GetCallerIdentity` devolve `user/vps-agent-*`) e, para cada cliente do manifesto com `secretsRoleArn`, `AssumeRole` aceito. O `doctor` não lê parâmetro nenhum.
- Logs: JSON em stdout (journald), `log/slog`, com o padrão `logId` da org (`<domínio>.<evento>`, campos planos, erro com `errorName`/`errorMessage` redigidos, nunca token nem valor de env).

## Convenções de código

Valem para todo o código Go e vão para o `AGENTS.md`.

### Arquitetura

- **Três camadas.** `cmd/vps-agent` é a app: lê flags, trata sinal, monta o grafo e chama um use case, sem regra nenhuma (fica fora da cobertura). `internal/core` tem as entidades e regras puras (`domain`), as interfaces dos recursos externos (`port`) e os casos de uso (`usecase`). `internal/infrastructure` são os adapters dos recursos externos: Easypanel, Docker, CMS, AWS, `/proc` e o `config.env`.
- **Ports pequenos e compartilhados.** Cada port tem de um a três métodos e mora em `core/port`, porque mais de um use case consome o mesmo recurso (o Easypanel serve `reconcile`, `plan` e `doctor`).
- **Domínio puro.** `core/domain` não faz I/O, não lê relógio nem gera aleatório: recebe `now time.Time` como valor. Os use cases é que usam os ports `Clock` e `Ticker`; o loop do `agent` recebe os intervalos por `Ticker`, e o teste controla o tempo sem `sleep`.
- **Injeção por construtor.** Toda dependência entra pelo construtor (`NewX(deps...) *X`). O único lugar que monta o grafo é `cmd/vps-agent/main.go`. Sem variável global mutável, sem `init()`, sem singleton.
- **DTO não vaza.** Tipos do tRPC do Easypanel, da Docker Engine API, do Strapi e do SDK da AWS ficam no adapter; um mapper converte para os tipos do `core` na fronteira.
- **Value objects com construtor que valida.** `Slug`, `ServiceName`, `SecretsRoleArn`, `SSMPath`, `DatabaseURL` e `Resources` só existem válidos (`NewSSMPath(raw) (SSMPath, error)`). A validação de formato mora neles, não espalhada nos chamadores.
- **Tipo `Secret`.** Valor de segredo, `DATABASE_URL` e a senha extraída dele são `Secret`, cujos `String`, `GoString`, `MarshalJSON` e `LogValue` (de `slog.LogValuer`) devolvem `[REDACTED]`. O valor real só sai por um método explícito, usado apenas no adapter do Easypanel.

### Padrões de projeto

- **Adapter:** Easypanel, Docker, CMS, STS/SSM e `/proc` atrás de ports.
- **Strategy:** um `ServiceKind` por `type` (`app`, `postgres`, `mysql`) sabe montar o `data` do schema, as procedures `services.<tipo>.*` e como o `DATABASE_URL` vira usuário, senha e nome do banco. Tipo novo é uma strategy nova, sem `switch` espalhado.
- **Command:** cada ação do plano (`CreateProject`, `CreateService`, `UpdateResources`, `AddSecrets`) é um valor imutável do `domain`; o use case `reconcile` aplica cada um pelo port do Easypanel.
- **Builder** só nos testes, para montar manifesto, estado do Easypanel e amostras do Docker com o mínimo de ruído.

### Boas práticas

- `context.Context` é o primeiro parâmetro de tudo que faz I/O; o timeout é de quem chama.
- Erros sobem embrulhados com `%w` e contexto (`fmt.Errorf("create service %s: %w", name, err)`). O `core` expõe erros sentinela (`ErrInvalidSecretPath`) e tipos de erro que os use cases classificam com `errors.Is`/`errors.As` em `pending` ou `failed`. `panic` só no `main`.
- Nomes no idioma do Go: pacote no singular e sem `utils`/`common`/`helpers`, sem repetir o nome do pacote no tipo (`reconcile.Plan`, não `reconcile.ReconcilePlan`), receiver curto.
- Sem código comentado, sem `TODO` sem issue, comentário só onde o porquê não é óbvio.

### Testes

- **Um teste por cenário.** Cada cenário é uma função `Test<Unidade>_<Cenário>` própria. Sem teste por tabela, sem `for`/`range` e sem `t.Run` dentro de teste.
- **Uma asserção por teste.** Cada teste termina com exatamente uma chamada ao `testkit`: `testkit.Equal(t, want, got)`, que compara o valor inteiro com `go-cmp` e imprime o diff, ou `testkit.ErrorIs(t, err, target)`. Para conferir vários campos, compara-se a struct esperada inteira, nunca campo a campo.
- **Arrange, act, assert** separados por uma linha em branco. O setup usa builders e funções `must...` do `testkit`, que chamam `t.Helper()` e `t.Fatal` em erro inesperado de preparo; isso não conta como asserção.
- **Fakes escritos à mão**, um por port, em `internal/testkit/fake/`. Sem framework de mock.
- **Imposto no lint.** Um analisador próprio em `internal/tools/testshape` (`golang.org/x/tools/go/analysis`) roda no `make lint` como binário à parte (`singlechecker`, `go run ./internal/tools/testshape/cmd/testshape ./...`), fora do `golangci-lint`, e falha se um `Test*` tem `for`, `range`, `t.Run` ou um número de asserções diferente de um. Contam como asserção `testkit.Equal`, `testkit.ErrorIs` e `analysistest.Run` (a asserção natural dos testes do próprio analisador). `internal/tools/testshape/cmd/` fica fora da cobertura e da mutação, como `cmd/`.
- `go-cmp` é a única dependência só de teste.
- `gremlins` e `govulncheck` ficam fixados no `go.mod` como `tool` (`go get -tool`) e rodam com `go tool <nome>`, o papel das `devDependencies`; não entram no binário. O `golangci-lint` fica fora do `go.mod`, como recomendam os mantenedores dele: `golangci/golangci-lint-action` no CI e `brew` na máquina local, com a versão fixada no workflow.

## Instalação e atualização

Pré-requisitos: VPS com Easypanel na versão suportada e **atualização automática do Easypanel desligada**, senão o painel se atualiza sozinho e o reconcile passa a `skipped` para sempre; um token de API do Easypanel; no CMS, o `infra-host` cadastrado mais o API token do agente; e, na AWS, o usuário `vps-agent-<host>` com sua access key e o role `vps-agent-secrets-reader` em cada conta dos clientes do host (ver _Dependências no cloud-iac_).

Uma única sessão SSH:

```bash
curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "Accept: application/octet-stream" \
  "<url do asset install.sh da release>" -o install.sh
sudo GITHUB_TOKEN=... CMS_URL=https://cms.heronlabs.com.br AGENT_TOKEN=... \
  AGENT_HOST_NAME=vps-heron-02 EASYPANEL_TOKEN=... \
  AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... bash install.sh
```

`install.sh`:

1. Detecta a arquitetura (`amd64`/`arm64`) e baixa da release (a `VERSION` pedida ou a mais recente) o binário e o `checksums.txt`. Como o repositório é privado, `browser_download_url` não aceita token. O script resolve o id de cada asset por `GET /repos/heronlabs/vps-agent/releases/latest` (ou `releases/tags/<tag>`) e baixa `GET /repos/heronlabs/vps-agent/releases/assets/<id>` com `Accept: application/octet-stream`. O `install.sh` da primeira vez sai pelo mesmo caminho. Usa `GITHUB_TOKEN`: um token fine-grained só com `Contents: read` neste repositório. Vazado, expõe só o código do agente.
2. Confere o sha256. Se não bate, para sem tocar no que está instalado.
3. Instala em `/usr/local/bin/vps-agent` e grava `/etc/vps-agent/config.env` (0600) **só se não existir**. Rodar de novo atualiza o binário e mantém a config.
4. A partir da fase 2 (ver _Fases_), instala `deploy/vps-agent.service` (`Restart=always`, `NoNewPrivileges=yes`, `ProtectSystem=strict`, `ProtectHome=yes`, `ReadWritePaths=/var/run/docker.sock`) e roda `systemctl daemon-reload` e `systemctl enable --now vps-agent`.
5. Roda `vps-agent doctor` e sai com o código dele.

O agente roda como root: ler os volumes e o `docker.sock` exige isso, e o `docker.sock` já equivale a root no host. O agente é tão sensível quanto o próprio Easypanel.

## Testes

- **Unitários** (`make test`): `go test -race ./...`; os testes de integração ficam fora por build tag. Cobertura de 100% das instruções em `internal/` (fora `internal/testkit/`), conferida no CI a partir do `coverprofile`; `cmd/` fica fora. O plano do reconcile tem um teste por cenário (ver _Convenções de código_): projeto faltando, serviço faltando, limites diferentes, imagem diferente (sem ação), serviço extra (sem ação), banco antes do app, `DATABASE_URL` válido virando usuário, senha e nome do banco, `DATABASE_URL` com esquema, host ou porta incompatível, `databaseUrl` em serviço `app`, segredo ausente em serviço existente (`AddSecrets`), segredo presente com valor diferente (sem ação), `secretsRoleArn` e `ssm` fora do formato.
- **Integração** (`make test-integration`): `go test -race -tags integration ./test/integration/...`, com `//go:build integration` em cada arquivo. O agente inteiro contra servidores falsos em `httptest`: um Easypanel servindo as fixtures de `testdata/easypanel/<versão>/`, um Docker num unix socket temporário, um CMS que valida o contrato e um STS/SSM falso (endpoint do SDK apontado para o `httptest`) que confere `ExternalId` e nega parâmetro fora da lista do role. Cobre:
  - Easypanel em outra versão: `skipped` e nenhuma escrita;
  - manifesto em 5xx: `skipped`;
  - falha num projeto não impede os outros;
  - parâmetro inexistente no SSM dá `pending` com a chave em `missingSecrets`;
  - `DATABASE_URL` inexistente não cria o banco e dá `pending`; existente cria o banco com usuário, senha e nome dele;
  - `AssumeRole` negado dá `failed` naquele projeto e os outros seguem;
  - parâmetro fora da lista do role (`AccessDenied` no SSM) dá `failed` naquele projeto;
  - o valor de segredo servido pelo SSM falso nunca aparece em log, report nem saída do `plan`;
  - `404 unknown-infra-host` é logado com o nome do host e o agente continua.
- **Mutação** (`make test-mutation`): `gremlins` sobre `internal/` (fora `internal/testkit/`), com limite de eficácia 100%, como o Stryker dos outros repositórios. Um mutante equivalente que não morre é resolvido refatorando o código.
- **Contrato com o Easypanel real** (`make test-contract`, fora do CI): roda `plan` e depois o reconcile de um manifesto de teste contra `EASYPANEL_URL`/`EASYPANEL_TOKEN` de uma VPS descartável. É obrigatório antes de mudar `SupportedEasypanelVersion`, e o resultado vai no corpo do PR.

## Repositório, hooks e CI/CD

### Criação

- `gh repo create heronlabs/vps-agent --private --add-readme`. O README inicial cria a `main`, e todo o scaffold entra por branch e PR: nunca há push direto na `main`. A pasta local já tem esta spec e não é git: `git init`, `remote add`, `fetch`, branch a partir de `origin/main`.
- Configuração do repositório: só squash merge e apagar branch depois do merge. Sem proteção de branch, como todos os repositórios da org: a action de release faz push de `version.txt` e `CHANGELOG.md` na `main` com o `github.token`, e uma `main` protegida recusaria esse push. "Nunca push direto na `main`" é regra de processo (PR, `/supera:ship`), não configuração do GitHub.
- Sem `LICENSE` (privado). `CODEOWNERS` com `* @heronlabs/devops`. `CONTRIBUTING.md` e `SECURITY.md` no molde de `action-tag-release-build`. `AGENTS.md` com o mapa, os comandos e as convenções deste repositório, e `CLAUDE.md` contendo `@AGENTS.md`.
- `version.txt` nasce com `0.0.0` no scaffold. Depois disso só a action de release escreve nele, assim como no `CHANGELOG.md`.
- `.claude/supera.json`:

```json
{
  "baseBranch": "main",
  "buildCommand": "make build",
  "lintCommand": "make lint",
  "mergeMethod": "squash",
  "remote": "origin",
  "testCommands": {
    "integration": "make test-integration",
    "unit": "make test"
  }
}
```

### Makefile

| Alvo | Faz |
|---|---|
| `make hooks` | `git config core.hooksPath .githooks` (o `prepare` dos repositórios pnpm) |
| `make build` | `CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/vps-agent`; `GOOS`, `GOARCH` e `BIN` vêm do ambiente para a matriz do CD |
| `make lint` | `gofmt -l` vazio, `go vet` e `golangci-lint` também com a tag `integration`, `testshape`, `go mod tidy -diff`, `shellcheck` dos scripts, `actionlint` |
| `make test` | unitários com cobertura e verificação dos 100% |
| `make test-integration` | integração (`-tags integration`) |
| `make test-mutation` | `go tool gremlins` |
| `make audit` | `go tool govulncheck ./...` |
| `make test-contract` | contrato contra Easypanel real (manual) |

A versão do Go fica em `go.mod` (`go` e `toolchain`): a estável corrente no dia do scaffold, atualizada pelo Dependabot.

### Git hook

`.githooks/pre-commit`, no molde do `action-tag-release-build`: só roda quando há `.go`, `go.mod` ou `go.sum` staged; senão sai 0. Quando roda:

```bash
#!/usr/bin/env bash
set -euo pipefail

git diff --cached --quiet -- '*.go' go.mod go.sum && exit 0

staged=$(git diff --cached --name-only --diff-filter=ACM -- '*.go')
if [ -n "$staged" ]; then
  unformatted=$(gofmt -l $staged)
  [ -z "$unformatted" ] || { echo "gofmt: $unformatted" >&2; exit 1; }
fi

make build >/dev/null
go vet ./...
go test ./... >/dev/null
```

O `bin/` fica no `.gitignore`: o binário nunca vai para o git, só para a release.

### `continuous-integration.yml`

Em `pull_request` para `main` e `workflow_dispatch`; `concurrency` com `cancel-in-progress: true`; `permissions: contents: read`; `runs-on: ubuntu-24.04`; actions de terceiros em tag de major flutuante, como no resto da org. Jobs em paralelo depois de `build`:

1. `Build`: `actions/setup-go` com `go-version-file: go.mod` e cache; `make build`.
2. `Lint`: `golangci/golangci-lint-action` com `install-only: true` e a versão fixada, depois `make lint`.
3. `Audit & Supply Chain`: `make audit`.
4. `Unit Tests`: `make test`; sobe `coverage.out` como artefato.
5. `Integration Tests` (depois de `Unit Tests`): `make test-integration`.
6. `Mutation Tests` (depois de `Unit Tests`): `make test-mutation`, resumo no `GITHUB_STEP_SUMMARY` e relatório como artefato.

### `continuous-deployment.yml`

Em `push` para `main` e `workflow_dispatch` com `semantic` (`major`/`minor`/`patch`); `permissions: contents: write`; `concurrency` sem cancelar, com `queue: max`.

1. `release`: `actions/checkout` com `fetch-depth: 0` e `heronlabs/action-tag-release-build@v7` (`ghToken`, `semantic`). A action escreve `version.txt` e `CHANGELOG.md`, cria a tag, move as tags flutuantes e publica a release. Saídas: `tag` e `version`.
2. `binaries` (precisa de `release`): checkout da `tag`, matriz `linux/amd64` e `linux/arm64`, `make build VERSION=<version>`, binários `vps-agent_<version>_linux_<arch>`, mais `checksums.txt` (sha256), `install.sh` e `vps-agent.service`. Tudo vai para a release com `gh release upload <tag> --clobber`.

### `dependabot.yml`

`gomod` e `github-actions`, diário, as actions agrupadas.

## Fases

0. **Captura (antes de qualquer código do reconcile).** No Easypanel de produção, ler a versão e registrá-la como `SupportedEasypanelVersion`. Pelo DevTools, capturar request e resposta de: `projects.listProjectsAndServices`, `projects.createProject`, `services.app.createService`, `services.postgres.createService`, `services.app.updateResources`, `services.postgres.updateResources`, `services.app.inspectService`, `services.postgres.inspectService` e a leitura de versão. Guardar sem segredos em `testdata/easypanel/<versão>/`. A captura responde:
   - a unidade de memória de `resources` (MB ou MiB);
   - se `templates.createFromSchema` cria o projeto sozinho ou exige que ele exista;
   - se `createFromSchema` adiciona serviço a um projeto que já tem outros, e o que faz com um serviço de mesmo nome (erro, ignora ou sobrescreve);
   - para o fallback, o payload de `services.<tipo>.createService`, `services.app.updateSourceImage`, `services.app.updateEnv` e `domains.*`;
   - os campos de credencial do banco no `data` de `postgres` e `mysql` (usuário, senha, nome do banco) e se o Easypanel aceita usuário e nome do banco escolhidos, não só a senha;
   - se `services.app.updateEnv` substitui o env inteiro e se a mudança exige deploy para valer (para `AddSecrets`);
   - o nome do label Swarm e o caminho dos bind mounts;
   - qual procedure devolve a versão.

   As respostas vão para esta spec antes da fase 2.
1. **Scaffold:** repositório, Makefile, hook, CI/CD, `version`, `doctor` (só config e Docker) e `report --once` com host e Docker. Na primeira release, o `install.sh` só instala o binário, sem unit systemd; `report --once` roda à mão na VPS atual para validar a medição.
2. **Report:** envio para `POST /api/infra-reports` e `run` só com o job de report. A unit systemd entra no `install.sh` aqui. Depende do PR do `webapps`.
3. **Reconcile:** `plan`, depois `run` com reconcile e segredos, e o `test-contract` numa VPS descartável. Depende do `cloud-iac` (ver _Dependências no cloud-iac_).
4. **Migração** (spec própria): VPS nova, clientes movidos no CMS, convergência, restore dos dumps, virada do DNS.

## Dependências no webapps (fora deste repositório)

Um PR no `heronlabs/webapps`, com spec própria lá, entregue antes da fase 2. Ele substitui o A2 PR 1:

- campo `secretsRoleArn` em `infra-client`;
- componente repetível `infra.service` em `infra-client`, com os campos do manifesto (`name`, `type`, `image`, `env`, `secrets` como repetível `key`/`ssm`, `domains`, `memoryReservationMiB`, `memoryLimitMiB`, `cpuReservation`, `cpuLimit`, `diskQuotaGB`);
- campo `databaseUrl` (referência `ssm`) em `infra.service`, obrigatório em `postgres`/`mysql` e proibido em `app`;
- validação ao salvar de `secretsRoleArn`, `ssm` e `databaseUrl` no mesmo formato que o agente aceita (role `vps-agent-secrets-reader`, nome de parâmetro SSM válido);
- validação ao salvar do serviço de banco contra o que o `infra-client` já tem: `type` igual a `databaseEngine` e `image` igual a `<databaseEngine>:<databaseVersion>`;
- validação ao salvar, pela regra de deploy start-first do `AGENTS.md` do `webapps` (o Swarm só agenda a task nova se a soma das reservas mais a dela couber no nó): `soma(cpuReservation) + max(cpuReservation) <= cpuVCpu` sobre todos os serviços do host, e a mesma regra para `memoryReservationMiB` contra a RAM entregue com `headroom`;
- rota `GET /api/infra-manifests/:host` e rota `POST /api/infra-reports` com o contrato acima, ambas recusando o que não for API token, no molde do A2;
- `lifecycle` ganha `failed` e um campo de motivo; o CMS deriva o `lifecycle` do `reconcile.state` como descrito;
- no admin: memória, CPU e disco por serviço com a porcentagem do limite, badge de medição velha (mais de 30 min), projeto `pending` com as chaves que faltam, aviso de host com `easypanelSupported: false` e o último `lastReconcile` (quando, resultado, erro);
- o roadmap `specs/2026-09-23-infra-roadmap.md` atualizado: item 2 sem `hcli agent` e item 3 declarativo, apontando para este repositório.

## Dependências no cloud-iac (fora deste repositório)

Entregues antes da fase 3, cada uma por `/supera:ship` no repositório dela:

- **`cloud-iac` (engine):** um `RoleModule` genérico, irmão do `UserModule`, lido de `role.yaml` como o `UserModule` lê `user.yaml`. Hoje a engine só tem a primitiva `Role` e o `ResourceStatement.attachStatementsToResourceRole`; o `crossAccount` do `ParameterModule` não serve (dá `ssm:GetParametersByPath` no path inteiro e não tem `ExternalId`). O módulo:
  - cria o role com **nome físico fixo** (`name` explícito, sem o sufixo aleatório do Pulumi), para o ARN `role/<nome>` ser previsível em toda conta;
  - monta a trust a partir de `trust` (`accountId`, `principalArnLike` opcional, `externalIds` opcional);
  - anexa `statements` com o mesmo formato do `UserModule`.

  ```yaml
  - name: vps-agent-secrets-reader
    trust:
      accountId: '088132334646'
      principalArnLike: arn:aws:iam::088132334646:user/vps-agent-*
      externalIds: [cms-heronlabs]
    statements:
      - name: ssm
        actions: ['ssm:GetParameters']
        resources:
          - 'arn:aws:ssm:us-east-1:088132334646:parameter/server-cms/DATABASE_URL'
          - 'arn:aws:ssm:us-east-1:088132334646:parameter/server-cms-deployer/*'
          - 'arn:aws:ssm:us-east-1:088132334646:parameter/server-cms-backup/*'
  ```

  O deploy do tenant usa a `main` da engine (`engine-ref` vazio), então o `RoleModule` precisa estar mergeado no `cloud-iac` antes do deploy do `role.yaml` no tenant.
- **`cloud-iac-heronlabs`:**
  - no `user.yaml`, `vps-agent-<host>` por VPS (com o `UserModule` que já existe), statement único `sts:AssumeRole` em `arn:aws:iam::*:role/vps-agent-secrets-reader`, e `userCredentials` gravando a access key em `/vps-agent/<host>/` (de onde o operador a copia para o `install.sh`);
  - um `role.yaml` com o `vps-agent-secrets-reader` da conta heronlabs, como no exemplo acima;
  - nenhum parâmetro novo nem movido: `server-cms-deployer`, `server-cms-backup` e `/server-cms/DATABASE_URL` ficam como estão;
  - antes da migração, o host do segredo `SERVER_CMS/DATABASE_URL` tem que ser `<slug>_<serviço de banco>` (ex.: `cms-heronlabs_cms-database`), senão o agente recusa o `DATABASE_URL`.
- **Repositório de tenant de cada outra conta:** o mesmo `role.yaml`, com o `externalIds` e os ARNs daquele cliente. Conta sem repositório de tenant cria o role à mão uma vez, a partir do JSON da policy que o módulo gera, até ganhar o seu.

## Execução

- **`vps-agent`, até a primeira release:** trabalho direto na raiz deste repositório, sem `/supera:ship`, mas em branch e PR (nunca push na `main`). Plano em `docs/superpowers/plans/2026-10-02-vps-agent-phase-1.md`. Depois da primeira release, toda mudança passa por `/supera:ship`.
- **`cloud-iac`, `cloud-iac-heronlabs` e `webapps`:** um `/supera:ship` por repositório, com os briefings de `docs/superpowers/plans/2026-10-02-ship-briefs.md`. Ordem: `cloud-iac` (`RoleModule`) é mergeado antes do deploy do `cloud-iac-heronlabs`; `webapps` é independente e precisa estar em produção antes da fase 2; os dois de IaC, antes da fase 3.
- **Fase 0** é captura manual no Easypanel de produção, antes do plano da fase 3.
- As fases 2 e 3 ganham plano próprio quando as dependências delas estiverem entregues.

## Fora de escopo (v1)

- Apagar projeto ou serviço, parar cliente `suspended` (o reconcile só ignora), trocar imagem ou env de serviço existente.
- Canal de comandos (backup agora, restore), histórico de medições, alertas.
- `pg_database_size` exato e conexões do banco.
- Atualização automática do agente: atualizar é rodar o `install.sh` de novo.
- Rotação automática de segredo (comparar o valor do SSM com o do Easypanel) e credencial do agente sem chave estática (IAM Roles Anywhere).
- Quem pode editar `infra.service`: hoje qualquer `authenticated` edita `infra-client`, e com um operador só isso fica assim. A spec do `webapps` registra o ponto.
