# QMesh Sidecar - Kubernetes Integration

## Обзор

QMesh Sidecar - это service mesh решение, работающее как sidecar-контейнер в Kubernetes подах. Оно обеспечивает:
- Обнаружение сервисов через gossip-протокол (UDP 4221)
- Туннелирование HTTP-запросов через QUIC (TCP 4224)

## Архитектура

```
Pod:
  ├── init-container (qmesh-seed-discovery)
  │   └── Обнаруживает адреса других узлов через Kubernetes API
  │       и записывает их в shared volume
  │
  ├── container (app)
  │   └── Основное приложение, использующее sidecar
  │
  └── container (qmesh-sidecar)
      └── Sidecar-прокси, читает seeds из /shared/gossip-seeds
```

## Быстрый старт

### 1. Сборка образа

```bash
make docker-build
# или
docker build -t qmesh-sidecar:latest .
```

### 2. Деплой

```bash
# Создание headless service и RBAC
kubectl apply -f deploy/k8s/qmesh-headless-service.yaml
kubectl apply -f deploy/k8s/sidecar-rbac.yaml

# Деплой приложения с sidecar
kubectl apply -f deploy/k8s/example-deployment.yaml
```

### 3. Проверка

```bash
kubectl get pods -l app=app-with-qmesh
kubectl logs <pod-name> -c qmesh-sidecar
```

## Использование в своем приложении

Добавьте в спецификацию Pod'а:

```yaml
spec:
  serviceAccountName: qmesh-sidecar
  
  initContainers:
    - name: qmesh-seed-discovery
      image: bitnami/kubectl:latest
      command:
        - /bin/bash
        - -c
        - |
          # Скрипт обнаружения узлов (см. example-deployment.yaml)
      env:
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
      volumeMounts:
        - name: shared-data
          mountPath: /shared
  
  containers:
    - name: qmesh-sidecar
      image: qmesh-sidecar:latest
      ports:
        - name: gossip
          containerPort: 4221
          protocol: UDP
        - name: quic
          containerPort: 4224
      volumeMounts:
        - name: shared-data
          mountPath: /shared
          readOnly: true
  
  volumes:
    - name: shared-data
      emptyDir: {}
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
|-----------|----------|--------------|
| GOSSIP_SEEDS | Адреса seed-узлов (ip:port) | "" |
| POD_IP | IP-адрес пода | - |

Sidecar также читает seeds из файла `/shared/gossip-seeds`, если переменная окружения не задана.

## Порты

| Порт | Протокол | Описание |
|------|----------|----------|
| 4221 | UDP | Gossip-протокол для обнаружения узлов |
| 4224 | TCP/QUIC | Туннелирование HTTP-запросов |

## Очистка

```bash
kubectl delete -f deploy/k8s/
```
