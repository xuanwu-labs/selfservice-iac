# deploy/

Aether 平台的部署清单与基础设施配置。

> **Status**: 规划中,尚未填充。

## 计划内容

```
deploy/
├── docker/
│   ├── Dockerfile.server       # 后端镜像
│   └── Dockerfile.web          # 前端镜像
├── compose/
│   └── docker-compose.yml      # 本地开发/自托管一键启动
├── k8s/                        # Kubernetes 清单(可选)
└── helm/                       # Helm chart(可选)
```

## 说明

- 本目录放**部署到运行环境**的清单(Dockerfile、k8s、compose)
- 生成的 Terraform/Terramate IaC 工作仓库**不在此处**——那是 Aether 平台**产出**给用户管理的基础设施,不是平台自身的部署
- 区分:`deploy/` = 部署 Aether 平台本身;Aether 管理的 IaC = 平台运行时生成的独立仓库

待部署方案确定后补充。
