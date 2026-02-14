# OMS Helm Chart

Production-ready Helm chart для Order Management System.

## Быстрый старт

### Установка

```bash
# Добавить chart repository (если опубликован)
helm repo add oms https://charts.example.com
helm repo update

# Или установить из локальной директории
helm install oms ./deploy/helm/oms -n oms --create-namespace

# С кастомными values
helm install oms ./deploy/helm/oms -n oms --create-namespace -f custom-values.yaml
```

### Обновление

```bash
# Обновить release
helm upgrade oms ./deploy/helm/oms -n oms

# С новыми values
helm upgrade oms ./deploy/helm/oms -n oms -f custom-values.yaml

# Dry-run для проверки
helm upgrade oms ./deploy/helm/oms -n oms --dry-run --debug
```

### Удаление

```bash
helm uninstall oms -n oms
```

## Конфигурация

### Основные параметры

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `replicaCount` | Количество реплик | `3` |
| `image.repository` | Docker image repository | `order-service` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Pull policy | `IfNotPresent` |

### Service

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `service.type` | Service type | `ClusterIP` |
| `service.grpcPort` | gRPC port | `50051` |
| `service.metricsPort` | Metrics port | `9090` |

### Resources

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |

### Autoscaling

| Параметр | Описание | Значение по умолчанию |
|----------|----------|----------------------|
| `autoscaling.enabled` | Enable HPA | `true` |
| `autoscaling.minReplicas` | Minimum replicas | `3` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU | `70` |
| `autoscaling.targetMemoryUtilizationPercentage` | Target Memory | `80` |

## Примеры использования

### Development

```yaml
# values-dev.yaml
replicaCount: 1
image:
  tag: "dev"
  pullPolicy: Always

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi

autoscaling:
  enabled: false

config:
  logLevel: "debug"
```

```bash
helm install oms ./deploy/helm/oms -n oms-dev --create-namespace -f values-dev.yaml
```

### Staging

```yaml
# values-staging.yaml
replicaCount: 2
image:
  tag: "staging"

config:
  logLevel: "info"
  kafka:
    brokers: "kafka-staging.kafka.svc.cluster.local:9092"

autoscaling:
  minReplicas: 2
  maxReplicas: 5
```

```bash
helm install oms ./deploy/helm/oms -n oms-staging --create-namespace -f values-staging.yaml
```

### Production

```yaml
# values-prod.yaml
replicaCount: 5
image:
  repository: registry.example.com/oms/order-service
  tag: "v1.0.0"
  pullPolicy: IfNotPresent

resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi

autoscaling:
  enabled: true
  minReplicas: 5
  maxReplicas: 20

config:
  logLevel: "warn"
  kafka:
    brokers: "kafka-prod.kafka.svc.cluster.local:9092"

podDisruptionBudget:
  enabled: true
  minAvailable: 3

externalService:
  enabled: true
  type: LoadBalancer
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

```bash
helm install oms ./deploy/helm/oms -n oms-prod --create-namespace -f values-prod.yaml
```

## Расширенная конфигурация

### Custom Environment Variables

```yaml
extraEnv:
- name: CUSTOM_VAR
  value: "custom-value"
- name: SECRET_VAR
  valueFrom:
    secretKeyRef:
      name: my-secret
      key: password
```

### Extra Volumes

```yaml
extraVolumes:
- name: config-volume
  configMap:
    name: my-config

extraVolumeMounts:
- name: config-volume
  mountPath: /config
  readOnly: true
```

### Node Selector

```yaml
nodeSelector:
  disktype: ssd
  zone: us-east-1a
```

### Tolerations

```yaml
tolerations:
- key: "dedicated"
  operator: "Equal"
  value: "oms"
  effect: "NoSchedule"
```

## Мониторинг

### Prometheus ServiceMonitor

```yaml
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
    scrapeTimeout: 10s
```

### Prometheus Rules

```yaml
monitoring:
  prometheusRule:
    enabled: true
```

## Безопасность

### Network Policy

По умолчанию включена. Для отключения:

```yaml
networkPolicy:
  enabled: false
```

### Security Context

Настроен для запуска от non-root пользователя с read-only filesystem.

## Тестирование

```bash
# Lint chart
helm lint ./deploy/helm/oms

# Template rendering
helm template oms ./deploy/helm/oms -n oms

# Dry-run install
helm install oms ./deploy/helm/oms -n oms --dry-run --debug

# Test release
helm test oms -n oms
```

## Дополнительные ресурсы

- [Helm Documentation](https://helm.sh/docs/)
- [Values Schema](./values.schema.json)
- [OMS Documentation](https://github.com/vladislavdragonenkov/oms)

## Contributing

1. Fork the repository
2. Create your feature branch
3. Update chart version in `Chart.yaml`
4. Test your changes
5. Submit a pull request

---

**Version:** 1.0.0  
**App Version:** 2.0.0  
**Maintainer:** Vladislav Dragonenkov
