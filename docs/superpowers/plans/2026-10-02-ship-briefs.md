# Briefings dos `/supera:ship` fora do vps-agent

Data: 2026-10-02. Fonte: `docs/superpowers/specs/2026-10-01-vps-agent-design.md` (este repositório). Um `/supera:ship` por repositório, cada um com o briefing abaixo como descrição da tarefa. Os trechos da spec estão copiados literalmente porque o repositório `heronlabs/vps-agent` é privado e ainda não existe no GitHub quando os ships começam.

## Ordem

1. `cloud-iac` (`RoleModule`) e `webapps` podem rodar em paralelo.
2. `cloud-iac-heronlabs` abre o PR em paralelo, mas o deploy dele só depois do merge do `cloud-iac`: o deploy do tenant usa a `main` da engine (`engine-ref` vazio).
3. `webapps` precisa estar em produção antes da fase 2 do vps-agent; os dois de IaC, antes da fase 3. Nenhum bloqueia a fase 1.

---

## 1. `iac/cloud-iac` — `RoleModule` genérico

```
/supera:ship Adicionar um RoleModule genérico à engine, irmão do UserModule, lido de `role.yaml` como o UserModule lê `user.yaml`.

Contexto: o vps-agent (agente Go que roda em cada VPS Easypanel) precisa assumir, em cada conta AWS de cliente, um role com nome fixo `vps-agent-secrets-reader` que lê uma lista explícita de parâmetros SSM. Hoje a engine só tem a primitiva `Role` (`src/infrastructure/aws/iam/role.ts`) e `ResourceStatement.attachStatementsToResourceRole`; o `crossAccount` do `ParameterModule` não serve (dá `ssm:GetParametersByPath` no path inteiro e não tem ExternalId) e NÃO deve ser alterado.

O que fazer, no molde de `src/application/user/` (`index.ts`, `factory.ts`, `variables.ts`) e com o mesmo wiring do `UserFactory`:
- `Stacks.ROLE = 'role'` e `src/application/role/{index,factory,variables}.ts`.
- Variáveis: `name: string`; `trust: { accountId: string; principalArnLike?: string; externalIds?: string[] }`; `statements: StatementVariables[]`; mais `RootVariables`.
- O role nasce com **nome físico fixo** igual a `name` (argumento `name` explícito no `Role.make`, sem o sufixo aleatório do Pulumi), para o ARN `arn:aws:iam::<conta>:role/<name>` ser previsível. `maxSessionDuration: 3600`.
- Trust policy: `Principal: {"AWS": "arn:aws:iam::<accountId>:root"}`, `Action: sts:AssumeRole`; `Condition.ArnLike["aws:PrincipalArn"] = principalArnLike` quando informado; `Condition.StringEquals["sts:ExternalId"] = externalIds` quando informado (lista).
- `statements` anexados com `attachStatementsToResourceRole`, igual ao UserModule do lado do usuário.
- Testes no padrão do repositório (cobertura e mutação como hoje), README/AGENTS com o exemplo abaixo.

Exemplo de `role.yaml` que o tenant vai usar:

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

Fora de escopo: qualquer mudança em ParameterModule, UserModule ou nos tenants.
```

---

## 2. `iac/cloud-iac-heronlabs` — usuário do agente e role da conta heronlabs

Host da VPS atual (`name` do `infra-host` no CMS): `srv655164`, confirmado pelo usuário em 2026-10-02.

```
/supera:ship Criar na conta heronlabs o usuário IAM do vps-agent e o role `vps-agent-secrets-reader`, sem mexer em nenhum parâmetro existente.

Depende do RoleModule do cloud-iac (PR "RoleModule genérico"); o deploy só depois do merge dele.

1. Pasta nova `environments/production/vps-agent/`.
2. `user.yaml` (UserModule que já existe):

  - name: vps-agent-srv655164
    statements:
      - name: sts
        actions: ['sts:AssumeRole']
        resources: ['arn:aws:iam::*:role/vps-agent-secrets-reader']
    userCredentials:
      - path: /vps-agent/srv655164
        name: AWS_ACCESS_KEY_ID
        type: KEY
      - path: /vps-agent/srv655164
        name: AWS_SECRET_ACCESS_KEY
        type: SECRET

3. `role.yaml` (RoleModule novo), exatamente:

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

4. README/AGENTS: registrar a pasta e que `externalIds` tem que ser igual ao slug do cliente no CMS (renomear o slug sem atualizar aqui faz o AssumeRole ser negado).

Fora de escopo: `server-cms-deployer`, `server-cms-backup`, `/server-cms/DATABASE_URL` e qualquer outro parâmetro ficam como estão.
```

---

## 3. `webapps` — manifesto, report e admin (substitui o A2 PR 1)

```
/supera:ship Implementar no CMS e no admin o lado do vps-agent: manifesto por host, report do agente e a visão de uso por serviço. Substitui o A2 PR 1 (`docs/superpowers/specs/2026-09-23-infra-host-report-design.md`); escrever antes a spec própria em `docs/superpowers/specs/` deste repositório a partir do briefing abaixo e atualizar `specs/2026-09-23-infra-roadmap.md` (item 2 sem `hcli agent`, item 3 declarativo, apontando para `heronlabs/vps-agent`).

O vps-agent é um binário Go por VPS Easypanel, só com conexões de saída. Ele lê o manifesto do host e envia o report a cada 10 min, autenticado por um API token custom do Strapi com só essas duas ações.
```

Trechos da spec do vps-agent que entram no briefing (copiar junto com o comando acima):

### Contrato com o CMS

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

### Dependências no webapps

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

### Segredos (contexto do `secretsRoleArn`, `secrets` e `databaseUrl`)

- O manifesto não carrega segredo, só referências.
- Senha do banco: o agente não gera senha. O serviço de banco nasce com usuário, senha e nome do banco do `DATABASE_URL` que já está no SSM, o mesmo que a app lê no boot. Banco recriado numa VPS nova e restaurado de dump aceita a app sem mudança nenhuma nela. A senha só vale na criação: as imagens do Postgres e do MySQL só a aplicam ao iniciar um volume vazio, então trocar o parâmetro depois não muda a senha de um banco existente. Rotação de senha de banco é manual (`ALTER USER` mais o parâmetro) e fica fora da v1.
- Os parâmetros ficam como estão hoje no SSM Parameter Store (`SecureString`) da conta AWS de cada cliente; nada é copiado nem movido para o agente. O agente lê três tipos:
  - o id e o secret do usuário IAM que a API usa para carregar o próprio env (ex.: `/server-cms-deployer/*`, que só lê `parameter/server-cms/*`);
  - o id e o secret do usuário IAM do backup do `terminal-cli` (ex.: `/server-cms-backup/*`);
  - o `DATABASE_URL` da app, só para criar o banco (ex.: `/server-cms/DATABASE_URL`).
  Esses usuários e parâmetros já são criados pelo `UserModule` e pelo `ParameterModule` do `cloud-iac`. O resto do env sensível da app continua onde já está: a app carrega do SSM no boot com `env-ssm` e `AWS_ENV_PATH`.
- Identidade do agente: um usuário IAM `vps-agent-srv655164` por VPS, na conta heronlabs (`088132334646`). A policy dele tem uma permissão só: `sts:AssumeRole` em `arn:aws:iam::*:role/vps-agent-secrets-reader`. Ele não lê SSM direto, nem na própria heronlabs.
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
  - access key do `vps-agent-srv655164` vazada: os parâmetros listados nos roles das contas que confiam no padrão `vps-agent-*`. Equivale a perder o host, cujo Easypanel já guarda esses valores em texto puro. Revogar é desativar a key na heronlabs, sem tocar nas contas dos clientes;
  - edição maliciosa no CMS: pode criar, num cliente, um serviço com imagem própria e apontar para segredo do mesmo cliente. É o mesmo risco que a imagem vinda do manifesto já tem (ver _Fora de escopo_).
- Rotação: o agente nunca sobrescreve valor presente. Para rotacionar na v1, o operador atualiza o parâmetro no SSM e apaga a chave no painel; o reconcile seguinte grava o valor novo.
- Configuração local em `/etc/vps-agent/config.env`, `root:root 0600`: `CMS_URL`, `AGENT_TOKEN`, `AGENT_HOST_NAME`, `EASYPANEL_URL` (padrão `http://127.0.0.1:3000`), `EASYPANEL_TOKEN`, `AWS_ACCESS_KEY_ID` e `AWS_SECRET_ACCESS_KEY` (do `vps-agent-srv655164`), `AWS_REGION` (padrão `us-east-1`), `RECONCILE_INTERVAL` (padrão `5m`), `REPORT_INTERVAL` (padrão `10m`). O agente lê as chaves AWS só desse arquivo, nunca de `~/.aws` nem de metadata de instância. Formato `KEY=VALUE`, uma por linha; linhas vazias e `#` ignoradas; espaços em volta da chave e do valor, `\r` de fim de linha e um par de aspas duplas em volta do valor são descartados; chave desconhecida ou repetida recusa o arquivo. Cada subcomando valida só o que usa: `report --once` não exige nada do CMS, do Easypanel nem da AWS; `plan` exige CMS e Easypanel; `run` exige tudo.
