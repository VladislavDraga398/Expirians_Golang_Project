# 🚢 Kubernetes Deployment

Production-ready Kubernetes манифесты для OMS.

## 📋 Содержимое

| Файл | Описание |
|------|----------|
| `namespace.yaml` | Namespace для изоляции |
| `configmap.yaml` | Конфигурация приложения |
| `rbac.yaml` | ServiceAccount, Role, RoleBinding |
| `deployment.yaml` | Deployment с 3 репликами |
| `service.yaml` | ClusterIP, Headless, LoadBalancer |
| `hpa.yaml` | HorizontalPodAutoscaler (3-10 pods) |
| `pdb.yaml` | PodDisruptionBudget (min 2 available) |
| `networkpolicy.yaml` | Network isolation |
| `kustomization.yaml` | Kustomize configuration |

## 🚀 Быстрый старт

### Предварительные требования

- Kubernetes cluster (v1.24+)
- kubectl configured
- Metrics Server (для HPA)
- Kafka cluster (опционально)

### Установка

#### Вариант 1: kubectl

```bash
# Применить все манифесты
kubectl apply -f deploy/k8s/

# Проверить статус
kubectl get pods -n oms
kubectl get svc -n oms
```

#### Вариант 2: Kustomize

```bash
# Применить через kustomize
kubectl apply -k deploy/k8s/

# Или с кастомизацией
kustomize build deploy/k8s/ | kubectl apply -f -
```

### Проверка

```bash
# Проверить pods
kubectl get pods -n oms -w

# Проверить logs
kubectl logs -n oms -l app=oms --tail=100 -f

# Проверить health
kubectl exec -n oms deployment/oms -- wget -qO- http://localhost:9090/healthz
```

## ⚙️ Конфигурация

### ConfigMap

Отредактируйте `configmap.yaml` для изменения конфигурации:

```yaml
data:
  OMS_GRPC_ADDR: ":50051"
  KAFKA_BROKERS: "kafka:9092"
  LOG_LEVEL: "info"
```

После изменений:
```bash
kubectl apply -f deploy/k8s/configmap.yaml
kubectl rollout restart deployment/oms -n oms
```

### Secrets (если нужны)

Создайте secrets для чувствительных данных:

```bash
kubectl create secret generic oms-secrets \
  --from-literal=db-password=secret \
  --from-literal=api-key=key \
  -n oms
```

Добавьте в deployment:
```yaml
envFrom:
- secretRef:
    name: oms-secrets
```

### Image

Обновите image в `kustomization.yaml`:

```yaml
images:
- name: order-service
  newName: your-registry/order-service
  newTag: v1.0.0
```

## 📊 Мониторинг

### Metrics

Prometheus автоматически обнаружит pods через annotations:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9090"
  prometheus.io/path: "/metrics"
```

### Logs

```bash
# Все pods
kubectl logs -n oms -l app=oms

# Конкретный pod
kubectl logs -n oms <pod-name>

# Follow logs
kubectl logs -n oms -l app=oms -f --tail=100
```

### Events

```bash
kubectl get events -n oms --sort-by='.lastTimestamp'
```

## 🔄 Обновление

### Rolling Update

```bash
# Обновить image
kubectl set image deployment/oms oms=order-service:v1.1.0 -n oms

# Проверить статус
kubectl rollout status deployment/oms -n oms

# История
kubectl rollout history deployment/oms -n oms
```

### Rollback

```bash
# Откатить к предыдущей версии
kubectl rollout undo deployment/oms -n oms

# Откатить к конкретной ревизии
kubectl rollout undo deployment/oms --to-revision=2 -n oms
```

## 📈 Автоскейлинг

### HPA Status

```bash
kubectl get hpa -n oms
kubectl describe hpa oms -n oms
```

### Ручное масштабирование

```bash
# Увеличить до 5 реплик
kubectl scale deployment/oms --replicas=5 -n oms

# Проверить
kubectl get pods -n oms
```

## 🔒 Безопасность

### Network Policy

Проверить network policy:

```bash
kubectl get networkpolicy -n oms
kubectl describe networkpolicy oms -n oms
```

### Security Context

Deployment использует:
- `runAsNonRoot: true`
- `readOnlyRootFilesystem: true`
- `allowPrivilegeEscalation: false`
- Dropped all capabilities

### RBAC

Минимальные права через ServiceAccount:
- Read ConfigMaps
- Read Secrets

## 🐛 Отладка

### Pod не запускается

```bash
# Проверить events
kubectl describe pod <pod-name> -n oms

# Проверить logs
kubectl logs <pod-name> -n oms

# Проверить предыдущий container (если crashed)
kubectl logs <pod-name> -n oms --previous
```

### Health checks failing

```bash
# Exec в pod
kubectl exec -it <pod-name> -n oms -- /bin/sh

# Проверить health endpoint
wget -qO- http://localhost:9090/healthz
```

### Network issues

```bash
# Проверить DNS
kubectl exec -it <pod-name> -n oms -- nslookup kubernetes.default

# Проверить connectivity
kubectl exec -it <pod-name> -n oms -- wget -qO- http://oms:50051
```

## 🧹 Удаление

```bash
# Удалить все ресурсы
kubectl delete -f deploy/k8s/

# Или через kustomize
kubectl delete -k deploy/k8s/

# Удалить namespace (удалит всё внутри)
kubectl delete namespace oms
```

## 📝 Best Practices

### 1. Resource Limits

Всегда указывайте requests и limits:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

### 2. Health Probes

Используйте все 3 типа:
- `livenessProbe` - перезапуск при зависании
- `readinessProbe` - исключение из балансировки
- `startupProbe` - для медленного старта

### 3. PodDisruptionBudget

Защита от одновременного удаления всех pods:
```yaml
spec:
  minAvailable: 2
```

### 4. Anti-Affinity

Распределение pods по разным нодам:
```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        topologyKey: kubernetes.io/hostname
```

### 5. Graceful Shutdown

```yaml
terminationGracePeriodSeconds: 30
```

## 🔗 Дополнительные ресурсы

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Kustomize](https://kustomize.io/)
- [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/)

---

**✅ Kubernetes manifests готовы к использованию!**
